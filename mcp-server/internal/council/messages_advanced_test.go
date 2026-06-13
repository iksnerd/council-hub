package council

import (
	"testing"
)

// ========== MoveMessages ==========

func TestMoveMessages(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("src-room", "Source", "", "", "", "", "")
	s.CreateRoom("dst-room", "Destination", "", "", "", "", "")

	id1, _ := s.PostMessage("src-room", "Claude", "Move me", "decision", "")
	id2, _ := s.PostMessage("src-room", "Gemini", "Move me too", "action", "")
	_, _ = s.PostMessage("src-room", "Claude", "Stay here", "message", "")

	moved, err := s.MoveMessages([]string{id1, id2}, "dst-room")
	if err != nil {
		t.Fatalf("MoveMessages failed: %v", err)
	}
	if moved != 2 {
		t.Errorf("expected 2 moved, got %d", moved)
	}

	// Verify messages are in dst-room
	msgs, err := s.GetTranscript("dst-room")
	if err != nil {
		t.Fatalf("GetTranscript dst-room failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages in dst-room, got %d", len(msgs))
	}

	// Verify src-room has only 1 message remaining
	srcMsgs, err := s.GetTranscript("src-room")
	if err != nil {
		t.Fatalf("GetTranscript src-room failed: %v", err)
	}
	if len(srcMsgs) != 1 {
		t.Errorf("expected 1 message remaining in src-room, got %d", len(srcMsgs))
	}
}

func TestMoveMessagesTargetNotFound(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mv-src", "Source", "", "", "", "", "")
	id, _ := s.PostMessage("mv-src", "Claude", "Hello", "message", "")

	_, err := s.MoveMessages([]string{id}, "nonexistent-room")
	if err == nil {
		t.Error("expected error for nonexistent target room, got nil")
	}
}

func TestMoveMessagesEmpty(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mv-empty-dst", "Dst", "", "", "", "", "")

	moved, err := s.MoveMessages([]string{}, "mv-empty-dst")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved != 0 {
		t.Errorf("expected 0 moved for empty IDs, got %d", moved)
	}
}

