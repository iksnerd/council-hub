package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ========== get_messages ==========

func TestHandleGetMessagesByRoomID(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-get-room")
	for i := 0; i < 5; i++ {
		mustPost(t, reg.Server, "h-get-room", "Claude", fmt.Sprintf("Msg %d", i))
	}

	// Browse by room_id + last_n
	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{
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
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-get-room-def")
	for i := 0; i < 15; i++ {
		mustPost(t, reg.Server, "h-get-room-def", "Claude", fmt.Sprintf("Msg %d", i))
	}

	// room_id without last_n should default to 10
	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "h-get-room-def",
	})
	text := resultText(res)
	if !strings.Contains(text, "10 message(s)") {
		t.Errorf("expected 10 messages with default limit, got: %s", text)
	}
}

func TestHandleGetMessagesByRoomIDNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "nonexistent",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for nonexistent room, got: %s", text)
	}
}

func TestHandleGetMessagesNoParams(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{})
	text := resultText(res)
	if !strings.Contains(text, "provide either message_ids or room_id") {
		t.Errorf("expected parameter error, got: %s", text)
	}
}

func TestHandleGetMessagesByRoomIDBadLimit(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-get-bad-lim")
	mustPost(t, reg.Server, "h-get-bad-lim", "Claude", "Hello")

	// Invalid last_n should fall back to 10
	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "h-get-bad-lim", LastN: "xyz",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message with fallback limit, got: %s", text)
	}
}

func TestHandleGetMessagesIDsTakePrecedence(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-get-prio")
	id1 := mustPost(t, reg.Server, "h-get-prio", "Claude", "Message one")
	mustPostTyped(t, reg.Server, "h-get-prio", "Gemini", "Message two", "review")

	// If both message_ids and room_id are provided, message_ids takes precedence
	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{
		MessageIDs: id1, RoomID: "h-get-prio",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message (by ID), got: %s", text)
	}
}

func TestHandleGetMessagesMissing(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{})
	if !strings.Contains(resultText(res), "provide either") {
		t.Error("expected error for missing params")
	}
}

func TestHandleGetMessagesNonExistentID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "fake-nonexistent-uuid"})
	if !strings.Contains(resultText(res), "No messages found") {
		t.Error("expected no messages for non-existent UUID")
	}
}

func TestHandleGetMessagesNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "99999"})
	if !strings.Contains(resultText(res), "No messages found") {
		t.Error("expected no messages found")
	}
}

func TestHandleGetMessagesMultiple(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-get-multi")
	id1 := mustPost(t, reg.Server, "h-get-multi", "Claude", "First")
	id2 := mustPostTyped(t, reg.Server, "h-get-multi", "Gemini", "Second", "review")

	res, _, _ := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: id1 + "," + id2})
	text := resultText(res)
	if !strings.Contains(text, "2 message(s)") {
		t.Errorf("expected 2 messages, got: %s", text)
	}
}

func TestHandleGetMessagesDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	_, _, err := reg.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "1"})
	if err == nil {
		t.Error("expected error")
	}
}

// read_recent was removed in v0.5.0 — use read_transcript(last_n=N) instead
