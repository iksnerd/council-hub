package handlers

import (
	"context"
	"strings"
	"testing"
)

func TestCreateRoom_relatedJSONArrayStringIsNormalisedAndDanglingWarned(t *testing.T) {
	reg := &Registry{Server: setupTestServer(t)}
	ctx := context.Background()
	if _, _, err := reg.handleCreateRoom(ctx, nil, CreateRoomInput{ID: "spec", Topic: "spec"}); err != nil {
		t.Fatal(err)
	}
	res, _, err := reg.handleCreateRoom(ctx, nil, CreateRoomInput{ID: "conn", Topic: "c", RelatedRooms: `["spec","ghost-room"]`})
	if err != nil {
		t.Fatal(err)
	}
	text := toolResultText(res)
	if !strings.Contains(text, "ghost-room") || !strings.Contains(text, "not found on this node") {
		t.Fatalf("missing dangling warning: %s", text)
	}
	room, _ := reg.Server.GetRoom("conn")
	if room.RelatedRooms != "spec,ghost-room" {
		t.Fatalf("stored %q", room.RelatedRooms)
	}
}

func TestUpdateRoom_addRelatedKeepsTypesAndListShowsThem(t *testing.T) {
	reg := &Registry{Server: setupTestServer(t)}
	ctx := context.Background()
	for _, id := range []string{"a", "b", "c"} {
		if _, _, err := reg.handleCreateRoom(ctx, nil, CreateRoomInput{ID: id, Topic: id}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := reg.handleUpdateRoom(ctx, nil, UpdateRoomInput{RoomID: "a", RelatedRooms: "continues:b"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := reg.handleUpdateRoom(ctx, nil, UpdateRoomInput{RoomID: "a", AddRelatedRooms: "blocks:c"}); err != nil {
		t.Fatal(err)
	}
	room, _ := reg.Server.GetRoom("a")
	if got := room.RelatedDisplay(); got != "continues:b, blocks:c" {
		t.Fatalf("got %q", got)
	}
	res, _, err := reg.handleListRooms(ctx, nil, ListRoomsInput{Verbose: "true"})
	if err != nil {
		t.Fatal(err)
	}
	if text := toolResultText(res); !strings.Contains(text, "continues:b") {
		t.Fatalf("list_rooms hides link type: %s", text)
	}
}
