package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

// Z4: read_room(cluster_wide, include_last_n) must route through read_transcript
// and actually return the room's recent messages, not just metadata.
func TestHandleReadRoomClusterIncludeLastN(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": map[string]any{
				"room": map[string]any{
					"id":          "read-room-id",
					"description": "Room Description",
					"status":      "active",
					"created_at":  "2026-04-01T12:00:00",
					"updated_at":  "2026-04-01T12:00:00",
					"source_node": "test_node@10.0.0.1",
				},
				"messages": []map[string]any{
					{"id": "m1", "room_id": "read-room-id", "author": "Claude", "content": "first msg", "message_type": "message", "timestamp": "2026-04-01T12:00:01"},
					{"id": "m2", "room_id": "read-room-id", "author": "Gemini", "content": "second msg", "message_type": "decision", "timestamp": "2026-04-01T12:00:02"},
				},
				"pinned": nil,
			},
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		PhoenixURL: server.URL,
	}

	result, _, err := reg.handleReadRoomCluster(ReadRoomInput{RoomID: "read-room-id", ClusterWide: "true", IncludeLastN: "5"})
	if err != nil {
		t.Fatalf("handleReadRoomCluster failed: %v", err)
	}

	if gotPath != "/api/internal/cluster/read_transcript" {
		t.Errorf("expected read_transcript endpoint, got: %s", gotPath)
	}

	text := resultText(result)
	if !strings.Contains(text, "Recent messages (2)") {
		t.Errorf("expected recent messages header, got: %s", text)
	}
	if !strings.Contains(text, "first msg") || !strings.Contains(text, "second msg") {
		t.Errorf("expected message bodies, got: %s", text)
	}
	if !strings.Contains(text, "(decision)") {
		t.Errorf("expected message type label, got: %s", text)
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
