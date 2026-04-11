package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== react_to_message ==========

func TestHandleReactToMessageMissingMessageID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReactToMessage(context.Background(), nil, ReactInput{
		Emoji: "👍", Author: "Claude",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing message_id")
	}
}

func TestHandleReactToMessageMissingEmoji(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReactToMessage(context.Background(), nil, ReactInput{
		MessageID: "some-id", Author: "Claude",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing emoji")
	}
}

func TestHandleReactToMessageMissingAuthor(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReactToMessage(context.Background(), nil, ReactInput{
		MessageID: "some-id", Emoji: "👍",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing author")
	}
}

func TestHandleReactToMessageNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReactToMessage(context.Background(), nil, ReactInput{
		MessageID: "nonexistent-id", Emoji: "👍", Author: "Claude",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for nonexistent message, got: %s", text)
	}
}

func TestHandleReactToMessageAdd(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "react-room")
	msgID := mustPost(t, reg.Server, "react-room", "Claude", "React to me")

	res, _, err := reg.handleReactToMessage(context.Background(), nil, ReactInput{
		MessageID: msgID, Emoji: "👍", Author: "Gemini",
	})
	if err != nil {
		t.Fatalf("handleReactToMessage error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "reaction added") {
		t.Errorf("expected 'reaction added', got: %s", text)
	}
	if !strings.Contains(text, "👍") {
		t.Errorf("expected emoji in response, got: %s", text)
	}
	if !strings.Contains(text, "Gemini") {
		t.Errorf("expected author in response, got: %s", text)
	}
}

func TestHandleReactToMessageToggleOff(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "react-toggle-room")
	msgID := mustPost(t, reg.Server, "react-toggle-room", "Claude", "Toggle me")

	// Add reaction
	reg.handleReactToMessage(context.Background(), nil, ReactInput{ //nolint:errcheck
		MessageID: msgID, Emoji: "❤️", Author: "Claude",
	})

	// Toggle it off
	res, _, _ := reg.handleReactToMessage(context.Background(), nil, ReactInput{
		MessageID: msgID, Emoji: "❤️", Author: "Claude",
	})
	text := resultText(res)
	if !strings.Contains(text, "reaction removed") {
		t.Errorf("expected 'reaction removed', got: %s", text)
	}
}

func TestHandleReactToMessageMultipleAuthors(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "react-multi-room")
	msgID := mustPost(t, reg.Server, "react-multi-room", "Claude", "Multi reactions")

	reg.handleReactToMessage(context.Background(), nil, ReactInput{ //nolint:errcheck
		MessageID: msgID, Emoji: "👍", Author: "Claude",
	})
	res, _, _ := reg.handleReactToMessage(context.Background(), nil, ReactInput{
		MessageID: msgID, Emoji: "👍", Author: "Gemini",
	})
	text := resultText(res)
	// Should show count of 2 for the emoji
	if !strings.Contains(text, "👍 2") {
		t.Errorf("expected '👍 2' in reaction summary, got: %s", text)
	}
}

func TestHandleReactToMessageDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	msgID := "hdb-msg"
	// get any message from the pre-populated server DB
	msgs, _ := reg.Server.GetRecentMessages("hdb-room", 1)
	if len(msgs) > 0 {
		msgID = msgs[0].ID
	}
	reg.Server.DB.Close()

	res, _, _ := reg.handleReactToMessage(context.Background(), nil, ReactInput{
		MessageID: msgID, Emoji: "👍", Author: "Claude",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error on closed DB, got: %s", text)
	}
}
