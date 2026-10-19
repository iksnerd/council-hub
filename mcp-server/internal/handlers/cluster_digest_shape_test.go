package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// digestPeer serves /nodes with two nodes and /get_digest with the given rows.
func digestPeer(t *testing.T, rows []map[string]any) *Registry {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/internal/cluster/")
		if name == "nodes" {
			_ = json.NewEncoder(w).Encode(map[string]any{"nodes": []map[string]any{{"node": "a@x"}, {"node": "b@x"}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": rows, "warnings": []string{}})
	}))
	t.Cleanup(srv.Close)
	reg := setupHandlerTest(t)
	reg.HTTPClient = &http.Client{Timeout: 2 * time.Second}
	reg.PhoenixURL = srv.URL
	mustCreateRoom(t, reg.Server, "local-room")
	mustPost(t, reg.Server, "local-room", "alice", "local news")
	return reg
}

func parseDigest(t *testing.T, text string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, text)
	}
	return m
}

func TestDigestClusterDefaultKeepsLocalShape(t *testing.T) {
	reg := digestPeer(t, []map[string]any{
		{"room_id": "peer-room", "new_message_count": 4, "latest_message_excerpt": "peer news",
			"latest_message_id": "ffffffff-0000-7000-8000-00000000ffff", "latest_author": "bob",
			"tags": "stale", "source_node": "peer@10.0.0.2"},
		{"room_id": "old-peer-room", "new_message_count": 1, "latest_message_excerpt": "legacy", "source_node": "peer@10.0.0.2"},
	})
	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{})
	if err != nil {
		t.Fatal(err)
	}
	m := parseDigest(t, resultText(res))
	if _, ok := m["results"]; ok {
		t.Errorf("legacy results key present: %v", m)
	}
	sum, ok := m["summary"].(map[string]any)
	if !ok {
		t.Fatalf("no summary: %v", m)
	}
	if sum["total"].(float64) != 3 {
		t.Errorf("summary.total = %v, want 3", sum["total"])
	}
	if sum["stale"].(float64) != 1 {
		t.Errorf("summary.stale = %v, want 1 (peer tag)", sum["stale"])
	}
	if m["note"] == nil {
		t.Errorf("note missing")
	}
	rooms := m["rooms"].([]any)
	byID := map[string]map[string]any{}
	for _, x := range rooms {
		r := x.(map[string]any)
		byID[r["room_id"].(string)] = r
	}
	for _, id := range []string{"local-room", "peer-room", "old-peer-room"} {
		r, ok := byID[id]
		if !ok {
			t.Fatalf("missing room %s: %v", id, rooms)
		}
		for _, k := range []string{"new_messages", "latest_message_id", "latest_author", "latest_excerpt", "tags", "decision_count", "synthesis_count", "source_node"} {
			if _, ok := r[k]; !ok {
				t.Errorf("room %s missing field %s: %v", id, k, r)
			}
		}
	}
	if byID["peer-room"]["new_messages"].(float64) != 4 || byID["peer-room"]["latest_author"] != "bob" {
		t.Errorf("peer fields not mapped: %v", byID["peer-room"])
	}
	if rooms[0].(map[string]any)["room_id"] != "peer-room" {
		t.Errorf("want newest latest_message_id first: %v", rooms)
	}
	warns, _ := m["warnings"].([]any)
	if len(warns) == 0 {
		t.Errorf("expected a degradation warning for the legacy peer row")
	}
}

func TestDigestClusterHonoursBriefAndLimit(t *testing.T) {
	reg := digestPeer(t, []map[string]any{
		{"room_id": "peer-a", "new_message_count": 2, "latest_message_id": "ffffffff-0000-7000-8000-00000000fff1", "source_node": "p@1"},
		{"room_id": "peer-b", "new_message_count": 2, "latest_message_id": "ffffffff-0000-7000-8000-00000000fff2", "source_node": "p@1"},
	})
	res, _, _ := reg.handleGetDigest(context.Background(), nil, DigestInput{Brief: "true", Limit: "2"})
	m := parseDigest(t, resultText(res))
	rooms := m["rooms"].([]any)
	if len(rooms) != 2 {
		t.Fatalf("limit ignored: %d rooms", len(rooms))
	}
	if m["summary"].(map[string]any)["truncated"].(float64) != 1 {
		t.Errorf("truncated not reported: %v", m["summary"])
	}
	first := rooms[0].(map[string]any)
	if _, ok := first["latest_excerpt"]; ok {
		t.Errorf("brief entry carries full fields: %v", first)
	}
	if first["source_node"] == nil || first["latest_message_id"] == nil {
		t.Errorf("brief entry lacks source_node/latest_message_id: %v", first)
	}
}

func TestDigestClusterExplicitSameShape(t *testing.T) {
	reg := digestPeer(t, nil)
	res, _, _ := reg.handleGetDigest(context.Background(), nil, DigestInput{ClusterWide: "true"})
	m := parseDigest(t, resultText(res))
	if m["summary"] == nil || m["rooms"] == nil {
		t.Errorf("explicit cluster_wide=true shape differs: %v", m)
	}
}

func TestDigestClusterNoWarningWhenPeerOnlyEchoesLocalRooms(t *testing.T) {
	reg := digestPeer(t, []map[string]any{
		{"room_id": "local-room", "new_message_count": 1, "latest_message_id": "ffffffff-0000-7000-8000-00000000fff1", "source_node": "self@10.0.0.1"},
	})
	res, _, _ := reg.handleGetDigest(context.Background(), nil, DigestInput{})
	m := parseDigest(t, resultText(res))
	if warns, _ := m["warnings"].([]any); len(warns) != 0 {
		t.Errorf("echoed local room produced warnings: %v", warns)
	}
}

func TestDigestClusterPeerCounts(t *testing.T) {
	reg := digestPeer(t, []map[string]any{
		{"room_id": "peer-new", "new_message_count": 2, "latest_message_id": "ffffffff-0000-7000-8000-00000000fff1",
			"decision_count": 3, "synthesis_count": 1, "source_node": "p@1"},
	})
	res, _, _ := reg.handleGetDigest(context.Background(), nil, DigestInput{})
	m := parseDigest(t, resultText(res))
	for _, x := range m["rooms"].([]any) {
		r := x.(map[string]any)
		if r["room_id"] == "peer-new" && (r["decision_count"].(float64) != 3 || r["synthesis_count"].(float64) != 1) {
			t.Errorf("peer counts not carried: %v", r)
		}
	}
	if warns, _ := m["warnings"].([]any); len(warns) != 0 {
		t.Errorf("peer supplying counts still warned: %v", warns)
	}
}

func TestDigestClusterOlderPeerWithoutCountsWarns(t *testing.T) {
	reg := digestPeer(t, []map[string]any{
		{"room_id": "peer-old", "new_message_count": 2, "latest_message_id": "ffffffff-0000-7000-8000-00000000fff1", "source_node": "p@1"},
	})
	res, _, _ := reg.handleGetDigest(context.Background(), nil, DigestInput{})
	m := parseDigest(t, resultText(res))
	warns, _ := m["warnings"].([]any)
	if len(warns) != 1 || !strings.Contains(warns[0].(string), "decision_count") {
		t.Errorf("want one decision_count degrade warning, got %v", warns)
	}
}
