package handlers

import (
	"context"
	"fmt"
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
		MessageID: fmt.Sprintf("%d", id),
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
	}
	if pinned.ID != id {
		t.Errorf("expected pinned message ID %d, got %d", id, pinned.ID)
	}
}

func TestHandlePinMessageToggle(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pin-toggle")
	id := mustPost(t, reg.Server, "pin-toggle", "Claude", "Pin/Unpin")

	// Pin it
	reg.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "pin-toggle", MessageID: fmt.Sprintf("%d", id)})

	// Unpin it (toggle)
	res, _, _ := reg.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID:    "pin-toggle",
		MessageID: fmt.Sprintf("%d", id),
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
	reg.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "pin-replace", MessageID: fmt.Sprintf("%d", id1)})

	// Pin second — should replace first
	reg.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "pin-replace", MessageID: fmt.Sprintf("%d", id2)})

	pinned, _ := reg.Server.GetPinnedMessage("pin-replace")
	if pinned == nil || pinned.ID != id2 {
		t.Errorf("expected message %d to be pinned, got %v", id2, pinned)
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
		MessageID: fmt.Sprintf("%d", id),
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
		RoomID: "pin-nf", MessageID: "99999",
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
		MessageID: fmt.Sprintf("%d", id),
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
	msgs, _ := reg.Server.GetMessagesByIDs([]int64{id})
	if len(msgs) == 0 || msgs[0].Content != "New content" {
		t.Errorf("expected content update, got: %v", msgs)
	}
}

func TestHandleUpdateMessageWithType(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-upd-type")
	id := mustPost(t, reg.Server, "h-upd-type", "Claude", "Msg")

	reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID:   fmt.Sprintf("%d", id),
		Content:     "Msg",
		MessageType: "decision",
	})

	msgs, _ := reg.Server.GetMessagesByIDs([]int64{id})
	if len(msgs) == 0 || msgs[0].MessageType != "decision" {
		t.Errorf("expected type update, got: %v", msgs)
	}
}

func TestHandleUpdateMessagePreservesFields(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-upd-pres")
	id := mustPost(t, reg.Server, "h-upd-pres", "Claude", "Original")

	reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: fmt.Sprintf("%d", id),
		Content:   "Updated",
	})

	msgs, _ := reg.Server.GetMessagesByIDs([]int64{id})
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
		MessageID: "99999",
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
		MessageID:   fmt.Sprintf("%d", id),
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

func TestHandleUpdateMessageNonParseableID(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: "not-a-number",
		Content:   "some content",
	})
	if !strings.Contains(resultText(res), "not a valid message ID") {
		t.Errorf("expected invalid ID error, got: %s", resultText(res))
	}
}

func TestHandlePinMessageNonParseableID(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "pin-parse-err")
	res, _, _ := reg.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID:    "pin-parse-err",
		MessageID: "not-a-number",
	})
	if !strings.Contains(resultText(res), "not a valid message ID") {
		t.Errorf("expected invalid ID error, got: %s", resultText(res))
	}
}

// ========== delete_messages ==========

func TestHandleDeleteMessages(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-delmsg")
	mustPost(t, reg.Server, "h-delmsg", "Claude", "Delete me")

	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{MessageIDs: "1"})
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

func TestHandleDeleteMessagesInvalidID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{MessageIDs: "abc"})
	if !strings.Contains(resultText(res), "not a valid") {
		t.Error("expected invalid ID error")
	}
}

func TestHandleDeleteMessagesDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	_, _, err := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{MessageIDs: "1"})
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
		MessageIDs: fmt.Sprintf("%d,%d", id1, id2),
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
	msgs, _ := reg.Server.GetMessagesByIDs([]int64{id1, id2})
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages to still exist, got %d", len(msgs))
	}
}

func TestHandleDeleteMessagesDryRunNotFound(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "dry-nf")
	id := mustPost(t, reg.Server, "dry-nf", "Claude", "Exists")

	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: fmt.Sprintf("%d,99999", id),
		DryRun:     "true",
	})
	text := resultText(res)

	if !strings.Contains(text, "1 message(s) would be deleted") {
		t.Errorf("expected '1 message(s) would be deleted', got: %s", text)
	}
	if !strings.Contains(text, "#99999") && !strings.Contains(text, "not found") {
		t.Error("expected not found indicator for missing ID")
	}
}

func TestHandleDeleteMessagesDryRunFalse(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "dry-false")
	id := mustPost(t, reg.Server, "dry-false", "Claude", "Delete me")

	// dry_run=false should still delete
	res, _, _ := reg.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: fmt.Sprintf("%d", id),
		DryRun:     "false",
	})
	text := resultText(res)
	if !strings.Contains(text, "Deleted 1 message") {
		t.Errorf("expected deletion confirmation, got: %s", text)
	}

	msgs, _ := reg.Server.GetMessagesByIDs([]int64{id})
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
		MessageIDs: fmt.Sprintf("%d", id),
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
		MessageIDs: fmt.Sprintf("%d", id),
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
