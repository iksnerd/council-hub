package council

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
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

func TestDeleteMessages(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("delmsg-room", "Delete messages test", "", "", "", "", "")
	id1, _ := s.PostMessage("delmsg-room", "Claude", "Keep this", "message", "")
	id2, _ := s.PostMessage("delmsg-room", "Gemini", "Delete this", "message", "")
	id3, _ := s.PostMessage("delmsg-room", "Claude", "Delete this too", "message", "")

	count, err := s.DeleteMessages([]string{id2, id3})
	if err != nil {
		t.Fatalf("deleteMessages failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 deleted, got %d", count)
	}

	msgs, _ := s.GetTranscript("delmsg-room")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(msgs))
	}
	if msgs[0].ID != id1 {
		t.Errorf("expected message %s to remain, got %s", id1, msgs[0].ID)
	}
}

func TestDeleteMessagesNonexistent(t *testing.T) {
	s := setupTestServer(t)

	count, err := s.DeleteMessages([]string{"fake-nonexistent-uuid"})
	if err != nil {
		t.Fatalf("deleteMessages failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 deleted, got %d", count)
	}
}

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

// -- postMessage: default type branch --

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

// -- deleteMessages with empty slice --

func TestDeleteMessagesEmptySlice(t *testing.T) {
	s := setupTestServer(t)
	count, err := s.DeleteMessages([]string{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

// -- postMessage: updated_at best-effort doesn't fail the operation --

func TestPostMessageUpdatedAtBestEffort(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "besteff-room")

	// Post should succeed even though updated_at UPDATE is best-effort
	id := mustPost(t, s, "besteff-room", "Claude", "Hello")
	if id == "" {
		t.Errorf("expected non-empty message ID")
	}
}

// -- insertSummary: updated_at best-effort doesn't fail the operation --

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

// ========== GetMessagesAfterID ==========

func TestGetMessagesAfterID(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "after-room")
	id1 := mustPost(t, s, "after-room", "Claude", "First")
	id2 := mustPost(t, s, "after-room", "Gemini", "Second")
	id3 := mustPost(t, s, "after-room", "Claude", "Third")

	msgs, err := s.GetMessagesAfterID("after-room", id1)
	if err != nil {
		t.Fatalf("GetMessagesAfterID failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after id %s, got %d", id1, len(msgs))
	}
	if msgs[0].ID != id2 {
		t.Errorf("expected first result id %s, got %s", id2, msgs[0].ID)
	}

	// After the last message — empty
	msgs, err = s.GetMessagesAfterID("after-room", id3)
	if err != nil {
		t.Fatalf("GetMessagesAfterID failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

// ========== GetLatestPerType ==========

func TestGetLatestPerType(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "latest-type")
	mustPostTyped(t, s, "latest-type", "Claude", "thought 1", "thought")
	mustPostTyped(t, s, "latest-type", "Claude", "thought 2", "thought")
	mustPostTyped(t, s, "latest-type", "Claude", "thought 3", "thought")
	mustPostTyped(t, s, "latest-type", "Gemini", "decision 1", "decision")
	mustPostTyped(t, s, "latest-type", "Gemini", "decision 2", "decision")
	mustPostTyped(t, s, "latest-type", "Claude", "code block", "code")

	msgs, err := s.GetLatestPerType("latest-type")
	if err != nil {
		t.Fatalf("GetLatestPerType failed: %v", err)
	}
	// Up to 2 per type: thought(2), decision(2), code(1) = 5
	if len(msgs) < 3 || len(msgs) > 6 {
		t.Fatalf("expected 3-6 messages (up to 2 per type), got %d", len(msgs))
	}
	types := map[string]int{}
	for _, m := range msgs {
		types[m.MessageType]++
	}
	if types["thought"] != 2 {
		t.Errorf("expected 2 thought messages, got %d", types["thought"])
	}
	if types["decision"] != 2 {
		t.Errorf("expected 2 decision messages, got %d", types["decision"])
	}
	if types["code"] != 1 {
		t.Errorf("expected 1 code message, got %d", types["code"])
	}
}

func TestGetLatestPerTypeEmpty(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "latest-empty")
	msgs, err := s.GetLatestPerType("latest-empty")
	if err != nil {
		t.Fatalf("GetLatestPerType failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

// ========== GetMessageCounts ==========

func TestGetMessageCounts(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "count-a")
	mustCreateRoom(t, s, "count-b")
	mustPost(t, s, "count-a", "Claude", "msg 1")
	mustPost(t, s, "count-a", "Claude", "msg 2")
	mustPost(t, s, "count-b", "Gemini", "msg 1")

	counts := s.GetMessageCounts()
	if counts["count-a"] != 2 {
		t.Errorf("expected 2 for count-a, got %d", counts["count-a"])
	}
	if counts["count-b"] != 1 {
		t.Errorf("expected 1 for count-b, got %d", counts["count-b"])
	}
}

func TestGetMessageCountsEmpty(t *testing.T) {
	s := setupTestServer(t)
	counts := s.GetMessageCounts()
	if len(counts) != 0 {
		t.Errorf("expected empty map, got %d entries", len(counts))
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

// ========== GetDigest ==========

func TestGetDigest(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "digest-a", withProject("proj-x"))
	mustCreateRoom(t, s, "digest-b", withProject("proj-x"))
	mustCreateRoom(t, s, "digest-c", withProject("proj-y"))
	mustPost(t, s, "digest-a", "Claude", "msg a1")
	mustPost(t, s, "digest-a", "Claude", "msg a2")
	mustPost(t, s, "digest-b", "Gemini", "msg b1")
	mustPost(t, s, "digest-c", "Amp", "msg c1")

	since := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")

	entries, err := s.GetDigest("", since)
	if err != nil {
		t.Fatalf("GetDigest failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 digest entries, got %d", len(entries))
	}

	// Filter by project
	entries, err = s.GetDigest("proj-x", since)
	if err != nil {
		t.Fatalf("GetDigest project filter failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for proj-x, got %d", len(entries))
	}
	for _, e := range entries {
		if e.NewMessages == 0 || e.LatestAuthor == "" {
			t.Errorf("entry missing fields: %+v", e)
		}
	}
}

func TestGetDigestNoResults(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "digest-empty")
	entries, err := s.GetDigest("", "2099-01-01 00:00:00")
	if err != nil {
		t.Fatalf("GetDigest failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestGetDigestTNormalization(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "digest-t")
	mustPost(t, s, "digest-t", "Claude", "msg")
	since := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02T15:04:05")
	entries, err := s.GetDigest("", since)
	if err != nil {
		t.Fatalf("GetDigest T-normalization failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

// ========== GetPinnedExcerpts ==========

func TestGetPinnedExcerpts(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "pin-excerpt-1")
	mustCreateRoom(t, s, "pin-excerpt-2")

	// Pin a message in room 1 only
	id1 := mustPost(t, s, "pin-excerpt-1", "Claude", "This is pinned content")
	s.PinMessage("pin-excerpt-1", id1)
	mustPost(t, s, "pin-excerpt-2", "Gemini", "Not pinned")

	excerpts := s.GetPinnedExcerpts([]string{"pin-excerpt-1", "pin-excerpt-2"})
	if _, ok := excerpts["pin-excerpt-1"]; !ok {
		t.Error("expected excerpt for pin-excerpt-1")
	}
	if _, ok := excerpts["pin-excerpt-2"]; ok {
		t.Error("expected no excerpt for pin-excerpt-2")
	}

	// Test truncation: pin a message with 100+ chars
	longContent := strings.Repeat("word ", 25) // 125 chars
	idLong := mustPost(t, s, "pin-excerpt-1", "Claude", longContent)
	s.PinMessage("pin-excerpt-1", idLong) // replaces existing pin

	excerpts = s.GetPinnedExcerpts([]string{"pin-excerpt-1"})
	excerpt := excerpts["pin-excerpt-1"]
	if !strings.HasSuffix(excerpt, "...") {
		t.Errorf("expected truncated excerpt ending with '...', got: %s", excerpt)
	}
	// Excerpt should be at most ~63 chars (60 + "...")
	if len(excerpt) > 65 {
		t.Errorf("expected excerpt <= 65 chars, got %d: %s", len(excerpt), excerpt)
	}
}

// ========== UUID migration ==========

func TestMigrateMessagesToUUIDs(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// 1. Create old integer-ID schema
	db.Exec(`CREATE TABLE rooms (id TEXT PRIMARY KEY, description TEXT, status TEXT DEFAULT 'active',
		project TEXT DEFAULT '', tech_stack TEXT DEFAULT '', tags TEXT DEFAULT '',
		system_prompt TEXT DEFAULT '', related_rooms TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	db.Exec(`CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, room_id TEXT, author TEXT,
		content TEXT, message_type TEXT DEFAULT 'message', is_summary BOOLEAN DEFAULT 0,
		reply_to INTEGER DEFAULT 0, pinned BOOLEAN DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(room_id) REFERENCES rooms(id))`)

	// 2. Insert rooms
	db.Exec(`INSERT INTO rooms (id, description) VALUES ('room-a', 'Test Room A')`)
	db.Exec(`INSERT INTO rooms (id, description) VALUES ('room-b', 'Test Room B')`)

	// 3. Insert messages with integer IDs (including reply_to cross-references)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (1, 'room-a', 'Claude', 'First message', 'message')`)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (2, 'room-a', 'Gemini', 'Second message', 'thought')`)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type, reply_to) VALUES (3, 'room-a', 'Claude', 'Reply to first', 'message', 1)`)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type, pinned) VALUES (4, 'room-b', 'Amp', 'Pinned in room-b', 'decision', 1)`)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type, is_summary) VALUES (5, 'room-b', 'Claude', 'Summary', 'message', 1)`)

	// 4. Run the migration
	if err := migrateMessagesToUUIDs(db); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// 5. Verify count — no data lost
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count)
	if count != 5 {
		t.Errorf("expected 5 messages after migration, got %d", count)
	}

	// 6. Load migrated messages in insertion order
	type migratedMsg struct {
		id, author, replyTo string
		pinned, isSummary   bool
	}
	rows, err := db.Query(`SELECT id, author, reply_to, pinned, is_summary FROM messages ORDER BY rowid ASC`)
	if err != nil {
		t.Fatalf("query after migration: %v", err)
	}
	var msgs []migratedMsg
	for rows.Next() {
		var m migratedMsg
		rows.Scan(&m.id, &m.author, &m.replyTo, &m.pinned, &m.isSummary)
		msgs = append(msgs, m)
	}
	rows.Close()
	if len(msgs) != 5 {
		t.Fatalf("expected 5 migrated messages, got %d", len(msgs))
	}

	// 7. All IDs are valid UUID v7 strings
	for _, m := range msgs {
		if !isValidUUIDv7(m.id) {
			t.Errorf("id %q is not a valid UUID v7", m.id)
		}
	}

	// 8. Insertion order preserved (Claude first, Gemini second, etc.)
	if msgs[0].author != "Claude" {
		t.Errorf("expected Claude first, got %s", msgs[0].author)
	}
	if msgs[1].author != "Gemini" {
		t.Errorf("expected Gemini second, got %s", msgs[1].author)
	}

	// 9. UUID v7 time ordering: later messages have lexicographically greater IDs
	if msgs[1].id <= msgs[0].id {
		t.Errorf("expected uuid[1] > uuid[0], got %s <= %s", msgs[1].id, msgs[0].id)
	}
	if msgs[2].id <= msgs[1].id {
		t.Errorf("expected uuid[2] > uuid[1], got %s <= %s", msgs[2].id, msgs[1].id)
	}

	// 10. reply_to is correctly translated (message 3 replied to message 1)
	replyMsg := msgs[2] // third message (was id=3, reply_to=1)
	if replyMsg.replyTo == "" {
		t.Error("reply_to should have been translated to UUID, got empty string")
	}
	if replyMsg.replyTo != msgs[0].id {
		t.Errorf("reply_to should point to first message UUID %s, got %s", msgs[0].id, replyMsg.replyTo)
	}

	// 11. Pinned flag preserved
	if !msgs[3].pinned {
		t.Error("pinned flag not preserved after migration")
	}

	// 12. is_summary flag preserved
	if !msgs[4].isSummary {
		t.Error("is_summary flag not preserved after migration")
	}

	// 13. Rooms table untouched
	var roomCount int
	db.QueryRow(`SELECT COUNT(*) FROM rooms`).Scan(&roomCount)
	if roomCount != 2 {
		t.Errorf("expected 2 rooms, got %d", roomCount)
	}

	// 14. Idempotency: running migration again is a no-op
	if err := migrateMessagesToUUIDs(db); err != nil {
		t.Errorf("second migration run failed: %v", err)
	}
	db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count)
	if count != 5 {
		t.Errorf("idempotency failed: count changed to %d", count)
	}
}

func isValidUUIDv7(s string) bool {
	if len(s) != 36 {
		return false
	}
	// UUID format: 8-4-4-4-12 hex chars with dashes, version nibble = 7
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	// Version nibble at position 14 must be '7'
	return s[14] == '7'
}

func TestPostMessageAutoClearSynthesis(t *testing.T) {
	s := setupTestServer(t)
	err := s.CreateRoom("synth-room", "", "", "", "foo,needs-synthesis,bar", "", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.PostMessage("synth-room", "Author", "Synthesis text", "synthesis", "")
	if err != nil {
		t.Fatal(err)
	}

	room, err := s.GetRoom("synth-room")
	if err != nil {
		t.Fatal(err)
	}
	if room.Tags != "foo,bar" {
		t.Errorf("Expected tags to be 'foo,bar', got '%s'", room.Tags)
	}
}

// ========== GetUnsummarizedMessages ==========

func TestGetUnsummarizedMessages(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "unsumm-room")
	mustPost(t, s, "unsumm-room", "Claude", "first")
	mustPost(t, s, "unsumm-room", "Claude", "second")

	msgs, err := s.GetUnsummarizedMessages("unsumm-room")
	if err != nil {
		t.Fatalf("GetUnsummarizedMessages error: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 unsummarized messages, got %d", len(msgs))
	}
}

func TestGetUnsummarizedMessagesAfterSummary(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "summ-then-more")
	mustPost(t, s, "summ-then-more", "Claude", "before summary")
	if err := s.InsertSummary("summ-then-more", "Summary of above"); err != nil {
		t.Fatalf("InsertSummary error: %v", err)
	}
	mustPost(t, s, "summ-then-more", "Claude", "after summary")

	msgs, err := s.GetUnsummarizedMessages("summ-then-more")
	if err != nil {
		t.Fatalf("GetUnsummarizedMessages error: %v", err)
	}
	// Only message after summary should be returned
	if len(msgs) != 1 {
		t.Errorf("expected 1 unsummarized message after summary, got %d", len(msgs))
	}
	if msgs[0].Content != "after summary" {
		t.Errorf("expected 'after summary', got %q", msgs[0].Content)
	}
}

func TestGetUnsummarizedMessagesEmpty(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "all-summarized")
	mustPost(t, s, "all-summarized", "Claude", "msg")
	if err := s.InsertSummary("all-summarized", "all caught up"); err != nil {
		t.Fatalf("InsertSummary error: %v", err)
	}

	msgs, err := s.GetUnsummarizedMessages("all-summarized")
	if err != nil {
		t.Fatalf("GetUnsummarizedMessages error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 unsummarized messages after summary covers all, got %d", len(msgs))
	}
}

