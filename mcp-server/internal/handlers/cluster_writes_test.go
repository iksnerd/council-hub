package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// portOf extracts the port from an httptest server URL like "http://127.0.0.1:54321".
func portOf(t *testing.T, url string) string {
	t.Helper()
	i := strings.LastIndex(url, ":")
	if i < 0 {
		t.Fatalf("no port in url %q", url)
	}
	return url[i+1:]
}

func TestPeerMCPURL(t *testing.T) {
	reg := &Registry{PeerMCPPort: "3001"}

	for _, tc := range []struct{ path, want string }{
		{internalPostPath, "http://10.0.0.5:3001/api/internal/post_to_room"},
		{internalStatusPath, "http://10.0.0.5:3001/api/internal/signal_status"},
	} {
		got, err := reg.peerMCPURL("council_hub@10.0.0.5", tc.path)
		if err != nil {
			t.Fatalf("peerMCPURL(%s) error: %v", tc.path, err)
		}
		if got != tc.want {
			t.Errorf("expected %q, got %q", tc.want, got)
		}
	}

	if _, err := reg.peerMCPURL("no-at-sign", internalPostPath); err == nil {
		t.Error("expected error for malformed node name")
	}
}

func TestInternalStatusHandlerRequiresSecret(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"
	if err := reg.Server.CreateRoom("owned", "Owner room", "", "", "", "", ""); err != nil {
		t.Fatalf("create room: %v", err)
	}

	handler := reg.InternalStatusHandler()
	body, _ := json.Marshal(internalStatusRequest{RoomID: "owned", Status: "resolved"})

	for _, tc := range []struct{ name, secret string }{
		{"no secret", ""},
		{"wrong secret", "wrong"},
	} {
		req := httptest.NewRequest(http.MethodPost, internalStatusPath, bytes.NewReader(body))
		if tc.secret != "" {
			req.Header.Set(clusterSecretHeader, tc.secret)
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: expected 403, got %d", tc.name, rec.Code)
		}
	}

	// The room must be untouched by the rejected calls.
	room, err := reg.Server.GetRoom("owned")
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	if room.Status == "resolved" {
		t.Error("rejected request still applied the status change")
	}
}

func TestInternalStatusHandlerAppliesLocally(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"
	if err := reg.Server.CreateRoom("owned", "Owner room", "", "", "", "", ""); err != nil {
		t.Fatalf("create room: %v", err)
	}

	handler := reg.InternalStatusHandler()
	body, _ := json.Marshal(internalStatusRequest{RoomID: "owned", Status: "resolved"})
	req := httptest.NewRequest(http.MethodPost, internalStatusPath, bytes.NewReader(body))
	req.Header.Set(clusterSecretHeader, "topsecret")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out internalStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Error != "" || out.Status != "resolved" {
		t.Errorf("expected success, got %+v", out)
	}

	room, err := reg.Server.GetRoom("owned")
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	if room.Status != "resolved" {
		t.Errorf("expected room resolved, got %q", room.Status)
	}
}

func TestInternalStatusHandlerValidatesInput(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"
	if err := reg.Server.CreateRoom("owned", "Owner room", "", "", "", "", ""); err != nil {
		t.Fatalf("create room: %v", err)
	}
	handler := reg.InternalStatusHandler()

	// A peer holding the cluster secret is still not trusted to send a valid
	// status or an existing room ID — this endpoint validates independently of
	// whatever the calling node's signal_status handler checked.
	for _, tc := range []struct{ name, room, status, wantErr string }{
		{"unknown status", "owned", "banana", "invalid status"},
		{"missing fields", "", "", "required"},
		{"unknown room", "nope", "resolved", "not found"},
	} {
		body, _ := json.Marshal(internalStatusRequest{RoomID: tc.room, Status: tc.status})
		req := httptest.NewRequest(http.MethodPost, internalStatusPath, bytes.NewReader(body))
		req.Header.Set(clusterSecretHeader, "topsecret")
		rec := httptest.NewRecorder()
		handler(rec, req)

		var out internalStatusResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		if !strings.Contains(out.Error, tc.wantErr) {
			t.Errorf("%s: expected error containing %q, got %q", tc.name, tc.wantErr, out.Error)
		}
	}

	room, err := reg.Server.GetRoom("owned")
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	if room.Status == "banana" {
		t.Error("invalid status was written")
	}
}

