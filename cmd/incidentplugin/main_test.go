package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/opsorch/opsorch-core/schema"
)

type stubProvider struct{}

func (stubProvider) Query(ctx context.Context, query schema.IncidentQuery) ([]schema.Incident, error) {
	return nil, nil
}
func (stubProvider) Get(ctx context.Context, id string) (schema.Incident, error) {
	return schema.Incident{}, nil
}
func (stubProvider) Create(ctx context.Context, in schema.CreateIncidentInput) (schema.Incident, error) {
	return schema.Incident{}, nil
}
func (stubProvider) Update(ctx context.Context, id string, in schema.UpdateIncidentInput) (schema.Incident, error) {
	return schema.Incident{}, nil
}
func (stubProvider) GetTimeline(ctx context.Context, id string) ([]schema.TimelineEntry, error) {
	return nil, nil
}
func (stubProvider) AppendTimeline(ctx context.Context, id string, entry schema.TimelineAppendInput) error {
	return nil
}

func TestEnsureProviderReturnsExisting(t *testing.T) {
	t.Cleanup(func() { provider = nil })
	existing := stubProvider{}
	provider = existing

	got, err := ensureProvider(map[string]any{"source": "ignored"})
	if err != nil {
		t.Fatalf("ensureProvider returned error: %v", err)
	}
	if got != existing {
		t.Fatalf("expected existing provider to be reused")
	}
}

func TestEnsureProviderCachesNewInstance(t *testing.T) {
	t.Cleanup(func() { provider = nil })

	cfg := map[string]any{
		"source":    "demo",
		"apiToken":  "token",
		"apiURL":    "https://api.pagerduty.com",
		"serviceID": "PXXXXXX",
		"fromEmail": "user@example.com",
	}

	first, err := ensureProvider(cfg)
	if err != nil {
		t.Fatalf("ensureProvider returned error: %v", err)
	}
	second, err := ensureProvider(map[string]any{"source": "other"})
	if err != nil {
		t.Fatalf("ensureProvider returned error on second call: %v", err)
	}
	if first != second {
		t.Fatalf("expected provider instance to be cached between calls")
	}

	if first == nil {
		t.Fatalf("expected provider instance to be non-nil")
	}
}

func TestRun(t *testing.T) {
	// Setup mock provider
	t.Cleanup(func() { provider = nil })
	provider = stubProvider{}

	// Prepare input
	req := map[string]any{
		"method":  "incident.query",
		"config":  map[string]any{"source": "test"},
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
		Result any    `json:"result"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error != "" {
		t.Fatalf("Plugin returned error: %s", resp.Error)
	}

	// stubProvider returns nil for Query, so result should be null (or empty list depending on impl, here nil)
	if resp.Result != nil {
		t.Errorf("Expected nil result from stub, got %v", resp.Result)
	}
}
