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

	got, err := reg.peerMCPURL("council_hub@10.0.0.5")
	if err != nil {
		t.Fatalf("peerMCPURL error: %v", err)
	}
	want := "http://10.0.0.5:3001/api/internal/post_to_room"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	if _, err := reg.peerMCPURL("no-at-sign"); err == nil {
		t.Error("expected error for malformed node name")
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
