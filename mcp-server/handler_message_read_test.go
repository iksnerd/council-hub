package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ========== get_messages ==========

func TestHandleGetMessagesByRoomID(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-get-room")
	for i := 0; i < 5; i++ {
		mustPost(t, cs, "h-get-room", "Claude", fmt.Sprintf("Msg %d", i))
	}

	// Browse by room_id + last_n
	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "h-get-room", LastN: "3",
	})
	text := resultText(res)
	if !strings.Contains(text, "3 message(s)") {
		t.Errorf("expected 3 messages, got: %s", text)
	}
	if !strings.Contains(text, "Msg 4") {
		t.Error("expected last message in browse result")
	}
	if strings.Contains(text, "Msg 0") {
		t.Error("Msg 0 should not be in last 3")
	}
}

func TestHandleGetMessagesByRoomIDDefaultLimit(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-get-room-def")
	for i := 0; i < 15; i++ {
		mustPost(t, cs, "h-get-room-def", "Claude", fmt.Sprintf("Msg %d", i))
	}

	// room_id without last_n should default to 10
	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "h-get-room-def",
	})
	text := resultText(res)
	if !strings.Contains(text, "10 message(s)") {
		t.Errorf("expected 10 messages with default limit, got: %s", text)
	}
}

func TestHandleGetMessagesByRoomIDNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "nonexistent",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for nonexistent room, got: %s", text)
	}
}

func TestHandleGetMessagesNoParams(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{})
	text := resultText(res)
	if !strings.Contains(text, "provide either message_ids or room_id") {
		t.Errorf("expected parameter error, got: %s", text)
	}
}

func TestHandleGetMessagesByRoomIDBadLimit(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-get-bad-lim")
	mustPost(t, cs, "h-get-bad-lim", "Claude", "Hello")

	// Invalid last_n should fall back to 10
	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "h-get-bad-lim", LastN: "xyz",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message with fallback limit, got: %s", text)
	}
}

func TestHandleGetMessagesIDsTakePrecedence(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-get-prio")
	mustPost(t, cs, "h-get-prio", "Claude", "Message one")
	mustPostTyped(t, cs, "h-get-prio", "Gemini", "Message two", "review")

	// If both message_ids and room_id are provided, message_ids takes precedence
	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		MessageIDs: "1", RoomID: "h-get-prio",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message (by ID), got: %s", text)
	}
}

func TestHandleGetMessagesMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{})
	if !strings.Contains(resultText(res), "provide either") {
		t.Error("expected error for missing params")
	}
}

func TestHandleGetMessagesInvalidID(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "abc"})
	if !strings.Contains(resultText(res), "not a valid") {
		t.Error("expected invalid ID error")
	}
}

func TestHandleGetMessagesNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "99999"})
	if !strings.Contains(resultText(res), "No messages found") {
		t.Error("expected no messages found")
	}
}

func TestHandleGetMessagesMultiple(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-get-multi")
	mustPost(t, cs, "h-get-multi", "Claude", "First")
	mustPostTyped(t, cs, "h-get-multi", "Gemini", "Second", "review")

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "1,2"})
	text := resultText(res)
	if !strings.Contains(text, "2 message(s)") {
		t.Errorf("expected 2 messages, got: %s", text)
	}
}

func TestHandleGetMessagesDBError(t *testing.T) {
	cs := setupHandlerServer(t)
	cs.db.Close()

	_, _, err := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "1"})
	if err == nil {
		t.Error("expected error")
	}
}

// ========== read_recent ==========

func TestHandleReadRecent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-recent")
	mustPost(t, cs, "h-recent", "Claude", "Message 1")
	mustPostTyped(t, cs, "h-recent", "Gemini", "Message 2", "review")

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent", Limit: "1",
	})
	text := resultText(res)
	if !strings.Contains(text, "last 1 message(s)") {
		t.Errorf("expected 1 message, got: %s", text)
	}
	if !strings.Contains(text, "Message 2") {
		t.Error("expected most recent message")
	}
}

func TestHandleReadRecentMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing room_id")
	}
}

func TestHandleReadRecentNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "nonexistent",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for nonexistent room, got: %s", text)
	}
}

func TestHandleReadRecentBadLimit(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-recent-bad")
	mustPost(t, cs, "h-recent-bad", "Claude", "Hello")

	// Invalid limit string should fall back to default 10
	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent-bad", Limit: "abc",
	})
	text := resultText(res)
	if !strings.Contains(text, "last 1 message(s)") {
		t.Errorf("expected fallback to default limit, got: %s", text)
	}
}

func TestHandleReadRecentWithSummary(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-recent-sum")
	cs.insertSummary("h-recent-sum", "Summary content")
	mustPost(t, cs, "h-recent-sum", "Claude", "After summary")

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent-sum", Limit: "10",
	})
	text := resultText(res)
	if !strings.Contains(text, "SUMMARY") {
		t.Error("expected summary rendering in read_recent")
	}
}

func TestHandleReadRecentReplyToPlainMessage(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-recent-reply-plain")
	id1 := mustPost(t, cs, "h-recent-reply-plain", "Claude", "Original")
	// Reply with default "message" type — tests the else-if branch for ReplyTo > 0 with message type
	mustPostReply(t, cs, "h-recent-reply-plain", "Gemini", "Replying", id1)

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent-reply-plain", Limit: "5",
	})
	text := resultText(res)
	expected := fmt.Sprintf("re: #%d", id1)
	if !strings.Contains(text, expected) {
		t.Errorf("expected reply tag in plain message, got: %s", text)
	}
}

func TestHandleReadRecentAllBranches(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-recent-all")
	id1 := mustPost(t, cs, "h-recent-all", "Claude", "Plain message")
	mustPostTyped(t, cs, "h-recent-all", "Gemini", "A thought", "thought")
	mustPostReply(t, cs, "h-recent-all", "Claude", "Reply plain", id1)
	cs.postMessage("h-recent-all", "Gemini", "Reply typed", "review", id1)
	cs.insertSummary("h-recent-all", "A summary")

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent-all", Limit: "50",
	})
	text := resultText(res)

	// Plain message (no type label)
	if !strings.Contains(text, "Claude:**") {
		t.Error("missing plain message rendering")
	}
	// Typed message
	if !strings.Contains(text, "Gemini (thought)") {
		t.Error("missing typed message rendering")
	}
	// Plain reply
	if !strings.Contains(text, fmt.Sprintf("Claude (re: #%d):", id1)) {
		t.Error("missing plain reply rendering")
	}
	// Typed reply
	if !strings.Contains(text, fmt.Sprintf("Gemini (review, re: #%d):", id1)) {
		t.Error("missing typed reply rendering")
	}
	// Summary
	if !strings.Contains(text, "SUMMARY") {
		t.Error("missing summary rendering")
	}
}

func TestHandleReadRecentDBError(t *testing.T) {
	cs := setupHandlerServer(t)
	// Drop messages but keep rooms so room lookup succeeds, query fails
	cs.db.Exec("DROP TABLE messages")

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{RoomID: "hdb-room"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}
