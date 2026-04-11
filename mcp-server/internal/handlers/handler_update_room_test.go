package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== update_room ==========

func TestHandleUpdateRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-update")

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "h-update", Topic: "Updated topic", RelatedRooms: "room-x",
	})
	text := resultText(res)
	if !strings.Contains(text, "topic") || !strings.Contains(text, "related_rooms") {
		t.Errorf("expected updated fields listed, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("h-update")
	if room.Description != "Updated topic" {
		t.Errorf("expected 'Updated topic', got '%s'", room.Description)
	}
	if room.RelatedRooms != "room-x" {
		t.Errorf("expected related_rooms 'room-x', got '%s'", room.RelatedRooms)
	}
}

func TestHandleUpdateRoomMissingID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleUpdateRoomNoFields(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{RoomID: "x"})
	text := resultText(res)
	if !strings.Contains(text, "at least one field") {
		t.Errorf("expected field error, got: %s", text)
	}
}

func TestHandleUpdateRoomAllFields(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-update-all")

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID:       "h-update-all",
		Topic:        "New topic",
		Project:      "New project",
		TechStack:    "New tech",
		Tags:         "new-tag",
		SystemPrompt: "New prompt",
		RelatedRooms: "room-a",
	})
	text := resultText(res)
	for _, field := range []string{"topic", "project", "tech_stack", "tags", "system_prompt", "related_rooms"} {
		if !strings.Contains(text, field) {
			t.Errorf("expected %s in updated fields, got: %s", field, text)
		}
	}
}

func TestHandleUpdateRoomNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "ghost", Topic: "X",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleUpdateRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "hdb-room", Topic: "new",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// ========== update_room add_tags / remove_tags ==========

func TestHandleUpdateRoomAddTags(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-addtags", withTags("go,api"))

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "h-addtags", AddTags: "mcp",
	})
	text := resultText(res)
	if !strings.Contains(text, "add_tags") {
		t.Errorf("expected add_tags in updated fields, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("h-addtags")
	for _, tag := range []string{"go", "api", "mcp"} {
		if !strings.Contains(room.Tags, tag) {
			t.Errorf("expected tag %q in %q", tag, room.Tags)
		}
	}
}

func TestHandleUpdateRoomRemoveTags(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-removetags", withTags("go,api,mcp"))

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "h-removetags", RemoveTags: "mcp",
	})
	text := resultText(res)
	if !strings.Contains(text, "remove_tags") {
		t.Errorf("expected remove_tags in updated fields, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("h-removetags")
	if strings.Contains(room.Tags, "mcp") {
		t.Errorf("expected mcp removed from tags, got: %q", room.Tags)
	}
	for _, tag := range []string{"go", "api"} {
		if !strings.Contains(room.Tags, tag) {
			t.Errorf("expected tag %q still present in %q", tag, room.Tags)
		}
	}
}

func TestHandleUpdateRoomAddAndRemoveTags(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-addremove", withTags("go,api"))

	_, _, _ = reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "h-addremove", AddTags: "mcp", RemoveTags: "api",
	})

	room, _ := reg.Server.GetRoom("h-addremove")
	if strings.Contains(room.Tags, "api") {
		t.Errorf("expected api removed, got: %q", room.Tags)
	}
	for _, tag := range []string{"go", "mcp"} {
		if !strings.Contains(room.Tags, tag) {
			t.Errorf("expected tag %q in %q", tag, room.Tags)
		}
	}
}

func TestHandleUpdateRoomAddTagsDuplicate(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-adddup", withTags("go,api"))

	_, _, _ = reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "h-adddup", AddTags: "go",
	})

	room, _ := reg.Server.GetRoom("h-adddup")
	count := strings.Count(room.Tags, "go")
	if count != 1 {
		t.Errorf("expected go to appear once, got %d in %q", count, room.Tags)
	}
}

func TestHandleUpdateRoomRemoveNonexistentTag(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-removemissing", withTags("go,api"))

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "h-removemissing", RemoveTags: "nonexistent",
	})
	text := resultText(res)
	if strings.Contains(text, "Error") {
		t.Errorf("should not error on removing nonexistent tag, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("h-removemissing")
	for _, tag := range []string{"go", "api"} {
		if !strings.Contains(room.Tags, tag) {
			t.Errorf("expected tag %q still present in %q", tag, room.Tags)
		}
	}
}

// ========== update_room batch ==========

func TestHandleUpdateRoomBatch(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "batch-a")
	mustCreateRoom(t, reg.Server, "batch-b")

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomIDs: "batch-a,batch-b", Tags: "sprint-5",
	})
	text := resultText(res)
	if !strings.Contains(text, "batch-a") || !strings.Contains(text, "batch-b") {
		t.Errorf("expected both rooms in response, got: %s", text)
	}

	for _, id := range []string{"batch-a", "batch-b"} {
		room, _ := reg.Server.GetRoom(id)
		if room.Tags != "sprint-5" {
			t.Errorf("room %s: expected tags 'sprint-5', got '%s'", id, room.Tags)
		}
	}
}

func TestHandleUpdateRoomBatchPartialError(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "batch-good")

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomIDs: "batch-good,ghost-room", Tags: "foo",
	})
	text := resultText(res)
	if !strings.Contains(text, "batch-good") {
		t.Errorf("expected success line for batch-good, got: %s", text)
	}
	if !strings.Contains(text, "Error") || !strings.Contains(text, "ghost-room") {
		t.Errorf("expected error line for ghost-room, got: %s", text)
	}
}

func TestHandleUpdateRoomBatchNoIDs(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{Tags: "foo"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error when no IDs provided, got: %s", text)
	}
}

func TestHandleUpdateRoomIDAndRoomIDsCombo(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "combo-a")
	mustCreateRoom(t, reg.Server, "combo-b")

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "combo-a", RoomIDs: "combo-b", Tags: "merged",
	})
	text := resultText(res)
	if !strings.Contains(text, "combo-a") || !strings.Contains(text, "combo-b") {
		t.Errorf("expected both rooms updated, got: %s", text)
	}

	for _, id := range []string{"combo-a", "combo-b"} {
		room, _ := reg.Server.GetRoom(id)
		if room.Tags != "merged" {
			t.Errorf("room %s: expected tags 'merged', got '%s'", id, room.Tags)
		}
	}
}
