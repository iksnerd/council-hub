package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== pin_message ==========

func TestHandlePinMessage(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pin-room")
	id := mustPostTyped(t, reg.Server, "pin-room", "Claude", "Important TL;DR", "decision")

	res, _, err := reg.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID:    "pin-room",
		MessageID: id,
	})
	if err != nil {
		t.Fatalf("handlePinMessage error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "pinned") {
		t.Errorf("expected 'pinned' in result, got: %s", text)
	}

	// Verify message is actually pinned in DB
	pinned, err := reg.Server.GetPinnedMessage("pin-room")
	if err != nil {
		t.Fatalf("getPinnedMessage error: %v", err)
	}
	if pinned == nil {
		t.Fatal("expected pinned message, got nil")
	} else if pinned.ID != id {
		t.Errorf("expected pinned message ID %s, got %s", id, pinned.ID)
	}
}

func TestHandlePinMessageToggle(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pin-toggle")
	id := mustPost(t, reg.Server, "pin-toggle", "Claude", "Pin/Unpin")

	// Pin it
	reg.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "pin-toggle", MessageID: id})

	// Unpin it (toggle)
	res, _, _ := reg.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID:    "pin-toggle",
		MessageID: id,
	})
	text := resultText(res)
	if !strings.Contains(text, "unpinned") {
		t.Errorf("expected 'unpinned' in result, got: %s", text)
	}

	pinned, _ := reg.Server.GetPinnedMessage("pin-toggle")
	if pinned != nil {
		t.Error("expected no pinned message after toggle")
	}
}

func TestHandlePinMessageReplacesExisting(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pin-replace")
	id1 := mustPost(t, reg.Server, "pin-replace", "Claude", "First")
	id2 := mustPost(t, reg.Server, "pin-replace", "Gemini", "Second")

	// Pin first
	reg.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "pin-replace", MessageID: id1})

	// Pin second — should replace first
	reg.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "pin-replace", MessageID: id2})

	pinned, _ := reg.Server.GetPinnedMessage("pin-replace")
	if pinned == nil || pinned.ID != id2 {
		t.Errorf("expected message %s to be pinned, got %v", id2, pinned)
	}
}

func TestHandlePinMessageWrongRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "room-a")
	mustCreateRoom(t, reg.Server, "room-b")
	id := mustPost(t, reg.Server, "room-a", "Claude", "Msg A")

	// Try to pin room-a's message in room-b
	res, _, _ := reg.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID:    "room-b",
		MessageID: id,
	})
	text := resultText(res)
	if !strings.Contains(text, "belongs to room 'room-a'") {
		t.Errorf("expected room mismatch error, got: %s", text)
	}
}

func TestHandlePinMessageMissingFields(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handlePinMessage(context.Background(), nil, PinMessageInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing fields")
	}
}

func TestHandlePinMessageNotFound(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pin-nf")
	res, _, _ := reg.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID: "pin-nf", MessageID: "fake-nonexistent-uuid",
	})
	if !strings.Contains(resultText(res), "not found") {
		t.Error("expected not found error")
	}
}

// ========== update_message ==========

func TestHandleUpdateMessage(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-upd-msg")
	id := mustPost(t, reg.Server, "h-upd-msg", "Claude", "Old content")

	res, _, err := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: id,
		Content:   "New content",
	})
	if err != nil {
		t.Fatalf("handleUpdateMessage error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "updated") {
		t.Errorf("expected success message, got: %s", text)
	}

	// Verify in DB
	msgs, _ := reg.Server.GetMessagesByIDs([]string{id})
	if len(msgs) == 0 || msgs[0].Content != "New content" {
		t.Errorf("expected content update, got: %v", msgs)
	}
}

func TestHandleUpdateMessageWithType(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-upd-type")
	id := mustPost(t, reg.Server, "h-upd-type", "Claude", "Msg")

	reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID:   id,
		Content:     "Msg",
		MessageType: "decision",
	})

	msgs, _ := reg.Server.GetMessagesByIDs([]string{id})
	if len(msgs) == 0 || msgs[0].MessageType != "decision" {
		t.Errorf("expected type update, got: %v", msgs)
	}
}

