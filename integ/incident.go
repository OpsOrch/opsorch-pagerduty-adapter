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
	"github.com/opsorch/opsorch-pagerduty-adapter/incident"
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
	serviceID := os.Getenv("PAGERDUTY_SERVICE_ID")
	fromEmail := os.Getenv("PAGERDUTY_FROM_EMAIL")
	apiURL := os.Getenv("PAGERDUTY_API_URL")
	if apiURL == "" {
		apiURL = "https://api.pagerduty.com" // Default to production API
	}

	if apiToken == "" {
		log.Fatal("PAGERDUTY_API_TOKEN environment variable is required")
	}

	// Determine which tests to run based on available credentials
	runIncidentTests := serviceID != "" && fromEmail != ""
	runServiceTests := true // Can always run service tests with just apiToken

	fmt.Println("=================================")
	fmt.Println("PagerDuty Adapter Integration Test")
	fmt.Println("=================================")
	fmt.Printf("API URL: %s\n", apiURL)
	if runIncidentTests {
		fmt.Printf("Service ID: %s\n", serviceID)
		fmt.Printf("Testing: Incident + Service adapters\n")
	} else {
		fmt.Printf("Testing: Service adapter only (set PAGERDUTY_SERVICE_ID and PAGERDUTY_FROM_EMAIL for incident tests)\n")
	}
	fmt.Printf("Started: %s\n\n", startTime.Format("2006-01-02 15:04:05"))

	ctx := context.Background()
	var services []schema.Service // Declare here so it's available to both incident and service tests

	// Run incident adapter tests only if credentials are provided
	if runIncidentTests {
		// Create the incident provider
		config := map[string]any{
			"apiToken":  apiToken,
			"apiURL":    apiURL,
			"serviceID": serviceID,
			"fromEmail": fromEmail,
		}

		provider, err := incident.New(config)
		if err != nil {
			log.Fatalf("Failed to create PagerDuty incident provider: %v", err)
		}

		// Test 1: Query all incidents
		fmt.Println("\n=== Test 1: Query All Incidents ===")
		incidents, err := provider.Query(ctx, schema.IncidentQuery{})
		if err != nil {
			testResult("Query all incidents", err)
		} else {
			fmt.Printf("Found %d incidents\n", len(incidents))
			for i, inc := range incidents {
				if i >= 5 {
					fmt.Printf("... and %d more\n", len(incidents)-5)
					break
				}
				fmt.Printf("  [%d] ID: %s, Title: %s, Status: %s, Severity: %s\n",
					i+1, inc.ID, inc.Title, inc.Status, inc.Severity)

				// Validate required fields
				if inc.ID == "" || inc.Title == "" || inc.Status == "" {
					testResult("Validate incident fields", fmt.Errorf("missing required fields in incident %d", i))
				}
			}
			if len(incidents) > 0 {
				testResult("Query all incidents", nil)
			} else {
				fmt.Println("⚠️  No incidents found (this is okay if your service has no incidents)")
				totalTests++
			}
		}

		// Test 2: Query open incidents
		fmt.Println("\n=== Test 2: Query Open Incidents ===")
		openIncidents, err := provider.Query(ctx, schema.IncidentQuery{
			Statuses: []string{"open"},
		})
		if err != nil {
			testResult("Query open incidents", err)
		} else {
			fmt.Printf("Found %d open incidents\n", len(openIncidents))
			// Verify all returned incidents are actually open
			allOpen := true
			for _, inc := range openIncidents {
				if inc.Status != "open" {
					allOpen = false
					testResult("Validate status filter", fmt.Errorf("expected open status, got %s", inc.Status))
					break
				}
			}
			if allOpen {
				testResult("Query open incidents", nil)
			}
		}

		// Test 3: Get a specific incident if we have any
		if len(incidents) > 0 {
			fmt.Println("\n=== Test 3: Get Specific Incident ===")
			firstIncident := incidents[0]
			inc, err := provider.Get(ctx, firstIncident.ID)
			if err != nil {
				testResult("Get incident by ID", err)
			} else {
				fmt.Printf("Retrieved incident:\n")
				fmt.Printf("  ID: %s\n", inc.ID)
				fmt.Printf("  Title: %s\n", inc.Title)
				fmt.Printf("  Status: %s\n", inc.Status)
				fmt.Printf("  Severity: %s\n", inc.Severity)
				fmt.Printf("  Service: %s\n", inc.Service)
				fmt.Printf("  Created: %s\n", inc.CreatedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("  Updated: %s\n", inc.UpdatedAt.Format("2006-01-02 15:04:05"))

				// Validate data integrity
				if inc.ID != firstIncident.ID {
					testResult("Validate incident ID", fmt.Errorf("ID mismatch: expected %s, got %s", firstIncident.ID, inc.ID))
				} else if inc.Metadata == nil {
					testResult("Validate incident metadata", fmt.Errorf("metadata is nil"))
				} else {
					testResult("Get incident by ID", nil)
				}
			}

			// Test 4: Get timeline for the incident
			fmt.Println("\n=== Test 4: Get Incident Timeline ===")
			timeline, err := provider.GetTimeline(ctx, firstIncident.ID)
			if err != nil {
				testResult("Get incident timeline", err)
			} else {
				fmt.Printf("Timeline has %d entries\n", len(timeline))
				for i, entry := range timeline {
					if i >= 3 {
						fmt.Printf("... and %d more\n", len(timeline)-3)
						break
					}
					fmt.Printf("  [%d] %s: %s\n", i+1, entry.At.Format("2006-01-02 15:04:05"), entry.Body)

					// Validate timeline entry fields
					if entry.ID == "" || entry.IncidentID == "" || entry.Body == "" {
						testResult("Validate timeline entry", fmt.Errorf("missing required fields in entry %d", i))
						break
					}
				}
				testResult("Get incident timeline", nil)
			}
		}

		// Test 5: Error handling - invalid incident ID
		fmt.Println("\n=== Test 5: Error Handling - Invalid ID ===")
		_, err = provider.Get(ctx, "INVALID_ID_9999")
		if err != nil {
			fmt.Printf("Correctly handled invalid ID: %v\n", err)
			testResult("Error handling for invalid ID", nil)
		} else {
			testResult("Error handling for invalid ID", fmt.Errorf("should have returned error for invalid ID"))
		}

		// Test 6: Create a new incident
		fmt.Println("\n=== Test 6: Create New Incident ===")
		newIncident, err := provider.Create(ctx, schema.CreateIncidentInput{
			Title:    "Integration Test Incident",
			Status:   "open",
			Severity: "critical",
			Service:  "Test Service",
			Fields: map[string]any{
				"body": "This is a test incident created by the OpsOrch PagerDuty adapter integration test.",
			},
		})
		if err != nil {
			testResult("Create incident", err)
		} else {
			fmt.Printf("Successfully created incident:\n")
			fmt.Printf("  ID: %s\n", newIncident.ID)
			fmt.Printf("  Title: %s\n", newIncident.Title)
			fmt.Printf("  Status: %s\n", newIncident.Status)
			fmt.Printf("  Severity: %s\n", newIncident.Severity)
			fmt.Printf("  Created: %s\n", newIncident.CreatedAt.Format("2006-01-02 15:04:05"))

			// Validate created incident matches input
			if newIncident.Title != "Integration Test Incident" {
				testResult("Validate created incident title", fmt.Errorf("title mismatch"))
			} else if newIncident.Status != "open" {
				testResult("Validate created incident status", fmt.Errorf("status mismatch"))
			} else {
				testResult("Create incident", nil)
			}

			// Test 7: Append a note to the timeline
			fmt.Println("\n=== Test 7: Add Note to Incident ===")
			err = provider.AppendTimeline(ctx, newIncident.ID, schema.TimelineAppendInput{
				At:   time.Now(),
				Kind: "note",
				Body: "This is a test note added via the integration test.",
			})
			testResult("Append timeline note", err)

			// Test 8: Update the incident status
			fmt.Println("\n=== Test 8: Update Incident Status ===")
			newStatus := "acknowledged"
			updatedIncident, err := provider.Update(ctx, newIncident.ID, schema.UpdateIncidentInput{
				Status: &newStatus,
			})
			if err != nil {
				testResult("Update incident status", err)
			} else {
				if updatedIncident.Status != "acknowledged" {
					testResult("Validate updated status", fmt.Errorf("expected acknowledged, got %s", updatedIncident.Status))
				} else {
					fmt.Printf("✅ Incident status updated to: %s\n", updatedIncident.Status)
					testResult("Update incident status", nil)
				}
			}

			// Test 9: Cleanup - Resolve the test incident
			fmt.Println("\n=== Test 9: Cleanup - Resolve Test Incident ===")
			resolvedStatus := "resolved"
			_, err = provider.Update(ctx, newIncident.ID, schema.UpdateIncidentInput{
				Status: &resolvedStatus,
			})
			if err != nil {
				fmt.Printf("⚠️  Warning: Could not resolve test incident %s: %v\n", newIncident.ID, err)
				fmt.Printf("   Please manually resolve this incident in PagerDuty\n")
			} else {
				fmt.Printf("✅ Test incident %s resolved\n", newIncident.ID)
			}
			testResult("Cleanup test incident", err)
		}

		// Test 10: Query incidents by severity
		fmt.Println("\n=== Test 10: Query by Severity ===")
		criticalIncidents, err := provider.Query(ctx, schema.IncidentQuery{
			Severities: []string{"critical"},
		})
		if err != nil {
			testResult("Query by severity", err)
		} else {
			fmt.Printf("Found %d critical incidents\n", len(criticalIncidents))
			allCritical := true
			for _, inc := range criticalIncidents {
				if inc.Severity != "critical" {
					allCritical = false
					testResult("Validate severity filter", fmt.Errorf("expected critical, got %s", inc.Severity))
					break
				}
			}
			if allCritical {
				testResult("Query by severity", nil)
			}
		}

		// Test 11: Query incidents by Scope.Service (using service name lookup)
		if len(services) > 0 {
			fmt.Println("\n=== Test 11: Query by Scope.Service ===")
			// Use a service name that we know exists
			serviceName := services[0].Name
			fmt.Printf("Querying incidents for service: %s\n", serviceName)

			scopedIncidents, err := provider.Query(ctx, schema.IncidentQuery{
				Scope: schema.QueryScope{Service: serviceName},
			})
			if err != nil {
				testResult("Query by Scope.Service", err)
			} else {
				fmt.Printf("Found %d incidents for service '%s'\n", len(scopedIncidents), serviceName)
				testResult("Query by Scope.Service", nil)
			}
		}

		// Test 12: Query incidents with combined filters
		fmt.Println("\n=== Test 12: Query with Combined Filters ===")
		combinedIncidents, err := provider.Query(ctx, schema.IncidentQuery{
			Statuses:   []string{"open", "acknowledged"},
			Severities: []string{"critical", "high"},
			Limit:      20,
		})
		if err != nil {
			testResult("Query with combined filters", err)
		} else {
			fmt.Printf("Found %d incidents (open/acknowledged + critical/high severity)\n", len(combinedIncidents))
			// Validate all results match filters
			allMatch := true
			for _, inc := range combinedIncidents {
				statusMatch := inc.Status == "open" || inc.Status == "acknowledged"
				severityMatch := inc.Severity == "critical" || inc.Severity == "high"
				if !statusMatch || !severityMatch {
					allMatch = false
					testResult("Validate combined filters", fmt.Errorf("incident %s doesn't match filters", inc.ID))
					break
				}
			}
			if allMatch {
				testResult("Query with combined filters", nil)
			}
		}
	} else {
		fmt.Println("\n⏭️  Skipping incident tests (PAGERDUTY_SERVICE_ID and PAGERDUTY_FROM_EMAIL not set)")
	}

	// Run service adapter tests (always available)
	if runServiceTests {
		fmt.Println("\n=== Service Adapter Tests ===")

		serviceConfig := map[string]any{
			"apiToken": apiToken,
			"apiURL":   apiURL,
		}

		serviceProvider, err := service.New(serviceConfig)
		if err != nil {
			log.Fatalf("Failed to create PagerDuty service provider: %v", err)
		}

		// Test: Query All Services
		fmt.Println("\n=== Test: Query All Services ===")
		services, err = serviceProvider.Query(ctx, schema.ServiceQuery{})
		if err != nil {
			testResult("Query all services", err)
		} else {
			fmt.Printf("Found %d services\n", len(services))
			for i, svc := range services {
				if i >= 5 {
					fmt.Printf("... and %d more\n", len(services)-5)
					break
				}
				fmt.Printf("  [%d] ID: %s, Name: %s\n", i+1, svc.ID, svc.Name)
				if svc.Metadata["status"] != nil {
					fmt.Printf("      Status: %v\n", svc.Metadata["status"])
				}

				// Validate service fields
				if svc.ID == "" || svc.Name == "" {
					testResult("Validate service fields", fmt.Errorf("missing required fields in service %d", i))
					break
				}
			}
			if len(services) > 0 {
				testResult("Query all services", nil)
			}

			// Test 13: Filter services by name
			if len(services) > 0 {
				fmt.Println("\n=== Test 13: Filter Services by Name ===")
				firstServiceName := services[0].Name
				searchTerm := firstServiceName
				if len(firstServiceName) > 5 {
					searchTerm = firstServiceName[:5]
				}

				filteredServices, err := serviceProvider.Query(ctx, schema.ServiceQuery{
					Name: searchTerm,
				})
				if err != nil {
					testResult("Filter services by name", err)
				} else {
					fmt.Printf("Found %d services matching '%s'\n", len(filteredServices), searchTerm)
					for i, svc := range filteredServices {
						if i >= 3 {
							fmt.Printf("... and %d more\n", len(filteredServices)-3)
							break
						}
						fmt.Printf("  [%d] %s\n", i+1, svc.Name)
					}
					// All filtered services should contain the search term
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
		}
	} // End service tests

	// Print summary
	duration := time.Since(startTime)
	fmt.Println("\n=================================")
	fmt.Println("Test Summary")
	fmt.Println("=================================")
	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Passed: %d ✅\n", passedTests)
	fmt.Printf("Failed: %d ❌\n", failedTests)
	fmt.Printf("Duration: %v\n", duration.Round(time.Millisecond))
	fmt.Printf("Success Rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)

	if failedTests == 0 {
		fmt.Println("\n✅ All tests passed successfully!")
	} else {
		fmt.Printf("\n⚠️  %d test(s) failed. Please review the output above.\n", failedTests)
		os.Exit(1)
	}
}
