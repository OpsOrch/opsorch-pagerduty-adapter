package incident

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
	if cfg.DefaultSeverity != "critical" {
		t.Fatalf("expected default severity, got %q", cfg.DefaultSeverity)
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
		"source":          "demo",
		"defaultSeverity": "high",
		"apiToken":        " token ",
		"apiURL":          " https://example.com ",
		"serviceID":       " PXXXXXX ",
		"fromEmail":       " user@example.com ",
	})
	if cfg.Source != "demo" {
		t.Fatalf("expected overridden source, got %q", cfg.Source)
	}
	if cfg.DefaultSeverity != "high" {
		t.Fatalf("expected overridden severity, got %q", cfg.DefaultSeverity)
	}
	if cfg.APIToken != "token" {
		t.Fatalf("expected token to be trimmed, got %q", cfg.APIToken)
	}
	if cfg.APIURL != "https://example.com" {
		t.Fatalf("expected API URL override, got %q", cfg.APIURL)
	}
	if cfg.ServiceID != "PXXXXXX" {
		t.Fatalf("expected serviceID to be trimmed, got %q", cfg.ServiceID)
	}
	if cfg.FromEmail != "user@example.com" {
		t.Fatalf("expected fromEmail to be trimmed, got %q", cfg.FromEmail)
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	if _, err := New(map[string]any{}); err == nil {
		t.Fatalf("expected error when apiToken missing")
	}
	if _, err := New(map[string]any{"apiToken": "token", "apiURL": ""}); err == nil {
		t.Fatalf("expected error when apiURL missing")
	}
	if _, err := New(map[string]any{"apiToken": "token", "apiURL": "https://api.pagerduty.com"}); err == nil {
		t.Fatalf("expected error when serviceID missing")
	}
	if _, err := New(map[string]any{"apiToken": "token", "apiURL": "https://api.pagerduty.com", "serviceID": "PXXXXXX"}); err == nil {
		t.Fatalf("expected error when fromEmail missing")
	}
}

func TestCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/incidents" && r.Method == "POST" {
			// Verify headers
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Token token=") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.Header.Get("From") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"incident": map[string]any{
					"id":           "PINCIDENT1",
					"incident_key": "key123",
					"title":        "Test incident",
					"status":       "triggered",
					"urgency":      "high",
					"service": map[string]any{
						"id":      "PXXXXXX",
						"summary": "Test Service",
					},
					"body": map[string]any{
						"details": "Test incident description",
					},
					"created_at": "2025-11-21T10:00:00Z",
					"updated_at": "2025-11-21T10:00:00Z",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg: Config{
			Source:          "pagerduty",
			APIToken:        "test-token",
			APIURL:          server.URL,
			ServiceID:       "PXXXXXX",
			FromEmail:       "user@example.com",
			DefaultSeverity: "critical",
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	inc, err := p.Create(ctx, schema.CreateIncidentInput{
		Title:    "Test incident",
		Status:   "open",
		Severity: "critical",
		Service:  "Test Service",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if inc.ID != "PINCIDENT1" {
		t.Errorf("ID = %v, want PINCIDENT1", inc.ID)
	}
	if inc.Title != "Test incident" {
		t.Errorf("Title = %v, want Test incident", inc.Title)
	}
	if inc.Status != "open" {
		t.Errorf("Status = %v, want open", inc.Status)
	}
	if inc.Severity != "critical" {
		t.Errorf("Severity = %v, want critical", inc.Severity)
	}
	if inc.Description != "Test incident description" {
		t.Errorf("Description = %v, want Test incident description", inc.Description)
	}
}

func TestGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/incidents/PINCIDENT1" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"incident": map[string]any{
					"id":           "PINCIDENT1",
					"incident_key": "key123",
					"title":        "Test incident",
					"status":       "acknowledged",
					"urgency":      "high",
					"service": map[string]any{
						"id":      "PXXXXXX",
						"summary": "Test Service",
					},
					"body": map[string]any{
						"details": "Acknowledged incident details",
					},
					"created_at": "2025-11-21T10:00:00Z",
					"updated_at": "2025-11-21T11:00:00Z",
				},
			})
			return
		}
		if r.URL.Path == "/incidents/NOTFOUND" && r.Method == "GET" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg: Config{
			Source:    "pagerduty",
			APIToken:  "test-token",
			APIURL:    server.URL,
			ServiceID: "PXXXXXX",
			FromEmail: "user@example.com",
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	t.Run("get existing incident", func(t *testing.T) {
		inc, err := p.Get(ctx, "PINCIDENT1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if inc.ID != "PINCIDENT1" {
			t.Errorf("ID = %v, want PINCIDENT1", inc.ID)
		}
		if inc.Status != "acknowledged" {
			t.Errorf("Status = %v, want acknowledged", inc.Status)
		}
		if inc.Description != "Acknowledged incident details" {
			t.Errorf("Description = %v, want Acknowledged incident details", inc.Description)
		}
	})

	t.Run("get non-existent incident", func(t *testing.T) {
		_, err := p.Get(ctx, "NOTFOUND")
		if err != errNotFound {
			t.Errorf("Get() error = %v, want errNotFound", err)
		}
	})

	t.Run("get incident without body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/incidents/PNOBODY" && r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"incident": map[string]any{
						"id":           "PNOBODY",
						"incident_key": "nobodykey",
						"title":        "Incident without body",
						"status":       "triggered",
						"urgency":      "high",
						"service": map[string]any{
							"id":      "PXXXXXX",
							"summary": "Test Service",
						},
						// No body field
						"created_at": "2025-11-21T10:00:00Z",
						"updated_at": "2025-11-21T10:00:00Z",
					},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		prov := &PagerDutyProvider{
			cfg: Config{
				Source:    "pagerduty",
				APIToken:  "test-token",
				APIURL:    server.URL,
				ServiceID: "PXXXXXX",
				FromEmail: "user@example.com",
			},
			client: &http.Client{},
		}

		inc, err := prov.Get(context.Background(), "PNOBODY")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if inc.Description != "" {
			t.Errorf("Description = %v, want empty string", inc.Description)
		}
		if inc.ID != "PNOBODY" {
			t.Errorf("ID = %v, want PNOBODY", inc.ID)
		}
	})
}

func TestQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/incidents") && r.Method == "GET" {
			query := r.URL.Query()

			var incidents []map[string]any

			// Filter by status
			if statuses := query["statuses[]"]; len(statuses) > 0 && statuses[0] == "triggered" {
				incidents = []map[string]any{
					{
						"id":           "PINCIDENT1",
						"incident_key": "key1",
						"title":        "Open incident",
						"status":       "triggered",
						"urgency":      "high",
						"service": map[string]any{
							"id":      "PXXXXXX",
							"summary": "Test Service",
						},
						"created_at": "2025-11-21T10:00:00Z",
						"updated_at": "2025-11-21T10:00:00Z",
					},
				}
			} else {
				incidents = []map[string]any{
					{
						"id":           "PINCIDENT1",
						"incident_key": "key1",
						"title":        "Incident 1",
						"status":       "triggered",
						"urgency":      "high",
						"service": map[string]any{
							"id":      "PXXXXXX",
							"summary": "Test Service",
						},
						"created_at": "2025-11-21T10:00:00Z",
						"updated_at": "2025-11-21T10:00:00Z",
					},
					{
						"id":           "PINCIDENT2",
						"incident_key": "key2",
						"title":        "Incident 2",
						"status":       "acknowledged",
						"urgency":      "low",
						"service": map[string]any{
							"id":      "PXXXXXX",
							"summary": "Test Service",
						},
						"created_at": "2025-11-21T09:00:00Z",
						"updated_at": "2025-11-21T10:30:00Z",
					},
				}
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"incidents": incidents,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg: Config{
			Source:    "pagerduty",
			APIToken:  "test-token",
			APIURL:    server.URL,
			ServiceID: "PXXXXXX",
			FromEmail: "user@example.com",
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	t.Run("query all", func(t *testing.T) {
		incidents, err := p.Query(ctx, schema.IncidentQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(incidents) != 2 {
			t.Errorf("len(incidents) = %v, want 2", len(incidents))
		}
	})

	t.Run("query with status filter", func(t *testing.T) {
		incidents, err := p.Query(ctx, schema.IncidentQuery{
			Statuses: []string{"open"},
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(incidents) != 1 {
			t.Errorf("len(incidents) = %v, want 1", len(incidents))
		}
	})
}

func TestQueryFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/incidents") && r.Method == "GET" {
			query := r.URL.Query()

			// Check severities -> urgencies[]
			if urgencies := query["urgencies[]"]; len(urgencies) > 0 && urgencies[0] == "high" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"incidents": []map[string]any{}})
				return
			}

			// Check Metadata["team_id"] -> team_ids[]
			if teams := query["team_ids[]"]; len(teams) > 0 && teams[0] == "TEAM1" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"incidents": []map[string]any{}})
				return
			}

			// Check Metadata["service_id"] -> service_ids[]
			if services := query["service_ids[]"]; len(services) > 0 && services[0] == "S1" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"incidents": []map[string]any{}})
				return
			}

			// Check Metadata["incident_key"] -> incident_key
			if query.Get("incident_key") == "KEY1" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"incidents": []map[string]any{}})
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

	tests := []struct {
		name  string
		query schema.IncidentQuery
	}{
		{
			name:  "severities",
			query: schema.IncidentQuery{Severities: []string{"critical"}},
		},
		{
			name:  "metadata service_id",
			query: schema.IncidentQuery{Metadata: map[string]any{"service_id": "S1"}},
		},
		{
			name:  "metadata team_id",
			query: schema.IncidentQuery{Metadata: map[string]any{"team_id": "TEAM1"}},
		},
		{
			name:  "metadata incident_key",
			query: schema.IncidentQuery{Metadata: map[string]any{"incident_key": "KEY1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Query(ctx, tt.query)
			if err != nil {
				t.Errorf("Query() error = %v", err)
			}
		})
	}
}

