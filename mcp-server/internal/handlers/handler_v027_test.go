package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== v0.27.0: list_rooms(project_not_in=...) ==========

func TestListRoomsProjectNotInExcludes(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "alpha-1", withProject("project-alpha"))
	mustCreateRoom(t, reg.Server, "beta-1", withProject("project-beta"))
	mustCreateRoom(t, reg.Server, "legacy-1", withProject("legacy-x"))

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{
		ProjectNotIn: "project-alpha,project-beta",
	})
	text := resultText(res)
	if strings.Contains(text, "alpha-1") || strings.Contains(text, "beta-1") {
		t.Errorf("active projects should be excluded, got: %s", text)
	}
	if !strings.Contains(text, "legacy-1") {
		t.Errorf("legacy room should remain, got: %s", text)
	}
}

// ========== v0.27.0: list_rooms(related_to=...) ==========

func TestListRoomsRelatedToFiltersNeighbors(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "hub")
	mustCreateRoom(t, reg.Server, "neighbor", withRelatedRooms("hub"))
	mustCreateRoom(t, reg.Server, "outsider")

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{
		RelatedTo: "hub",
	})
	text := resultText(res)
	if !strings.Contains(text, "neighbor") {
		t.Errorf("neighbor should be returned, got: %s", text)
	}
	if strings.Contains(text, "outsider") {
		t.Errorf("outsider should be filtered out, got: %s", text)
	}
}

// ========== v0.27.0: rename_project ==========

func TestHandleRenameProject(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "r1", withProject("pi-go"))
	mustCreateRoom(t, reg.Server, "r2", withProject("pi-go"))
	mustCreateRoom(t, reg.Server, "r3", withProject("other"))

	res, _, _ := reg.handleRenameProject(context.Background(), nil, RenameProjectInput{
		From: "pi-go", To: "go-pilot",
	})
	text := resultText(res)
	if !strings.Contains(text, "2 room") {
		t.Errorf("expected '2 room(s)' in result, got: %s", text)
	}

	res, _, _ = reg.handleRenameProject(context.Background(), nil, RenameProjectInput{From: "pi-go"})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error when 'to' is missing")
	}
}

// ========== v0.27.0: update_room(where_project=...) bulk tagging ==========

func TestUpdateRoomWhereProjectBulkAddTag(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "rA", withProject("legacy-x"), withTags("foo"))
	mustCreateRoom(t, reg.Server, "rB", withProject("legacy-x"))
	mustCreateRoom(t, reg.Server, "rC", withProject("active"))

	_, _, _ = reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		WhereProject: "legacy-x",
		AddTags:      "deprecated",
	})

	for _, id := range []string{"rA", "rB"} {
		room, _ := reg.Server.GetRoom(id)
		if !strings.Contains(room.Tags, "deprecated") {
			t.Errorf("expected room %s to have 'deprecated' tag, got tags=%q", id, room.Tags)
		}
	}
	rC, _ := reg.Server.GetRoom("rC")
	if strings.Contains(rC.Tags, "deprecated") {
		t.Errorf("rC (different project) should not have been touched, got tags=%q", rC.Tags)
	}
}

// ========== v0.27.0: bulk_status_update(auto_archive_days) ==========

func TestBulkStatusUpdateAutoArchive(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)
	mustCreateRoom(t, reg.Server, "old-room")
	mustPost(t, reg.Server, "old-room", "claude", "ancient")
	// Backdate the message timestamp 30 days
	if _, err := reg.Server.DB.Exec(`UPDATE messages SET timestamp = datetime('now', '-30 days') WHERE room_id = 'old-room'`); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs:         "old-room",
		Status:          "resolved",
		AutoArchiveDays: "7",
	})
	text := resultText(res)
	if !strings.Contains(text, "Auto-archived 1 room") {
		t.Errorf("expected auto-archive line in output, got: %s", text)
	}
	if _, err := reg.Server.GetRoom("old-room"); err == nil {
		t.Error("expected room to have been deleted after auto-archive")
	}
}

func TestBulkStatusUpdateAutoArchiveSkipsRecent(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)
	mustCreateRoom(t, reg.Server, "fresh-room")
	mustPost(t, reg.Server, "fresh-room", "claude", "fresh")

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs:         "fresh-room",
		Status:          "resolved",
		AutoArchiveDays: "30",
	})
	if strings.Contains(resultText(res), "Auto-archived") {
		t.Errorf("recent room should not auto-archive, got: %s", resultText(res))
	}
	if _, err := reg.Server.GetRoom("fresh-room"); err != nil {
		t.Errorf("recent room should still exist: %v", err)
	}
}

func TestBulkStatusUpdateAutoArchiveInvalidValue(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "r")
	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "r", Status: "resolved", AutoArchiveDays: "-1",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Errorf("expected error for negative auto_archive_days, got: %s", resultText(res))
	}
}
