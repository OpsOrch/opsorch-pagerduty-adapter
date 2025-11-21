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
	})

	t.Run("get non-existent incident", func(t *testing.T) {
		_, err := p.Get(ctx, "NOTFOUND")
		if err != errNotFound {
			t.Errorf("Get() error = %v, want errNotFound", err)
		}
	})
}

func TestList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/incidents") && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"incidents": []map[string]any{
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
				},
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

	incidents, err := p.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(incidents) != 2 {
		t.Errorf("len(incidents) = %v, want 2", len(incidents))
	}
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

	t.Run("query with text search", func(t *testing.T) {
		incidents, err := p.Query(ctx, schema.IncidentQuery{
			Query: "Incident 2",
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(incidents) != 1 {
			t.Errorf("len(incidents) = %v, want 1", len(incidents))
		}
		if incidents[0].Title != "Incident 2" {
			t.Errorf("Title = %v, want Incident 2", incidents[0].Title)
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
