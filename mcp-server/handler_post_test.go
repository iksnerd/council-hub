package main

import (
	"context"
	"strings"
	"testing"
)

// ========== post_to_room ==========

func TestHandlePostToRoom(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-post")

	res, _, err := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post", Author: "Claude", Message: "Hello", MessageType: "thought",
	})
	if err != nil {
		t.Fatalf("handlePostToRoom error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("unexpected result: %s", text)
	}
}

func TestHandlePostToRoomMissingFields(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing fields, got: %s", text)
	}
}

func TestHandlePostToRoomInvalidType(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-post-bad")

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post-bad", Author: "Claude", Message: "Hello", MessageType: "invalid",
	})
	text := resultText(res)
	if !strings.Contains(text, "Invalid message_type") {
		t.Errorf("expected invalid type error, got: %s", text)
	}
}

func TestHandlePostToRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "nonexistent", Author: "Claude", Message: "Hello",
	})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found error, got: %s", text)
	}
}

func TestHandlePostToRoomDefaultType(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-post-default")

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post-default", Author: "Claude", Message: "Hello",
	})
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("expected success, got: %s", text)
	}
}

func TestHandlePostToRoomWithReplyTo(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-reply")
	id := mustPost(t, cs, "h-reply", "Claude", "Original")

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-reply", Author: "Gemini", Message: "Reply",
		ReplyTo: "1",
	})
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("expected success, got: %s", text)
	}

	msgs, _ := cs.getRecentMessages("h-reply", 2)
	// The reply should be the second message
	found := false
	for _, m := range msgs {
		if m.Author == "Gemini" && m.ReplyTo == id {
			found = true
		}
	}
	if !found {
		t.Error("reply_to not preserved through handler")
	}
}

func TestHandlePostToRoomInvalidReplyTo(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-reply-bad")

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-reply-bad", Author: "Claude", Message: "Hello",
		ReplyTo: "not-a-number",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for bad reply_to, got: %s", text)
	}
}

func TestHandlePostToRoomDBError(t *testing.T) {
	cs := setupHandlerServer(t)
	// Post needs getRoom to succeed first, so we drop messages table instead
	cs.db.Exec("DROP TABLE messages")

	_, _, err := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "hdb-room", Author: "Claude", Message: "fail",
	})
	if err == nil {
		t.Error("expected error")
	}
}
