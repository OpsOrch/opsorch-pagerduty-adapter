package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opsorch/opsorch-core/schema"
)

func TestParseConfigDefaults(t *testing.T) {
	cfg := parseConfig(map[string]any{})
	if cfg.Source != "pagerduty" {
		t.Fatalf("expected default source, got %q", cfg.Source)
	}
	if cfg.APIURL != "https://api.pagerduty.com" {
		t.Fatalf("expected default API URL, got %q", cfg.APIURL)
	}
	if cfg.APIToken != "" {
		t.Fatalf("expected empty API token by default")
	}
}

func TestParseConfigOverride(t *testing.T) {
	cfg := parseConfig(map[string]any{
		"source":   "demo",
		"apiToken": " token ",
		"apiURL":   " https://example.com ",
	})
	if cfg.Source != "demo" {
		t.Fatalf("expected overridden source, got %q", cfg.Source)
	}
	if cfg.APIToken != "token" {
		t.Fatalf("expected token to be trimmed, got %q", cfg.APIToken)
	}
	if cfg.APIURL != "https://example.com" {
		t.Fatalf("expected API URL override, got %q", cfg.APIURL)
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	if _, err := New(map[string]any{}); err == nil {
		t.Fatalf("expected error when apiToken missing")
	}
	// apiURL has a default, so it should succeed with just apiToken
	if _, err := New(map[string]any{"apiToken": "token"}); err != nil {
		t.Fatalf("expected success with apiToken (apiURL has default), got: %v", err)
	}
}

func TestQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/services") && r.Method == "GET" {
			// Verify headers
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Token token=") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"services": []map[string]any{
					{
						"id":          "PSERVICE1",
						"name":        "Production API",
						"summary":     "Main production API service",
						"description": "Handles all production traffic",
						"status":      "active",
						"created_at":  "2025-11-01T10:00:00Z",
						"updated_at":  "2025-11-20T15:30:00Z",
						"escalation_policy": map[string]any{
							"id":      "PESCAL1",
							"type":    "escalation_policy_reference",
							"summary": "Production Escalation",
						},
						"teams": []map[string]any{
							{
								"id":      "PTEAM1",
								"type":    "team_reference",
								"summary": "Platform Team",
							},
						},
					},
					{
						"id":          "PSERVICE2",
						"name":        "Database Service",
						"summary":     "PostgreSQL database",
						"description": "Primary database",
						"status":      "active",
						"created_at":  "2025-11-01T11:00:00Z",
						"updated_at":  "2025-11-15T12:00:00Z",
						"escalation_policy": map[string]any{
							"id":      "PESCAL1",
							"type":    "escalation_policy_reference",
							"summary": "Production Escalation",
						},
						"teams": []map[string]any{},
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg: Config{
			Source:   "pagerduty",
			APIToken: "test-token",
			APIURL:   server.URL,
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	services, err := p.Query(ctx, schema.ServiceQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(services) != 2 {
		t.Errorf("len(services) = %v, want 2", len(services))
	}
	if services[0].ID != "PSERVICE1" {
		t.Errorf("services[0].ID = %v, want PSERVICE1", services[0].ID)
	}
	if services[0].Name != "Production API" {
		t.Errorf("services[0].Name = %v, want Production API", services[0].Name)
	}
	if services[0].Metadata["status"] != "active" {
		t.Errorf("services[0].Metadata[status] = %v, want active", services[0].Metadata["status"])
	}
}

func TestQueryWithNameFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/services") && r.Method == "GET" {
			query := r.URL.Query()

			// Check if query filter is applied
			if query.Get("query") != "Database" {
				t.Errorf("expected query=Database, got query=%s", query.Get("query"))
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"services": []map[string]any{
					{
						"id":          "PSERVICE2",
						"name":        "Database Service",
						"summary":     "PostgreSQL database",
						"description": "Primary database",
						"status":      "active",
						"created_at":  "2025-11-01T11:00:00Z",
						"updated_at":  "2025-11-15T12:00:00Z",
						"escalation_policy": map[string]any{
							"id":      "PESCAL1",
							"type":    "escalation_policy_reference",
							"summary": "Production Escalation",
						},
						"teams": []map[string]any{},
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg: Config{
			Source:   "pagerduty",
			APIToken: "test-token",
			APIURL:   server.URL,
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	services, err := p.Query(ctx, schema.ServiceQuery{Name: "Database"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(services) != 1 {
		t.Errorf("len(services) = %v, want 1", len(services))
	}
	if services[0].Name != "Database Service" {
		t.Errorf("services[0].Name = %v, want Database Service", services[0].Name)
	}
}

func TestQueryFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/services") && r.Method == "GET" {
			query := r.URL.Query()

			// Check team_ids filter from Scope.Team
			if query.Get("team_ids[]") == "TEAM1" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"services": []map[string]any{}})
				return
			}

			// Check team_ids filter from Metadata
			if query.Get("team_ids[]") == "TEAM2" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"services": []map[string]any{}})
				return
			}

			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg:    Config{APIToken: "token", APIURL: server.URL},
		client: &http.Client{},
	}
	ctx := context.Background()

	// Test Metadata["team_id"] -> team_ids[]
	_, err := p.Query(ctx, schema.ServiceQuery{
		Metadata: map[string]any{"team_id": "TEAM2"},
	})
	if err != nil {
		t.Errorf("Query with Metadata[team_id] failed: %v", err)
	}
}

func TestQueryWithScope(t *testing.T) {
	// Mock server that handles both /teams and /services
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/teams":
			// Return teams for Scope.Team lookup
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"teams": []map[string]any{
					{"id": "TEAM1", "name": "Platform Team"},
					{"id": "TEAM2", "name": "Platform Infrastructure"},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/services"):
			query := r.URL.Query()
			// Verify that team_ids were translated and passed
			teamIDs := query["team_ids[]"]

			if len(teamIDs) > 0 {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"services": []map[string]any{
						{
							"id":     "SVC1",
							"name":   "Production API",
							"status": "active",
						},
					},
				})
				return
			}
			w.WriteHeader(http.StatusBadRequest)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg:    Config{APIToken: "token", APIURL: server.URL},
		client: &http.Client{},
	}
	ctx := context.Background()

	services, err := p.Query(ctx, schema.ServiceQuery{
		Scope: schema.QueryScope{Team: "Platform"},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}
}

func TestQueryAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal Server Error"}`))
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg: Config{
			Source:   "pagerduty",
			APIToken: "test-token",
			APIURL:   server.URL,
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	_, err := p.Query(ctx, schema.ServiceQuery{})
	if err == nil {
		t.Fatal("expected error for API failure")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status code 500, got %v", err)
	}
}
