package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTranscriptServer(t *testing.T, msgs []map[string]any, pinned map[string]any, warnings []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		result := map[string]any{
			"room": map[string]any{
				"id": "tr-room", "description": "Test", "status": "active",
			},
			"messages": msgs,
		}
		if pinned != nil {
			result["pinned"] = pinned
		}
		json.NewEncoder(w).Encode(map[string]any{"results": result, "warnings": warnings})
	}))
}

func TestHandleReadTranscriptClusterAfterID(t *testing.T) {
	server := newTranscriptServer(t, []map[string]any{
		{"id": "msg-1", "author": "Claude", "content": "First", "message_type": "message", "timestamp": "2026-04-01T12:00:00"},
		{"id": "msg-2", "author": "Gemini", "content": "Second", "message_type": "message", "timestamp": "2026-04-01T12:01:00"},
		{"id": "msg-3", "author": "Claude", "content": "Third", "message_type": "message", "timestamp": "2026-04-01T12:02:00"},
	}, nil, nil)
	defer server.Close()

	reg := &Registry{HTTPClient: &http.Client{Timeout: 5 * time.Second}, PhoenixURL: server.URL}
	result, _, err := reg.handleReadTranscriptCluster(ReadTranscriptInput{AfterID: "msg-1"}, "tr-room")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(result)
	if strings.Contains(text, "First") {
		t.Error("msg-1 should be excluded by after_id filter")
	}
	if !strings.Contains(text, "Second") || !strings.Contains(text, "Third") {
		t.Errorf("expected Second and Third in result, got: %s", text)
	}
}

func TestHandleReadTranscriptClusterChangelogMode(t *testing.T) {
	server := newTranscriptServer(t, []map[string]any{
		{"id": "m1", "author": "Claude", "content": "just a thought", "message_type": "thought", "timestamp": "2026-04-01T12:00:00"},
		{"id": "m2", "author": "Claude", "content": "we decided X", "message_type": "decision", "timestamp": "2026-04-01T12:01:00"},
		{"id": "m3", "author": "Gemini", "content": "deployed Y", "message_type": "action", "timestamp": "2026-04-01T12:02:00"},
	}, nil, nil)
	defer server.Close()

	reg := &Registry{HTTPClient: &http.Client{Timeout: 5 * time.Second}, PhoenixURL: server.URL}
	result, _, err := reg.handleReadTranscriptCluster(ReadTranscriptInput{Mode: "changelog"}, "tr-room")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(result)
	if strings.Contains(text, "just a thought") {
		t.Error("changelog mode should exclude thoughts")
	}
	if !strings.Contains(text, "we decided X") || !strings.Contains(text, "deployed Y") {
		t.Errorf("expected decisions and actions in changelog, got: %s", text)
	}
}

func TestHandleReadTranscriptClusterSummaryMode(t *testing.T) {
	server := newTranscriptServer(t, []map[string]any{
		{"id": "m1", "author": "Claude", "content": "old thought", "message_type": "thought", "timestamp": "2026-04-01T12:00:00"},
		{"id": "m2", "author": "Claude", "content": "new thought", "message_type": "thought", "timestamp": "2026-04-01T12:01:00"},
		{"id": "m3", "author": "Gemini", "content": "the decision", "message_type": "decision", "timestamp": "2026-04-01T12:02:00"},
	}, nil, nil)
	defer server.Close()

	reg := &Registry{HTTPClient: &http.Client{Timeout: 5 * time.Second}, PhoenixURL: server.URL}
	result, _, err := reg.handleReadTranscriptCluster(ReadTranscriptInput{Mode: "summary"}, "tr-room")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(result)
	if strings.Contains(text, "old thought") {
		t.Error("summary mode should deduplicate, only showing latest per type")
	}
	if !strings.Contains(text, "new thought") {
		t.Errorf("expected latest thought in summary mode, got: %s", text)
	}
	if !strings.Contains(text, "the decision") {
		t.Errorf("expected decision in summary mode, got: %s", text)
	}
}

