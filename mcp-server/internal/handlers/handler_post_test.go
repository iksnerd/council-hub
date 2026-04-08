package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== post_to_room ==========

func TestHandlePostToRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-post")

	res, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
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
	reg := setupHandlerTest(t)

	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing fields, got: %s", text)
	}
}

func TestHandlePostToRoomInvalidType(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-post-bad")

	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post-bad", Author: "Claude", Message: "Hello", MessageType: "invalid",
	})
	text := resultText(res)
	if !strings.Contains(text, "Invalid message_type") {
		t.Errorf("expected invalid type error, got: %s", text)
	}
}

func TestHandlePostToRoomNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "nonexistent", Author: "Claude", Message: "Hello",
	})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found error, got: %s", text)
	}
}

func TestHandlePostToRoomDefaultType(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-post-default")

	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post-default", Author: "Claude", Message: "Hello",
	})
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("expected success, got: %s", text)
	}
}

func TestHandlePostToRoomWithReplyTo(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-reply")
	id := mustPost(t, reg.Server, "h-reply", "Claude", "Original")

	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-reply", Author: "Gemini", Message: "Reply",
		ReplyTo: id,
	})
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("expected success, got: %s", text)
	}

	msgs, _ := reg.Server.GetRecentMessages("h-reply", 2)
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

func TestHandlePostToRoomAnyReplyToAccepted(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-reply-any")

	// Any string is accepted as reply_to (UUID or otherwise)
	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-reply-any", Author: "Claude", Message: "Hello",
		ReplyTo: "any-string-reply-to",
	})
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("expected success for any string reply_to, got: %s", text)
	}
}

func TestHandlePostToRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	// Post needs getRoom to succeed first, so we drop messages table instead
	reg.Server.DB.Exec("DROP TABLE messages")

	_, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "hdb-room", Author: "Claude", Message: "fail",
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestHandlePostToRoomEmoji(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-post-emoji")

	content := "Hello 🌍 from Claude 🤖 — distributed systems are 💯"
	res, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post-emoji", Author: "Claude", Message: content,
	})
	if err != nil {
		t.Fatalf("handlePostToRoom error: %v", err)
	}
	if !strings.Contains(resultText(res), "posted") {
		t.Errorf("expected success, got: %s", resultText(res))
	}

	msgs, err := reg.Server.GetRecentMessages("h-post-emoji", 1)
	if err != nil {
		t.Fatalf("GetRecentMessages error: %v", err)
	}
	if len(msgs) == 0 || msgs[0].Content != content {
		t.Errorf("emoji content not round-tripped correctly, got: %q", msgs[0].Content)
	}
}

func TestHandlePostToRoomSingleQuote(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-post-quote")

	content := "it's O'Brien's room; don't forget"
	res, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post-quote", Author: "Claude", Message: content,
	})
	if err != nil {
		t.Fatalf("handlePostToRoom error: %v", err)
	}
	if !strings.Contains(resultText(res), "posted") {
		t.Errorf("expected success, got: %s", resultText(res))
	}

	msgs, err := reg.Server.GetRecentMessages("h-post-quote", 1)
	if err != nil {
		t.Fatalf("GetRecentMessages error: %v", err)
	}
	if len(msgs) == 0 || msgs[0].Content != content {
		t.Errorf("single-quote content not round-tripped, got: %q", msgs[0].Content)
	}
}

func TestHandleSearchMessagesLikeWildcards(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-post-wildcards")
	mustPost(t, reg.Server, "h-post-wildcards", "Claude", "normal message here")

	// LIKE wildcards in query should not cause a crash or return unexpected rows
	res, _, err := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-post-wildcards", Query: "%_wildcards%",
	})
	if err != nil {
		t.Fatalf("handleSearchMessages error: %v", err)
	}
	// The literal string "%_wildcards%" should not match "normal message here"
	text := resultText(res)
	if strings.Contains(text, "normal message here") {
		t.Errorf("LIKE wildcards in query should be treated as literals, got: %s", text)
	}
}

// ========== mentions + get_mentions ==========

func TestHandlePostToRoomWithMentions(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-mention")

	res, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-mention", Author: "agent-a", Message: "Hey @agent-b, check this out",
		Mentions: "agent-b",
	})
	if err != nil {
		t.Fatalf("handlePostToRoom error: %v", err)
	}
	if !strings.Contains(resultText(res), "posted") {
		t.Errorf("expected posted confirmation, got: %s", resultText(res))
	}

	// Verify mentions stored in DB
	msgs, _ := reg.Server.GetRecentMessages("h-mention", 1)
	if len(msgs) == 0 {
		t.Fatal("no messages found")
	}
	if msgs[0].Mentions != "agent-b" {
		t.Errorf("expected mentions 'agent-b', got '%s'", msgs[0].Mentions)
	}
}

func TestHandleGetMentions(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-get-mentions")

	// Post two messages that mention claude
	reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-get-mentions", Author: "gemini", Message: "Claude, please review",
		Mentions: "claude",
	})
	reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-get-mentions", Author: "amp", Message: "Also for claude and gemini",
		Mentions: "claude,gemini",
	})
	// Post without mentioning claude
	reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-get-mentions", Author: "system", Message: "No mentions",
	})

	res, _, err := reg.handleGetMentions(context.Background(), nil, GetMentionsInput{Author: "claude"})
	if err != nil {
		t.Fatalf("handleGetMentions error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "claude") {
		t.Errorf("expected mention of claude in output, got: %s", text)
	}
	if !strings.Contains(text, "Found 2") {
		t.Errorf("expected 2 mentions, got: %s", text)
	}
}

func TestHandleGetMentionsMissingAuthor(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleGetMentions(context.Background(), nil, GetMentionsInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Errorf("expected error for missing author, got: %s", resultText(res))
	}
}

func TestHandleGetMentionsNone(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-no-mentions")
	mustPost(t, reg.Server, "h-no-mentions", "bot", "hello world")

	res, _, err := reg.handleGetMentions(context.Background(), nil, GetMentionsInput{Author: "nobody"})
	if err != nil {
		t.Fatalf("handleGetMentions error: %v", err)
	}
	if !strings.Contains(resultText(res), "No messages mention") {
		t.Errorf("expected 'No messages mention', got: %s", resultText(res))
	}
}
