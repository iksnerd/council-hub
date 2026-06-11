package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleReadNotebookCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			// Deliberately out of chronological order — the handler must
			// re-sort by UUIDv7 ID before rendering.
			"results": []map[string]any{
				{
					"id":           "019eb700-0000-7000-8000-000000000002",
					"room_id":      "nb-remote",
					"author":       "gemini",
					"content":      "remote action {sha:abc1234}",
					"message_type": "action",
					"timestamp":    "2026-06-10T09:30:00",
					"source_node":  "council_hub@10.0.0.5",
					"repo":         "alice/widgets",
				},
				{
					"id":           "019eb700-0000-7000-8000-000000000001",
					"room_id":      "nb-local",
					"author":       "claude",
					"content":      "local decision",
					"message_type": "decision",
					"timestamp":    "2026-06-09T11:00:00",
					"source_node":  "council_hub@10.0.0.4",
					"repo":         "",
				},
			},
			"warnings": []string{"bob@10.0.0.6: timeout"},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleReadNotebookCluster(ReadNotebookInput{Project: "nb-proj", ClusterWide: "true"})
	if err != nil {
		t.Fatalf("handleReadNotebookCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "# Notebook — nb-proj (cluster-wide)") {
		t.Errorf("missing cluster notebook header, got: %s", text)
	}
	// Re-sorted: the 06-09 decision renders before the 06-10 action
	decisionPos := strings.Index(text, "local decision")
	actionPos := strings.Index(text, "remote action")
	if decisionPos == -1 || actionPos == -1 || decisionPos > actionPos {
		t.Errorf("entries not in chronological order, got: %s", text)
	}
	// Day headers from both dates
	if !strings.Contains(text, "## 2026-06-09") || !strings.Contains(text, "## 2026-06-10") {
		t.Errorf("missing day headers, got: %s", text)
	}
	// Node tags and per-entry repo resolution
	if !strings.Contains(text, "[council_hub@10.0.0.5]") {
		t.Errorf("missing source node tag, got: %s", text)
	}
	if !strings.Contains(text, "https://github.com/alice/widgets/commit/abc1234") {
		t.Errorf("commit ref not resolved against entry repo, got: %s", text)
	}
	if !strings.Contains(text, "Cluster Warning:") {
		t.Errorf("missing cluster warning, got: %s", text)
	}
	if !strings.Contains(text, `"latest_message_id":"019eb700-0000-7000-8000-000000000002"`) {
		t.Errorf("missing cursor footer, got: %s", text)
	}
}

func TestHandleReadNotebookClusterEmpty(t *testing.T) {
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

	result, _, err := reg.handleReadNotebookCluster(ReadNotebookInput{Project: "nb-proj"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resultText(result), "No notebook entries for project 'nb-proj' on any cluster node.") {
		t.Errorf("expected empty cluster message, got: %s", resultText(result))
	}
}
