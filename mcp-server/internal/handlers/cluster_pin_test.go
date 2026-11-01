package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// peerPair wires a caller registry to a fake owner node: one httptest server that
// answers Phoenix's locate_room and (unless oldPeer) the owner's internal pin
// endpoint. The returned owner registry holds the room.
func peerPair(t *testing.T, oldPeer bool) (caller, owner *Registry) {
	t.Helper()
	owner = setupHandlerTest(t)
	owner.ClusterSecret = "s3"
	caller = setupHandlerTest(t)
	caller.ClusterSecret = "s3"

	mux := http.NewServeMux()
	mux.HandleFunc("/api/internal/cluster/locate_room", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"nodes":["council_hub@127.0.0.1"]}`))
	})
	if !oldPeer {
		mux.HandleFunc(internalPinPath, owner.InternalPinHandler())
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	caller.PhoenixURL = srv.URL
	caller.HTTPClient = srv.Client()
	caller.PeerMCPPort = u.Port()
	return caller, owner
}

func TestPinMessageForwardsToOwner(t *testing.T) {
	caller, owner := peerPair(t, false)
	mustCreateRoom(t, owner.Server, "far")
	id := mustPost(t, owner.Server, "far", "peer", "pin me")

	res, _, _ := caller.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "far", MessageID: id[:8]})
	text := resultText(res)
	if !strings.Contains(text, "forwarded to cluster node council_hub@127.0.0.1") || !strings.Contains(text, "pinned") {
		t.Fatalf("expected forwarded-pin report, got: %s", text)
	}
	msgs, _ := owner.Server.GetRecentMessages("far", 5)
	if len(msgs) != 1 || !msgs[0].Pinned {
		t.Errorf("pin should have landed on the owner: %+v", msgs)
	}
}

func TestPinMessageOldPeerSaysNothingChanged(t *testing.T) {
	caller, owner := peerPair(t, true)
	mustCreateRoom(t, owner.Server, "far")
	id := mustPost(t, owner.Server, "far", "peer", "pin me")

	res, _, _ := caller.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "far", MessageID: id})
	text := resultText(res)
	if !strings.Contains(text, "older version") || !strings.Contains(text, "Nothing was changed") || !strings.Contains(text, "post_to_room(pin=true)") {
		t.Errorf("old peer should be named with remedy, got: %s", text)
	}
}

func TestInternalPinHandlerRejectsBadSecret(t *testing.T) {
	_, owner := peerPair(t, false)
	req := httptest.NewRequest(http.MethodPost, internalPinPath, strings.NewReader(`{"room_id":"x","message_id":"y"}`))
	req.Header.Set(clusterSecretHeader, "wrong")
	rec := httptest.NewRecorder()
	owner.InternalPinHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

func TestArchiveRoomPeerOwnedWritesNothing(t *testing.T) {
	caller, owner := peerPair(t, false)
	mustCreateRoom(t, owner.Server, "far")

	res, _, _ := caller.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "far", Delete: "true"})
	text := resultText(res)
	if !strings.Contains(text, "council_hub@127.0.0.1") || !strings.Contains(text, "Nothing was archived") {
		t.Errorf("expected owner + remedy up front, got: %s", text)
	}
	if archives, _ := caller.Server.ListArchives(); len(archives) != 0 {
		t.Errorf("no archive may be written locally: %v", archives)
	}
}
