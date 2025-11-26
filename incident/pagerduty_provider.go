package incident

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	coreincident "github.com/opsorch/opsorch-core/incident"
	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-pagerduty-adapter/common"
)

// ProviderName is the registry key under which this adapter registers.
const ProviderName = "pagerduty"

// AdapterVersion and RequiresCore express compatibility.
const (
	AdapterVersion = "0.1.0"
	RequiresCore   = ">=0.1.0"
)

var errNotFound = errors.New("incident not found")

// Config captures decrypted configuration from OpsOrch Core.
type Config struct {
	Source          string
	DefaultSeverity string
	APIToken        string
	APIURL          string
	ServiceID       string // PagerDuty service ID for creating incidents
	FromEmail       string // Email address of a valid PagerDuty user
}

// PagerDutyProvider integrates with PagerDuty REST API v2.
type PagerDutyProvider struct {
	cfg    Config
	client *http.Client
}

// New constructs the provider from decrypted config.
func New(cfg map[string]any) (coreincident.Provider, error) {
	parsed := parseConfig(cfg)
	if parsed.APIToken == "" {
		return nil, errors.New("pagerduty apiToken is required")
	}
	if parsed.APIURL == "" {
		return nil, errors.New("pagerduty apiURL is required")
	}
	if parsed.ServiceID == "" {
		return nil, errors.New("pagerduty serviceID is required")
	}
	if parsed.FromEmail == "" {
		return nil, errors.New("pagerduty fromEmail is required")
	}
	return &PagerDutyProvider{
		cfg:    parsed,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func init() {
	_ = coreincident.RegisterProvider(ProviderName, New)
}

// Get returns a single incident by ID from PagerDuty.
func (p *PagerDutyProvider) Get(ctx context.Context, id string) (schema.Incident, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.APIURL+"/incidents/"+id, nil)
	if err != nil {
		return schema.Incident{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+p.cfg.APIToken)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")

	resp, err := p.client.Do(req)
	if err != nil {
		return schema.Incident{}, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return schema.Incident{}, errNotFound
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return schema.Incident{}, fmt.Errorf("pagerduty api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Incident pdIncident `json:"incident"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return schema.Incident{}, fmt.Errorf("decode response: %w", err)
	}

	return convertPDIncident(result.Incident, p.cfg.Source), nil
}

// Create creates a new incident in PagerDuty.
func (p *PagerDutyProvider) Create(ctx context.Context, in schema.CreateIncidentInput) (schema.Incident, error) {
	payload := map[string]any{
		"incident": map[string]any{
			"type":  "incident",
			"title": in.Title,
			"service": map[string]string{
				"id":   p.cfg.ServiceID,
				"type": "service_reference",
			},
			"urgency": mapSeverityToUrgency(defaultString(in.Severity, p.cfg.DefaultSeverity)),
		},
	}

	// Add body if provided
	if in.Fields != nil {
		if body, ok := in.Fields["body"]; ok {
			payload["incident"].(map[string]any)["body"] = map[string]any{
				"type":    "incident_body",
				"details": body,
			}
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return schema.Incident{}, fmt.Errorf("marshal create payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.cfg.APIURL+"/incidents", bytes.NewReader(body))
	if err != nil {
		return schema.Incident{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+p.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")
	req.Header.Set("From", p.cfg.FromEmail)

	resp, err := p.client.Do(req)
	if err != nil {
		return schema.Incident{}, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return schema.Incident{}, fmt.Errorf("pagerduty api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Incident pdIncident `json:"incident"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return schema.Incident{}, fmt.Errorf("decode response: %w", err)
	}

	return convertPDIncident(result.Incident, p.cfg.Source), nil
}

// Update modifies an incident in PagerDuty.
func (p *PagerDutyProvider) Update(ctx context.Context, id string, in schema.UpdateIncidentInput) (schema.Incident, error) {
	payload := map[string]any{
		"incident": map[string]any{
			"type": "incident",
		},
	}

	if in.Title != nil {
		payload["incident"].(map[string]any)["title"] = *in.Title
	}

	if in.Status != nil {
		payload["incident"].(map[string]any)["status"] = mapStatusToPD(*in.Status)
	}

	if in.Severity != nil {
		payload["incident"].(map[string]any)["urgency"] = mapSeverityToUrgency(*in.Severity)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return schema.Incident{}, fmt.Errorf("marshal update payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", p.cfg.APIURL+"/incidents/"+id, bytes.NewReader(body))
	if err != nil {
		return schema.Incident{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+p.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")
	req.Header.Set("From", p.cfg.FromEmail)

	resp, err := p.client.Do(req)
	if err != nil {
		return schema.Incident{}, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return schema.Incident{}, errNotFound
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return schema.Incident{}, fmt.Errorf("pagerduty api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Incident pdIncident `json:"incident"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return schema.Incident{}, fmt.Errorf("decode response: %w", err)
	}

	return convertPDIncident(result.Incident, p.cfg.Source), nil
}

// Query searches for incidents in PagerDuty.
func (p *PagerDutyProvider) Query(ctx context.Context, q schema.IncidentQuery) ([]schema.Incident, error) {
	params := url.Values{}

	if q.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", q.Limit))
	} else {
		params.Set("limit", "100")
	}

	if len(q.Statuses) > 0 {
		for _, status := range q.Statuses {
			params.Add("statuses[]", mapStatusToPD(status))
		}
	}

	if len(q.Severities) > 0 {
		for _, severity := range q.Severities {
			params.Add("urgencies[]", mapSeverityToUrgency(severity))
		}
	}

	// Translate Scope fields to PagerDuty IDs via lookups
	if q.Scope.Service != "" {
		serviceIDs, err := common.LookupServiceIDsByName(ctx, p.client, p.cfg.APIURL, p.cfg.APIToken, q.Scope.Service)
		if err != nil {
			return nil, fmt.Errorf("lookup service by name %q: %w", q.Scope.Service, err)
		}
		for _, id := range serviceIDs {
			params.Add("service_ids[]", id)
		}
	}

	if q.Scope.Team != "" {
		teamIDs, err := common.LookupTeamIDsByName(ctx, p.client, p.cfg.APIURL, p.cfg.APIToken, q.Scope.Team)
		if err != nil {
			return nil, fmt.Errorf("lookup team by name %q: %w", q.Scope.Team, err)
		}
		for _, id := range teamIDs {
			params.Add("team_ids[]", id)
		}
	}

	// Map known metadata fields to API filters
	if len(q.Metadata) > 0 {
		if v, ok := q.Metadata["service_id"].(string); ok && v != "" {
			params.Add("service_ids[]", v)
		}
		if v, ok := q.Metadata["team_id"].(string); ok && v != "" {
			params.Add("team_ids[]", v)
		}
		if v, ok := q.Metadata["incident_key"].(string); ok && v != "" {
			params.Set("incident_key", v)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.APIURL+"/incidents?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+p.cfg.APIToken)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pagerduty api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Incidents []pdIncident `json:"incidents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	incidents := make([]schema.Incident, len(result.Incidents))
	for i, pdInc := range result.Incidents {
		incidents[i] = convertPDIncident(pdInc, p.cfg.Source)
	}

	return incidents, nil
}

// GetTimeline returns the log entries (timeline) for an incident from PagerDuty.
func (p *PagerDutyProvider) GetTimeline(ctx context.Context, id string) ([]schema.TimelineEntry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.APIURL+"/incidents/"+id+"/log_entries", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+p.cfg.APIToken)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pagerduty api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		LogEntries []pdLogEntry `json:"log_entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	entries := make([]schema.TimelineEntry, len(result.LogEntries))
	for i, le := range result.LogEntries {
		entries[i] = convertPDLogEntry(le, id)
	}

	return entries, nil
}

// AppendTimeline adds a note to an incident in PagerDuty.
func (p *PagerDutyProvider) AppendTimeline(ctx context.Context, id string, entry schema.TimelineAppendInput) error {
	payload := map[string]any{
		"note": map[string]any{
			"content": entry.Body,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal note payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.cfg.APIURL+"/incidents/"+id+"/notes", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+p.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")
	req.Header.Set("From", p.cfg.FromEmail)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pagerduty api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func parseConfig(cfg map[string]any) Config {
	out := Config{
		Source:          "pagerduty",
		DefaultSeverity: "critical",
		APIURL:          "https://api.pagerduty.com",
	}
	if v, ok := cfg["source"].(string); ok && v != "" {
		out.Source = v
	}
	if v, ok := cfg["defaultSeverity"].(string); ok && v != "" {
		out.DefaultSeverity = v
	}
	if v, ok := cfg["apiToken"].(string); ok {
		out.APIToken = strings.TrimSpace(v)
	}
	if v, ok := cfg["apiURL"].(string); ok && v != "" {
		out.APIURL = strings.TrimSpace(v)
	}
	if v, ok := cfg["serviceID"].(string); ok {
		out.ServiceID = strings.TrimSpace(v)
	}
	if v, ok := cfg["fromEmail"].(string); ok {
		out.FromEmail = strings.TrimSpace(v)
	}
	return out
}

// pdIncident represents a PagerDuty incident from the API.
type pdIncident struct {
	ID          string `json:"id"`
	IncidentKey string `json:"incident_key"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Urgency     string `json:"urgency"`
	HTMLURL     string `json:"html_url"`
	Service     struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
		HTMLURL string `json:"html_url"`
	} `json:"service"`
	Assignments []struct {
		Assignee struct {
			ID      string `json:"id"`
			Summary string `json:"summary"`
			HTMLURL string `json:"html_url"`
		} `json:"assignee"`
	} `json:"assignments"`
	LastStatusChangeAt string `json:"last_status_change_at"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// pdLogEntry represents a PagerDuty log entry.
type pdLogEntry struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
	Agent     *struct {
		Summary string `json:"summary"`
	} `json:"agent"`
}

func convertPDIncident(pdInc pdIncident, source string) schema.Incident {
	inc := schema.Incident{
		ID:       pdInc.ID,
		Title:    pdInc.Title,
		Status:   mapPDStatusToOpsOrch(pdInc.Status),
		Severity: mapUrgencyToSeverity(pdInc.Urgency),
		Service:  pdInc.Service.Summary,
		Metadata: map[string]any{
			"source":                source,
			"incident_key":          pdInc.IncidentKey,
			"service_id":            pdInc.Service.ID,
			"service_url":           pdInc.Service.HTMLURL,
			"html_url":              pdInc.HTMLURL,
			"last_status_change_at": pdInc.LastStatusChangeAt,
		},
	}

	if len(pdInc.Assignments) > 0 {
		assignees := make([]map[string]string, len(pdInc.Assignments))
		for i, assignment := range pdInc.Assignments {
			assignees[i] = map[string]string{
				"id":       assignment.Assignee.ID,
				"name":     assignment.Assignee.Summary,
				"html_url": assignment.Assignee.HTMLURL,
			}
		}
		inc.Metadata["assignments"] = assignees
	}

	if createdAt, err := time.Parse(time.RFC3339, pdInc.CreatedAt); err == nil {
		inc.CreatedAt = createdAt
	}
	if updatedAt, err := time.Parse(time.RFC3339, pdInc.UpdatedAt); err == nil {
		inc.UpdatedAt = updatedAt
	}

	return inc
}

func convertPDLogEntry(le pdLogEntry, incidentID string) schema.TimelineEntry {
	entry := schema.TimelineEntry{
		ID:         le.ID,
		IncidentID: incidentID,
		Kind:       le.Type,
		Body:       le.Summary,
		Metadata: map[string]any{
			"type": le.Type,
		},
	}

	if at, err := time.Parse(time.RFC3339, le.CreatedAt); err == nil {
		entry.At = at
	}

	if le.Agent != nil {
		entry.Actor = map[string]any{
			"name": le.Agent.Summary,
		}
	}

	return entry
}

func defaultString(val string, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}

// mapSeverityToUrgency maps OpsOrch severity to PagerDuty urgency.
func mapSeverityToUrgency(severity string) string {
	switch strings.ToLower(severity) {
	case "critical", "sev1", "p1":
		return "high"
	case "high", "sev2", "p2":
		return "high"
	case "medium", "sev3", "p3":
		return "low"
	case "low", "sev4", "p4":
		return "low"
	default:
		return "high"
	}
}

// mapUrgencyToSeverity maps PagerDuty urgency to OpsOrch severity.
func mapUrgencyToSeverity(urgency string) string {
	switch strings.ToLower(urgency) {
	case "high":
		return "critical"
	case "low":
		return "medium"
	default:
		return "medium"
	}
}

// mapStatusToPD maps OpsOrch status to PagerDuty status.
func mapStatusToPD(status string) string {
	switch strings.ToLower(status) {
	case "open", "triggered":
		return "triggered"
	case "acknowledged", "investigating":
		return "acknowledged"
	case "resolved", "closed":
		return "resolved"
	default:
		return status
	}
}

// mapPDStatusToOpsOrch maps PagerDuty status to OpsOrch status.
func mapPDStatusToOpsOrch(status string) string {
	switch strings.ToLower(status) {
	case "triggered":
		return "open"
	case "acknowledged":
		return "acknowledged"
	case "resolved":
		return "resolved"
	default:
		return status
	}
}