func TestHandleReadTranscriptClusterLastN(t *testing.T) {
	server := newTranscriptServer(t, []map[string]any{
		{"id": "m1", "author": "Claude", "content": "first", "message_type": "message", "timestamp": "2026-04-01T12:00:00"},
		{"id": "m2", "author": "Claude", "content": "second", "message_type": "message", "timestamp": "2026-04-01T12:01:00"},
		{"id": "m3", "author": "Claude", "content": "third", "message_type": "message", "timestamp": "2026-04-01T12:02:00"},
	}, nil, nil)
	defer server.Close()

	reg := &Registry{HTTPClient: &http.Client{Timeout: 5 * time.Second}, PhoenixURL: server.URL}
	result, _, err := reg.handleReadTranscriptCluster(ReadTranscriptInput{LastN: "2"}, "tr-room")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(result)
	if strings.Contains(text, "first") {
		t.Error("last_n=2 should exclude first message")
	}
	if !strings.Contains(text, "second") || !strings.Contains(text, "third") {
		t.Errorf("expected last 2 messages, got: %s", text)
	}
}

func TestHandleReadTranscriptClusterAfterIDWithPinned(t *testing.T) {
	pinned := map[string]any{
		"id": "pinned-msg", "author": "Claude", "content": "pinned context",
		"message_type": "decision", "timestamp": "2026-04-01T11:00:00",
	}
	server := newTranscriptServer(t, []map[string]any{
		{"id": "msg-1", "author": "Claude", "content": "before", "message_type": "message", "timestamp": "2026-04-01T12:00:00"},
		{"id": "msg-2", "author": "Claude", "content": "after", "message_type": "message", "timestamp": "2026-04-01T12:01:00"},
	}, pinned, nil)
	defer server.Close()

	reg := &Registry{HTTPClient: &http.Client{Timeout: 5 * time.Second}, PhoenixURL: server.URL}
	result, _, err := reg.handleReadTranscriptCluster(ReadTranscriptInput{AfterID: "msg-1"}, "tr-room")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(result)
	if !strings.Contains(text, "pinned context") {
		t.Errorf("expected pinned message prepended for afterID delta read, got: %s", text)
	}
	if !strings.Contains(text, "after") {
		t.Errorf("expected message after afterID, got: %s", text)
	}
}

func TestHandleReadTranscriptClusterWarnings(t *testing.T) {
	server := newTranscriptServer(t, []map[string]any{
		{"id": "m1", "author": "Claude", "content": "msg", "message_type": "message", "timestamp": "2026-04-01T12:00:00"},
	}, nil, []string{"node@dead unreachable"})
	defer server.Close()

	reg := &Registry{HTTPClient: &http.Client{Timeout: 5 * time.Second}, PhoenixURL: server.URL}
	result, _, err := reg.handleReadTranscriptCluster(ReadTranscriptInput{}, "tr-room")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(result)
	if !strings.Contains(text, "node@dead unreachable") {
		t.Errorf("expected cluster warning in output, got: %s", text)
	}
}

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

func TestHandleListRoomsCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":          "cluster-room",
					"description": "A cluster room",
					"status":      "active",
					"project":     "proj",
					"updated_at":  "2026-04-01T12:00:00",
					"source_node": "council_hub@10.0.0.5",
				},
			},
			"warnings": []string{"council_hub@10.0.0.7 unreachable"},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleListRoomsCluster(ListRoomsInput{Project: "proj"})
	if err != nil {
		t.Fatalf("handleListRoomsCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "1 room(s) across cluster") {
		t.Errorf("expected cluster header, got: %s", text)
	}
	if !strings.Contains(text, "council_hub@10.0.0.5") {
		t.Errorf("expected node tag, got: %s", text)
	}
	if !strings.Contains(text, "council_hub@10.0.0.7 unreachable") {
		t.Errorf("expected warning, got: %s", text)
	}
}

func TestHandleRoomStatsCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": map[string]any{
				"room_id":           "stats-room",
				"status":            "active",
				"message_count":     42,
				"participants":      map[string]int{"Claude": 30, "Gemini": 12},
				"type_counts":       map[string]int{"message": 35, "decision": 7},
				"first_message":     "2026-03-01T10:00:00",
				"last_message":      "2026-04-01T12:00:00",
				"latest_message_id": "abc12345",
				"source_node":       "council_hub@10.0.0.5",
			},
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleRoomStatsCluster(RoomStatsInput{RoomID: "stats-room"})
	if err != nil {
		t.Fatalf("handleRoomStatsCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "council_hub@10.0.0.5") {
		t.Errorf("expected node tag, got: %s", text)
	}
	if !strings.Contains(text, "42") {
		t.Errorf("expected message count, got: %s", text)
	}
}

func TestHandleRoomStatsClusterNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results":  nil,
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleRoomStatsCluster(RoomStatsInput{RoomID: "nonexistent"})
	if err != nil {
		t.Fatalf("handleRoomStatsCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "not found on any cluster node") {
		t.Errorf("expected not found message, got: %s", text)
	}
}

func TestHandleReadRoomCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":            "read-room-id",
					"description":   "Room Description",
					"status":        "active",
					"project":       "Test Project",
					"tech_stack":    "Go",
					"tags":          "test",
					"system_prompt": "You are testing.",
					"related_rooms": "other-room",
					"created_at":    "2026-04-01T12:00:00",
					"updated_at":    "2026-04-01T12:00:00",
					"source_node":   "test_node@10.0.0.1",
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

	result, _, err := reg.handleReadRoomCluster(ReadRoomInput{RoomID: "read-room-id", ClusterWide: "true"})
	if err != nil {
		t.Fatalf("handleReadRoomCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "read-room-id") {
		t.Errorf("expected room ID, got: %s", text)
	}
	if !strings.Contains(text, "Room Description") {
		t.Errorf("expected description, got: %s", text)
	}
	if !strings.Contains(text, "test_node@10.0.0.1") {
		t.Errorf("expected node tag, got: %s", text)
	}
}

func TestHandleReadRoomClusterNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results":  []map[string]any{},
			"warnings": []string{"node unreachable"},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleReadRoomCluster(ReadRoomInput{RoomID: "nonexistent", ClusterWide: "true"})
	if err != nil {
		t.Fatalf("handleReadRoomCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "not found on any cluster node") {
		t.Errorf("expected not found message, got: %s", text)
	}
	if !strings.Contains(text, "node unreachable") {
		t.Errorf("expected warning, got: %s", text)
	}
}

func TestHandleReadTranscriptCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": map[string]any{
				"room": map[string]any{
					"id":          "transcript-room",
					"description": "Topic details",
					"status":      "active",
					"project":     "proj",
				},
				"messages": []map[string]any{
					{
						"id":           "msg-1",
						"author":       "Claude",
						"content":      "First message",
						"message_type": "message",
						"timestamp":    "2026-04-01T12:00:00",
					},
					{
						"id":           "msg-2",
						"author":       "Gemini",
						"content":      "Second message",
						"message_type": "decision",
						"timestamp":    "2026-04-01T12:05:00",
					},
				},
				"pinned": map[string]any{
					"id":           "msg-2",
					"author":       "Gemini",
					"content":      "Second message",
					"message_type": "decision",
					"timestamp":    "2026-04-01T12:05:00",
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

	result, _, err := reg.handleReadTranscriptCluster(ReadTranscriptInput{RoomID: "transcript-room", ClusterWide: "true"}, "transcript-room")
	if err != nil {
		t.Fatalf("handleReadTranscriptCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "# COUNCIL ROOM: transcript-room") {
		t.Errorf("expected room header, got: %s", text)
	}
	if !strings.Contains(text, "First message") || !strings.Contains(text, "Second message") {
		t.Errorf("expected messages in transcript, got: %s", text)
	}
}

func TestHandleReadTranscriptClusterNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results":  nil,
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleReadTranscriptCluster(ReadTranscriptInput{RoomID: "nonexistent"}, "nonexistent")
	if err != nil {
		t.Fatalf("handleReadTranscriptCluster failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "not found on any cluster node") {
		t.Errorf("expected not found message, got: %s", text)
	}
}

func TestHandleListRoomsBranching(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "branch-list-room", withProject("branch-proj"))

	// Local path (default)
	args := ListRoomsInput{Project: "branch-proj"}
	result, _, err := reg.handleListRooms(nil, nil, args)
	if err != nil {
		t.Fatalf("local list rooms failed: %v", err)
	}
	text := resultText(result)
	if !strings.Contains(text, "branch-list-room") {
		t.Errorf("expected local result, got: %s", text)
	}

	// Cluster path (no Phoenix running, should get error message)
	args.ClusterWide = "true"
	result, _, err = reg.handleListRooms(nil, nil, args)
	if err != nil {
		t.Fatalf("cluster list rooms should not return error: %v", err)
	}
	text = resultText(result)
	if !strings.Contains(text, "Error: cluster list rooms failed") {
		t.Errorf("expected cluster error, got: %s", text)
	}
}

