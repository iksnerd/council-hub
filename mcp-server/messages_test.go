package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestPostMessage(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("msg-room", "Message test", "", "", "", "", "")

	id1, err := cs.postMessage("msg-room", "Claude", "Hello from Claude", "message", 0)
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}
	id2, err := cs.postMessage("msg-room", "Gemini", "Hello from Gemini", "message", 0)
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}

	if id2 <= id1 {
		t.Errorf("expected id2 > id1, got id1=%d id2=%d", id1, id2)
	}

	msgs, err := cs.getTranscript("msg-room")
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
	cs := setupTestServer(t)
	cs.createRoom("type-room", "Type test", "", "", "", "", "")

	cs.postMessage("type-room", "Claude", "I think we should...", "thought", 0)
	cs.postMessage("type-room", "Gemini", "Let's go with RS256", "decision", 0)
	cs.postMessage("type-room", "Claude", "func main() {}", "code", 0)

	msgs, _ := cs.getTranscript("type-room")
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
	cs := setupTestServer(t)
	cs.createRoom("critique-room", "Critique test", "", "", "", "", "")

	_, err := cs.postMessage("critique-room", "Claude", "This approach has flaws", "critique", 0)
	if err != nil {
		t.Fatalf("postMessage with critique type failed: %v", err)
	}

	msgs, _ := cs.getTranscript("critique-room")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].MessageType != "critique" {
		t.Errorf("expected 'critique', got '%s'", msgs[0].MessageType)
	}

	room, _ := cs.getRoom("critique-room")
	transcript := formatTranscript(room, msgs)
	if !strings.Contains(transcript, "Claude (critique)") {
		t.Error("transcript missing critique message type")
	}
}

func TestPostMessageWithReplyTo(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("reply-room", "Reply test", "", "", "", "", "")

	id1, _ := cs.postMessage("reply-room", "Claude", "Original message", "message", 0)
	id2, _ := cs.postMessage("reply-room", "Gemini", "Replying to Claude", "review", id1)

	msgs, _ := cs.getTranscript("reply-room")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].ReplyTo != id1 {
		t.Errorf("expected reply_to %d, got %d", id1, msgs[1].ReplyTo)
	}

	// Verify transcript rendering includes reply tag
	room, _ := cs.getRoom("reply-room")
	transcript := formatTranscript(room, msgs)
	expected := fmt.Sprintf("re: #%d", id1)
	if !strings.Contains(transcript, expected) {
		t.Errorf("transcript missing reply tag '%s'", expected)
	}

	// Verify non-reply message has no reply tag
	if msgs[0].ReplyTo != 0 {
		t.Errorf("expected reply_to 0 for original message, got %d", msgs[0].ReplyTo)
	}

	// Get by ID and verify reply_to is preserved
	fetched, _ := cs.getMessagesByIDs([]int64{id2})
	if len(fetched) != 1 {
		t.Fatalf("expected 1 message, got %d", len(fetched))
	}
	if fetched[0].ReplyTo != id1 {
		t.Errorf("getMessagesByIDs: expected reply_to %d, got %d", id1, fetched[0].ReplyTo)
	}
}

