package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	// Mock PagerDuty API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/services") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"services": [
					{
						"id": "P12345",
						"name": "Test Service",
						"status": "active"
					}
				]
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Prepare input request
	req := map[string]any{
		"method": "service.query",
		"config": map[string]any{
			"apiToken": "test-token",
			"apiURL":   server.URL,
		},
		"payload": map[string]any{},
	}
	reqBytes, _ := json.Marshal(req)
	input := bytes.NewBuffer(reqBytes)

	// Capture output
	var output bytes.Buffer

	// Run plugin
	run(input, &output)

	// Verify output
	var resp struct {
		Result []map[string]any `json:"result"`
		Error  string           `json:"error"`
	}
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error != "" {
		t.Fatalf("Plugin returned error: %s", resp.Error)
	}

	if len(resp.Result) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(resp.Result))
	}

	if resp.Result[0]["id"] != "P12345" {
		t.Errorf("Expected service ID P12345, got %v", resp.Result[0]["id"])
	}
}

func TestRunInvalidConfig(t *testing.T) {
	req := map[string]any{
		"method": "service.query",
		"config": map[string]any{}, // Missing API token
	}
	reqBytes, _ := json.Marshal(req)
	input := bytes.NewBuffer(reqBytes)
	var output bytes.Buffer

	run(input, &output)

	var resp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(output.Bytes(), &resp)

	if resp.Error == "" {
		t.Error("Expected error for missing config, got success")
	}
}