func TestHandleRoomStatsBranching(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "branch-stats-room")
	mustPost(t, reg.Server, "branch-stats-room", "Claude", "Hello")

	// Local path (default)
	args := RoomStatsInput{RoomID: "branch-stats-room"}
	result, _, err := reg.handleRoomStats(nil, nil, args)
	if err != nil {
		t.Fatalf("local room stats failed: %v", err)
	}
	text := resultText(result)
	if !strings.Contains(text, "branch-stats-room") {
		t.Errorf("expected local result, got: %s", text)
	}

	// Cluster path (no Phoenix running, should get error message)
	args.ClusterWide = "true"
	result, _, err = reg.handleRoomStats(nil, nil, args)
	if err != nil {
		t.Fatalf("cluster room stats should not return error: %v", err)
	}
	text = resultText(result)
	if !strings.Contains(text, "Error: cluster room stats failed") {
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

func TestHandleListRoomsClusterVerbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":            "verbose-room",
					"description":   "A verbose room description",
					"status":        "active",
					"project":       "proj",
					"tech_stack":    "Elixir, Go",
					"tags":          "distributed,erlang",
					"related_rooms": "other-room",
					"updated_at":    "2026-04-01T12:00:00",
					"source_node":   "council_hub@10.0.0.5",
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

	result, _, err := reg.handleListRoomsCluster(ListRoomsInput{Verbose: "true"})
	if err != nil {
		t.Fatalf("verbose list rooms failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "Tech: Elixir, Go") {
		t.Errorf("expected tech stack in verbose output, got: %s", text)
	}
	if !strings.Contains(text, "tags: distributed,erlang") {
		t.Errorf("expected tags in verbose output, got: %s", text)
	}
	if !strings.Contains(text, "Related: other-room") {
		t.Errorf("expected related rooms in verbose output, got: %s", text)
	}
	if !strings.Contains(text, "council_hub@10.0.0.5") {
		t.Errorf("expected node tag in verbose output, got: %s", text)
	}
}

func TestHandleListRoomsClusterCompact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":          "compact-room",
					"description": "Short topic",
					"status":      "active",
					"project":     "proj",
					"updated_at":  "2026-04-01T12:00:00",
					"source_node": "node@host",
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

	// Default (compact) mode
	result, _, err := reg.handleListRoomsCluster(ListRoomsInput{})
	if err != nil {
		t.Fatalf("compact list rooms failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "compact-room") {
		t.Errorf("expected room id, got: %s", text)
	}
	// Compact mode should NOT show tech stack or related rooms
	if strings.Contains(text, "Tech:") {
		t.Errorf("compact mode should not show tech stack, got: %s", text)
	}
}

func TestHandleRoomStatsClusterFullFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": map[string]any{
				"room_id":           "full-stats",
				"status":            "resolved",
				"message_count":     100,
				"participants":      map[string]int{"Alice": 60, "Bob": 40},
				"type_counts":       map[string]int{"message": 70, "decision": 20, "action": 10},
				"first_message":     "2026-01-01T10:00:00",
				"last_message":      "2026-04-01T15:30:00",
				"latest_message_id": "deadbeef",
				"source_node":       "council_hub@10.0.0.5",
			},
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleRoomStatsCluster(RoomStatsInput{RoomID: "full-stats"})
	if err != nil {
		t.Fatalf("full stats failed: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "resolved") {
		t.Errorf("expected status, got: %s", text)
	}
	if !strings.Contains(text, "100") {
		t.Errorf("expected message count, got: %s", text)
	}
	if !strings.Contains(text, "deadbeef") {
		t.Errorf("expected latest message id, got: %s", text)
	}
	if !strings.Contains(text, "Alice") || !strings.Contains(text, "Bob") {
		t.Errorf("expected participants, got: %s", text)
	}
	if !strings.Contains(text, "decision") {
		t.Errorf("expected type counts, got: %s", text)
	}
	if !strings.Contains(text, "2026-01-01") {
		t.Errorf("expected first message timestamp, got: %s", text)
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

func TestHandleListRoomsClusterEmpty(t *testing.T) {
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

	result, _, err := reg.handleListRoomsCluster(ListRoomsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(result)
	if !strings.Contains(text, "No rooms found") {
		t.Errorf("expected no-rooms message, got: %s", text)
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
