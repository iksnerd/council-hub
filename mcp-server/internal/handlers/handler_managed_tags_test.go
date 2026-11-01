package handlers

import (
	"context"
	"strings"
	"testing"
)

func TestCreateRoomIgnoresManagedTags(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{ID: "mt-create", Topic: "t", Tags: "release,resolved,stale-plan"})
	if err != nil {
		t.Fatal(err)
	}
	room, _ := reg.Server.GetRoom("mt-create")
	if room.Tags != "release" {
		t.Errorf("managed tags must be dropped, got %q", room.Tags)
	}
	if text := resultText(res); !strings.Contains(text, "hub-managed") {
		t.Errorf("expected a warning, got: %s", text)
	}
}

func TestUpdateRoomIgnoresManagedAddTags(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{ID: "mt-add", Topic: "t", Tags: "keep"})
	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{RoomID: "mt-add", AddTags: "needs-synthesis,synthesis-complete,extra"})
	room, _ := reg.Server.GetRoom("mt-add")
	for _, bad := range []string{"needs-synthesis", "synthesis-complete"} {
		if strings.Contains(room.Tags, bad) {
			t.Errorf("%s must not be settable, got %q", bad, room.Tags)
		}
	}
	if !strings.Contains(room.Tags, "extra") || !strings.Contains(room.Tags, "keep") {
		t.Errorf("ordinary tags must still apply, got %q", room.Tags)
	}
	if !strings.Contains(resultText(res), "hub-managed") {
		t.Error("expected a warning")
	}
}

func TestUpdateRoomTagOverwriteKeepsLinterFlags(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{ID: "mt-over", Topic: "t", Tags: "old"})
	reg.Server.DB.Exec(`UPDATE rooms SET tags = 'old,stale-plan' WHERE id = 'mt-over'`)
	reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{RoomID: "mt-over", Tags: "fresh,resolved"})
	room, _ := reg.Server.GetRoom("mt-over")
	if !strings.Contains(room.Tags, "stale-plan") || !strings.Contains(room.Tags, "fresh") || strings.Contains(room.Tags, "resolved") || strings.Contains(room.Tags, "old") {
		t.Errorf("overwrite should keep linter flag, drop managed input, replace labels: %q", room.Tags)
	}
}
