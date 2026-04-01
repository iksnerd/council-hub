package main

import (
	"context"
	"strings"
	"testing"
)

// ========== v0.5.0: bidirectional related_rooms via handlers ==========

func TestHandleCreateRoomBidirectionalLinks(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "bidir-target")

	res, _, _ := cs.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "bidir-source", Topic: "Source room", RelatedRooms: "bidir-target",
	})
	text := resultText(res)
	if !strings.Contains(text, "bidirectional") {
		t.Errorf("expected bidirectional mention in response, got: %s", text)
	}

	// Verify reverse link was created
	tgt, _ := cs.getRoom("bidir-target")
	if !strings.Contains(tgt.RelatedRooms, "bidir-source") {
		t.Errorf("expected reverse link to bidir-source, got: '%s'", tgt.RelatedRooms)
	}
}

func TestHandleUpdateRoomBidirectionalLinks(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "upd-bidir-a")
	mustCreateRoom(t, cs, "upd-bidir-b")

	cs.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "upd-bidir-a", RelatedRooms: "upd-bidir-b",
	})

	b, _ := cs.getRoom("upd-bidir-b")
	if !strings.Contains(b.RelatedRooms, "upd-bidir-a") {
		t.Errorf("expected reverse link, got: '%s'", b.RelatedRooms)
	}
}

// ========== v0.5.0: enriched create_room response ==========

func TestCreateRoomResponseIncludesMetadata(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "rich-create", Topic: "My topic", Project: "my-proj", Tags: "alpha,beta",
	})
	text := resultText(res)

	if !strings.Contains(text, "**Topic:** My topic") {
		t.Error("expected topic in response")
	}
	if !strings.Contains(text, "**Project:** my-proj") {
		t.Error("expected project in response")
	}
	if !strings.Contains(text, "**Tags:** alpha,beta") {
		t.Error("expected tags in response")
	}
}

// ========== v0.5.0: enriched signal_status response ==========

func TestSignalStatusResponseIncludesContext(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "status-ctx", withDescription("Bug fix room"), withProject("my-proj"))

	res, _, _ := cs.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "status-ctx", Status: "resolved",
	})
	text := resultText(res)

	if !strings.Contains(text, "**resolved**") {
		t.Errorf("expected bold status, got: %s", text)
	}
	if !strings.Contains(text, "Bug fix room") {
		t.Error("expected topic in response")
	}
	if !strings.Contains(text, "my-proj") {
		t.Error("expected project in response")
	}
}

// ========== v0.5.0: enriched update_room response ==========

func TestUpdateRoomResponseIncludesState(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "upd-state", withProject("proj-a"), withTags("v1"))

	res, _, _ := cs.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "upd-state", Tags: "v2,released",
	})
	text := resultText(res)

	if !strings.Contains(text, "Current state") {
		t.Error("expected current state section")
	}
	if !strings.Contains(text, "v2,released") {
		t.Error("expected updated tags in state")
	}
	if !strings.Contains(text, "proj-a") {
		t.Error("expected project in state")
	}
}

// ========== v0.5.0: post_to_room JSON cursor ==========

func TestPostToRoomJSONCursor(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "json-cursor")

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "json-cursor", Author: "Claude", Message: "Test",
	})
	text := resultText(res)

	if !strings.Contains(text, `"message_id":`) {
		t.Errorf("expected JSON message_id, got: %s", text)
	}
	if !strings.Contains(text, `"room_id": "json-cursor"`) {
		t.Errorf("expected JSON room_id, got: %s", text)
	}
	if !strings.Contains(text, `"latest_message_id":`) {
		t.Errorf("expected JSON latest_message_id, got: %s", text)
	}
}

