package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ========== bulk_status_update ==========

func TestHandleBulkStatusUpdate(t *testing.T) {
	cs := setupTestServer(t)
	registerTools(cs)
	mustCreateRoom(t, cs, "bulk-1")
	mustCreateRoom(t, cs, "bulk-2")
	mustCreateRoom(t, cs, "bulk-3")

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "bulk-1,bulk-2,bulk-3", Status: "resolved",
	})
	text := resultText(res)

	if !strings.Contains(text, "Updated 3 room(s)") {
		t.Errorf("expected 3 updated, got: %s", text)
	}
	for _, id := range []string{"bulk-1", "bulk-2", "bulk-3"} {
		room, _ := cs.getRoom(id)
		if room.Status != "resolved" {
			t.Errorf("room '%s' should be resolved, got '%s'", id, room.Status)
		}
	}
}

func TestHandleBulkStatusUpdatePartialFailure(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "bulk-ok")

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "bulk-ok,nonexistent-room", Status: "paused",
	})
	text := resultText(res)
	if !strings.Contains(text, "Updated 1 room(s)") {
		t.Errorf("expected 1 updated, got: %s", text)
	}
	if !strings.Contains(text, "Not found: nonexistent-room") {
		t.Errorf("expected not found for nonexistent room, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateInvalidStatus(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "x", Status: "invalid",
	})
	text := resultText(res)
	if !strings.Contains(text, "Invalid status") {
		t.Errorf("expected invalid status error, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateMissingIDs(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "", Status: "active",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing room_ids, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateEmptyIDs(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: ",,,", Status: "active",
	})
	text := resultText(res)
	if !strings.Contains(text, "No valid room IDs") {
		t.Errorf("expected no valid IDs message, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateWithMessage(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "bulk-msg-1")
	mustCreateRoom(t, cs, "bulk-msg-2")

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "bulk-msg-1,bulk-msg-2",
		Status:  "resolved",
		Message: "Closed: all issues fixed in PR #42.",
		Author:  "Claude",
	})
	text := resultText(res)
	if !strings.Contains(text, "Updated 2 room(s)") {
		t.Errorf("expected 2 updated, got: %s", text)
	}

	msgs1, _ := cs.getRecentMessages("bulk-msg-1", 1)
	if len(msgs1) != 1 || !strings.Contains(msgs1[0].Content, "PR #42") {
		t.Error("expected closing message in room 1")
	}
	if msgs1[0].MessageType != "decision" {
		t.Errorf("expected decision type, got '%s'", msgs1[0].MessageType)
	}
}

func TestHandleBulkStatusUpdateMessageWithoutAuthor(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "x", Status: "resolved", Message: "Closing",
	})
	text := resultText(res)
	if !strings.Contains(text, "author is required") {
		t.Errorf("expected author required error, got: %s", text)
	}
}

// ========== get_or_create_room ==========

func TestHandleGetOrCreateRoomNew(t *testing.T) {
	cs := setupHandlerTest(t)

	res, _, _ := cs.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-new", Topic: "New room", Project: "proj", SystemPrompt: "Be helpful.",
	})
	text := resultText(res)

	if !strings.Contains(text, "Created") {
		t.Errorf("expected 'Created', got: %s", text)
	}
	if !strings.Contains(text, "Be helpful.") {
		t.Error("expected system prompt in response")
	}

	room, err := cs.getRoom("upsert-new")
	if err != nil {
		t.Fatalf("room should exist: %v", err)
	}
	if room.Project != "proj" {
		t.Errorf("expected project 'proj', got '%s'", room.Project)
	}
}

func TestHandleGetOrCreateRoomExisting(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "upsert-existing", withDescription("Already here"), withProject("proj"), withSystemPrompt("Prompt text."))
	mustPostTyped(t, cs, "upsert-existing", "Claude", "First message", "decision")
	mustPostTyped(t, cs, "upsert-existing", "Gemini", "Second message", "action")

	res, _, _ := cs.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-existing",
	})
	text := resultText(res)

	if !strings.Contains(text, "Found") {
		t.Errorf("expected 'Found', got: %s", text)
	}
	if !strings.Contains(text, "Recent messages") {
		t.Error("expected recent messages for existing room")
	}
	if !strings.Contains(text, "First message") || !strings.Contains(text, "Second message") {
		t.Error("expected both messages in response")
	}
}

func TestHandleGetOrCreateRoomExistingEmpty(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "upsert-empty")

	res, _, _ := cs.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-empty",
	})
	text := resultText(res)
	if !strings.Contains(text, "Found") {
		t.Error("expected Found")
	}
	if !strings.Contains(text, "No messages yet") {
		t.Errorf("expected 'No messages yet', got: %s", text)
	}
}

func TestHandleGetOrCreateRoomMissingID(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing id, got: %s", text)
	}
}

func TestHandleGetOrCreateRoomCustomLastN(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "upsert-lastn")
	for i := 0; i < 10; i++ {
		mustPost(t, cs, "upsert-lastn", "Claude", fmt.Sprintf("Msg %d", i))
	}

	res, _, _ := cs.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-lastn", LastN: "2",
	})
	text := resultText(res)
	if !strings.Contains(text, "Recent messages (2)") {
		t.Errorf("expected 2 recent messages, got: %s", text)
	}
	if !strings.Contains(text, "Msg 9") {
		t.Error("expected last message")
	}
	if strings.Contains(text, "Msg 0") {
		t.Error("should not contain old messages with last_n=2")
	}
}
