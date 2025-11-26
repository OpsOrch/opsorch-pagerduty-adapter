package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// LookupServiceIDsByName queries PagerDuty services by name and returns matching service IDs.
func LookupServiceIDsByName(ctx context.Context, client *http.Client, apiURL, apiToken, name string) ([]string, error) {
	params := url.Values{}
	params.Set("query", name)
	params.Set("limit", "100")

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL+"/services?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+apiToken)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pagerduty api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Services []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"services"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Filter to exact or fuzzy matches
	var ids []string
	lowerName := strings.ToLower(name)
	for _, svc := range result.Services {
		if strings.Contains(strings.ToLower(svc.Name), lowerName) {
			ids = append(ids, svc.ID)
		}
	}

	return ids, nil
}

// LookupTeamIDsByName queries PagerDuty teams by name and returns matching team IDs.
func LookupTeamIDsByName(ctx context.Context, client *http.Client, apiURL, apiToken, name string) ([]string, error) {
	params := url.Values{}
	params.Set("query", name)
	params.Set("limit", "100")

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL+"/teams?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+apiToken)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pagerduty api error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Teams []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"teams"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Filter to exact or fuzzy matches
	var ids []string
	lowerName := strings.ToLower(name)
	for _, team := range result.Teams {
		if strings.Contains(strings.ToLower(team.Name), lowerName) {
			ids = append(ids, team.ID)
		}
	}

	return ids, nil
}
