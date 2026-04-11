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
