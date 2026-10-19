package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakePhoenix serves /cluster/nodes with nodeCount nodes and counts every call
// by endpoint. slow makes the fan-out endpoints sleep past the client timeout.
type fakePhoenix struct {
	srv   *httptest.Server
	calls map[string]*atomic.Int32
}

func newFakePhoenix(t *testing.T, nodeCount int, slow bool) *fakePhoenix {
	t.Helper()
	f := &fakePhoenix{calls: map[string]*atomic.Int32{}}
	for _, p := range []string{"nodes", "search_messages", "get_mentions", "list_rooms", "get_digest", "get_messages"} {
		f.calls[p] = &atomic.Int32{}
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/internal/cluster/")
		if c, ok := f.calls[name]; ok {
			c.Add(1)
		}
		if name == "nodes" {
			nodes := []map[string]any{}
			for i := 0; i < nodeCount; i++ {
				nodes = append(nodes, map[string]any{"node": "n@" + string(rune('a'+i))})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
			return
		}
		if slow {
			time.Sleep(600 * time.Millisecond)
		}
		var results any = []any{}
		switch name {
		case "search_messages", "get_messages":
			results = []map[string]any{{
				"id": "019f0000-0000-7000-8000-000000000002", "room_id": "peer-room", "author": "bob",
				"content": "needle from peer", "message_type": "message", "timestamp": "2026-10-01T10:00:00",
				"source_node": "peer@10.0.0.2",
			}}
		case "list_rooms":
			results = []map[string]any{{"id": "peer-room", "description": "peer topic", "status": "active", "source_node": "peer@10.0.0.2"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results, "warnings": []string{}})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakePhoenix) count(name string) int { return int(f.calls[name].Load()) }

func clusteredRegistry(t *testing.T, f *fakePhoenix) *Registry {
	t.Helper()
	reg := setupHandlerTest(t)
	reg.HTTPClient = &http.Client{Timeout: 300 * time.Millisecond}
	reg.PhoenixURL = f.srv.URL
	mustCreateRoom(t, reg.Server, "local-room")
	mustPost(t, reg.Server, "local-room", "alice", "needle from local")
	return reg
}

func TestDefaultReadsStayLocalWithNoPeers(t *testing.T) {
	f := newFakePhoenix(t, 1, false)
	reg := clusteredRegistry(t, f)
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle"})
	text := resultText(res)
	if !strings.Contains(text, "needle from local") || strings.Contains(text, "peer") {
		t.Errorf("want local-only result: %s", text)
	}
	if f.count("search_messages") != 0 {
		t.Errorf("made %d cluster calls with no peers", f.count("search_messages"))
	}
	if strings.Contains(text, "cluster-wide by default") {
		t.Errorf("unexpected default note: %s", text)
	}
}

func TestDefaultSearchGoesClusterWideWithPeer(t *testing.T) {
	f := newFakePhoenix(t, 2, false)
	reg := clusteredRegistry(t, f)
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle"})
	text := resultText(res)
	for _, want := range []string{"needle from local", "needle from peer", "peer@10.0.0.2", "cluster-wide by default; pass cluster_wide=false for this node only"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in: %s", want, text)
		}
	}
}

func TestDefaultListRoomsDigestAndMessagesGoClusterWide(t *testing.T) {
	f := newFakePhoenix(t, 2, false)
	reg := clusteredRegistry(t, f)
	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{})
	if !strings.Contains(resultText(res), "peer-room") || !strings.Contains(resultText(res), "cluster-wide by default") {
		t.Errorf("list_rooms: %s", resultText(res))
	}
	dres, _, _ := reg.handleGetDigest(context.Background(), nil, DigestInput{Since: "2000-01-01T00:00:00"})
	var d map[string]any
	if err := json.Unmarshal([]byte(resultText(dres)), &d); err != nil || d["note"] == nil {
		t.Errorf("digest must stay JSON with a note: %v %s", err, resultText(dres))
	}
	if f.count("get_digest") != 1 {
		t.Errorf("digest cluster calls = %d", f.count("get_digest"))
	}
	_, _, _ = reg.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "019f0000-0000-7000-8000-000000000002"})
	if f.count("get_messages") != 1 {
		t.Errorf("get_messages by ids cluster calls = %d", f.count("get_messages"))
	}
	_, _, _ = reg.handleGetMessages(context.Background(), nil, GetMessagesInput{RoomID: "local-room"})
	if f.count("get_messages") != 1 {
		t.Errorf("get_messages by room must stay local")
	}
}

func TestDefaultUnreadDigestStaysLocal(t *testing.T) {
	f := newFakePhoenix(t, 2, false)
	reg := clusteredRegistry(t, f)
	_, _, _ = reg.handleGetDigest(context.Background(), nil, DigestInput{UnreadOnly: "true", Agent: "x"})
	if f.count("get_digest") != 0 {
		t.Errorf("unread_only digest uses node-local cursors but fanned out")
	}
}

