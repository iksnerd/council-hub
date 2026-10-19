package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"council-hub/internal/council"
)

func post(t *testing.T, reg *Registry, in PostToRoomInput) string {
	t.Helper()
	res, _, err := reg.handlePostToRoom(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("handlePostToRoom: %v", err)
	}
	return resultText(res)
}

func pinnedID(t *testing.T, reg *Registry, room string) string {
	t.Helper()
	m, err := reg.Server.GetPinnedMessage(room)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		return ""
	}
	return m.ID
}

// ---- item 1: fresh peer pin guard ----

func TestPostPinRefusesToReplaceFreshPeerPin(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pg")
	peerID, _ := reg.Server.PostMessage("pg", "peer", "peer synthesis", "synthesis", "")
	if _, err := reg.Server.PinMessage("pg", peerID); err != nil {
		t.Fatal(err)
	}

	out := post(t, reg, PostToRoomInput{RoomID: "pg", Author: "me", Message: "mine", MessageType: "synthesis", Pin: "true"})
	if !strings.Contains(out, "NOT pinned") || !strings.Contains(out, peerID) || !strings.Contains(out, "peer") || !strings.Contains(out, "force=true") {
		t.Errorf("expected refusal naming the peer pin and force, got: %s", out)
	}
	if got := pinnedID(t, reg, "pg"); got != peerID {
		t.Errorf("peer pin must survive, pinned=%s", got)
	}
	if !strings.Contains(out, "posted") {
		t.Errorf("message itself must still post: %s", out)
	}
}

func TestPostPinForceReplacesFreshPeerPin(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pg2")
	peerID, _ := reg.Server.PostMessage("pg2", "peer", "peer synthesis", "synthesis", "")
	_, _ = reg.Server.PinMessage("pg2", peerID)

	out := post(t, reg, PostToRoomInput{RoomID: "pg2", Author: "me", Message: "mine", Pin: "true", Force: "true"})
	if !strings.Contains(out, "pinned") || strings.Contains(out, "NOT pinned") {
		t.Errorf("force should pin: %s", out)
	}
	if got := pinnedID(t, reg, "pg2"); got == peerID || got == "" {
		t.Errorf("pin should have moved, got %s", got)
	}
}

func TestPostPinReplacesOwnOrStalePin(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pg3")
	own, _ := reg.Server.PostMessage("pg3", "me", "old mine", "synthesis", "")
	_, _ = reg.Server.PinMessage("pg3", own)
	out := post(t, reg, PostToRoomInput{RoomID: "pg3", Author: "me", Message: "new mine", Pin: "true"})
	if strings.Contains(out, "NOT pinned") {
		t.Errorf("own pin may be replaced freely: %s", out)
	}

	mustCreateRoom(t, reg.Server, "pg4")
	old, _ := reg.Server.PostMessage("pg4", "peer", "old peer", "synthesis", "")
	_, _ = reg.Server.PinMessage("pg4", old)
	if _, err := reg.Server.DB.Exec(`UPDATE messages SET timestamp = datetime('now','-20 hours') WHERE id = ?`, old); err != nil {
		t.Fatal(err)
	}
	out = post(t, reg, PostToRoomInput{RoomID: "pg4", Author: "me", Message: "new", Pin: "true"})
	if strings.Contains(out, "NOT pinned") {
		t.Errorf("a pin older than the window may be replaced: %s", out)
	}
}

// ---- item 2: resolved rooms ----

func TestPostToResolvedRoomWarnsAndReopens(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "rs")
	_ = reg.Server.UpdateStatus("rs", "resolved")

	out := post(t, reg, PostToRoomInput{RoomID: "rs", Author: "me", Message: "late"})
	if !strings.Contains(out, "resolved") || !strings.Contains(out, "reopen=true") {
		t.Errorf("expected resolved warning, got: %s", out)
	}
	if room, _ := reg.Server.GetRoom("rs"); room.Status != "resolved" {
		t.Errorf("status must be unchanged without reopen, got %s", room.Status)
	}

	out = post(t, reg, PostToRoomInput{RoomID: "rs", Author: "me", Message: "again", Reopen: "true"})
	if !strings.Contains(out, "reopened") {
		t.Errorf("expected reopened note: %s", out)
	}
	if room, _ := reg.Server.GetRoom("rs"); room.Status != "active" {
		t.Errorf("expected active, got %s", room.Status)
	}
}

