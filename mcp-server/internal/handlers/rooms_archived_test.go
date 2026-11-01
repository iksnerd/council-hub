package handlers

import (
	"context"
	"strings"
	"testing"
)

func TestListRoomsShowsArchivedAndFilters(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.Server.CreateRoom("ra-done", "archived one", "p", "", "", "", "")
	reg.Server.CreateRoom("ra-todo", "to archive", "p", "", "", "", "")
	reg.Server.UpdateStatus("ra-todo", "resolved")
	reg.Server.PostMessage("ra-done", "c", "hi", "message", "")
	if _, err := reg.Server.ArchiveRoom("ra-done"); err != nil {
		t.Fatal(err)
	}

	res, out, err := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Status: "resolved", Archived: "false"})
	if err != nil {
		t.Fatal(err)
	}
	txt := resultText(res)
	if !strings.Contains(txt, "ra-todo") || strings.Contains(txt, "ra-done") {
		t.Fatalf("archived=false should list only ra-todo:\n%s", txt)
	}
	if len(out.Rooms) != 1 || out.Rooms[0].ArchivedAt != "" {
		t.Errorf("structured: %+v", out.Rooms)
	}

	res, out, _ = reg.handleListRooms(context.Background(), nil, ListRoomsInput{Status: "resolved", Archived: "true"})
	txt = resultText(res)
	if !strings.Contains(txt, "ra-done") || strings.Contains(txt, "ra-todo") {
		t.Fatalf("archived=true should list only ra-done:\n%s", txt)
	}
	if !strings.Contains(txt, "archived ") || !strings.Contains(txt, "last msg ") {
		t.Errorf("compact line lacks markers:\n%s", txt)
	}
	if len(out.Rooms) != 1 || out.Rooms[0].ArchivedAt == "" || out.Rooms[0].LastActivity == "" {
		t.Errorf("structured fields missing: %+v", out.Rooms)
	}

	res, _, _ = reg.handleListRooms(context.Background(), nil, ListRoomsInput{Status: "resolved", Archived: "true", Verbose: "true"})
	if txt = resultText(res); !strings.Contains(txt, "Archived:") || !strings.Contains(txt, "Last message:") {
		t.Errorf("verbose lacks fields:\n%s", txt)
	}

	res, _, _ = reg.handleListRooms(context.Background(), nil, ListRoomsInput{Archived: "maybe"})
	if txt = resultText(res); !strings.Contains(txt, "Error") {
		t.Errorf("bad archived value should error: %s", txt)
	}
}

func TestPostToArchivedRoomWarns(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.Server.CreateRoom("pa-room", "x", "p", "", "", "", "")
	reg.Server.PostMessage("pa-room", "c", "first", "message", "")

	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{RoomID: "pa-room", Author: "c", Message: "before archive"})
	if strings.Contains(resultText(res), "archive") {
		t.Errorf("unarchived room should not warn: %s", resultText(res))
	}

	if _, err := reg.Server.ArchiveRoom("pa-room"); err != nil {
		t.Fatal(err)
	}
	res, _, _ = reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{RoomID: "pa-room", Author: "c", Message: "after archive"})
	txt := resultText(res)
	if !strings.Contains(txt, "not in the archive file") || !strings.Contains(txt, "archive_room") {
		t.Errorf("expected archive warning, got: %s", txt)
	}
	if !strings.Contains(strings.ToLower(txt), "posted") {
		t.Errorf("post must still land: %s", txt)
	}
}
