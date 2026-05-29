package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleSearchMessagesCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
					"source_node":  "council_hub@10.0.0.5",
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

	args := SearchMessagesInput{
		Query:       "hello",
		ClusterWide: "true",
	}

	result, output, err := reg.handleSearchMessagesCluster(args)
	if err != nil {
		t.Fatalf("handleSearchMessagesCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "1 message(s) across cluster") {
		t.Errorf("expected cluster header, got: %s", text)
	}
	if !strings.Contains(text, "council_hub@10.0.0.5") {
		t.Errorf("expected node tag in output, got: %s", text)
	}
	if output.Message == "" {
		t.Error("expected non-empty output message")
	}
}

func TestHandleSearchMessagesClusterFullContent(t *testing.T) {
	longContent := strings.Repeat("word ", 100) // 500 chars
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":           "full-content-id",
					"room_id":      "test-room",
					"author":       "Claude",
					"content":      longContent,
					"message_type": "message",
					"timestamp":    "2026-04-01T12:00:00",
					"source_node":  "node@host",
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

	// 1. Test without full_content (should truncate)
	resultTruncated, _, _ := reg.handleSearchMessagesCluster(SearchMessagesInput{
		Query:       "word",
		ClusterWide: "true",
	})
	textTrunc := resultText(resultTruncated)
	if !strings.Contains(textTrunc, "...") {
		t.Errorf("expected truncated excerpt when full_content is not true")
	}

	// 2. Test with full_content (should NOT truncate)
	resultFull, _, _ := reg.handleSearchMessagesCluster(SearchMessagesInput{
		Query:       "word",
		FullContent: "true",
		ClusterWide: "true",
	})
	textFull := resultText(resultFull)
	if strings.Contains(textFull, "...") {
		t.Errorf("expected full excerpt, got truncation indicator '...': %s", textFull)
	}
	if len(textFull) < len(longContent) {
		t.Errorf("expected full length text")
	}
}

func TestHandleSearchMessagesBranching(t *testing.T) {
	// Verify that cluster_wide=true routes to cluster handler
	// and default routes to local handler
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "branch-room")
	mustPost(t, reg.Server, "branch-room", "Claude", "Hello local")

	// Local path (default)
	args := SearchMessagesInput{RoomID: "branch-room"}
	result, _, err := reg.handleSearchMessages(nil, nil, args)
	if err != nil {
		t.Fatalf("local search failed: %v", err)
	}
	text := resultText(result)
	if !strings.Contains(text, "Hello local") {
		t.Errorf("expected local result, got: %s", text)
	}

	// Cluster path (no Phoenix running, should get error message)
	args.ClusterWide = "true"
	result, _, err = reg.handleSearchMessages(nil, nil, args)
	if err != nil {
		t.Fatalf("cluster search should not return error: %v", err)
	}
	text = resultText(result)
	if !strings.Contains(text, "Error: cluster search failed") {
		t.Errorf("expected cluster error, got: %s", text)
	}
}

func TestHandleSearchMessagesClusterSummaryOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":           "abc12345-1234-5678-9012-123456789abc",
					"room_id":      "test-room",
					"author":       "Claude",
					"content":      "A short message for summary",
					"message_type": "thought",
					"timestamp":    "2026-04-01T12:00:00",
					"source_node":  "council_hub@10.0.0.5",
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

	args := SearchMessagesInput{
		Query:       "test",
		SummaryOnly: "true",
		ClusterWide: "true",
	}

	result, _, err := reg.handleSearchMessagesCluster(args)
	if err != nil {
		t.Fatalf("handleSearchMessagesCluster failed: %v", err)
	}

	text := resultText(result)
	// Summary mode uses compact pipe-delimited format
	if !strings.Contains(text, "council_hub@10.0.0.5") {
		t.Errorf("expected node tag in summary, got: %s", text)
	}
	if !strings.Contains(text, "thought") {
		t.Errorf("expected message type in summary, got: %s", text)
	}
}

func TestHandleSearchMessagesClusterSummaryExcerptTruncation(t *testing.T) {
	longContent := strings.Repeat("word ", 50) // 250 chars
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":           "trunc-msg-id-1234-5678-9012-123456789abc",
					"room_id":      "test-room",
					"author":       "Claude",
					"content":      longContent,
					"message_type": "message",
					"timestamp":    "2026-04-01T12:00:00",
					"source_node":  "node@host",
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

	result, _, _ := reg.handleSearchMessagesCluster(SearchMessagesInput{
		Query:       "word",
		SummaryOnly: "true",
	})

	text := resultText(result)
	// Excerpt should be truncated to ~120 chars + "..."
	if !strings.Contains(text, "...") {
		t.Errorf("expected truncated excerpt, got: %s", text)
	}
}

func TestHandleSearchMessagesClusterEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results":  []map[string]any{},
			"warnings": []string{"node@dead unreachable"},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleSearchMessagesCluster(SearchMessagesInput{Query: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "No messages found") {
		t.Errorf("expected no-results message, got: %s", text)
	}
	if !strings.Contains(text, "node@dead unreachable") {
		t.Errorf("expected warning in empty results, got: %s", text)
	}
}

func TestHandleGetMessagesCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":           "msg-123",
					"room_id":      "test-room",
					"author":       "Claude",
					"content":      "Test get_messages cluster",
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

	result, _, err := reg.handleGetMessagesCluster(GetMessagesInput{MessageIDs: "msg-123", ClusterWide: "true"})
	if err != nil {
		t.Fatalf("handleGetMessagesCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "1 message(s) across cluster") {
		t.Errorf("expected cluster header, got: %s", text)
	}
	if !strings.Contains(text, "node@10.0.0.1") {
		t.Errorf("expected node tag, got: %s", text)
	}
	if !strings.Contains(text, "Test get_messages cluster") {
		t.Errorf("expected message content, got: %s", text)
	}
}

func TestHandleGetMessagesClusterEmpty(t *testing.T) {
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

	result, _, err := reg.handleGetMessagesCluster(GetMessagesInput{MessageIDs: "nonexistent", ClusterWide: "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "No messages found on any cluster node") {
		t.Errorf("expected no-results message, got: %s", text)
	}
}

// Z2: delta reads cluster-wide must forward after_id to the Phoenix API.
func TestHandleGetMessagesClusterForwardsAfterID(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
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

	_, _, err := reg.handleGetMessagesCluster(GetMessagesInput{RoomID: "r1", AfterID: "msg-cursor", ClusterWide: "true"})
	if err != nil {
		t.Fatalf("handleGetMessagesCluster failed: %v", err)
	}

	if gotBody["after_id"] != "msg-cursor" {
		t.Errorf("expected after_id forwarded to Phoenix, got body: %v", gotBody)
	}
}

// Z3: empty cluster search must explain that message bodies are node-local.
func TestHandleSearchMessagesClusterEmptyNodeLocalNote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results":  []map[string]any{},
			"warnings": []string{"node@dead unreachable"},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleSearchMessagesCluster(SearchMessagesInput{Query: "nope", ClusterWide: "true"})
	if err != nil {
		t.Fatalf("handleSearchMessagesCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "node-local") {
		t.Errorf("expected node-local explanation in empty result, got: %s", text)
	}
	if !strings.Contains(text, "**Cluster Warning:** node@dead unreachable") {
		t.Errorf("expected standardized Cluster Warning format, got: %s", text)
	}
}
