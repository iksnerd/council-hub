package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const unknownClusterNote = "cluster state unknown; showing this node only"

func nodesServer(t *testing.T, status int, nodes int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		ns := make([]string, nodes)
		for i := range ns {
			ns[i] = "n"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"nodes": ns})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A failed node-count lookup is not "no peers": the note must say it is unknown.
func TestLocalOnlyNoteSaysUnknownWhenProbeFails(t *testing.T) {
	srv := nodesServer(t, http.StatusInternalServerError, 0)
	reg := &Registry{HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	if got := reg.localOnlyNote(); !strings.Contains(got, unknownClusterNote) {
		t.Errorf("localOnlyNote = %q, want it to contain %q", got, unknownClusterNote)
	}
	if reg.peersConnected() {
		t.Error("peersConnected must be false while the state is unknown")
	}
}

// Not configured and COUNCIL_UI=off are real answers: a known single node.
func TestLocalOnlyNoteSilentForKnownSingleNode(t *testing.T) {
	reg := &Registry{}
	if got := reg.localOnlyNote(); got != "" {
		t.Errorf("not configured: note = %q, want empty", got)
	}
	t.Setenv("COUNCIL_UI", "off")
	srv := nodesServer(t, http.StatusInternalServerError, 0)
	reg = &Registry{HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	if got := reg.localOnlyNote(); got != "" {
		t.Errorf("COUNCIL_UI=off: note = %q, want empty", got)
	}
}

// A one-off failure must not hide a connected peer for the long window.
func TestUnknownNodeCountRecoversQuickly(t *testing.T) {
	old := nodeCountFailTTL
	nodeCountFailTTL = 10 * time.Millisecond
	t.Cleanup(func() { nodeCountFailTTL = old })

	status := http.StatusInternalServerError
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"nodes": []string{"a", "b"}})
	}))
	defer srv.Close()
	reg := &Registry{HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	if reg.peersConnected() {
		t.Fatal("first probe fails; no peers expected")
	}
	status = http.StatusOK
	time.Sleep(30 * time.Millisecond)
	if !reg.peersConnected() {
		t.Error("peer must be seen once the failure window passes")
	}
}

func TestRouteClusterReadUnknownStateAppendsNote(t *testing.T) {
	srv := nodesServer(t, http.StatusInternalServerError, 0)
	reg := &Registry{HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	clusterCalled := false
	ok, res, _, err := reg.routeClusterRead("", true,
		func() (*mcp.CallToolResult, ToolOutput, error) { clusterCalled = true; return textResult("wide") },
		func() (*mcp.CallToolResult, ToolOutput, error) { return textResult("local rows") })
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, want a handled local answer", ok, err)
	}
	if clusterCalled {
		t.Error("cluster path must not run while the state is unknown")
	}
	text := resultText(res)
	if !strings.Contains(text, "local rows") || !strings.Contains(text, unknownClusterNote) {
		t.Errorf("got %q", text)
	}
}

// A result whose text happens to start with "Error: cluster" is a result, not a
// failure; only a returned error triggers the local fallback.
func TestRouteClusterReadFallsBackOnErrorNotOnText(t *testing.T) {
	srv := nodesServer(t, http.StatusOK, 2)
	reg := &Registry{HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	localCalled := false
	local := func() (*mcp.CallToolResult, ToolOutput, error) { localCalled = true; return textResult("local") }

	_, res, _, _ := reg.routeClusterRead("", true,
		func() (*mcp.CallToolResult, ToolOutput, error) { return textResult("Error: cluster notes are great") }, local)
	if localCalled || !strings.Contains(resultText(res), "Error: cluster notes are great") {
		t.Errorf("text must not trigger fallback; localCalled=%v text=%q", localCalled, resultText(res))
	}

	_, res, _, _ = reg.routeClusterRead("", true,
		func() (*mcp.CallToolResult, ToolOutput, error) {
			return nil, ToolOutput{}, clusterErr("search", errBoom{})
		}, local)
	if !localCalled || !strings.Contains(resultText(res), "Cluster Warning") {
		t.Errorf("typed error must fall back with a warning; localCalled=%v text=%q", localCalled, resultText(res))
	}

	// An explicit cluster_wide=true keeps the readable "Error: ..." text.
	_, res, _, err := reg.routeClusterRead("true", true,
		func() (*mcp.CallToolResult, ToolOutput, error) {
			return nil, ToolOutput{}, clusterErr("search", errBoom{})
		}, local)
	if err != nil || !strings.HasPrefix(resultText(res), "Error: cluster search failed") {
		t.Errorf("explicit read: err=%v text=%q", err, resultText(res))
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

// list_rooms: the peer filters resolved rooms itself, before cutting to the window.
func TestListRoomsClusterPushesExcludeStatusToPeers(t *testing.T) {
	var listParams map[string]any
	var refsParams map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		_ = json.NewDecoder(r.Body).Decode(&p)
		if strings.HasSuffix(r.URL.Path, "/room_refs") {
			refsParams = p
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"id": "old", "status": "resolved"}, {"id": "old", "status": "resolved"}}, "warnings": []string{}})
			return
		}
		listParams = p
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{
			{"id": "live", "status": "active", "updated_at": "2026-04-01T12:00:00", "source_node": "a@1"},
			{"id": "old", "status": "resolved", "updated_at": "2026-04-01T11:00:00", "source_node": "a@1"},
		}, "warnings": []string{}})
	}))
	defer srv.Close()
	reg := &Registry{HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	_, out, err := reg.handleListRoomsCluster(ListRoomsInput{Limit: "10"})
	if err != nil {
		t.Fatal(err)
	}
	if listParams["exclude_status"] != "resolved" {
		t.Errorf("exclude_status not sent to peers: %v", listParams)
	}
	if _, bad := refsParams["exclude_status"]; bad || refsParams["status"] != "resolved" {
		t.Errorf("room_refs must count resolved rooms without exclude_status: %v", refsParams)
	}
	if out.OmittedResolved != 1 {
		t.Errorf("omitted = %d, want 1 (deduped by id)", out.OmittedResolved)
	}
	if len(out.Rooms) != 1 {
		t.Errorf("an older peer ignoring exclude_status is still post-filtered; rooms = %d", len(out.Rooms))
	}
}