// ---- item 3: scoped ids and ready-to-paste ref ----

func TestPostScopedSupersedesAndRef(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "sa")
	mustCreateRoom(t, reg.Server, "sb")
	for _, row := range [][2]string{{"01a0aaaa-0000-7000-8000-000000000001", "sa"}, {"01a0aaaa-0000-7000-8000-000000000002", "sb"}} {
		if _, err := reg.Server.DB.Exec(`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, ?, 'x', 'x', 'note')`, row[0], row[1]); err != nil {
			t.Fatal(err)
		}
	}
	out := post(t, reg, PostToRoomInput{RoomID: "sa", Author: "me", Message: "reply", ReplyTo: "01a0aaaa"})
	if strings.Contains(out, "Error") {
		t.Fatalf("prefix shared with another room is not ambiguity: %s", out)
	}
	if !strings.Contains(out, "\"ref\": \"sa#") {
		t.Errorf("expected room#fullid ref in response: %s", out)
	}
	// room#prefix form pins the room explicitly.
	out = post(t, reg, PostToRoomInput{RoomID: "sa", Author: "me", Message: "cross", ReplyTo: "sb#01a0aaaa"})
	if strings.Contains(out, "Error") {
		t.Fatalf("room#prefix should resolve: %s", out)
	}
	msgs, _ := reg.Server.GetRecentMessages("sa", 5)
	found := false
	for _, m := range msgs {
		if m.ReplyTo == "01a0aaaa-0000-7000-8000-000000000002" {
			found = true
		}
	}
	if !found {
		t.Errorf("room#prefix reply_to not stored as sb's message")
	}
}

func TestPinMessageScopedToRoomPrefix(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pa")
	mustCreateRoom(t, reg.Server, "pb")
	for _, row := range [][2]string{{"01a0dddd-0000-7000-8000-000000000001", "pa"}, {"01a0dddd-0000-7000-8000-000000000002", "pb"}} {
		_, _ = reg.Server.DB.Exec(`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, ?, 'x', 'x', 'note')`, row[0], row[1])
	}
	res, _, _ := reg.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "pa", MessageID: "01a0dddd"})
	if out := resultText(res); !strings.Contains(out, "pinned") {
		t.Errorf("unique-in-room prefix should pin: %s", out)
	}
}

func TestPinMessageAmbiguousListsRoomAndType(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pc")
	for _, id := range []string{"01a0eeee-0000-7000-8000-000000000001", "01a0eeee-0000-7000-8000-000000000002"} {
		_, _ = reg.Server.DB.Exec(`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, 'pc', 'x', 'x', 'decision')`, id)
	}
	res, _, _ := reg.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "pc", MessageID: "01a0eeee"})
	out := resultText(res)
	if !strings.Contains(out, "ambiguous") || !strings.Contains(out, "in pc") || !strings.Contains(out, "(decision)") {
		t.Errorf("ambiguity should list room and type: %s", out)
	}
}

// ---- items 4-7: tags, links, idempotency, posted_from ----

func TestPostTagsStoredAndNormalized(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "tg")
	out := post(t, reg, PostToRoomInput{RoomID: "tg", Author: "me", Message: "fb", Tags: " meta-feedback, x ,meta-feedback"})
	if strings.Contains(out, "Error") {
		t.Fatal(out)
	}
	msgs, _ := reg.Server.GetRecentMessages("tg", 5)
	if len(msgs) != 1 || msgs[0].Tags != "meta-feedback,x" {
		t.Errorf("tags = %+v", msgs)
	}
}

func TestPostLinksCreated(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "lk")
	a, _ := reg.Server.PostMessage("lk", "x", "a", "note", "")
	b, _ := reg.Server.PostMessage("lk", "x", "b", "note", "")
	out := post(t, reg, PostToRoomInput{RoomID: "lk", Author: "me", Message: "c", Links: "refines:#" + a[:30] + ", contradicts:" + b})
	if strings.Contains(out, "Error") {
		t.Fatal(out)
	}
	msgs, _ := reg.Server.GetRecentMessages("lk", 5)
	var newID string
	for _, m := range msgs {
		if m.Author == "me" {
			newID = m.ID
		}
	}
	links, _, err := reg.Server.GetLinks(newID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %+v", links)
	}
}

