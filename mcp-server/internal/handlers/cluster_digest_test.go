package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleGetDigestCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"room_id":                "digest-room",
					"new_message_count":      5,
					"latest_message_excerpt": "A new important update",
					"source_node":            "node@10.0.0.1",
				},
			},
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleGetDigestCluster(DigestInput{Project: "proj", Since: "2026-04-01T12:00:00", ClusterWide: "true"})
	if err != nil {
		t.Fatalf("handleGetDigestCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "digest-room") {
		t.Errorf("expected room ID, got: %s", text)
	}
	if !strings.Contains(text, "node@10.0.0.1") {
		t.Errorf("expected node tag, got: %s", text)
	}
}

func TestHandleGetDigestClusterEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results":  []map[string]any{},
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleGetDigestCluster(DigestInput{Since: "2026-04-01T12:00:00", ClusterWide: "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "\"results\": []") {
		t.Errorf("expected empty JSON results, got: %s", text)
	}
}