// The friction this fixes: a room owned by a peer could be closed out with
// post_to_room (which proxies) but signal_status 404'd, so the Knowledge Linter
// kept flagging rooms whose work was finished and recorded.
func TestSignalStatusProxiesToOwner(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"

	var received *internalStatusRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cluster/locate_room"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"nodes": []string{"peer@127.0.0.1"}, "warnings": []string{}})
		case strings.HasSuffix(r.URL.Path, internalStatusPath):
			if r.Header.Get(clusterSecretHeader) != "topsecret" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			var in internalStatusRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			received = &in
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(internalStatusResponse{RoomID: in.RoomID, Status: in.Status})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	reg.PhoenixURL = server.URL
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	reg.PeerMCPPort = portOf(t, server.URL)

	res, _, err := reg.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "remote-room", Status: "resolved",
	})
	if err != nil {
		t.Fatalf("handleSignalStatus error: %v", err)
	}

	if received == nil {
		t.Fatal("expected status change to be forwarded to owner")
	}
	if received.RoomID != "remote-room" || received.Status != "resolved" {
		t.Errorf("forwarded wrong payload: %+v", received)
	}
	if text := resultText(res); !strings.Contains(text, "peer@127.0.0.1") {
		t.Errorf("expected owner node in response, got: %s", text)
	}
}

func TestSignalStatusLocalRoomDoesNotProxy(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"
	if err := reg.Server.CreateRoom("mine", "Local room", "", "", "", "", ""); err != nil {
		t.Fatalf("create room: %v", err)
	}

	// Any call to the cluster locator would be a bug: the room is right here.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected cluster call to %s for a local room", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	reg.PhoenixURL = server.URL
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}

	if _, _, err := reg.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "mine", Status: "paused",
	}); err != nil {
		t.Fatalf("handleSignalStatus error: %v", err)
	}

	room, err := reg.Server.GetRoom("mine")
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	if room.Status != "paused" {
		t.Errorf("expected paused, got %q", room.Status)
	}
}

func TestSignalStatusUnknownRoomWithNoOwnerStillErrors(t *testing.T) {
	reg := setupHandlerTest(t)

	// No owner reported → the caller must still get the plain not-found error,
	// not a cluster-flavoured one.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"nodes": []string{}, "warnings": []string{}})
	}))
	defer server.Close()
	reg.PhoenixURL = server.URL
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}

	res, _, err := reg.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "ghost", Status: "resolved",
	})
	if err != nil {
		t.Fatalf("handleSignalStatus error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not-found error, got: %s", text)
	}
	if strings.Contains(text, "cluster node") {
		t.Errorf("should not mention a cluster owner when none was found: %s", text)
	}
}

func TestInternalPostHandlerRequiresSecret(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"
	if err := reg.Server.CreateRoom("owned", "Owner room", "", "", "", "", ""); err != nil {
		t.Fatalf("create room: %v", err)
	}

	handler := reg.InternalPostHandler()
	body, _ := json.Marshal(internalPostRequest{RoomID: "owned", Author: "Remote", Message: "hi"})

	// No secret header → forbidden.
	req := httptest.NewRequest(http.MethodPost, "/api/internal/post_to_room", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 without secret, got %d", rec.Code)
	}

	// Wrong secret → forbidden.
	req = httptest.NewRequest(http.MethodPost, "/api/internal/post_to_room", bytes.NewReader(body))
	req.Header.Set(clusterSecretHeader, "wrong")
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 with wrong secret, got %d", rec.Code)
	}
}

func TestInternalPostHandlerWritesLocally(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"
	if err := reg.Server.CreateRoom("owned", "Owner room", "", "", "", "", ""); err != nil {
		t.Fatalf("create room: %v", err)
	}

	handler := reg.InternalPostHandler()
	body, _ := json.Marshal(internalPostRequest{RoomID: "owned", Author: "Remote", Message: "cross-node hello", MessageType: "decision"})
	req := httptest.NewRequest(http.MethodPost, "/api/internal/post_to_room", bytes.NewReader(body))
	req.Header.Set(clusterSecretHeader, "topsecret")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out internalPostResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Error != "" || out.MessageID == "" {
		t.Errorf("expected success, got %+v", out)
	}

	// Message must be persisted locally.
	msgs, _ := reg.Server.GetRecentMessages("owned", 10)
	if len(msgs) != 1 || msgs[0].Content != "cross-node hello" {
		t.Errorf("expected message persisted, got %+v", msgs)
	}
}

func TestInternalPostHandlerUnknownRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"

	handler := reg.InternalPostHandler()
	body, _ := json.Marshal(internalPostRequest{RoomID: "ghost", Author: "Remote", Message: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/api/internal/post_to_room", bytes.NewReader(body))
	req.Header.Set(clusterSecretHeader, "topsecret")
	rec := httptest.NewRecorder()
	handler(rec, req)

	var out internalPostResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Error == "" {
		t.Error("expected error for unknown room on owner node")
	}
}

// End-to-end proxy: a single httptest server stands in for both the Phoenix
// locate_room API and the owner node's internal write endpoint.
func TestPostToRoomProxiesToOwner(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"

	var receivedWrite *internalPostRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cluster/locate_room"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"nodes": []string{"peer@127.0.0.1"}, "warnings": []string{}})
		case strings.HasSuffix(r.URL.Path, "/api/internal/post_to_room"):
			if r.Header.Get(clusterSecretHeader) != "topsecret" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			var in internalPostRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			receivedWrite = &in
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(internalPostResponse{MessageID: "deadbeef-0000", RoomID: in.RoomID})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	reg.PhoenixURL = server.URL
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	reg.PeerMCPPort = portOf(t, server.URL)

	// Room does NOT exist locally → must proxy to the located owner.
	res, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "remote-room", Author: "Local", Message: "hello peer", MessageType: "message",
	})
	if err != nil {
		t.Fatalf("handlePostToRoom error: %v", err)
	}

	if receivedWrite == nil {
		t.Fatal("expected write to be forwarded to owner")
	}
	if receivedWrite.Message != "hello peer" {
		t.Errorf("forwarded wrong message: %+v", receivedWrite)
	}
	if text := resultText(res); !strings.Contains(text, "peer@127.0.0.1") {
		t.Errorf("expected owner node in response, got: %s", text)
	}
}

// Z1: create_room must refuse to shadow a room a public peer already owns.
func TestCreateRoomConflictGuard(t *testing.T) {
	reg := setupHandlerTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"nodes": []string{"peer@10.0.0.9"}, "warnings": []string{}})
	}))
	defer server.Close()

	reg.PhoenixURL = server.URL
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{ID: "taken", Topic: "x"})
	if err != nil {
		t.Fatalf("handleCreateRoom error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "peer@10.0.0.9") || !strings.Contains(text, "already exists") {
		t.Errorf("expected conflict error naming owner, got: %s", text)
	}

	// The local shadow must NOT have been created.
	if _, gerr := reg.Server.GetRoom("taken"); gerr == nil {
		t.Error("expected no local shadow room to be created")
	}
}

// When no peer owns the ID, creation proceeds normally.
func TestCreateRoomNoConflictCreatesNormally(t *testing.T) {
	reg := setupHandlerTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"nodes": []string{}, "warnings": []string{}})
	}))
	defer server.Close()

	reg.PhoenixURL = server.URL
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}

	if _, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{ID: "fresh", Topic: "x"}); err != nil {
		t.Fatalf("handleCreateRoom error: %v", err)
	}
	if _, gerr := reg.Server.GetRoom("fresh"); gerr != nil {
		t.Error("expected room to be created when no peer owns the ID")
	}
}