func TestReplyToInReadRecent(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("reply-recent", "Reply recent test", "", "", "", "", "")

	id1, _ := cs.postMessage("reply-recent", "Claude", "First", "message", 0)
	cs.postMessage("reply-recent", "Gemini", "Reply to first", "critique", id1)

	msgs, err := cs.getRecentMessages("reply-recent", 2)
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
	cs := setupTestServer(t)
	cs.createRoom("search-room-1", "Room 1", "proj", "", "", "", "")
	cs.createRoom("search-room-2", "Room 2", "proj", "", "", "", "")
	cs.postMessage("search-room-1", "Claude", "JWT token validation is broken", "thought", 0)
	cs.postMessage("search-room-1", "Gemini", "I agree about the JWT issue", "review", 0)
	cs.postMessage("search-room-2", "Claude", "Database migration complete", "action", 0)

	// Search by keyword
	msgs, err := cs.searchMessages("JWT", "", "", "", 20)
	if err != nil {
		t.Fatalf("searchMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages with 'JWT', got %d", len(msgs))
	}

	// Search by author
	msgs, _ = cs.searchMessages("", "Claude", "", "", 20)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages from Claude, got %d", len(msgs))
	}

	// Search by message type
	msgs, _ = cs.searchMessages("", "", "review", "", 20)
	if len(msgs) != 1 {
		t.Errorf("expected 1 review message, got %d", len(msgs))
	}

	// Search scoped to room
	msgs, _ = cs.searchMessages("", "Claude", "", "search-room-2", 20)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message from Claude in search-room-2, got %d", len(msgs))
	}

	// No results
	msgs, _ = cs.searchMessages("nonexistent", "", "", "", 20)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestSearchMessagesGlobal(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("search-global-a", "Room A", "proj", "", "", "", "")
	cs.createRoom("search-global-b", "Room B", "proj", "", "", "", "")
	cs.postMessage("search-global-a", "Claude", "BEP 44 analysis here", "thought", 0)
	cs.postMessage("search-global-b", "Gemini", "BEP 46 analysis here", "review", 0)

	msgs, err := cs.searchMessages("BEP", "", "", "", 20)
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
	cs := setupTestServer(t)
	cs.createRoom("search-snippet", "Snippet test", "", "", "", "", "")
	longContent := strings.Repeat("A", 400)
	cs.postMessage("search-snippet", "Claude", longContent, "message", 0)

	msgs, err := cs.searchMessages("AAAA", "", "", "search-snippet", 1)
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
	cs := setupTestServer(t)
	cs.createRoom("getmsg-room", "Get messages test", "", "", "", "", "")
	id1, _ := cs.postMessage("getmsg-room", "Claude", "Full content of message one with lots of detail", "thought", 0)

	msgs, err := cs.getMessagesByIDs([]int64{id1})
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
	cs := setupTestServer(t)
	cs.createRoom("getmsg-room-a", "Room A", "", "", "", "", "")
	cs.createRoom("getmsg-room-b", "Room B", "", "", "", "", "")
	id1, _ := cs.postMessage("getmsg-room-a", "Claude", "Message in room A", "message", 0)
	id2, _ := cs.postMessage("getmsg-room-b", "Gemini", "Message in room B", "review", 0)
	id3, _ := cs.postMessage("getmsg-room-a", "Amp", "Another in room A", "decision", 0)

	msgs, err := cs.getMessagesByIDs([]int64{id1, id2, id3})
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
	cs := setupTestServer(t)

	msgs, err := cs.getMessagesByIDs([]int64{99999, 99998})
	if err != nil {
		t.Fatalf("getMessagesByIDs failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestGetMessagesByIDsEmpty(t *testing.T) {
	cs := setupTestServer(t)

	msgs, err := cs.getMessagesByIDs([]int64{})
	if err != nil {
		t.Fatalf("getMessagesByIDs failed: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil, got %v", msgs)
	}
}

func TestGetRecentMessages(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("recent-room", "Recent test", "", "", "", "", "")
	for i := 0; i < 10; i++ {
		cs.postMessage("recent-room", "Claude", fmt.Sprintf("Message %d", i), "message", 0)
	}

	msgs, err := cs.getRecentMessages("recent-room", 3)
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
	cs := setupTestServer(t)
	cs.createRoom("recent-default", "Default limit test", "", "", "", "", "")
	for i := 0; i < 15; i++ {
		cs.postMessage("recent-default", "Claude", fmt.Sprintf("Message %d", i), "message", 0)
	}

	msgs, err := cs.getRecentMessages("recent-default", 0)
	if err != nil {
		t.Fatalf("getRecentMessages failed: %v", err)
	}
	if len(msgs) != 10 {
		t.Errorf("expected default 10 messages, got %d", len(msgs))
	}
}

func TestGetRecentMessagesOverLimit(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("recent-cap", "Cap test", "", "", "", "", "")
	for i := 0; i < 60; i++ {
		cs.postMessage("recent-cap", "Claude", fmt.Sprintf("Message %d", i), "message", 0)
	}

	msgs, err := cs.getRecentMessages("recent-cap", 100)
	if err != nil {
		t.Fatalf("getRecentMessages failed: %v", err)
	}
	if len(msgs) != 50 {
		t.Errorf("expected capped 50 messages, got %d", len(msgs))
	}
}

func TestGetRecentMessagesEmptyRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("recent-empty", "Empty room", "", "", "", "", "")

	msgs, err := cs.getRecentMessages("recent-empty", 5)
	if err != nil {
		t.Fatalf("getRecentMessages failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestGetRecentMessagesNotFound(t *testing.T) {
	cs := setupTestServer(t)

	_, err := cs.getRecentMessages("nonexistent", 5)
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestDeleteMessages(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("delmsg-room", "Delete messages test", "", "", "", "", "")
	id1, _ := cs.postMessage("delmsg-room", "Claude", "Keep this", "message", 0)
	id2, _ := cs.postMessage("delmsg-room", "Gemini", "Delete this", "message", 0)
	id3, _ := cs.postMessage("delmsg-room", "Claude", "Delete this too", "message", 0)

	count, err := cs.deleteMessages([]int64{id2, id3})
	if err != nil {
		t.Fatalf("deleteMessages failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 deleted, got %d", count)
	}

	msgs, _ := cs.getTranscript("delmsg-room")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(msgs))
	}
	if msgs[0].ID != id1 {
		t.Errorf("expected message %d to remain, got %d", id1, msgs[0].ID)
	}
}

func TestDeleteMessagesNonexistent(t *testing.T) {
	cs := setupTestServer(t)

	count, err := cs.deleteMessages([]int64{99999})
	if err != nil {
		t.Fatalf("deleteMessages failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 deleted, got %d", count)
	}
}
