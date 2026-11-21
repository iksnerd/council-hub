package handlers

import (
	"context"
	"strings"
	"testing"
)

func TestMarkReadSingleRoomWithoutCursorPointsAtWildcard(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "aex-a")
	res, _, _ := reg.handleMarkRead(context.Background(), nil, MarkReadInput{RoomID: "aex-a", Agent: "x"})
	want := `cursor (message ID) is required for a single room. To mark every room read, pass room_id="*" with no cursor.`
	if !strings.Contains(resultText(res), want) {
		t.Errorf("got: %s", resultText(res))
	}
}

func TestPostToMissingRoomSuggestsClosestAndGetOrCreate(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "auth-migration")
	mustCreateRoom(t, reg.Server, "unrelated-thing")
	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{RoomID: "auth-migrations", Author: "a", Message: "hi"})
	text := resultText(res)
	if !strings.Contains(text, "Room 'auth-migrations' not found.") ||
		!strings.Contains(text, "Closest: auth-migration.") ||
		!strings.Contains(text, "get_or_create_room(id=auth-migrations, project=..., repo=...)") ||
		!strings.Contains(text, "list_rooms(search=auth-migrations)") {
		t.Errorf("got: %s", text)
	}
	if strings.Contains(text, "unrelated-thing") || strings.Contains(text, "Create it first") {
		t.Errorf("got: %s", text)
	}
}

func TestPostToMissingRoomWithNoCloseMatchOmitsClosest(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{RoomID: "zzz-nothing", Author: "a", Message: "hi"})
	if strings.Contains(resultText(res), "Closest:") {
		t.Errorf("got: %s", resultText(res))
	}
}

func TestClosestRoomIDsCapsAtThree(t *testing.T) {
	reg := setupHandlerTest(t)
	for _, id := range []string{"foo-1", "foo-2", "foo-3", "foo-4", "foo-5"} {
		mustCreateRoom(t, reg.Server, id)
	}
	if got := reg.Server.ClosestRoomIDs("foo", 3); len(got) != 3 {
		t.Errorf("got %v", got)
	}
}

func TestGetOrCreateRoomEchoesProjectOnCreateAndFound(t *testing.T) {
	reg := setupHandlerTest(t)
	in := GetOrCreateRoomInput{ID: "aex-proj", Topic: "t", Project: "myproj"}
	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, in)
	if !strings.Contains(resultText(res), "**Project:** myproj") {
		t.Errorf("create: %s", resultText(res))
	}
	res, _, _ = reg.handleGetOrCreateRoom(context.Background(), nil, in)
	if !strings.Contains(resultText(res), "**Project:** myproj") {
		t.Errorf("found: %s", resultText(res))
	}
}
