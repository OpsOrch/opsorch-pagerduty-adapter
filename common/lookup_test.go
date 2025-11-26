package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookupServiceIDsByName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/services" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Verify query parameter
		query := r.URL.Query().Get("query")
		if query == "" {
			t.Error("expected query parameter")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"services": []map[string]any{
				{
					"id":   "SVCID1",
					"name": "Production API",
				},
				{
					"id":   "SVCID2",
					"name": "Production Database",
				},
				{
					"id":   "SVCID3",
					"name": "Staging API",
				},
			},
		})
	}))
	defer server.Close()

	client := &http.Client{}
	ctx := context.Background()

	t.Run("exact match", func(t *testing.T) {
		ids, err := LookupServiceIDsByName(ctx, client, server.URL, "token", "Production API")
		if err != nil {
			t.Fatalf("LookupServiceIDsByName() error = %v", err)
		}
		if len(ids) != 1 {
			t.Errorf("expected 1 ID, got %d", len(ids))
		}
		if ids[0] != "SVCID1" {
			t.Errorf("expected SVCID1, got %s", ids[0])
		}
	})

	t.Run("fuzzy match", func(t *testing.T) {
		ids, err := LookupServiceIDsByName(ctx, client, server.URL, "token", "production")
		if err != nil {
			t.Fatalf("LookupServiceIDsByName() error = %v", err)
		}
		if len(ids) != 2 {
			t.Errorf("expected 2 IDs (Production API and Production Database), got %d", len(ids))
		}
	})

	t.Run("no match", func(t *testing.T) {
		ids, err := LookupServiceIDsByName(ctx, client, server.URL, "token", "nonexistent")
		if err != nil {
			t.Fatalf("LookupServiceIDsByName() error = %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("expected 0 IDs, got %d", len(ids))
		}
	})
}

func TestLookupTeamIDsByName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Verify query parameter
		query := r.URL.Query().Get("query")
		if query == "" {
			t.Error("expected query parameter")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"teams": []map[string]any{
				{
					"id":   "TEAM1",
					"name": "Platform Team",
				},
				{
					"id":   "TEAM2",
					"name": "Backend Team",
				},
				{
					"id":   "TEAM3",
					"name": "Platform Infrastructure",
				},
			},
		})
	}))
	defer server.Close()

	client := &http.Client{}
	ctx := context.Background()

	t.Run("exact match", func(t *testing.T) {
		ids, err := LookupTeamIDsByName(ctx, client, server.URL, "token", "Platform Team")
		if err != nil {
			t.Fatalf("LookupTeamIDsByName() error = %v", err)
		}
		if len(ids) != 1 {
			t.Errorf("expected 1 ID, got %d", len(ids))
		}
		if ids[0] != "TEAM1" {
			t.Errorf("expected TEAM1, got %s", ids[0])
		}
	})

	t.Run("fuzzy match", func(t *testing.T) {
		ids, err := LookupTeamIDsByName(ctx, client, server.URL, "token", "platform")
		if err != nil {
			t.Fatalf("LookupTeamIDsByName() error = %v", err)
		}
		if len(ids) != 2 {
			t.Errorf("expected 2 IDs (Platform Team and Platform Infrastructure), got %d", len(ids))
		}
	})

	t.Run("no match", func(t *testing.T) {
		ids, err := LookupTeamIDsByName(ctx, client, server.URL, "token", "nonexistent")
		if err != nil {
			t.Fatalf("LookupTeamIDsByName() error = %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("expected 0 IDs, got %d", len(ids))
		}
	})
}

func TestLookupServiceIDsByName_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	client := &http.Client{}
	ctx := context.Background()

	_, err := LookupServiceIDsByName(ctx, client, server.URL, "token", "test")
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestLookupTeamIDsByName_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "access denied"}`))
	}))
	defer server.Close()

	client := &http.Client{}
	ctx := context.Background()

	_, err := LookupTeamIDsByName(ctx, client, server.URL, "token", "test")
	if err == nil {
		t.Error("expected error for API failure")
	}
}
