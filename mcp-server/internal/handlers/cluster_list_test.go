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

func TestHandleListRoomsClusterForwardsAllFilters(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
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

	_, _, err := reg.handleListRoomsCluster(ListRoomsInput{
		Project:      "keep",
		ProjectNotIn: "old,graveyard",
		RelatedTo:    "root-room",
		Tag:          "meta",
		Status:       "active",
		Search:       "council",
		Limit:        "25",
		Offset:       "5",
	})
	if err != nil {
		t.Fatalf("handleListRoomsCluster failed: %v", err)
	}

	for key, want := range map[string]string{
		"project":        "keep",
		"project_not_in": "old,graveyard",
		"related_to":     "root-room",
		"tag":            "meta",
		"status":         "active",
		"search":         "council",
		"limit":          "30", // limit+offset: the page is cut after the merge
		"offset":         "0",
	} {
		if got[key] != want {
			t.Fatalf("expected %s=%q, got %#v in %#v", key, want, got[key], got)
		}
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

	// Cluster path includes local results even when Phoenix fan-out is unavailable.
	args.ClusterWide = "true"
	result, _, err = reg.handleListRooms(nil, nil, args)
	if err != nil {
		t.Fatalf("cluster list rooms should not return error: %v", err)
	}
	text = resultText(result)
	if !strings.Contains(text, "branch-list-room") || !strings.Contains(text, "peer fan-out unavailable") {
		t.Errorf("expected local result plus peer warning, got: %s", text)
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

func TestHandleListRoomsClusterReportsOmittedResolved(t *testing.T) {
	var refsParams map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/room_refs") {
			_ = json.NewDecoder(r.Body).Decode(&refsParams)
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "old-a", "status": "resolved", "source_node": "a@1"},
					{"id": "old-b", "status": "resolved", "source_node": "b@1"},
					{"id": "old-a", "status": "resolved", "source_node": "b@1"},
				},
				"warnings": []string{},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": "live", "description": "d", "status": "active", "project": "p", "updated_at": "2026-04-01T12:00:00", "source_node": "a@1"},
				{"id": "old-a", "description": "d", "status": "resolved", "project": "p", "updated_at": "2026-04-01T11:00:00", "source_node": "a@1"},
			},
			"warnings": []string{},
		})
	}))
	defer server.Close()

	reg := &Registry{HTTPClient: &http.Client{Timeout: 5 * time.Second}, PhoenixURL: server.URL}
	result, out, err := reg.handleListRoomsCluster(ListRoomsInput{Project: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if out.OmittedResolved != 2 {
		t.Errorf("omitted_resolved = %d, want 2 (distinct ids)", out.OmittedResolved)
	}
	if !strings.Contains(resultText(result), "2 resolved room(s) not shown") {
		t.Errorf("missing note: %s", resultText(result))
	}
	if refsParams["status"] != "resolved" || refsParams["project"] != "p" {
		t.Errorf("room_refs params wrong: %v", refsParams)
	}

	// status=all lifts the filter, so nothing is omitted and no count is asked for.
	refsParams = nil
	_, out, _ = reg.handleListRoomsCluster(ListRoomsInput{Project: "p", Status: "all"})
	if out.OmittedResolved != 0 || refsParams != nil {
		t.Errorf("status=all should not count: omitted=%d refs=%v", out.OmittedResolved, refsParams)
	}
}
