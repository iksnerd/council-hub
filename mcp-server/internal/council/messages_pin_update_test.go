package council

import (
	"strings"
	"testing"
)

// ========== DB-level pinMessage tests ==========

func TestPinMessageDB(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "db-pin")
	id := mustPost(t, s, "db-pin", "Claude", "Pin me")

	pinned, err := s.PinMessage("db-pin", id)
	if err != nil {
		t.Fatalf("pinMessage error: %v", err)
	}
	if !pinned {
		t.Error("expected pinned=true")
	}

	// Verify
	msg, _ := s.GetPinnedMessage("db-pin")
	if msg == nil || msg.ID != id {
		t.Error("getPinnedMessage should return the pinned message")
	}
}

func TestPostMessageWithRefsStoresSupersedes(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "sup-room")
	oldID := mustPostTyped(t, s, "sup-room", "Claude", "v1 synthesis", "synthesis")

	newID, err := s.PostMessageWithRefs("sup-room", "Claude", "v2 synthesis", "synthesis", "", "", oldID)
	if err != nil {
		t.Fatalf("PostMessageWithRefs error: %v", err)
	}

	m, err := s.GetMessageByID(newID)
	if err != nil {
		t.Fatalf("GetMessageByID error: %v", err)
	}
	if m.Supersedes != oldID {
		t.Errorf("expected supersedes=%s, got '%s'", oldID, m.Supersedes)
	}
}

func TestTranscriptRendersSupersededByBacklink(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "backlink-room")
	oldID := mustPostTyped(t, s, "backlink-room", "Claude", "v1 synthesis", "synthesis")
	if _, err := s.PinMessage("backlink-room", oldID); err != nil {
		t.Fatal(err)
	}
	newID, err := s.PostMessageWithRefs("backlink-room", "Claude", "v2 synthesis", "synthesis", "", "", oldID)
	if err != nil {
		t.Fatal(err)
	}

	room, _ := s.GetRoom("backlink-room")
	msgs, err := s.GetTranscript("backlink-room")
	if err != nil {
		t.Fatalf("GetTranscript error: %v", err)
	}
	tr := FormatTranscript(room, msgs)
	// The new message shows the forward link; the old (pinned) one shows the backlink.
	if !strings.Contains(tr, "supersedes #"+oldID[:8]) {
		t.Errorf("expected forward 'supersedes' link to %.8s in transcript", oldID)
	}
	if !strings.Contains(tr, "superseded by #"+newID[:8]) {
		t.Errorf("expected 'superseded by' backlink to %.8s in transcript:\n%s", newID, tr)
	}
}

func TestLintStalePinsFlagsSupersededPin(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "superseded-pin")
	oldID := mustPostTyped(t, s, "superseded-pin", "Claude", "v1", "synthesis")
	if _, err := s.PinMessage("superseded-pin", oldID); err != nil {
		t.Fatal(err)
	}
	// One superseding synthesis — below the 5-update heuristic, but a definitive stale pin.
	if _, err := s.PostMessageWithRefs("superseded-pin", "Claude", "v2", "synthesis", "", "", oldID); err != nil {
		t.Fatal(err)
	}

	s.lintStalePins()

	room, _ := s.GetRoom("superseded-pin")
	if !hasTag(room.Tags, "stale-pin") {
		t.Errorf("a superseded pin should be flagged stale-pin even below the update threshold, got '%s'", room.Tags)
	}
}

func TestPinMessageChainsSynthesis(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "chain-room")
	v1 := mustPostTyped(t, s, "chain-room", "Claude", "v1", "synthesis")
	if _, err := s.PinMessage("chain-room", v1); err != nil {
		t.Fatal(err)
	}
	v2 := mustPostTyped(t, s, "chain-room", "Claude", "v2", "synthesis")
	if _, err := s.PinMessage("chain-room", v2); err != nil {
		t.Fatal(err)
	}

	m, _ := s.GetMessageByID(v2)
	if m.Supersedes != v1 {
		t.Errorf("pinning v2 synthesis over v1 should set supersedes=%s, got '%s'", v1, m.Supersedes)
	}
}

func TestPinMessageNoChainForNonSynthesis(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "nochain-room")
	v1 := mustPostTyped(t, s, "nochain-room", "Claude", "v1", "synthesis")
	if _, err := s.PinMessage("nochain-room", v1); err != nil {
		t.Fatal(err)
	}
	// Pinning a decision (not a synthesis) over a synthesis must not auto-link.
	dec := mustPostTyped(t, s, "nochain-room", "Claude", "a decision", "decision")
	if _, err := s.PinMessage("nochain-room", dec); err != nil {
		t.Fatal(err)
	}

	m, _ := s.GetMessageByID(dec)
	if m.Supersedes != "" {
		t.Errorf("non-synthesis pin should not set supersedes, got '%s'", m.Supersedes)
	}
}

