package council

import (
	"fmt"
	"strings"
	"testing"
)

func TestPostMessage(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("msg-room", "Message test", "", "", "", "", "")

	id1, err := s.PostMessage("msg-room", "Claude", "Hello from Claude", "message", 0)
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}
	id2, err := s.PostMessage("msg-room", "Gemini", "Hello from Gemini", "message", 0)
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}

	if id2 <= id1 {
		t.Errorf("expected id2 > id1, got id1=%d id2=%d", id1, id2)
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

	s.PostMessage("type-room", "Claude", "I think we should...", "thought", 0)
	s.PostMessage("type-room", "Gemini", "Let's go with RS256", "decision", 0)
	s.PostMessage("type-room", "Claude", "func main() {}", "code", 0)

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

	_, err := s.PostMessage("critique-room", "Claude", "This approach has flaws", "critique", 0)
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

	id1, _ := s.PostMessage("reply-room", "Claude", "Original message", "message", 0)
	id2, _ := s.PostMessage("reply-room", "Gemini", "Replying to Claude", "review", id1)

	msgs, _ := s.GetTranscript("reply-room")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].ReplyTo != id1 {
		t.Errorf("expected reply_to %d, got %d", id1, msgs[1].ReplyTo)
	}

	// Verify transcript rendering includes reply tag
	room, _ := s.GetRoom("reply-room")
	transcript := FormatTranscript(room, msgs)
	expected := fmt.Sprintf("re: #%d", id1)
	if !strings.Contains(transcript, expected) {
		t.Errorf("transcript missing reply tag '%s'", expected)
	}

	// Verify non-reply message has no reply tag
	if msgs[0].ReplyTo != 0 {
		t.Errorf("expected reply_to 0 for original message, got %d", msgs[0].ReplyTo)
	}

	// Get by ID and verify reply_to is preserved
	fetched, _ := s.GetMessagesByIDs([]int64{id2})
	if len(fetched) != 1 {
		t.Fatalf("expected 1 message, got %d", len(fetched))
	}
	if fetched[0].ReplyTo != id1 {
		t.Errorf("getMessagesByIDs: expected reply_to %d, got %d", id1, fetched[0].ReplyTo)
	}
}

func TestReplyToInReadRecent(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("reply-recent", "Reply recent test", "", "", "", "", "")

	id1, _ := s.PostMessage("reply-recent", "Claude", "First", "message", 0)
	s.PostMessage("reply-recent", "Gemini", "Reply to first", "critique", id1)

	msgs, err := s.GetRecentMessages("reply-recent", 2)
	if err != nil {
		t.Fatalf("getRecentMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].ReplyTo != id1 {
		t.Errorf("expected reply_to %d, got %d", id1, msgs[1].ReplyTo)
	}
	if msgs[1].MessageType != "critique" {
		t.Errorf("expected message_type 'critique', got '%s'", msgs[1].MessageType)
	}
}

func TestSearchMessages(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("search-room-1", "Room 1", "proj", "", "", "", "")
	s.CreateRoom("search-room-2", "Room 2", "proj", "", "", "", "")
	s.PostMessage("search-room-1", "Claude", "JWT token validation is broken", "thought", 0)
	s.PostMessage("search-room-1", "Gemini", "I agree about the JWT issue", "review", 0)
	s.PostMessage("search-room-2", "Claude", "Database migration complete", "action", 0)

	// Search by keyword
	msgs, err := s.SearchMessages("JWT", "", "", "", "", 20)
	if err != nil {
		t.Fatalf("searchMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages with 'JWT', got %d", len(msgs))
	}

	// Search by author
	msgs, _ = s.SearchMessages("", "Claude", "", "", "", 20)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages from Claude, got %d", len(msgs))
	}

	// Search by message type
	msgs, _ = s.SearchMessages("", "", "review", "", "", 20)
	if len(msgs) != 1 {
		t.Errorf("expected 1 review message, got %d", len(msgs))
	}

	// Search scoped to room
	msgs, _ = s.SearchMessages("", "Claude", "", "search-room-2", "", 20)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message from Claude in search-room-2, got %d", len(msgs))
	}

	// No results
	msgs, _ = s.SearchMessages("nonexistent", "", "", "", "", 20)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestSearchMessagesGlobal(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("search-global-a", "Room A", "proj", "", "", "", "")
	s.CreateRoom("search-global-b", "Room B", "proj", "", "", "", "")
	s.PostMessage("search-global-a", "Claude", "BEP 44 analysis here", "thought", 0)
	s.PostMessage("search-global-b", "Gemini", "BEP 46 analysis here", "review", 0)

	msgs, err := s.SearchMessages("BEP", "", "", "", "", 20)
	if err != nil {
		t.Fatalf("searchMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 global results, got %d", len(msgs))
	}
	rooms := map[string]bool{}
	for _, m := range msgs {
		rooms[m.RoomID] = true
	}
	if len(rooms) != 2 {
		t.Errorf("expected results from 2 rooms, got %d", len(rooms))
	}
}

