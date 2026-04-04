package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClusterCallSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/internal/cluster/search_messages") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":           "abc12345",
					"room_id":      "test-room",
					"author":       "Claude",
					"content":      "Hello cluster",
					"message_type": "message",
					"timestamp":    "2026-04-01T12:00:00",
					"source_node":  "node@10.0.0.1",
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

	raw, warnings, err := reg.clusterCall("search_messages", map[string]any{"query": "hello"})
	if err != nil {
		t.Fatalf("clusterCall failed: %v", err)
	}

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	var results []ClusterSearchResult
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Author != "Claude" {
		t.Errorf("expected author Claude, got %s", results[0].Author)
	}
	if results[0].SourceNode != "node@10.0.0.1" {
		t.Errorf("expected source_node node@10.0.0.1, got %s", results[0].SourceNode)
	}
}

func TestClusterCallWithWarnings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results":  []map[string]any{},
			"warnings": []string{"node@10.0.0.2 unreachable"},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	_, warnings, err := reg.clusterCall("list_rooms", map[string]any{})
	if err != nil {
		t.Fatalf("clusterCall failed: %v", err)
	}

	if len(warnings) != 1 || warnings[0] != "node@10.0.0.2 unreachable" {
		t.Errorf("expected warning about unreachable node, got %v", warnings)
	}
}

func TestClusterCallHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	_, _, err := reg.clusterCall("search_messages", map[string]any{})
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "returned 500") {
		t.Errorf("expected 500 in error, got: %s", err.Error())
	}
}

func TestClusterCallNoConfig(t *testing.T) {
	reg := &Registry{}
	_, _, err := reg.clusterCall("any", nil)
	if err == nil {
		t.Fatal("expected error when not configured")
	}
}

func TestClusterCallConnectionRefused(t *testing.T) {
	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
		PhoenixURL: "http://127.0.0.1:59999", // nothing listening
	}

	_, _, err := reg.clusterCall("search_messages", map[string]any{"query": "test"})
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	if !strings.Contains(err.Error(), "cluster call") {
		t.Errorf("expected 'cluster call' in error, got: %s", err.Error())
	}
}

func TestClusterCallMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	_, _, err := reg.clusterCall("search_messages", map[string]any{})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected 'decode' in error, got: %s", err.Error())
	}
}

func TestFormatClusterWarnings(t *testing.T) {
	var b strings.Builder
	b.WriteString("Some results")

	formatClusterWarnings(&b, []string{"node-a timed out", "node-b unreachable"})

	text := b.String()
	if !strings.Contains(text, "node-a timed out") {
		t.Errorf("expected warning for node-a, got: %s", text)
	}
	if !strings.Contains(text, "node-b unreachable") {
		t.Errorf("expected warning for node-b, got: %s", text)
	}

	// No warnings should not add separator
	var b2 strings.Builder
	b2.WriteString("Clean results")
	formatClusterWarnings(&b2, nil)
	if strings.Contains(b2.String(), "---") {
		t.Error("should not add separator when no warnings")
	}
}

func TestClusterCallTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 50 * time.Millisecond},
		PhoenixURL: server.URL,
	}

	_, _, err := reg.clusterCall("search_messages", map[string]any{"query": "test"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "cluster call") {
		t.Errorf("expected 'cluster call' in error, got: %s", err.Error())
	}
}