func TestRoomStatsMessagesSincePin(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "since-pin")
	pin := mustPostTyped(t, s, "since-pin", "Claude", "TL;DR", "synthesis")
	if _, err := s.PinMessage("since-pin", pin); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		mustPostTyped(t, s, "since-pin", "Claude", "update", "decision")
	}

	stats, err := s.GetRoomStats("since-pin")
	if err != nil {
		t.Fatalf("GetRoomStats error: %v", err)
	}
	if stats.PinnedMessageID != pin {
		t.Errorf("expected PinnedMessageID=%s, got '%s'", pin, stats.PinnedMessageID)
	}
	if stats.MessagesSincePin != 3 {
		t.Errorf("expected MessagesSincePin=3, got %d", stats.MessagesSincePin)
	}
}

func TestRoomStatsNoPinZeroSincePin(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "no-pin-stats")
	mustPost(t, s, "no-pin-stats", "Claude", "hello")

	stats, _ := s.GetRoomStats("no-pin-stats")
	if stats.PinnedMessageID != "" || stats.MessagesSincePin != 0 {
		t.Errorf("expected no pin / 0 since-pin, got id='%s' n=%d", stats.PinnedMessageID, stats.MessagesSincePin)
	}
}

func TestGetPinnedMessageNone(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "no-pin")

	msg, err := s.GetPinnedMessage("no-pin")
	if err != nil {
		t.Fatalf("getPinnedMessage error: %v", err)
	}
	if msg != nil {
		t.Error("expected nil when no message is pinned")
	}
}

// ========== DB-level updateMessage tests ==========

func TestUpdateMessageDB(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "db-update")
	id := mustPost(t, s, "db-update", "Claude", "Original")

	m, err := s.UpdateMessage(id, "Updated", "")
	if err != nil {
		t.Fatalf("updateMessage error: %v", err)
	}
	if m.Content != "Updated" {
		t.Errorf("expected 'Updated', got '%s'", m.Content)
	}
	if m.MessageType != "message" {
		t.Errorf("type should be preserved as 'message', got '%s'", m.MessageType)
	}
}

func TestUpdateMessageDBWithType(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "db-uptype")
	id := mustPost(t, s, "db-uptype", "Claude", "Original")

	m, err := s.UpdateMessage(id, "Now a decision", "decision")
	if err != nil {
		t.Fatalf("updateMessage error: %v", err)
	}
	if m.MessageType != "decision" {
		t.Errorf("expected type 'decision', got '%s'", m.MessageType)
	}
}

func TestUpdateMessageDBNotFound(t *testing.T) {
	s := setupTestServer(t)

	_, err := s.UpdateMessage("fake-nonexistent-uuid", "Nope", "")
	if err == nil {
		t.Error("expected error for nonexistent message")
	}
}

// ========== PinMessage — toggle and multi-room edge cases ==========

func TestPinMessageToggleOff(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "pin-toggle")
	id := mustPost(t, s, "pin-toggle", "Claude", "Pin me")

	pinned, _ := s.PinMessage("pin-toggle", id)
	if !pinned {
		t.Fatal("expected pinned=true on first call")
	}
	pinned, err := s.PinMessage("pin-toggle", id)
	if err != nil {
		t.Fatalf("toggle off failed: %v", err)
	}
	if pinned {
		t.Error("expected pinned=false after toggle")
	}
	msg, _ := s.GetPinnedMessage("pin-toggle")
	if msg != nil {
		t.Error("expected no pinned message after toggle off")
	}
}

func TestPinMessageReplacesExisting(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "pin-replace")
	id1 := mustPost(t, s, "pin-replace", "Claude", "First")
	id2 := mustPost(t, s, "pin-replace", "Gemini", "Second")

	s.PinMessage("pin-replace", id1)
	pinned, err := s.PinMessage("pin-replace", id2)
	if err != nil || !pinned {
		t.Fatalf("expected pin to succeed: pinned=%v err=%v", pinned, err)
	}
	msg, _ := s.GetPinnedMessage("pin-replace")
	if msg == nil || msg.ID != id2 {
		t.Errorf("expected pinned id %s, got %v", id2, msg)
	}
}

func TestPinMessageWrongRoom(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "pin-a")
	mustCreateRoom(t, s, "pin-b")
	id := mustPost(t, s, "pin-a", "Claude", "In room A")

	_, err := s.PinMessage("pin-b", id)
	if err == nil {
		t.Error("expected error when pinning message from wrong room")
	}
	if !strings.Contains(err.Error(), "belongs to room") {
		t.Errorf("unexpected error: %v", err)
	}
}