func TestExplicitFalseForcesLocalAndKeepsFooter(t *testing.T) {
	f := newFakePhoenix(t, 2, false)
	reg := clusteredRegistry(t, f)
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle", ClusterWide: "false"})
	if strings.Contains(resultText(res), "peer") || f.count("search_messages") != 0 {
		t.Errorf("explicit false must stay local: %s", resultText(res))
	}
	mres, _, _ := reg.handleGetMentions(context.Background(), nil, GetMentionsInput{Author: "claude", ClusterWide: "false"})
	if !strings.Contains(resultText(mres), "local node only; 2 nodes in the cluster") {
		t.Errorf("forced-local mentions need the footer: %s", resultText(mres))
	}
}

func TestClusterReadsLocalEnvRestoresOptIn(t *testing.T) {
	t.Setenv("COUNCIL_CLUSTER_READS", "local")
	f := newFakePhoenix(t, 2, false)
	reg := clusteredRegistry(t, f)
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle"})
	if strings.Contains(resultText(res), "peer") || f.count("search_messages") != 0 {
		t.Errorf("COUNCIL_CLUSTER_READS=local must keep reads local: %s", resultText(res))
	}
	res, _, _ = reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle", ClusterWide: "true"})
	if !strings.Contains(resultText(res), "needle from peer") {
		t.Errorf("explicit true must still fan out: %s", resultText(res))
	}
}

func TestDefaultClusterReadDegradesWhenPeerTimesOut(t *testing.T) {
	f := newFakePhoenix(t, 2, true)
	reg := clusteredRegistry(t, f)
	res, _, err := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle"})
	if err != nil {
		t.Fatalf("fan-out failure must not error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "needle from local") || !strings.Contains(text, "Cluster Warning") {
		t.Errorf("want local results plus warning: %s", text)
	}
	mres, _, _ := reg.handleGetMentions(context.Background(), nil, GetMentionsInput{Author: "claude"})
	if !strings.Contains(resultText(mres), "Cluster Warning") {
		t.Errorf("mentions must warn: %s", resultText(mres))
	}
}

func TestDefaultReadRoomLocalHitStaysLocalMissGoesWide(t *testing.T) {
	f := newFakePhoenix(t, 2, false)
	reg := clusteredRegistry(t, f)
	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "local-room"})
	if strings.Contains(resultText(res), "cluster-wide by default") || f.count("list_rooms") != 0 {
		t.Errorf("a local hit must not fan out: %s", resultText(res))
	}
	_, _, _ = reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "peer-room"})
	if f.count("list_rooms") != 1 {
		t.Errorf("a local miss should consult the cluster, calls=%d", f.count("list_rooms"))
	}
}

func TestPeerCountIsCached(t *testing.T) {
	f := newFakePhoenix(t, 2, false)
	reg := clusteredRegistry(t, f)
	for i := 0; i < 5; i++ {
		_, _, _ = reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle"})
	}
	if n := f.count("nodes"); n != 1 {
		t.Errorf("node count fetched %d times, want 1 (10s cache)", n)
	}
}

func TestUIOffNeverCallsPhoenix(t *testing.T) {
	t.Setenv("COUNCIL_UI", "off")
	f := newFakePhoenix(t, 2, false)
	reg := clusteredRegistry(t, f)
	_, _, _ = reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle"})
	if f.count("nodes") != 0 || f.count("search_messages") != 0 {
		t.Errorf("COUNCIL_UI=off must not touch Phoenix")
	}
}

// A peer-count lookup that hangs (Phoenix's ClusterManager blocked on a dead
// peer) must cost a default read at most the short probe timeout, fall open to
// local, and not be retried inside the failure cache window.
func TestDefaultReadFailsOpenWhenNodeLookupHangs(t *testing.T) {
	var nodeCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/nodes") {
			nodeCalls.Add(1)
			select {
			case <-time.After(6 * time.Second):
			case <-r.Context().Done():
			}
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	reg := setupHandlerTest(t)
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second} // production-like multicall timeout
	reg.PhoenixURL = srv.URL
	mustCreateRoom(t, reg.Server, "local-room")
	mustPost(t, reg.Server, "local-room", "alice", "needle from local")

	start := time.Now()
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle"})
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("read took %v with a hung node lookup, want < 2s", d)
	}
	if text := resultText(res); !strings.Contains(text, "needle from local") {
		t.Fatalf("want local result, got: %s", text)
	}

	start = time.Now()
	_, _, _ = reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "needle"})
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("second read took %v, want it served from the failure cache", d)
	}
	if n := nodeCalls.Load(); n != 1 {
		t.Errorf("node lookup called %d times, want 1 inside the cache window", n)
	}
}

func TestNodeCountCacheTTLs(t *testing.T) {
	if nodeCountFailTTL <= nodeCountTTL {
		t.Errorf("failure TTL %v must exceed success TTL %v", nodeCountFailTTL, nodeCountTTL)
	}
	if nodeProbeTimeout > time.Second {
		t.Errorf("probe timeout %v must be <= 1s", nodeProbeTimeout)
	}
}
