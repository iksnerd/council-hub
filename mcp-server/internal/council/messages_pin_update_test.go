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