func TestListRoomsClusterWarnsWhenWindowExceedsPeerClamp(t *testing.T) {
	var listParams map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		_ = json.NewDecoder(r.Body).Decode(&p)
		if !strings.HasSuffix(r.URL.Path, "/room_refs") {
			listParams = p
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{}, "warnings": []string{}})
	}))
	defer srv.Close()
	reg := &Registry{HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	res, out, err := reg.handleListRoomsCluster(ListRoomsInput{Limit: "100", Offset: "50", Status: "all"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(out.Warnings, "|") + resultText(res)
	if !strings.Contains(joined, "first 100 rooms per peer") {
		t.Errorf("want a clamp warning, got %q", joined)
	}
	if listParams["limit"] != "100" {
		t.Errorf("limit sent = %v, want the clamp 100", listParams["limit"])
	}
}

func roomPeerServer(t *testing.T, count int, updated string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{
			{"id": "shared", "description": "peer copy", "status": "active", "updated_at": updated, "source_node": "peer@1", "message_count": count},
		}, "warnings": []string{}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// An empty local stub must not hide the real room on a peer, even when the
// stub was touched more recently.
func TestReadRoomClusterPrefersCopyWithMoreMessages(t *testing.T) {
	srv := roomPeerServer(t, 5, "2026-01-01T00:00:00")
	reg := &Registry{Server: setupTestServer(t), HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	if err := reg.Server.CreateRoom("shared", "local stub", "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	res, _, err := reg.handleReadRoomCluster(ReadRoomInput{RoomID: "shared", ClusterWide: "true"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(res), "peer copy") {
		t.Errorf("peer copy with more messages should win: %s", resultText(res))
	}
}

func TestReadRoomClusterTieGoesToLatestUpdate(t *testing.T) {
	srv := roomPeerServer(t, 0, "2999-01-01T00:00:00")
	reg := &Registry{Server: setupTestServer(t), HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	if err := reg.Server.CreateRoom("shared", "local stub", "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	res, _, _ := reg.handleReadRoomCluster(ReadRoomInput{RoomID: "shared", ClusterWide: "true"})
	if !strings.Contains(resultText(res), "peer copy") {
		t.Errorf("tie on count: later updated_at should win: %s", resultText(res))
	}
}

// The default read_room goes cluster-wide on a local hit when a peer holds a
// better copy, and stays local when it does not.
func TestReadRoomDefaultChecksPeersOnLocalHit(t *testing.T) {
	peer := roomPeerServer(t, 5, "2026-01-01T00:00:00")
	reg := &Registry{Server: setupTestServer(t), HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: peer.URL}
	reg.nodeN, reg.nodeAt = 2, time.Now()
	if err := reg.Server.CreateRoom("shared", "local stub", "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	res, _, err := reg.handleReadRoom(nil, nil, ReadRoomInput{RoomID: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultText(res), "peer copy") {
		t.Errorf("default read_room should surface the peer's fuller copy: %s", resultText(res))
	}

	for i := 0; i < 6; i++ {
		if _, err := reg.Server.PostMessage("shared", "a", "m", "message", ""); err != nil {
			t.Fatal(err)
		}
	}
	res, _, _ = reg.handleReadRoom(nil, nil, ReadRoomInput{RoomID: "shared"})
	if strings.Contains(resultText(res), "peer copy") {
		t.Errorf("local copy with more messages must stay authoritative: %s", resultText(res))
	}
}

// get_digest: the same rule picks between a local and a peer row.
func TestDigestClusterPeerRowWithMoreMessagesReplacesLocalStub(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{
			{"room_id": "shared", "new_message_count": 9, "latest_message_excerpt": "peer excerpt", "latest_message_id": "0001", "source_node": "peer@1"},
		}, "warnings": []string{}})
	}))
	defer srv.Close()
	reg := &Registry{Server: setupTestServer(t), HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
	if err := reg.Server.CreateRoom("shared", "stub", "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Server.PostMessage("shared", "a", "local excerpt", "message", ""); err != nil {
		t.Fatal(err)
	}
	res, _, err := reg.handleGetDigestCluster(DigestInput{Since: "2000-01-01T00:00:00"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(res)
	if !strings.Contains(text, "peer excerpt") || strings.Contains(text, "local excerpt") {
		t.Errorf("the peer's 9-message row should replace the local 1-message row: %s", text)
	}
}
