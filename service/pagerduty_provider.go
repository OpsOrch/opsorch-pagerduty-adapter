package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/opsorch/opsorch-core/schema"
	coreservice "github.com/opsorch/opsorch-core/service"
	"github.com/opsorch/opsorch-pagerduty-adapter/common"
)

// ProviderName is the registry key under which this adapter registers.
const ProviderName = "pagerduty"

// Config captures decrypted configuration from OpsOrch Core.
type Config struct {
	Source   string
	APIToken string
	APIURL   string
}

// PagerDutyProvider integrates with PagerDuty REST API v2 for services.
type PagerDutyProvider struct {
	cfg    Config
	client *http.Client
}

// New constructs the provider from decrypted config.
func New(cfg map[string]any) (coreservice.Provider, error) {
	parsed := parseConfig(cfg)
	if parsed.APIToken == "" {
		return nil, errors.New("pagerduty apiToken is required")
	}
	if parsed.APIURL == "" {
		return nil, errors.New("pagerduty apiURL is required")
	}
	return &PagerDutyProvider{
		cfg:    parsed,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func init() {
	_ = coreservice.RegisterProvider(ProviderName, New)
}

// Query searches for services in PagerDuty.
func (p *PagerDutyProvider) Query(ctx context.Context, q schema.ServiceQuery) ([]schema.Service, error) {
	params := url.Values{}

	if q.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", q.Limit))
	} else {
		params.Set("limit", "100")
	}

	if q.Name != "" {
		params.Set("query", q.Name)
	}

	// Translate Scope.Team to PagerDuty team IDs via lookup
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
		if v, ok := q.Metadata["team_id"].(string); ok && v != "" {
			params.Add("team_ids[]", v)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.cfg.APIURL+"/services?"+params.Encode(), nil)
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
		Services []pdService `json:"services"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	services := make([]schema.Service, len(result.Services))
	for i, pdSvc := range result.Services {
		services[i] = convertPDService(pdSvc, p.cfg.Source)
	}

	return services, nil
}

func parseConfig(cfg map[string]any) Config {
	out := Config{
		Source: "pagerduty",
		APIURL: "https://api.pagerduty.com",
	}
	if v, ok := cfg["source"].(string); ok && v != "" {
		out.Source = v
	}
	if v, ok := cfg["apiToken"].(string); ok {
		out.APIToken = strings.TrimSpace(v)
	}
	if v, ok := cfg["apiURL"].(string); ok && v != "" {
		out.APIURL = strings.TrimSpace(v)
	}
	return out
}

// pdService represents a PagerDuty service from the API.
type pdService struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Summary          string `json:"summary"`
	Description      string `json:"description"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
	HTMLURL          string `json:"html_url"`
	AlertCreation    string `json:"alert_creation"`
	EscalationPolicy struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Summary string `json:"summary"`
	} `json:"escalation_policy"`
	Teams []struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Summary string `json:"summary"`
	} `json:"teams"`
}

func convertPDService(pdSvc pdService, source string) schema.Service {
	svc := schema.Service{
		ID:   pdSvc.ID,
		Name: pdSvc.Name,
		Tags: map[string]string{},
		Metadata: map[string]any{
			"source":         source,
			"summary":        pdSvc.Summary,
			"description":    pdSvc.Description,
			"status":         pdSvc.Status,
			"html_url":       pdSvc.HTMLURL,
			"alert_creation": pdSvc.AlertCreation,
		},
	}

	if pdSvc.EscalationPolicy.ID != "" {
		svc.Metadata["escalation_policy"] = map[string]any{
			"id":      pdSvc.EscalationPolicy.ID,
			"summary": pdSvc.EscalationPolicy.Summary,
		}
	}

	if len(pdSvc.Teams) > 0 {
		teams := make([]map[string]any, len(pdSvc.Teams))
		for i, team := range pdSvc.Teams {
			teams[i] = map[string]any{
				"id":      team.ID,
				"summary": team.Summary,
			}
			// Also add team as tag
			svc.Tags[fmt.Sprintf("team_%d", i)] = team.Summary
		}
		svc.Metadata["teams"] = teams
	}

	return svc
}
