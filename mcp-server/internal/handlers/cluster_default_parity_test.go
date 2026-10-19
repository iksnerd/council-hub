package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// parityPeer fakes Phoenix with two nodes. results maps an endpoint to its rows;
// bodies records the last JSON body posted per endpoint.
func parityPeer(t *testing.T, results map[string]any) (*Registry, map[string]map[string]any) {
	t.Helper()
	var mu sync.Mutex
	bodies := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/internal/cluster/")
		if name == "nodes" {
			_ = json.NewEncoder(w).Encode(map[string]any{"nodes": []map[string]any{{"node": "a@x"}, {"node": "b@x"}}})
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		bodies[name] = body
		mu.Unlock()
		res, ok := results[name]
		if !ok {
			res = []any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": res, "warnings": []string{}})
	}))
	t.Cleanup(srv.Close)
	reg := setupHandlerTest(t)
	reg.HTTPClient = &http.Client{Timeout: 2 * time.Second}
	reg.PhoenixURL = srv.URL
	return reg, bodies
}

func peerRoom(id, status, updated string) map[string]any {
	return map[string]any{"id": id, "description": "peer " + id, "status": status, "updated_at": updated, "source_node": "peer@10.0.0.2"}
}

func TestListRoomsDefaultClusterHidesResolvedLikeLocal(t *testing.T) {
	reg, bodies := parityPeer(t, map[string]any{"list_rooms": []map[string]any{
		peerRoom("peer-active", "active", "2026-10-01T10:00:00"),
		peerRoom("peer-done", "resolved", "2026-10-02T10:00:00"),
	}})
	mustCreateRoom(t, reg.Server, "local-room")

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if strings.Contains(text, "peer-done") {
		t.Errorf("resolved peer room shown by default, local view hides it: %s", text)
	}
	if !strings.Contains(text, "peer-active") || !strings.Contains(text, "local-room") {
		t.Errorf("active rooms missing: %s", text)
	}

	res, _, _ = reg.handleListRooms(context.Background(), nil, ListRoomsInput{Status: "all"})
	if !strings.Contains(resultText(res), "peer-done") {
		t.Errorf("status=all must include resolved: %s", resultText(res))
	}
	if s, _ := bodies["list_rooms"]["status"].(string); s == "all" {
		t.Errorf("status=all leaked to peers, which would match nothing")
	}
}

func TestListRoomsDefaultClusterHonoursLimitAndOffset(t *testing.T) {
	var rows []map[string]any
	for i := 1; i <= 4; i++ {
		rows = append(rows, peerRoom(fmt.Sprintf("peer-%d", i), "active", fmt.Sprintf("2026-10-0%dT10:00:00", i)))
	}
	reg, _ := parityPeer(t, map[string]any{"list_rooms": rows})

	res, out, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Limit: "2"})
	if len(out.Rooms) != 2 {
		t.Errorf("limit=2 returned %d rooms: %s", len(out.Rooms), resultText(res))
	}
	_, out, _ = reg.handleListRooms(context.Background(), nil, ListRoomsInput{Limit: "2", Offset: "2"})
	if len(out.Rooms) != 2 || out.Rooms[0].ID != "peer-2" {
		t.Errorf("offset=2 page wrong: %+v", out.Rooms)
	}
}

func TestSearchDefaultClusterHonoursLimit(t *testing.T) {
	var rows []map[string]any
	for i := 1; i <= 4; i++ {
		rows = append(rows, map[string]any{
			"id": fmt.Sprintf("019f0000-0000-7000-8000-00000000000%d", i), "room_id": "peer-room", "author": "bob",
			"content": "needle", "message_type": "message", "timestamp": "2026-10-01T10:00:00", "source_node": "peer@10.0.0.2",
		})
	}
	reg, _ := parityPeer(t, map[string]any{"search_messages": rows})
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle", Limit: "2"})
	if n := strings.Count(resultText(res), "**#"); n != 2 {
		t.Errorf("limit=2 gave %d results: %s", n, resultText(res))
	}
}

func TestDefaultReadsStayLocalWhenCaseNotSupportedByCluster(t *testing.T) {
	reg, bodies := parityPeer(t, nil)
	mustCreateRoom(t, reg.Server, "local-room")
	mustPost(t, reg.Server, "local-room", "alice", "needle")

	// include_related expands rooms locally; the cluster path cannot.
	_, _, _ = reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle", RoomID: "local-room", IncludeRelated: "true"})
	if _, called := bodies["search_messages"]; called {
		t.Errorf("include_related search went cluster-wide by default")
	}
	// history=true walks revision chains locally only.
	_, _, _ = reg.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "019f0000-0000-7000-8000-000000000001", History: "true"})
	if _, called := bodies["get_messages"]; called {
		t.Errorf("history=true get_messages went cluster-wide by default")
	}
}

func TestReadRoomClusterHeaderOnlyIsSaid(t *testing.T) {
	reg, _ := parityPeer(t, map[string]any{"list_rooms": []map[string]any{peerRoom("peer-only", "active", "2026-10-01T10:00:00")}})
	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "peer-only"})
	text := resultText(res)
	if !strings.Contains(text, "peer-only") || !strings.Contains(text, "include_last_n") {
		t.Errorf("header-only peer read should say how to get messages: %s", text)
	}
}
