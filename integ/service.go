//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-pagerduty-adapter/service"
)

func main() {
	// Test statistics
	var totalTests, passedTests, failedTests int
	startTime := time.Now()

	testResult := func(name string, err error) {
		totalTests++
		if err != nil {
			failedTests++
			log.Printf("❌ %s: %v", name, err)
		} else {
			passedTests++
			fmt.Printf("✅ %s passed\n", name)
		}
	}

	// Configuration from environment variables
	apiToken := os.Getenv("PAGERDUTY_API_TOKEN")
	apiURL := os.Getenv("PAGERDUTY_API_URL")
	if apiURL == "" {
		apiURL = "https://api.pagerduty.com"
	}

	if apiToken == "" {
		log.Fatal("PAGERDUTY_API_TOKEN environment variable is required")
	}

	fmt.Println("=================================")
	fmt.Println("PagerDuty Service Adapter Test")
	fmt.Println("=================================")
	fmt.Printf("API URL: %s\n", apiURL)
	fmt.Printf("Started: %s\n\n", startTime.Format("2006-01-02 15:04:05"))

	// Create the service provider
	serviceConfig := map[string]any{
		"apiToken": apiToken,
		"apiURL":   apiURL,
	}

	provider, err := service.New(serviceConfig)
	if err != nil {
		log.Fatalf("Failed to create PagerDuty service provider: %v", err)
	}

	ctx := context.Background()

	// Test 1: Query all services
	fmt.Println("\n=== Test 1: Query All Services ===")
	services, err := provider.Query(ctx, schema.ServiceQuery{})
	if err != nil {
		testResult("Query all services", err)
	} else {
		fmt.Printf("Found %d services\n", len(services))
		for i, svc := range services {
			if i >= 10 {
				fmt.Printf("... and %d more\n", len(services)-10)
				break
			}
			fmt.Printf("  [%d] ID: %s, Name: %s\n", i+1, svc.ID, svc.Name)
			if svc.Metadata["status"] != nil {
				fmt.Printf("      Status: %v\n", svc.Metadata["status"])
			}
			if svc.Metadata["description"] != nil && svc.Metadata["description"] != "" {
				desc := svc.Metadata["description"].(string)
				if len(desc) > 60 {
					desc = desc[:60] + "..."
				}
				fmt.Printf("      Description: %s\n", desc)
			}

			// Validate service fields
			if svc.ID == "" || svc.Name == "" {
				testResult("Validate service fields", fmt.Errorf("missing required fields in service %d", i))
				break
			}
		}
		if len(services) > 0 {
			testResult("Query all services", nil)
		} else {
			fmt.Println("⚠️  No services found")
			totalTests++
		}

		// Test 2: Query with limit
		if len(services) > 5 {
			fmt.Println("\n=== Test 2: Query with Limit ===")
			limitedServices, err := provider.Query(ctx, schema.ServiceQuery{
				Limit: 5,
			})
			if err != nil {
				testResult("Query with limit", err)
			} else {
				fmt.Printf("Requested limit 5, got %d services\n", len(limitedServices))
				if len(limitedServices) <= 5 {
					testResult("Query with limit", nil)
				} else {
					testResult("Query with limit", fmt.Errorf("expected max 5 services, got %d", len(limitedServices)))
				}
			}
		}

		// Test 3: Filter services by name
		if len(services) > 0 {
			fmt.Println("\n=== Test 3: Filter Services by Name ===")
			firstServiceName := services[0].Name
			searchTerm := firstServiceName
			// Use first word or first 5 chars
			if idx := strings.Index(firstServiceName, " "); idx > 0 {
				searchTerm = firstServiceName[:idx]
			} else if len(firstServiceName) > 5 {
				searchTerm = firstServiceName[:5]
			}

			filteredServices, err := provider.Query(ctx, schema.ServiceQuery{
				Name: searchTerm,
			})
			if err != nil {
				testResult("Filter services by name", err)
			} else {
				fmt.Printf("Found %d services matching '%s'\n", len(filteredServices), searchTerm)
				for i, svc := range filteredServices {
					if i >= 5 {
						fmt.Printf("... and %d more\n", len(filteredServices)-5)
						break
					}
					fmt.Printf("  [%d] %s\n", i+1, svc.Name)
				}
				// All filtered services should contain the search term (case-insensitive)
				allMatch := true
				for _, svc := range filteredServices {
					if !strings.Contains(strings.ToLower(svc.Name), strings.ToLower(searchTerm)) {
						allMatch = false
						testResult("Validate service name filter", fmt.Errorf("service %s does not match filter %s", svc.Name, searchTerm))
						break
					}
				}
				if allMatch {
					testResult("Filter services by name", nil)
				}
			}
		}

		// Test 4: Validate service metadata
		if len(services) > 0 {
			fmt.Println("\n=== Test 4: Validate Service Metadata ===")
			firstService := services[0]
			fmt.Printf("Inspecting service: %s\n", firstService.Name)
			fmt.Printf("  Metadata keys: %v\n", getKeys(firstService.Metadata))

			hasRequiredMetadata := firstService.Metadata != nil &&
				firstService.Metadata["source"] != nil &&
				firstService.Metadata["status"] != nil

			if hasRequiredMetadata {
				fmt.Printf("  Source: %v\n", firstService.Metadata["source"])
				fmt.Printf("  Status: %v\n", firstService.Metadata["status"])
				testResult("Validate service metadata", nil)
			} else {
				testResult("Validate service metadata", fmt.Errorf("missing required metadata fields"))
			}
		}

		// Test 5: Query services by Scope.Team (if teams metadata is available)
		if len(services) > 0 {
			fmt.Println("\n=== Test 5: Query by Scope.Team ===")
			// Find a service that has teams in metadata
			var teamName string
			for _, svc := range services {
				if teams, ok := svc.Metadata["teams"].([]map[string]any); ok && len(teams) > 0 {
					if name, ok := teams[0]["summary"].(string); ok && name != "" {
						teamName = name
						break
					}
				}
			}

			if teamName != "" {
				fmt.Printf("Querying services for team: %s\n", teamName)
				teamServices, err := provider.Query(ctx, schema.ServiceQuery{
					Scope: schema.QueryScope{Team: teamName},
				})
				if err != nil {
					testResult("Query by Scope.Team", err)
				} else {
					fmt.Printf("Found %d services for team '%s'\n", len(teamServices), teamName)
					testResult("Query by Scope.Team", nil)
				}
			} else {
				fmt.Println("⏭️  Skipping Scope.Team test (no teams found in service metadata)")
			}
		}
	}

	// Print summary
	duration := time.Since(startTime)
	fmt.Println("\n=================================")
	fmt.Println("Test Summary")
	fmt.Println("=================================")
	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Passed: %d ✅\n", passedTests)
	fmt.Printf("Failed: %d ❌\n", failedTests)
	fmt.Printf("Duration: %v\n", duration.Round(time.Millisecond))
	if totalTests > 0 {
		fmt.Printf("Success Rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)
	}

	if failedTests == 0 {
		fmt.Println("\n✅ All tests passed successfully!")
	} else {
		fmt.Printf("\n⚠️  %d test(s) failed. Please review the output above.\n", failedTests)
		os.Exit(1)
	}
}

func getKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