// ========== GetRoomsNeedingSummary ==========

func TestGetRoomsNeedingSummary(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "needs-summ")
	for i := 0; i < 6; i++ {
		mustPost(t, s, "needs-summ", "Claude", "msg")
	}
	mustCreateRoom(t, s, "below-threshold")
	mustPost(t, s, "below-threshold", "Claude", "just one")

	rooms, err := s.GetRoomsNeedingSummary(5)
	if err != nil {
		t.Fatalf("GetRoomsNeedingSummary error: %v", err)
	}
	found := false
	for _, id := range rooms {
		if id == "needs-summ" {
			found = true
		}
		if id == "below-threshold" {
			t.Errorf("below-threshold should not appear (only 1 msg, threshold=5)")
		}
	}
	if !found {
		t.Errorf("needs-summ should appear (6 msgs > threshold 5), got: %v", rooms)
	}
}

func TestGetRoomsNeedingSummaryAfterSummaryInserted(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "caught-up")
	for i := 0; i < 6; i++ {
		mustPost(t, s, "caught-up", "Claude", "msg")
	}
	if err := s.InsertSummary("caught-up", "all summarized"); err != nil {
		t.Fatalf("InsertSummary error: %v", err)
	}

	rooms, err := s.GetRoomsNeedingSummary(5)
	if err != nil {
		t.Fatalf("GetRoomsNeedingSummary error: %v", err)
	}
	for _, id := range rooms {
		if id == "caught-up" {
			t.Errorf("caught-up should not appear after summary inserted")
		}
	}
}

func TestGetRoomsNeedingSummaryEmpty(t *testing.T) {
	s := setupTestServer(t)
	rooms, err := s.GetRoomsNeedingSummary(5)
	if err != nil {
		t.Fatalf("GetRoomsNeedingSummary error: %v", err)
	}
	if len(rooms) != 0 {
		t.Errorf("expected no rooms needing summary, got: %v", rooms)
	}
}

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

	msgs, err := s.GetMentions("claude", 20)
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

	msgs, err := s.GetMentions("claude", 20)
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

	msgs, err := s.GetMentions("claude", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 fuzzy matches for 'claude' (claude + claude-sonnet), got %d", len(msgs))
	}
}

func TestGetMentionsLimit(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mention-limit", "Limit test", "", "", "", "", "")

	for i := 0; i < 5; i++ {
		s.PostMessageWithMentions("mention-limit", "bot", "ping", "message", "", "claude")
	}

	msgs, err := s.GetMentions("claude", 3)
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
