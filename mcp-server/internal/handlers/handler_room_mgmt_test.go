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

func TestHandleDeleteRoomCascadeClean(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-cascade-a")
	mustCreateRoom(t, reg.Server, "h-cascade-b")
	reg.Server.UpdateRoom("h-cascade-b", "", "", "", "", "", "h-cascade-a")

	res, _, _ := reg.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "h-cascade-a"})
	if !strings.Contains(resultText(res), "permanently deleted") {
		t.Fatalf("expected deleted confirmation, got: %s", resultText(res))
	}

	roomB, err := reg.Server.GetRoom("h-cascade-b")
	if err != nil {
		t.Fatalf("GetRoom failed: %v", err)
	}
	if strings.Contains(roomB.RelatedRooms, "h-cascade-a") {
		t.Errorf("h-cascade-b still references deleted h-cascade-a: %q", roomB.RelatedRooms)
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

// ========== list_archives ==========

func TestHandleListArchivesEmpty(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)

	res, _, _ := reg.handleListArchives(context.Background(), nil, ListArchivesInput{})
	if !strings.Contains(resultText(res), "No archives found") {
		t.Error("expected no archives message")
	}
}

func TestHandleListArchives(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)
	mustCreateRoom(t, reg.Server, "h-list-arch-a")
	mustPost(t, reg.Server, "h-list-arch-a", "Claude", "Message A")
	mustCreateRoom(t, reg.Server, "h-list-arch-b")
	mustPost(t, reg.Server, "h-list-arch-b", "Claude", "Message B")

	reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "h-list-arch-a"}) //nolint:errcheck
	reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "h-list-arch-b"}) //nolint:errcheck

	res, _, _ := reg.handleListArchives(context.Background(), nil, ListArchivesInput{})
	text := resultText(res)
	if !strings.Contains(text, "h-list-arch-a") {
		t.Error("expected h-list-arch-a in listing")
	}
	if !strings.Contains(text, "h-list-arch-b") {
		t.Error("expected h-list-arch-b in listing")
	}
	if !strings.Contains(text, "Found 2 archive") {
		t.Errorf("expected count in header, got: %s", text)
	}
}

// ========== read_archive ==========

func TestHandleReadArchive(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)
	mustCreateRoom(t, reg.Server, "h-read-arch")
	mustPost(t, reg.Server, "h-read-arch", "Claude", "Archived content here")

	reg.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "h-read-arch"}) //nolint:errcheck

	res, _, _ := reg.handleReadArchive(context.Background(), nil, ReadArchiveInput{RoomID: "h-read-arch"})
	text := resultText(res)
	if !strings.Contains(text, "h-read-arch") {
		t.Error("expected room ID in archive content")
	}
	if !strings.Contains(text, "Archived content here") {
		t.Error("expected message content in archive")
	}
}

func TestHandleReadArchiveNotFound(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)

	res, _, _ := reg.handleReadArchive(context.Background(), nil, ReadArchiveInput{RoomID: "ghost-archive"})
	if !strings.Contains(resultText(res), "not found") {
		t.Error("expected not found error")
	}
}

func TestHandleReadArchiveMissingID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReadArchive(context.Background(), nil, ReadArchiveInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing room_id")
	}
}