// clusterStub serves locate_room (reporting `owner`, or nobody when owner is "")
// plus the internal write endpoints, so a handler's cluster path can be driven
// without a real peer.
func clusterStub(t *testing.T, owner string, onPost func(internalPostRequest)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cluster/locate_room"):
			nodes := []string{}
			if owner != "" {
				nodes = append(nodes, owner)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"nodes": nodes, "warnings": []string{}})
		case strings.HasSuffix(r.URL.Path, internalPostPath):
			var in internalPostRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			if onPost != nil {
				onPost(in)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(internalPostResponse{MessageID: "deadbeef-0000", RoomID: in.RoomID, Pinned: in.Pin == "true"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// The close-out workflow is post synthesis -> pin -> resolve. Pin used to be
// dropped on the proxy path, so a peer-owned room could never get its pin
// updated from here and stayed flagged stale-pin forever.
func TestPostToRoomForwardsPinToOwner(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"

	var got *internalPostRequest
	server := clusterStub(t, "peer@127.0.0.1", func(in internalPostRequest) { got = &in })
	defer server.Close()
	reg.PhoenixURL = server.URL
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	reg.PeerMCPPort = portOf(t, server.URL)

	res, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "remote-room", Author: "Local", Message: "closing synthesis",
		MessageType: "synthesis", Pin: "true",
	})
	if err != nil {
		t.Fatalf("handlePostToRoom error: %v", err)
	}
	if got == nil {
		t.Fatal("write was not forwarded")
	}
	if got.Pin != "true" {
		t.Errorf("pin not forwarded: %+v", got)
	}
	if text := resultText(res); !strings.Contains(text, "pinned") {
		t.Errorf("expected the response to report the pin, got: %s", text)
	}
}

func TestPostToRoomReportsPinFailureOnOwner(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"

	// Owner accepts the write but reports the pin did not land.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cluster/locate_room"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"nodes": []string{"peer@127.0.0.1"}, "warnings": []string{}})
		default:
			var in internalPostRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(internalPostResponse{MessageID: "deadbeef-0000", RoomID: in.RoomID, Pinned: false})
		}
	}))
	defer server.Close()
	reg.PhoenixURL = server.URL
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	reg.PeerMCPPort = portOf(t, server.URL)

	res, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "remote-room", Author: "Local", Message: "hi", MessageType: "synthesis", Pin: "true",
	})
	if err != nil {
		t.Fatalf("handlePostToRoom error: %v", err)
	}
	// Must not claim a pin that didn't happen.
	if text := resultText(res); !strings.Contains(text, "pin failed") {
		t.Errorf("expected an honest pin-failure note, got: %s", text)
	}
}

func TestInternalPostHandlerAppliesPin(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "topsecret"
	if err := reg.Server.CreateRoom("owned", "Owner room", "", "", "", "", ""); err != nil {
		t.Fatalf("create room: %v", err)
	}

	handler := reg.InternalPostHandler()
	body, _ := json.Marshal(internalPostRequest{
		RoomID: "owned", Author: "Remote", Message: "pin me", MessageType: "synthesis", Pin: "true",
	})
	req := httptest.NewRequest(http.MethodPost, internalPostPath, bytes.NewReader(body))
	req.Header.Set(clusterSecretHeader, "topsecret")
	rec := httptest.NewRecorder()
	handler(rec, req)

	var out internalPostResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.Pinned {
		t.Errorf("expected pinned=true, got %+v", out)
	}

	pinned, err := reg.Server.GetPinnedMessage("owned")
	if err != nil {
		t.Fatalf("get pinned message: %v", err)
	}
	if pinned == nil || pinned.ID != out.MessageID {
		t.Errorf("expected pinned message %q, got %+v", out.MessageID, pinned)
	}
}

// A room on a peer must not report as "not found" — that reads as "does not
// exist" and invites recreating it as a local shadow.
func TestLocalOnlyToolsNameTheOwningNode(t *testing.T) {
	server := clusterStub(t, "peer@127.0.0.1", nil)
	defer server.Close()

	cases := []struct {
		name   string
		call   func(reg *Registry) string
		expect string
	}{
		{"read_transcript", func(reg *Registry) string {
			res, _, _ := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{RoomID: "remote-room"})
			return resultText(res)
		}, "cluster_wide=true"},
		{"read_room", func(reg *Registry) string {
			res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "remote-room"})
			return resultText(res)
		}, "cluster_wide=true"},
		{"archive_room", func(reg *Registry) string {
			res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "remote-room"})
			return resultText(res)
		}, "local-only"},
		{"delete_room", func(reg *Registry) string {
			res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "remote-room"})
			return resultText(res)
		}, "local-only"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := setupHandlerTest(t)
			reg.PhoenixURL = server.URL
			reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}

			text := tc.call(reg)
			if !strings.Contains(text, "peer@127.0.0.1") {
				t.Errorf("expected the owning node to be named, got: %s", text)
			}
			if !strings.Contains(text, tc.expect) {
				t.Errorf("expected remedy %q, got: %s", tc.expect, text)
			}
		})
	}
}

// With no peer owning the room, the error must stay exactly as it was — a
// non-clustered deployment should see no change at all.
func TestRoomMissWithNoOwnerIsUnannotated(t *testing.T) {
	server := clusterStub(t, "", nil)
	defer server.Close()

	reg := setupHandlerTest(t)
	reg.PhoenixURL = server.URL
	reg.HTTPClient = &http.Client{Timeout: 5 * time.Second}

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not-found, got: %s", text)
	}
	if strings.Contains(text, "cluster node") {
		t.Errorf("must not mention a cluster node when none owns it: %s", text)
	}
}