func TestMoveMessagesPreservesMetadata(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mv-meta-src", "Source", "", "", "", "", "")
	s.CreateRoom("mv-meta-dst", "Destination", "", "", "", "", "")

	id, _ := s.PostMessage("mv-meta-src", "Gemini", "Important decision", "decision", "")

	_, err := s.MoveMessages([]string{id}, "mv-meta-dst")
	if err != nil {
		t.Fatalf("MoveMessages failed: %v", err)
	}

	msgs, err := s.GetTranscript("mv-meta-dst")
	if err != nil {
		t.Fatalf("GetTranscript failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	m := msgs[0]
	if m.Author != "Gemini" {
		t.Errorf("expected author Gemini, got %s", m.Author)
	}
	if m.MessageType != "decision" {
		t.Errorf("expected type decision, got %s", m.MessageType)
	}
	if m.RoomID != "mv-meta-dst" {
		t.Errorf("expected room mv-meta-dst, got %s", m.RoomID)
	}
}

// ========== GetMentions ==========

func TestGetMentions(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mention-room", "Mention tests", "", "", "", "", "")

	// Post with mentions — one message mentioning claude, one mentioning both
	_, err := s.PostMessageWithMentions("mention-room", "gemini-cli", "Hey @claude, please review this", "thought", "", "claude")
	if err != nil {
		t.Fatalf("PostMessageWithMentions failed: %v", err)
	}
	_, err = s.PostMessageWithMentions("mention-room", "amp", "Pinging @claude and @gemini-cli", "action", "", "claude,gemini-cli")
	if err != nil {
		t.Fatalf("PostMessageWithMentions failed: %v", err)
	}
	// Post without mentions — should not appear
	s.PostMessage("mention-room", "gemini-cli", "No mentions here", "message", "")

	msgs, err := s.GetMentions("claude", "", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 mentions of claude, got %d", len(msgs))
	}
}

func TestGetMentionsNotFound(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mention-empty", "Empty mention room", "", "", "", "", "")
	s.PostMessage("mention-empty", "gemini-cli", "No mentions here", "message", "")

	msgs, err := s.GetMentions("claude", "", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 mentions, got %d", len(msgs))
	}
}

func TestGetMentionsBoundary(t *testing.T) {
	// "claude" fuzzy-matches "claude-sonnet" and "claude" — both should be returned.
	// Fuzzy matching is intentional: "claude" matches "Claude Code (Opus)", "claude-code", etc.
	s := setupTestServer(t)
	s.CreateRoom("boundary-room", "Boundary test", "", "", "", "", "")

	s.PostMessageWithMentions("boundary-room", "system", "For claude-sonnet only", "message", "", "claude-sonnet")
	s.PostMessageWithMentions("boundary-room", "system", "For claude only", "message", "", "claude")

	msgs, err := s.GetMentions("claude", "", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 fuzzy matches for 'claude' (claude + claude-sonnet), got %d", len(msgs))
	}
}

func TestGetMentionsProjectFilter(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("proj-a-room", "Project A", "alpha", "", "", "", "")
	s.CreateRoom("proj-b-room", "Project B", "beta", "", "", "", "")

	s.PostMessageWithMentions("proj-a-room", "bot", "@claude in alpha", "message", "", "claude")
	s.PostMessageWithMentions("proj-b-room", "bot", "@claude in beta", "message", "", "claude")

	all, err := s.GetMentions("claude", "", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 mentions across all projects, got %d", len(all))
	}

	scoped, err := s.GetMentions("claude", "alpha", 20)
	if err != nil {
		t.Fatalf("GetMentions(project) failed: %v", err)
	}
	if len(scoped) != 1 {
		t.Fatalf("expected 1 mention scoped to 'alpha', got %d", len(scoped))
	}
	if scoped[0].RoomID != "proj-a-room" {
		t.Errorf("expected mention from 'proj-a-room', got '%s'", scoped[0].RoomID)
	}
}

func TestGetMentionsLimit(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mention-limit", "Limit test", "", "", "", "", "")

	for i := 0; i < 5; i++ {
		s.PostMessageWithMentions("mention-limit", "bot", "ping", "message", "", "claude")
	}

	msgs, err := s.GetMentions("claude", "", 3)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 (limit), got %d", len(msgs))
	}
}

// ========== UpdateMessageWithExpected (optimistic concurrency) ==========

func TestUpdateMessageWithExpectedMatch(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("occ-room", "OCC test", "", "", "", "", "")
	id, _ := s.PostMessage("occ-room", "Claude", "original", "message", "")

	m, err := s.UpdateMessageWithExpected(id, "updated", "", "original")
	if err != nil {
		t.Fatalf("UpdateMessageWithExpected failed: %v", err)
	}
	if m.Content != "updated" {
		t.Errorf("expected content 'updated', got '%s'", m.Content)
	}
}

func TestUpdateMessageWithExpectedMismatch(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("occ-mismatch", "OCC mismatch", "", "", "", "", "")
	id, _ := s.PostMessage("occ-mismatch", "Claude", "original", "message", "")

	_, err := s.UpdateMessageWithExpected(id, "updated", "", "stale")
	if err == nil {
		t.Fatal("expected error for content mismatch, got nil")
	}
	changed, ok := err.(*ErrContentChanged)
	if !ok {
		t.Fatalf("expected *ErrContentChanged, got %T: %v", err, err)
	}
	if changed.CurrentContent != "original" {
		t.Errorf("expected current content 'original', got '%s'", changed.CurrentContent)
	}
}

func TestUpdateMessageWithExpectedEmpty(t *testing.T) {
	// Empty expected_content = blind overwrite (same as UpdateMessage)
	s := setupTestServer(t)
	s.CreateRoom("occ-empty", "OCC empty", "", "", "", "", "")
	id, _ := s.PostMessage("occ-empty", "Claude", "original", "message", "")

	m, err := s.UpdateMessageWithExpected(id, "blind update", "", "")
	if err != nil {
		t.Fatalf("UpdateMessageWithExpected with empty expected failed: %v", err)
	}
	if m.Content != "blind update" {
		t.Errorf("expected 'blind update', got '%s'", m.Content)
	}
}

func TestPostMessageWithMentionsStoredAndRetrievable(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("pmwm-room", "Test", "", "", "", "", "")

	id, err := s.PostMessageWithMentions("pmwm-room", "agent-a", "content", "thought", "", "agent-b,agent-c")
	if err != nil {
		t.Fatalf("PostMessageWithMentions: %v", err)
	}

	msgs, err := s.GetMessagesByIDs([]string{id})
	if err != nil || len(msgs) == 0 {
		t.Fatalf("GetMessagesByIDs failed: %v", err)
	}
	if msgs[0].Mentions != "agent-b,agent-c" {
		t.Errorf("expected mentions 'agent-b,agent-c', got '%s'", msgs[0].Mentions)
	}
}

func TestMarkReadAndGetCursor(t *testing.T) {
	s := setupTestServer(t)

	// GetCursor returns empty string when no cursor exists
	cursor, err := s.GetCursor("claude", "some-room")
	if err != nil {
		t.Fatalf("GetCursor on missing cursor: %v", err)
	}
	if cursor != "" {
		t.Errorf("expected empty cursor, got %q", cursor)
	}

	// MarkRead stores a cursor
	if err := s.MarkRead("claude", "room-a", "msg-001"); err != nil {
		t.Fatalf("MarkRead failed: %v", err)
	}
	cursor, err = s.GetCursor("claude", "room-a")
	if err != nil {
		t.Fatalf("GetCursor after MarkRead: %v", err)
	}
	if cursor != "msg-001" {
		t.Errorf("expected cursor 'msg-001', got %q", cursor)
	}

	// MarkRead overwrites with a newer cursor
	if err := s.MarkRead("claude", "room-a", "msg-002"); err != nil {
		t.Fatalf("MarkRead overwrite failed: %v", err)
	}
	cursor, err = s.GetCursor("claude", "room-a")
	if err != nil {
		t.Fatalf("GetCursor after overwrite: %v", err)
	}
	if cursor != "msg-002" {
		t.Errorf("expected cursor 'msg-002', got %q", cursor)
	}

	// Different agents have independent cursors for the same room
	if err := s.MarkRead("gemini", "room-a", "msg-050"); err != nil {
		t.Fatalf("MarkRead (gemini) failed: %v", err)
	}
	claudeCursor, _ := s.GetCursor("claude", "room-a")
	geminiCursor, _ := s.GetCursor("gemini", "room-a")
	if claudeCursor != "msg-002" {
		t.Errorf("claude cursor changed after gemini mark_read: got %q", claudeCursor)
	}
	if geminiCursor != "msg-050" {
		t.Errorf("expected gemini cursor 'msg-050', got %q", geminiCursor)
	}

	// Same agent, different rooms are independent
	if err := s.MarkRead("claude", "room-b", "msg-099"); err != nil {
		t.Fatalf("MarkRead (room-b) failed: %v", err)
	}
	cursorA, _ := s.GetCursor("claude", "room-a")
	cursorB, _ := s.GetCursor("claude", "room-b")
	if cursorA != "msg-002" {
		t.Errorf("room-a cursor changed after room-b mark_read: got %q", cursorA)
	}
	if cursorB != "msg-099" {
		t.Errorf("expected room-b cursor 'msg-099', got %q", cursorB)
	}
}

func TestGetMessagesFromIDInclusive(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("src", "Source room", "", "", "", "", "")

	id1, _ := s.PostMessage("src", "alice", "first", "message", "")
	id2, _ := s.PostMessage("src", "alice", "second", "message", "")
	id3, _ := s.PostMessage("src", "alice", "third", "message", "")

	// From id2 inclusive: should get id2 and id3.
	msgs, err := s.GetMessagesFromIDInclusive("src", id2)
	if err != nil {
		t.Fatalf("GetMessagesFromIDInclusive failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != id2 || msgs[1].ID != id3 {
		t.Errorf("unexpected message IDs: %v", []string{msgs[0].ID, msgs[1].ID})
	}
	_ = id1
}

func TestGetMessageByID(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-x", "Test room", "", "", "", "", "")
	id, _ := s.PostMessage("room-x", "bob", "hello", "thought", "")

	m, err := s.GetMessageByID(id)
	if err != nil {
		t.Fatalf("GetMessageByID failed: %v", err)
	}
	if m.ID != id || m.RoomID != "room-x" || m.Author != "bob" {
		t.Errorf("unexpected message: %+v", m)
	}

	_, err = s.GetMessageByID("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent ID, got nil")
	}
}
