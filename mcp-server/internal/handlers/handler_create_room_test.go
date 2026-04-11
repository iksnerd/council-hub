package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== create_room ==========

func TestHandleCreateRoom(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "h-room", Topic: "Handler test", Project: "proj",
	})
	if err != nil {
		t.Fatalf("handleCreateRoom error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "h-room") || !strings.Contains(text, "created") {
		t.Errorf("unexpected result: %s", text)
	}

	room, _ := reg.Server.GetRoom("h-room")
	if room.Project != "proj" {
		t.Errorf("expected project 'proj', got '%s'", room.Project)
	}
}

func TestHandleCreateRoomMissingID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing ID, got: %s", text)
	}
}

func TestHandleCreateRoomWithRelatedRooms(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "h-create-related", Topic: "With links", RelatedRooms: "a,b,c",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "created") {
		t.Errorf("expected created, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("h-create-related")
	if room.RelatedRooms != "a,b,c" {
		t.Errorf("expected related_rooms 'a,b,c', got '%s'", room.RelatedRooms)
	}
}

func TestHandleCreateRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	_, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{ID: "fail"})
	if err == nil {
		t.Error("expected error")
	}
}

// ========== duplicate room detection ==========

func TestHandleCreateRoomDuplicateWarning(t *testing.T) {
	reg := setupHandlerTest(t)
	// Create an existing room with overlapping tags.
	mustCreateRoom(t, reg.Server, "existing-auth", withProject("myapp"), withTags("go,auth,api"))

	// Create a new room with overlapping tags — should get a warning.
	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID:      "new-auth-service",
		Topic:   "Authentication service",
		Project: "myapp",
		Tags:    "go,auth,backend",
	})
	text := resultText(res)
	if !strings.Contains(text, "new-auth-service") {
		t.Errorf("expected new room in response, got: %s", text)
	}
	if !strings.Contains(text, "Similar room") {
		t.Errorf("expected duplicate warning, got: %s", text)
	}
	if !strings.Contains(text, "existing-auth") {
		t.Errorf("expected existing room ID in warning, got: %s", text)
	}
}

func TestHandleCreateRoomNoDuplicateWhenUnrelated(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "infra-room", withProject("ops"), withTags("kubernetes,terraform"))

	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID:      "auth-room",
		Topic:   "User authentication",
		Project: "myapp",
		Tags:    "oauth,jwt",
	})
	text := resultText(res)
	if strings.Contains(text, "Similar room") {
		t.Errorf("unexpected duplicate warning for unrelated rooms: %s", text)
	}
}

func TestHandleGetOrCreateRoomDuplicateWarning(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "existing-cache", withProject("perf"), withTags("redis,caching,backend"))

	// get_or_create a new room (different ID) with overlapping tags.
	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID:      "new-cache-layer",
		Topic:   "Cache layer design",
		Project: "perf",
		Tags:    "redis,caching,go",
	})
	text := resultText(res)
	if !strings.Contains(text, "Similar room") {
		t.Errorf("expected duplicate warning on get_or_create, got: %s", text)
	}
	if !strings.Contains(text, "existing-cache") {
		t.Errorf("expected existing room in warning, got: %s", text)
	}
}

func TestHandleGetOrCreateRoomNoWarningOnExisting(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "my-room", withProject("proj"), withTags("go,api"))

	// Fetching an existing room should NOT trigger duplicate warning.
	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "my-room",
	})
	text := resultText(res)
	if strings.Contains(text, "Similar room") {
		t.Errorf("unexpected duplicate warning when fetching existing room: %s", text)
	}
}

// ========== create_room templates ==========

func TestHandleCreateRoomTemplate(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-room", Template: "decision-log",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "decision-log") {
		t.Errorf("expected template name in response, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("tpl-room")
	if !strings.Contains(room.Tags, "decision") {
		t.Errorf("expected decision tag from template, got: %s", room.Tags)
	}
	if room.SystemPrompt == "" {
		t.Errorf("expected system_prompt from template, got empty")
	}
}

func TestHandleCreateRoomTemplateOverride(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-override", Template: "sprint", Tags: "custom-tag",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resultText(res)

	room, _ := reg.Server.GetRoom("tpl-override")
	if room.Tags != "custom-tag" {
		t.Errorf("explicit tags should override template, got: %s", room.Tags)
	}
	if room.SystemPrompt == "" {
		t.Errorf("template system_prompt should still apply when tags were overridden")
	}
}

func TestHandleCreateRoomTemplateUnknown(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-bad", Template: "nonexistent",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for unknown template, got: %s", text)
	}
	// Should list valid template names
	if !strings.Contains(text, "decision-log") {
		t.Errorf("expected available template names in error, got: %s", text)
	}
}

func TestHandleCreateRoomTemplateInitialMsg(t *testing.T) {
	reg := setupHandlerTest(t)

	_, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-msg", Template: "bug",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs, _ := reg.Server.GetRecentMessages("tpl-msg", 10)
	if len(msgs) == 0 {
		t.Fatal("expected initial message to be posted")
	}
	if msgs[0].Author != "system" {
		t.Errorf("expected author 'system', got '%s'", msgs[0].Author)
	}
	if !strings.Contains(msgs[0].Content, "Bug investigation") {
		t.Errorf("unexpected initial message content: %s", msgs[0].Content)
	}
}

func TestHandleCreateRoomTemplateNoInitialMsgIfExists(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "tpl-exists")

	_, _, _ = reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-exists", Template: "bug",
	})

	msgs, _ := reg.Server.GetRecentMessages("tpl-exists", 10)
	if len(msgs) != 0 {
		t.Errorf("expected no initial message for pre-existing room, got %d messages", len(msgs))
	}
}