func TestPostLinksValidationWritesNothing(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "lv")
	a, _ := reg.Server.PostMessage("lv", "x", "a", "note", "")
	for name, links := range map[string]string{
		"bad relation": "frobs:" + a,
		"unknown id":   "refines:ffffffff-0000-7000-8000-000000000000",
		"malformed":    "refines",
	} {
		out := post(t, reg, PostToRoomInput{RoomID: "lv", Author: "me", Message: "c", Links: "refines:" + a + "," + links})
		if !strings.Contains(out, "Error") || !strings.Contains(out, "links") {
			t.Errorf("%s: expected links error, got %s", name, out)
		}
	}
	msgs, _ := reg.Server.GetRecentMessages("lv", 10)
	if len(msgs) != 1 {
		t.Errorf("nothing may be half-written, got %d messages", len(msgs))
	}
}

func TestPostIdempotencyKeyReturnsExisting(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "id")
	first := post(t, reg, PostToRoomInput{RoomID: "id", Author: "me", Message: "once", IdempotencyKey: "k"})
	second := post(t, reg, PostToRoomInput{RoomID: "id", Author: "me", Message: "once", IdempotencyKey: "k"})
	if !strings.Contains(second, "Duplicate") {
		t.Errorf("retry should say duplicate: %s", second)
	}
	msgs, _ := reg.Server.GetRecentMessages("id", 10)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(first, msgs[0].ID) || !strings.Contains(second, msgs[0].ID) {
		t.Errorf("both responses should carry the same id")
	}
}

func TestPostedFromRecordedAndShown(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pf")
	post(t, reg, PostToRoomInput{RoomID: "pf", Author: "claude-code", Message: "hi", Workspace: "/work/trees/alpha/"})
	msgs, _ := reg.Server.GetRecentMessages("pf", 5)
	if len(msgs) != 1 || msgs[0].PostedFrom != "/work/trees/alpha" {
		t.Fatalf("posted_from = %+v", msgs)
	}
	room, _ := reg.Server.GetRoom("pf")
	tr := council.FormatTranscript(room, msgs)
	if !strings.Contains(tr, "claude-code @alpha") {
		t.Errorf("transcript header should show origin: %s", tr)
	}
}

// ---- cross-node: the owner re-validates and applies the same options ----

func TestInternalPostHandlerAppliesNewOptions(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.ClusterSecret = "s3"
	mustCreateRoom(t, reg.Server, "xn")
	peerID, _ := reg.Server.PostMessage("xn", "peer", "p", "synthesis", "")
	_, _ = reg.Server.PinMessage("xn", peerID)
	_ = reg.Server.UpdateStatus("xn", "resolved")

	call := func(in internalPostRequest) internalPostResponse {
		body, _ := json.Marshal(in)
		req := httptest.NewRequest(http.MethodPost, internalPostPath, strings.NewReader(string(body)))
		req.Header.Set(clusterSecretHeader, "s3")
		rec := httptest.NewRecorder()
		reg.InternalPostHandler().ServeHTTP(rec, req)
		var out internalPostResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}

	out := call(internalPostRequest{RoomID: "xn", Author: "me", Message: "m", Pin: "true", Tags: "a,b", IdempotencyKey: "k", Reopen: "true"})
	if out.Error != "" || out.Pinned || !out.PinRefused {
		t.Errorf("fresh peer pin must be refused on the owner: %+v", out)
	}
	if room, _ := reg.Server.GetRoom("xn"); room.Status != "active" {
		t.Errorf("reopen should apply on the owner, got %s", room.Status)
	}
	again := call(internalPostRequest{RoomID: "xn", Author: "me", Message: "m", IdempotencyKey: "k"})
	if !again.Duplicate || again.MessageID != out.MessageID {
		t.Errorf("idempotency should hold across the proxy: %+v vs %+v", again, out)
	}
	bad := call(internalPostRequest{RoomID: "xn", Author: "me", Message: "m", Links: "frobs:" + peerID})
	if bad.Error == "" {
		t.Errorf("owner must re-validate links")
	}
}
