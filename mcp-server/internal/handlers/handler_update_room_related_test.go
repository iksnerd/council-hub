package handlers

import (
	"context"
	"strings"
	"testing"
)

func TestHandleUpdateRoomRelatedRoomsReportsRemoved(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "rel-main")
	_ = reg.Server.UpdateRoom("rel-main", "", "", "", "", "", "", "", "rel-a,rel-b")

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "rel-main", RelatedRooms: "rel-b,rel-c",
	})
	text := resultText(res)
	if !strings.Contains(text, "Removed related rooms: rel-a") {
		t.Errorf("expected removed ids listed, got: %s", text)
	}
	if !strings.Contains(text, "rel-b,rel-c") {
		t.Errorf("expected final list, got: %s", text)
	}
	if !strings.Contains(text, "add_related_rooms") {
		t.Errorf("expected hint about add_related_rooms, got: %s", text)
	}
}

func TestHandleUpdateRoomAddRelatedRoomsMerges(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "rel-m")
	_ = reg.Server.UpdateRoom("rel-m", "", "", "", "", "", "", "", "rel-a,rel-b")

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "rel-m", AddRelatedRooms: "rel-b,rel-c",
	})
	text := resultText(res)
	if strings.Contains(text, "Removed related rooms") {
		t.Errorf("merge must not remove anything, got: %s", text)
	}
	room, _ := reg.Server.GetRoom("rel-m")
	if room.RelatedRooms != "rel-a,rel-b,rel-c" {
		t.Errorf("expected merged list, got %q", room.RelatedRooms)
	}
}

func TestHandleUpdateRoomAddRelatedRoomsConflict(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "rel-x")
	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "rel-x", RelatedRooms: "a", AddRelatedRooms: "b",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Errorf("expected error when both given, got: %s", resultText(res))
	}
}