func TestHandleUpdateMessagePreservesFields(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-upd-pres")
	id := mustPost(t, reg.Server, "h-upd-pres", "Claude", "Original")

	reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: id,
		Content:   "Updated",
	})

	msgs, _ := reg.Server.GetMessagesByIDs([]string{id})
	if len(msgs) == 0 {
		t.Fatal("message lost")
	}
	msg := msgs[0]
	if msg.Author != "Claude" {
		t.Error("author should be preserved")
	}
	if msg.RoomID != "h-upd-pres" {
		t.Error("room_id should be preserved")
	}
}

func TestHandleUpdateMessageNotFound(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: "fake-nonexistent-uuid",
		Content:   "New",
	})
	if !strings.Contains(resultText(res), "not found") {
		t.Error("expected not found error")
	}
}

func TestHandleUpdateMessageInvalidType(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-upd-bad-type")
	id := mustPost(t, reg.Server, "h-upd-bad-type", "Claude", "Msg")

	res, _, _ := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID:   id,
		Content:     "Msg",
		MessageType: "invalid",
	})
	if !strings.Contains(resultText(res), "invalid message_type") {
		t.Error("expected invalid type error")
	}
}

func TestHandleUpdateMessageMissingFields(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing fields")
	}
}

func TestHandleUpdateMessageNonExistentID(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: "not-a-real-uuid",
		Content:   "some content",
	})
	if !strings.Contains(resultText(res), "not found") {
		t.Errorf("expected not found error, got: %s", resultText(res))
	}
}

func TestHandleUpdateMessageDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	_, _, err := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: "some-id",
		Content:   "content",
	})
	if err == nil {
		t.Error("expected error from closed DB")
	}
}

func TestHandlePinMessageNonExistentID(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pin-parse-err")
	res, _, _ := reg.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID:    "pin-parse-err",
		MessageID: "not-a-real-uuid",
	})
	if !strings.Contains(resultText(res), "not found") {
		t.Errorf("expected not found error, got: %s", resultText(res))
	}
}

// ========== delete_messages ==========

func TestHandleDeleteMessages(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-delmsg")
	id := mustPost(t, reg.Server, "h-delmsg", "Claude", "Delete me")

	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{MessageIDs: id})
	text := resultText(res)
	if !strings.Contains(text, "Deleted 1") {
		t.Errorf("expected 1 deleted, got: %s", text)
	}
}

func TestHandleDeleteMessagesMissing(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error")
	}
}

func TestHandleDeleteMessagesNonExistentID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{MessageIDs: "fake-nonexistent-uuid"})
	// Non-existent IDs delete 0 messages — not an error
	if !strings.Contains(resultText(res), "Deleted 0") {
		t.Errorf("expected 0 deleted for non-existent ID, got: %s", resultText(res))
	}
}

func TestHandleDeleteMessagesDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	_, _, err := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{MessageIDs: "some-uuid"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestHandleDeleteMessagesDryRun(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "dry-room")
	id1 := mustPost(t, reg.Server, "dry-room", "Claude", "Message one")
	id2 := mustPostTyped(t, reg.Server, "dry-room", "Gemini", "Message two", "thought")

	res, _, err := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: id1 + "," + id2,
		DryRun:     "true",
	})
	if err != nil {
		t.Fatalf("handleDeleteMessages dry_run error: %v", err)
	}
	text := resultText(res)

	if !strings.Contains(text, "DRY RUN") {
		t.Error("expected 'DRY RUN' in output")
	}
	if !strings.Contains(text, "2 message(s) would be deleted") {
		t.Errorf("expected '2 message(s) would be deleted', got: %s", text)
	}
	if !strings.Contains(text, "Claude") {
		t.Error("expected author 'Claude' in output")
	}
	if !strings.Contains(text, "Gemini") {
		t.Error("expected author 'Gemini' in output")
	}
	if !strings.Contains(text, "Message one") {
		t.Error("expected message content excerpt in output")
	}

	// Verify messages still exist
	msgs, _ := reg.Server.GetMessagesByIDs([]string{id1, id2})
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages to still exist, got %d", len(msgs))
	}
}

func TestHandleDeleteMessagesDryRunNotFound(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "dry-nf")
	id := mustPost(t, reg.Server, "dry-nf", "Claude", "Exists")

	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: id + ",fake-nonexistent-id",
		DryRun:     "true",
	})
	text := resultText(res)

	if !strings.Contains(text, "1 message(s) would be deleted") {
		t.Errorf("expected '1 message(s) would be deleted', got: %s", text)
	}
	if !strings.Contains(text, "not found") {
		t.Error("expected not found indicator for missing ID")
	}
}