func TestSearchMessagesSnippetLength(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("search-snippet", "Snippet test", "", "", "", "", "")
	longContent := strings.Repeat("A", 400)
	s.PostMessage("search-snippet", "Claude", longContent, "message", 0)

	msgs, err := s.SearchMessages("AAAA", "", "", "search-snippet", "", 1)
	if err != nil {
		t.Fatalf("searchMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if len(msgs[0].Content) != 400 {
		t.Errorf("expected full 400 char content from DB, got %d", len(msgs[0].Content))
	}
}

func TestGetMessagesByIDs(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("getmsg-room", "Get messages test", "", "", "", "", "")
	id1, _ := s.PostMessage("getmsg-room", "Claude", "Full content of message one with lots of detail", "thought", 0)

	msgs, err := s.GetMessagesByIDs([]int64{id1})
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
	id1, _ := s.PostMessage("getmsg-room-a", "Claude", "Message in room A", "message", 0)
	id2, _ := s.PostMessage("getmsg-room-b", "Gemini", "Message in room B", "review", 0)
	id3, _ := s.PostMessage("getmsg-room-a", "Amp", "Another in room A", "decision", 0)

	msgs, err := s.GetMessagesByIDs([]int64{id1, id2, id3})
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

	msgs, err := s.GetMessagesByIDs([]int64{99999, 99998})
	if err != nil {
		t.Fatalf("getMessagesByIDs failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestGetMessagesByIDsEmpty(t *testing.T) {
	s := setupTestServer(t)

	msgs, err := s.GetMessagesByIDs([]int64{})
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
		s.PostMessage("recent-room", "Claude", fmt.Sprintf("Message %d", i), "message", 0)
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
		s.PostMessage("recent-default", "Claude", fmt.Sprintf("Message %d", i), "message", 0)
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
		s.PostMessage("recent-cap", "Claude", fmt.Sprintf("Message %d", i), "message", 0)
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
	id1, _ := s.PostMessage("delmsg-room", "Claude", "Keep this", "message", 0)
	id2, _ := s.PostMessage("delmsg-room", "Gemini", "Delete this", "message", 0)
	id3, _ := s.PostMessage("delmsg-room", "Claude", "Delete this too", "message", 0)

	count, err := s.DeleteMessages([]int64{id2, id3})
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
		t.Errorf("expected message %d to remain, got %d", id1, msgs[0].ID)
	}
}

func TestDeleteMessagesNonexistent(t *testing.T) {
	s := setupTestServer(t)

	count, err := s.DeleteMessages([]int64{99999})
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

	_, err := s.UpdateMessage(99999, "Nope", "")
	if err == nil {
		t.Error("expected error for nonexistent message")
	}
}

// -- postMessage: default type branch --

func TestPostMessageDefaultType(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "default-type")

	id, err := s.PostMessage("default-type", "Claude", "Hello", "", 0)
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}

	msgs, _ := s.GetMessagesByIDs([]int64{id})
	if msgs[0].MessageType != "message" {
		t.Errorf("expected default type 'message', got '%s'", msgs[0].MessageType)
	}
}

// -- searchMessages limit edge cases --

func TestSearchMessagesLimitClamping(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "search-clamp")
	for i := 0; i < 5; i++ {
		mustPost(t, s, "search-clamp", "Claude", "keyword")
	}

	// Negative limit should clamp to 20
	msgs, _ := s.SearchMessages("keyword", "", "", "", "", -5)
	if len(msgs) != 5 {
		t.Errorf("expected 5 (all) with clamped limit, got %d", len(msgs))
	}

	// Over-100 limit should clamp to 20
	msgs, _ = s.SearchMessages("keyword", "", "", "", "", 200)
	if len(msgs) != 5 {
		t.Errorf("expected 5 with clamped limit, got %d", len(msgs))
	}
}

// -- deleteMessages with empty slice --

func TestDeleteMessagesEmptySlice(t *testing.T) {
	s := setupTestServer(t)
	count, err := s.DeleteMessages([]int64{})
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
	if id <= 0 {
		t.Errorf("expected positive message ID, got %d", id)
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
