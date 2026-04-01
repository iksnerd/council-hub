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

// ========== read_room ==========

func TestHandleReadRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-read", withDescription("Read test"), withProject("proj"), withTechStack("Go"), withTags("tag1"), withSystemPrompt("prompt"), withRelatedRooms("related-a"))

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "h-read"})
	text := resultText(res)
	if !strings.Contains(text, "Read test") {
		t.Error("missing topic")
	}
	if !strings.Contains(text, "proj") {
		t.Error("missing project")
	}
	if !strings.Contains(text, "Related Rooms:** related-a") {
		t.Error("missing related rooms")
	}
}

func TestHandleReadRoomMissing(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleReadRoomNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found, got: %s", text)
	}
}

func TestHandleReadRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "hdb-room"})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found, got: %s", text)
	}
}

// ========== delete_room ==========

func TestHandleDeleteRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-del")

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "h-del"})
	text := resultText(res)
	if !strings.Contains(text, "permanently deleted") {
		t.Errorf("expected deleted, got: %s", text)
	}
}

func TestHandleDeleteRoomMissing(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing room_id")
	}
}

func TestHandleDeleteRoomNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleDeleteRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "hdb-room"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// ========== archive_room ==========

func TestHandleArchiveRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-archive")
	mustPost(t, reg.Server, "h-archive", "Claude", "Archive me")

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "h-archive"})
	text := resultText(res)
	if !strings.Contains(text, "archived") {
		t.Errorf("expected archived, got: %s", text)
	}
}

func TestHandleArchiveRoomAndDelete(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-archive-del")
	mustPost(t, reg.Server, "h-archive-del", "Claude", "Gone")

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{
		RoomID: "h-archive-del", Delete: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "deleted") {
		t.Errorf("expected deleted, got: %s", text)
	}

	_, err := reg.Server.GetRoom("h-archive-del")
	if err == nil {
		t.Error("room should be deleted after archive+delete")
	}
}

func TestHandleArchiveRoomMissing(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error")
	}
}

func TestHandleArchiveRoomNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleArchiveRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	res, _, _ := reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "hdb-room"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}