func TestHandleDeleteMessagesDryRunFalse(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "dry-false")
	id := mustPost(t, reg.Server, "dry-false", "Claude", "Delete me")

	// dry_run=false should still delete
	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: id,
		DryRun:     "false",
	})
	text := resultText(res)
	if !strings.Contains(text, "Deleted 1 message") {
		t.Errorf("expected deletion confirmation, got: %s", text)
	}

	msgs, _ := reg.Server.GetMessagesByIDs([]string{id})
	if len(msgs) != 0 {
		t.Error("message should be deleted when dry_run=false")
	}
}

func TestHandleDeleteMessagesDryRunOmitted(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "dry-omit")
	id := mustPost(t, reg.Server, "dry-omit", "Claude", "Delete me too")

	// No dry_run param — should delete (backward compatible)
	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: id,
	})
	text := resultText(res)
	if !strings.Contains(text, "Deleted 1 message") {
		t.Errorf("expected deletion confirmation, got: %s", text)
	}
}

func TestHandleDeleteMessagesDryRunExcerptTruncation(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "dry-trunc")
	longContent := strings.Repeat("A", 200)
	id := mustPost(t, reg.Server, "dry-trunc", "Claude", longContent)

	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: id,
		DryRun:     "true",
	})
	text := resultText(res)

	// Should be truncated to ~120 chars + "..."
	if !strings.Contains(text, "...") {
		t.Error("expected truncated content with '...' in dry run output")
	}
	if strings.Contains(text, longContent) {
		t.Error("full content should not appear in dry run output")
	}
}

// ========== update_message optimistic concurrency ==========

func TestHandleUpdateMessageExpectedContentMatch(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "occ-match")
	id := mustPost(t, reg.Server, "occ-match", "Claude", "original content")

	// Update succeeds when expected_content matches current content
	res, _, err := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID:       id,
		Content:         "updated content",
		ExpectedContent: "original content",
	})
	if err != nil {
		t.Fatalf("handleUpdateMessage error: %v", err)
	}
	if !strings.Contains(resultText(res), "updated") {
		t.Errorf("expected success, got: %s", resultText(res))
	}
}

func TestHandleUpdateMessageExpectedContentMismatch(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "occ-mismatch")
	id := mustPost(t, reg.Server, "occ-mismatch", "Claude", "original content")

	// Update fails when expected_content doesn't match
	res, _, err := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID:       id,
		Content:         "new content",
		ExpectedContent: "stale content from last read",
	})
	if err != nil {
		t.Fatalf("handleUpdateMessage returned unexpected error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "content changed") {
		t.Errorf("expected concurrency error message, got: %s", text)
	}
	// Response should include current content so agent can merge
	if !strings.Contains(text, "original content") {
		t.Errorf("expected current content in error response, got: %s", text)
	}
}

func TestHandleUpdateMessageNoExpectedContent(t *testing.T) {
	// Omitting expected_content = blind overwrite (existing behaviour unchanged)
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "occ-blind")
	id := mustPost(t, reg.Server, "occ-blind", "Claude", "original")

	res, _, err := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: id,
		Content:   "blind overwrite",
	})
	if err != nil {
		t.Fatalf("handleUpdateMessage error: %v", err)
	}
	if !strings.Contains(resultText(res), "updated") {
		t.Errorf("expected success, got: %s", resultText(res))
	}
}

// ========== fork_thread ==========

