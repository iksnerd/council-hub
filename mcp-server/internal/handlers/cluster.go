package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// clusterCall makes an HTTP POST to the Phoenix internal cluster API.
func (r *Registry) clusterCall(endpoint string, params map[string]any) (json.RawMessage, []string, error) {
	if r.HTTPClient == nil || r.PhoenixURL == "" {
		return nil, nil, fmt.Errorf("cluster queries not configured (no Phoenix URL)")
	}

	body, err := json.Marshal(params)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal params: %w", err)
	}

	url := r.PhoenixURL + "/api/internal/cluster/" + endpoint
	resp, err := r.HTTPClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("cluster call to %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("cluster %s returned %d: %s", endpoint, resp.StatusCode, string(msg))
	}

	var raw struct {
		Results  json.RawMessage `json:"results"`
		Warnings []string        `json:"warnings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("decode cluster response: %w", err)
	}

	return raw.Results, raw.Warnings, nil
}