func TestQueryWithScope(t *testing.T) {
	// Mock server that handles both /services, /teams, and /incidents
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/services":
			// Return services for Scope.Service lookup
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"services": []map[string]any{
					{"id": "SVC1", "name": "Production API"},
					{"id": "SVC2", "name": "Production Database"},
				},
			})
		case r.URL.Path == "/teams":
			// Return teams for Scope.Team lookup
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"teams": []map[string]any{
					{"id": "TEAM1", "name": "Platform Team"},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/incidents"):
			query := r.URL.Query()
			// Verify that service_ids or team_ids were translated and passed
			serviceIDs := query["service_ids[]"]
			teamIDs := query["team_ids[]"]

			if len(serviceIDs) > 0 || len(teamIDs) > 0 {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"incidents": []map[string]any{
						{
							"id":      "INC1",
							"title":   "Test Incident",
							"status":  "triggered",
							"urgency": "high",
							"service": map[string]any{"id": "SVC1", "summary": "Production API"},
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

	t.Run("scope service", func(t *testing.T) {
		incidents, err := p.Query(ctx, schema.IncidentQuery{
			Scope: schema.QueryScope{Service: "Production"},
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(incidents) != 1 {
			t.Errorf("expected 1 incident, got %d", len(incidents))
		}
	})

	t.Run("scope team", func(t *testing.T) {
		incidents, err := p.Query(ctx, schema.IncidentQuery{
			Scope: schema.QueryScope{Team: "Platform"},
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(incidents) != 1 {
			t.Errorf("expected 1 incident, got %d", len(incidents))
		}
	})
}

func TestQueryServiceIDFilter(t *testing.T) {
	// Test that configured serviceID is NOT automatically used in queries
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/incidents") && r.Method == "GET" {
			query := r.URL.Query()
			serviceIDs := query["service_ids[]"]

			// Return different incidents based on whether service filter is present
			var incidents []map[string]any
			if len(serviceIDs) > 0 {
				// Filtered query
				incidents = []map[string]any{
					{
						"id":         "INC1",
						"title":      "Filtered Incident",
						"status":     "triggered",
						"urgency":    "high",
						"service":    map[string]any{"id": serviceIDs[0], "summary": "Specific Service"},
						"created_at": "2025-11-21T10:00:00Z",
						"updated_at": "2025-11-21T10:00:00Z",
					},
				}
			} else {
				// Unfiltered query - returns incidents from all services
				incidents = []map[string]any{
					{
						"id":         "INC1",
						"title":      "Incident from Service A",
						"status":     "triggered",
						"urgency":    "high",
						"service":    map[string]any{"id": "SVC_A", "summary": "Service A"},
						"created_at": "2025-11-21T10:00:00Z",
						"updated_at": "2025-11-21T10:00:00Z",
					},
					{
						"id":         "INC2",
						"title":      "Incident from Service B",
						"status":     "acknowledged",
						"urgency":    "low",
						"service":    map[string]any{"id": "SVC_B", "summary": "Service B"},
						"created_at": "2025-11-21T09:00:00Z",
						"updated_at": "2025-11-21T10:30:00Z",
					},
				}
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"incidents": incidents,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg: Config{
			APIToken:  "token",
			APIURL:    server.URL,
			ServiceID: "DEFAULT_SVC",
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	t.Run("does not use configured serviceID when no filters", func(t *testing.T) {
		incidents, err := p.Query(ctx, schema.IncidentQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		// Should return incidents from all services
		if len(incidents) != 2 {
			t.Errorf("expected 2 incidents (from all services), got %d", len(incidents))
		}
	})

	t.Run("uses explicit metadata service_id filter", func(t *testing.T) {
		incidents, err := p.Query(ctx, schema.IncidentQuery{
			Metadata: map[string]any{"service_id": "CUSTOM_SVC"},
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		// Should return only filtered incidents
		if len(incidents) != 1 {
			t.Errorf("expected 1 incident (filtered), got %d", len(incidents))
		}
		if incidents[0].Title != "Filtered Incident" {
			t.Errorf("expected filtered incident, got %v", incidents[0].Title)
		}
	})
}

func TestUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/incidents/PINCIDENT1" && r.Method == "PUT" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"incident": map[string]any{
					"id":           "PINCIDENT1",
					"incident_key": "key123",
					"title":        "Updated incident",
					"status":       "resolved",
					"urgency":      "low",
					"service": map[string]any{
						"id":      "PXXXXXX",
						"summary": "Test Service",
					},
					"created_at": "2025-11-21T10:00:00Z",
					"updated_at": "2025-11-21T12:00:00Z",
				},
			})
			return
		}
		if r.URL.Path == "/incidents/NOTFOUND" && r.Method == "PUT" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &PagerDutyProvider{
		cfg: Config{
			Source:    "pagerduty",
			APIToken:  "test-token",
			APIURL:    server.URL,
			ServiceID: "PXXXXXX",
			FromEmail: "user@example.com",
		},
		client: &http.Client{},
	}
	ctx := context.Background()

	t.Run("update incident", func(t *testing.T) {
		newTitle := "Updated incident"
		newStatus := "resolved"
		inc, err := p.Update(ctx, "PINCIDENT1", schema.UpdateIncidentInput{
			Title:  &newTitle,
			Status: &newStatus,
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if inc.Title != "Updated incident" {
			t.Errorf("Title = %v, want Updated incident", inc.Title)
		}
		if inc.Status != "resolved" {
			t.Errorf("Status = %v, want resolved", inc.Status)
		}
	})

	t.Run("update non-existent incident", func(t *testing.T) {
		newTitle := "Test"
		_, err := p.Update(ctx, "NOTFOUND", schema.UpdateIncidentInput{
			Title: &newTitle,
		})
		if err != errNotFound {
			t.Errorf("Update() error = %v, want errNotFound", err)
		}
	})
}

func TestMappingFunctions(t *testing.T) {
	t.Run("mapSeverityToUrgency", func(t *testing.T) {
		tests := []struct {
			severity string
			urgency  string
		}{
			{"critical", "high"},
			{"sev1", "high"},
			{"high", "high"},
			{"medium", "low"},
			{"sev3", "low"},
			{"low", "low"},
		}
		for _, tt := range tests {
			if got := mapSeverityToUrgency(tt.severity); got != tt.urgency {
				t.Errorf("mapSeverityToUrgency(%q) = %q, want %q", tt.severity, got, tt.urgency)
			}
		}
	})

	t.Run("mapStatusToPD", func(t *testing.T) {
		tests := []struct {
			status   string
			pdStatus string
		}{
			{"open", "triggered"},
			{"triggered", "triggered"},
			{"acknowledged", "acknowledged"},
			{"investigating", "acknowledged"},
			{"resolved", "resolved"},
			{"closed", "resolved"},
		}
		for _, tt := range tests {
			if got := mapStatusToPD(tt.status); got != tt.pdStatus {
				t.Errorf("mapStatusToPD(%q) = %q, want %q", tt.status, got, tt.pdStatus)
			}
		}
	})
}
