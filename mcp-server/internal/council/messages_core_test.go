package council

import (
	"fmt"
	"strings"
	"testing"
)

func TestPostMessage(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("msg-room", "Message test", "", "", "", "", "")

	id1, err := s.PostMessage("msg-room", "Claude", "Hello from Claude", "message", "")
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}
	id2, err := s.PostMessage("msg-room", "Gemini", "Hello from Gemini", "message", "")
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}

	if id2 <= id1 {
		t.Errorf("expected id2 > id1 (UUID v7 ordering), got id1=%s id2=%s", id1, id2)
	}

	msgs, err := s.GetTranscript("msg-room")
	if err != nil {
		t.Fatalf("getTranscript failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Author != "Claude" || msgs[1].Author != "Gemini" {
		t.Errorf("unexpected message order: %s, %s", msgs[0].Author, msgs[1].Author)
	}
}

func TestMessageType(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("type-room", "Type test", "", "", "", "", "")

	s.PostMessage("type-room", "Claude", "I think we should...", "thought", "")
	s.PostMessage("type-room", "Gemini", "Let's go with RS256", "decision", "")
	s.PostMessage("type-room", "Claude", "func main() {}", "code", "")

	msgs, _ := s.GetTranscript("type-room")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].MessageType != "thought" {
		t.Errorf("expected 'thought', got '%s'", msgs[0].MessageType)
	}
	if msgs[1].MessageType != "decision" {
		t.Errorf("expected 'decision', got '%s'", msgs[1].MessageType)
	}
	if msgs[2].MessageType != "code" {
		t.Errorf("expected 'code', got '%s'", msgs[2].MessageType)
	}
}

func TestCritiqueMessageType(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("critique-room", "Critique test", "", "", "", "", "")

	_, err := s.PostMessage("critique-room", "Claude", "This approach has flaws", "critique", "")
	if err != nil {
		t.Fatalf("postMessage with critique type failed: %v", err)
	}

	msgs, _ := s.GetTranscript("critique-room")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].MessageType != "critique" {
		t.Errorf("expected 'critique', got '%s'", msgs[0].MessageType)
	}

	room, _ := s.GetRoom("critique-room")
	transcript := FormatTranscript(room, msgs)
	if !strings.Contains(transcript, "Claude (critique)") {
		t.Error("transcript missing critique message type")
	}
}

func TestPostMessageWithReplyTo(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("reply-room", "Reply test", "", "", "", "", "")

	id1, _ := s.PostMessage("reply-room", "Claude", "Original message", "message", "")
	id2, _ := s.PostMessage("reply-room", "Gemini", "Replying to Claude", "review", id1)

	msgs, _ := s.GetTranscript("reply-room")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].ReplyTo != id1 {
		t.Errorf("expected reply_to %s, got %s", id1, msgs[1].ReplyTo)
	}

	// Verify transcript rendering includes reply tag (first 8 chars of UUID)
	room, _ := s.GetRoom("reply-room")
	transcript := FormatTranscript(room, msgs)
	expected := fmt.Sprintf("re: #%s", id1[:8])
	if !strings.Contains(transcript, expected) {
		t.Errorf("transcript missing reply tag '%s'", expected)
	}

	// Verify non-reply message has no reply tag
	if msgs[0].ReplyTo != "" {
		t.Errorf("expected reply_to '' for original message, got %s", msgs[0].ReplyTo)
	}

	// Get by ID and verify reply_to is preserved
	fetched, _ := s.GetMessagesByIDs([]string{id2})
	if len(fetched) != 1 {
		t.Fatalf("expected 1 message, got %d", len(fetched))
	}
	if fetched[0].ReplyTo != id1 {
		t.Errorf("getMessagesByIDs: expected reply_to %s, got %s", id1, fetched[0].ReplyTo)
	}
}