func TestHandleForkThread(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "fork-src", withProject("my-project"), withTags("go"))
	mustPost(t, reg.Server, "fork-src", "Alice", "First message — stays")
	anchor := mustPost(t, reg.Server, "fork-src", "Bob", "Forked start")
	mustPost(t, reg.Server, "fork-src", "Alice", "Follow-up")

	res, _, err := reg.handleForkThread(context.Background(), nil, ForkThreadInput{
		StartMessageID: anchor,
		NewRoomID:      "fork-dst",
		Topic:          "Forked topic",
	})
	if err != nil {
		t.Fatalf("handleForkThread error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "Forked 2 message(s)") {
		t.Errorf("expected 2 messages forked, got: %s", text)
	}
	if !strings.Contains(text, "fork-dst") {
		t.Errorf("missing new room in output: %s", text)
	}

	// New room exists with right topic and inherited project.
	dst, err := reg.Server.GetRoom("fork-dst")
	if err != nil {
		t.Fatalf("new room not created: %v", err)
	}
	if dst.Description != "Forked topic" {
		t.Errorf("expected topic 'Forked topic', got %q", dst.Description)
	}
	if dst.Project != "my-project" {
		t.Errorf("expected inherited project 'my-project', got %q", dst.Project)
	}

	// Source room retains only the first message.
	srcMsgs, _ := reg.Server.GetRecentMessages("fork-src", 10)
	if len(srcMsgs) != 1 {
		t.Errorf("expected 1 message in source, got %d", len(srcMsgs))
	}

	// New room has the 2 moved messages.
	dstMsgs, _ := reg.Server.GetRecentMessages("fork-dst", 10)
	if len(dstMsgs) != 2 {
		t.Errorf("expected 2 messages in fork-dst, got %d", len(dstMsgs))
	}

	// Both rooms are bidirectionally linked.
	src, _ := reg.Server.GetRoom("fork-src")
	if !strings.Contains(src.RelatedRooms, "fork-dst") {
		t.Errorf("source not linked to fork-dst: %q", src.RelatedRooms)
	}
	if !strings.Contains(dst.RelatedRooms, "fork-src") {
		t.Errorf("fork-dst not linked to fork-src: %q", dst.RelatedRooms)
	}
}

func TestHandleForkThreadInheritsProject(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "fp-src", withProject("inherited-proj"))
	id := mustPost(t, reg.Server, "fp-src", "Alice", "Message")

	reg.handleForkThread(context.Background(), nil, ForkThreadInput{
		StartMessageID: id,
		NewRoomID:      "fp-dst",
	})

	dst, _ := reg.Server.GetRoom("fp-dst")
	if dst.Project != "inherited-proj" {
		t.Errorf("expected inherited project, got %q", dst.Project)
	}
	if dst.Description != "Forked from fp-src" {
		t.Errorf("expected default topic, got %q", dst.Description)
	}
}

func TestHandleForkThreadExplicitProjectAndTags(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "fpt-src", withProject("old-project"))
	id := mustPost(t, reg.Server, "fpt-src", "Alice", "Message")

	reg.handleForkThread(context.Background(), nil, ForkThreadInput{
		StartMessageID: id,
		NewRoomID:      "fpt-dst",
		Project:        "new-project",
		Tags:           "go,api",
	})

	dst, _ := reg.Server.GetRoom("fpt-dst")
	if dst.Project != "new-project" {
		t.Errorf("expected explicit project 'new-project', got %q", dst.Project)
	}
	if !strings.Contains(dst.Tags, "go") {
		t.Errorf("expected tags, got %q", dst.Tags)
	}
}

func TestHandleForkThreadMissingStartMessageID(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleForkThread(context.Background(), nil, ForkThreadInput{
		NewRoomID: "fork-x",
	})
	if !strings.Contains(resultText(res), "start_message_id is required") {
		t.Errorf("expected required error, got: %s", resultText(res))
	}
}

func TestHandleForkThreadMissingNewRoomID(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "fmr-src")
	id := mustPost(t, reg.Server, "fmr-src", "Alice", "Message")
	res, _, _ := reg.handleForkThread(context.Background(), nil, ForkThreadInput{
		StartMessageID: id,
	})
	if !strings.Contains(resultText(res), "new_room_id is required") {
		t.Errorf("expected required error, got: %s", resultText(res))
	}
}

func TestHandleForkThreadMessageNotFound(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleForkThread(context.Background(), nil, ForkThreadInput{
		StartMessageID: "nonexistent-uuid-00000000",
		NewRoomID:      "fork-nf",
	})
	if !strings.Contains(resultText(res), "not found") {
		t.Errorf("expected not found error, got: %s", resultText(res))
	}
}

func TestHandleForkThreadRoomAlreadyExists(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "fork-collision-src")
	mustCreateRoom(t, reg.Server, "fork-collision-dst")
	id := mustPost(t, reg.Server, "fork-collision-src", "Alice", "Message")

	res, _, _ := reg.handleForkThread(context.Background(), nil, ForkThreadInput{
		StartMessageID: id,
		NewRoomID:      "fork-collision-dst",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Errorf("expected error when new_room_id already exists, got: %s", resultText(res))
	}
}
