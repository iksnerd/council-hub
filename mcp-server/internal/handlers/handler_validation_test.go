package handlers

import (
	"context"
	"strings"
	"testing"
)

func TestValidateSize(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		if err := validateSize("field", "short", 100); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("at limit", func(t *testing.T) {
		val := strings.Repeat("a", 255)
		if err := validateSize("field", val, 255); err != nil {
			t.Errorf("expected no error at exact limit, got: %v", err)
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		val := strings.Repeat("a", 256)
		err := validateSize("field", val, 255)
		if err == nil {
			t.Error("expected error for oversized input")
		}
		if !strings.Contains(err.Error(), "exceeds maximum length") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("empty is ok", func(t *testing.T) {
		if err := validateSize("field", "", 10); err != nil {
			t.Errorf("expected no error for empty string, got: %v", err)
		}
	})
}

func TestValidateRoomMetadata(t *testing.T) {
	t.Run("all within limits", func(t *testing.T) {
		if err := validateRoomMetadata("topic", "proj", "Go", "tag", "prompt"); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("oversized topic", func(t *testing.T) {
		big := strings.Repeat("x", maxMetadataLen+1)
		err := validateRoomMetadata(big, "", "", "", "")
		if err == nil {
			t.Error("expected error for oversized topic")
		}
		if !strings.Contains(err.Error(), "topic") {
			t.Errorf("error should mention field name 'topic', got: %v", err)
		}
	})

	t.Run("oversized system_prompt", func(t *testing.T) {
		big := strings.Repeat("x", maxMetadataLen+1)
		err := validateRoomMetadata("", "", "", "", big)
		if err == nil {
			t.Error("expected error for oversized system_prompt")
		}
		if !strings.Contains(err.Error(), "system_prompt") {
			t.Errorf("error should mention 'system_prompt', got: %v", err)
		}
	})
}

func TestPostToRoomOversizedMessage(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-oversize")

	bigMsg := strings.Repeat("x", maxContentLen+1)
	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-oversize", Author: "Claude", Message: bigMsg,
	})
	text := resultText(res)
	if !strings.Contains(text, "exceeds maximum length") {
		t.Errorf("expected size validation error, got: %s", text)
	}
}

func TestPostToRoomOversizedAuthor(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-oversize-author")

	bigAuthor := strings.Repeat("x", maxAuthorLen+1)
	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-oversize-author", Author: bigAuthor, Message: "Hello",
	})
	text := resultText(res)
	if !strings.Contains(text, "exceeds maximum length") {
		t.Errorf("expected size validation error, got: %s", text)
	}
}

func TestCreateRoomOversizedID(t *testing.T) {
	reg := setupHandlerTest(t)

	bigID := strings.Repeat("x", maxIDLen+1)
	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: bigID,
	})
	text := resultText(res)
	if !strings.Contains(text, "exceeds maximum length") {
		t.Errorf("expected size validation error, got: %s", text)
	}
}

func TestCreateRoomOversizedMetadata(t *testing.T) {
	reg := setupHandlerTest(t)

	bigPrompt := strings.Repeat("x", maxMetadataLen+1)
	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "h-meta-big", SystemPrompt: bigPrompt,
	})
	text := resultText(res)
	if !strings.Contains(text, "exceeds maximum length") {
		t.Errorf("expected size validation error, got: %s", text)
	}
}

func TestGetOrCreateRoomOversizedID(t *testing.T) {
	reg := setupHandlerTest(t)

	bigID := strings.Repeat("x", maxIDLen+1)
	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: bigID,
	})
	text := resultText(res)
	if !strings.Contains(text, "exceeds maximum length") {
		t.Errorf("expected size validation error, got: %s", text)
	}
}

func TestUpdateMessageOversizedContent(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-update-big")
	msgID := mustPost(t, reg.Server, "h-update-big", "Claude", "original")

	bigContent := strings.Repeat("x", maxContentLen+1)
	res, _, _ := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: msgID, Content: bigContent,
	})
	text := resultText(res)
	if !strings.Contains(text, "exceeds maximum length") {
		t.Errorf("expected size validation error, got: %s", text)
	}
}

func TestPostToRoomWithinLimits(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-within-limits")

	// At-limit values should succeed
	msg := strings.Repeat("x", maxContentLen)
	res, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-within-limits", Author: "Claude", Message: msg,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("expected success, got: %s", text)
	}
}