func TestReplyToInReadRecent(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("reply-recent", "Reply recent test", "", "", "", "", "")

	id1, _ := s.PostMessage("reply-recent", "Claude", "First", "message", "")
	s.PostMessage("reply-recent", "Gemini", "Reply to first", "critique", id1)

	msgs, err := s.GetRecentMessages("reply-recent", 2)
	if err != nil {
		t.Fatalf("getRecentMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].ReplyTo != id1 {
		t.Errorf("expected reply_to %s, got %s", id1, msgs[1].ReplyTo)
	}
	if msgs[1].MessageType != "critique" {
		t.Errorf("expected message_type 'critique', got '%s'", msgs[1].MessageType)
	}
}

func TestGetMessagesByIDs(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("getmsg-room", "Get messages test", "", "", "", "", "")
	id1, _ := s.PostMessage("getmsg-room", "Claude", "Full content of message one with lots of detail", "thought", "")

	msgs, err := s.GetMessagesByIDs([]string{id1})
	if err != nil {
		t.Fatalf("getMessagesByIDs failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "Full content of message one with lots of detail" {
		t.Errorf("expected full content, got '%s'", msgs[0].Content)
	}
	if msgs[0].Author != "Claude" {
		t.Errorf("expected author 'Claude', got '%s'", msgs[0].Author)
	}
}

func TestGetMessagesByIDsMultiple(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("getmsg-room-a", "Room A", "", "", "", "", "")
	s.CreateRoom("getmsg-room-b", "Room B", "", "", "", "", "")
	id1, _ := s.PostMessage("getmsg-room-a", "Claude", "Message in room A", "message", "")
	id2, _ := s.PostMessage("getmsg-room-b", "Gemini", "Message in room B", "review", "")
	id3, _ := s.PostMessage("getmsg-room-a", "Amp", "Another in room A", "decision", "")

	msgs, err := s.GetMessagesByIDs([]string{id1, id2, id3})
	if err != nil {
		t.Fatalf("getMessagesByIDs failed: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].RoomID != "getmsg-room-a" || msgs[1].RoomID != "getmsg-room-b" || msgs[2].RoomID != "getmsg-room-a" {
		t.Errorf("unexpected room IDs: %s, %s, %s", msgs[0].RoomID, msgs[1].RoomID, msgs[2].RoomID)
	}
}

func TestGetMessagesByIDsNotFound(t *testing.T) {
	s := setupTestServer(t)

	msgs, err := s.GetMessagesByIDs([]string{"fake-uuid-1", "fake-uuid-2"})
	if err != nil {
		t.Fatalf("getMessagesByIDs failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestGetMessagesByIDsEmpty(t *testing.T) {
	s := setupTestServer(t)

	msgs, err := s.GetMessagesByIDs([]string{})
	if err != nil {
		t.Fatalf("getMessagesByIDs failed: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil, got %v", msgs)
	}
}

func TestGetRecentMessages(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("recent-room", "Recent test", "", "", "", "", "")
	for i := 0; i < 10; i++ {
		s.PostMessage("recent-room", "Claude", fmt.Sprintf("Message %d", i), "message", "")
	}

	msgs, err := s.GetRecentMessages("recent-room", 3)
	if err != nil {
		t.Fatalf("getRecentMessages failed: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, "Message 7") {
		t.Errorf("expected 'Message 7', got '%s'", msgs[0].Content)
	}
	if !strings.Contains(msgs[2].Content, "Message 9") {
		t.Errorf("expected 'Message 9', got '%s'", msgs[2].Content)
	}
}

func TestGetRecentMessagesDefault(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("recent-default", "Default limit test", "", "", "", "", "")
	for i := 0; i < 15; i++ {
		s.PostMessage("recent-default", "Claude", fmt.Sprintf("Message %d", i), "message", "")
	}

	msgs, err := s.GetRecentMessages("recent-default", 0)
	if err != nil {
		t.Fatalf("getRecentMessages failed: %v", err)
	}
	if len(msgs) != 10 {
		t.Errorf("expected default 10 messages, got %d", len(msgs))
	}
}

func TestGetRecentMessagesOverLimit(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("recent-cap", "Cap test", "", "", "", "", "")
	for i := 0; i < 60; i++ {
		s.PostMessage("recent-cap", "Claude", fmt.Sprintf("Message %d", i), "message", "")
	}

	msgs, err := s.GetRecentMessages("recent-cap", 100)
	if err != nil {
		t.Fatalf("getRecentMessages failed: %v", err)
	}
	if len(msgs) != 50 {
		t.Errorf("expected capped 50 messages, got %d", len(msgs))
	}
}

func TestGetRecentMessagesEmptyRoom(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("recent-empty", "Empty room", "", "", "", "", "")

	msgs, err := s.GetRecentMessages("recent-empty", 5)
	if err != nil {
		t.Fatalf("getRecentMessages failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestGetRecentMessagesNotFound(t *testing.T) {
	s := setupTestServer(t)

	_, err := s.GetRecentMessages("nonexistent", 5)
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestRetractMessages(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("delmsg-room", "Retract messages test", "", "", "", "", "")
	id1, _ := s.PostMessage("delmsg-room", "Claude", "Keep this", "message", "")
	id2, _ := s.PostMessage("delmsg-room", "Gemini", "Retract this", "message", "")
	id3, _ := s.PostMessage("delmsg-room", "Claude", "Retract this too", "message", "")

	count, err := s.RetractMessages([]string{id2, id3}, "claude")
	if err != nil {
		t.Fatalf("RetractMessages failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 retracted, got %d", count)
	}

	// Retract is the immutable counterpart to deletion: the nodes survive (all 3
	// still in the transcript), tombstoned rather than destroyed.
	msgs, _ := s.GetTranscript("delmsg-room")
	if len(msgs) != 3 {
		t.Fatalf("expected all 3 messages to survive retraction, got %d", len(msgs))
	}
	byID := map[string]Message{}
	for _, m := range msgs {
		byID[m.ID] = m
	}
	if byID[id1].RetractedAt.Valid {
		t.Error("id1 should not be retracted")
	}
	if !byID[id2].RetractedAt.Valid || byID[id2].RetractedBy != "claude" {
		t.Errorf("id2 should be retracted by claude, got valid=%v by=%q", byID[id2].RetractedAt.Valid, byID[id2].RetractedBy)
	}
	if !byID[id3].RetractedAt.Valid {
		t.Error("id3 should be retracted")
	}

	// Re-retracting an already-retracted node is a no-op.
	if c, _ := s.RetractMessages([]string{id2}, "gemini"); c != 0 {
		t.Errorf("re-retract should affect 0 rows, got %d", c)
	}
}

func TestPurgeMessages(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("purge-room", "Purge messages test", "", "", "", "", "")
	id1, _ := s.PostMessage("purge-room", "Claude", "Keep this", "message", "")
	id2, _ := s.PostMessage("purge-room", "Gemini", "Destroy this", "message", "")
	id3, _ := s.PostMessage("purge-room", "Claude", "Destroy this too", "message", "")

	count, err := s.PurgeMessages([]string{id2, id3})
	if err != nil {
		t.Fatalf("PurgeMessages failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 purged, got %d", count)
	}

	// Purge hard-deletes: only the kept message remains.
	msgs, _ := s.GetTranscript("purge-room")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(msgs))
	}
	if msgs[0].ID != id1 {
		t.Errorf("expected message %s to remain, got %s", id1, msgs[0].ID)
	}
}

func TestRetractMessagesNonexistent(t *testing.T) {
	s := setupTestServer(t)

	count, err := s.RetractMessages([]string{"fake-nonexistent-uuid"}, "claude")
	if err != nil {
		t.Fatalf("RetractMessages failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 retracted, got %d", count)
	}
}

func TestRetractMessagesEmptySlice(t *testing.T) {
	s := setupTestServer(t)
	count, err := s.RetractMessages([]string{}, "claude")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestPostMessageDefaultType(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "default-type")

	id, err := s.PostMessage("default-type", "Claude", "Hello", "", "")
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}

	msgs, _ := s.GetMessagesByIDs([]string{id})
	if msgs[0].MessageType != "message" {
		t.Errorf("expected default type 'message', got '%s'", msgs[0].MessageType)
	}
}

func TestPostMessageUpdatedAtBestEffort(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "besteff-room")

	// Post should succeed even though updated_at UPDATE is best-effort
	id := mustPost(t, s, "besteff-room", "Claude", "Hello")
	if id == "" {
		t.Errorf("expected non-empty message ID")
	}
}

func TestInsertSummaryUpdatedAtBestEffort(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "summary-besteff")

	err := s.InsertSummary("summary-besteff", "A summary")
	if err != nil {
		t.Fatalf("insertSummary failed: %v", err)
	}

	msgs, _ := s.GetTranscript("summary-besteff")
	if len(msgs) != 1 || !msgs[0].IsSummary {
		t.Error("expected 1 summary message")
	}
}
