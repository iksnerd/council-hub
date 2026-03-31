package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ========== pin_message tests ==========

func TestHandlePinMessage(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("pin-room", "Pin test", "", "", "", "", "")
	id, _ := cs.postMessage("pin-room", "Claude", "Important TL;DR", "decision", 0)

	res, _, err := cs.handlePinMessage(context.Background(), nil, PinMessageInput{
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
	pinned, err := cs.getPinnedMessage("pin-room")
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
	cs := setupTestServer(t)
	cs.createRoom("toggle-room", "Toggle test", "", "", "", "", "")
	id, _ := cs.postMessage("toggle-room", "Claude", "Pin me", "message", 0)
	idStr := fmt.Sprintf("%d", id)

	// Pin
	cs.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID: "toggle-room", MessageID: idStr,
	})

	// Unpin (toggle)
	res, _, err := cs.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID: "toggle-room", MessageID: idStr,
	})
	if err != nil {
		t.Fatalf("handlePinMessage toggle error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "unpinned") {
		t.Errorf("expected 'unpinned' in result, got: %s", text)
	}

	// Verify no pinned message
	pinned, _ := cs.getPinnedMessage("toggle-room")
	if pinned != nil {
		t.Error("expected no pinned message after toggle, got one")
	}
}

func TestHandlePinMessageReplacesExisting(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("replace-room", "Replace test", "", "", "", "", "")
	id1, _ := cs.postMessage("replace-room", "Claude", "First pin", "message", 0)
	id2, _ := cs.postMessage("replace-room", "Gemini", "Second pin", "decision", 0)

	// Pin first
	cs.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID: "replace-room", MessageID: fmt.Sprintf("%d", id1),
	})

	// Pin second — should unpin first
	cs.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID: "replace-room", MessageID: fmt.Sprintf("%d", id2),
	})

	pinned, _ := cs.getPinnedMessage("replace-room")
	if pinned == nil {
		t.Fatal("expected pinned message, got nil")
	}
	if pinned.ID != id2 {
		t.Errorf("expected message %d pinned, got %d", id2, pinned.ID)
	}

	// Verify first is unpinned
	msgs, _ := cs.getMessagesByIDs([]int64{id1})
	if len(msgs) > 0 && msgs[0].Pinned {
		t.Error("first message should be unpinned after replacement")
	}
}

func TestHandlePinMessageWrongRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("room-a", "Room A", "", "", "", "", "")
	cs.createRoom("room-b", "Room B", "", "", "", "", "")
	id, _ := cs.postMessage("room-a", "Claude", "In room A", "message", 0)

	res, _, _ := cs.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID:    "room-b",
		MessageID: fmt.Sprintf("%d", id),
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") || !strings.Contains(text, "room-a") {
		t.Errorf("expected error about wrong room, got: %s", text)
	}
}

func TestHandlePinMessageMissingFields(t *testing.T) {
	cs := setupTestServer(t)

	// Missing room_id
	res, _, _ := cs.handlePinMessage(context.Background(), nil, PinMessageInput{MessageID: "1"})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing room_id")
	}

	// Missing message_id
	res, _, _ = cs.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "room"})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing message_id")
	}

	// Invalid message_id
	res, _, _ = cs.handlePinMessage(context.Background(), nil, PinMessageInput{RoomID: "room", MessageID: "abc"})
	if !strings.Contains(resultText(res), "not a valid") {
		t.Error("expected error for invalid message_id")
	}
}

func TestHandlePinMessageNotFound(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("pin-nf", "Not found test", "", "", "", "", "")

	res, _, _ := cs.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID: "pin-nf", MessageID: "99999",
	})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found error, got: %s", text)
	}
}

func TestPinnedMessageInTranscript(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("transcript-pin", "Transcript pin test", "", "", "", "", "")
	cs.postMessage("transcript-pin", "Claude", "Regular message 1", "message", 0)
	id2, _ := cs.postMessage("transcript-pin", "Gemini", "This is the TL;DR", "decision", 0)
	cs.postMessage("transcript-pin", "Claude", "Regular message 3", "message", 0)

	cs.pinMessage("transcript-pin", id2)

	room, _ := cs.getRoom("transcript-pin")
	msgs, _ := cs.getTranscript("transcript-pin")
	transcript := formatTranscript(room, msgs)

	// Pinned message should appear before regular messages
	pinnedIdx := strings.Index(transcript, "PINNED")
	msg1Idx := strings.Index(transcript, "Regular message 1")
	msg3Idx := strings.Index(transcript, "Regular message 3")

	if pinnedIdx == -1 {
		t.Fatal("PINNED label not found in transcript")
	}
	if pinnedIdx > msg1Idx {
		t.Error("pinned message should appear before first regular message")
	}
	if pinnedIdx > msg3Idx {
		t.Error("pinned message should appear before last regular message")
	}

	// Pinned content should appear only once (not duplicated)
	count := strings.Count(transcript, "This is the TL;DR")
	if count != 1 {
		t.Errorf("pinned message content should appear exactly once, appeared %d times", count)
	}
}

func TestPinnedMessageInSummaryMode(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("summary-pin", "Summary pin test", "", "", "", "Test instructions", "")
	id, _ := cs.postMessage("summary-pin", "Claude", "Pinned summary content", "decision", 0)
	cs.postMessage("summary-pin", "Gemini", "A thought", "thought", 0)

	cs.pinMessage("summary-pin", id)

	res, _, err := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "summary-pin", Mode: "summary",
	})
	if err != nil {
		t.Fatalf("handleReadTranscript error: %v", err)
	}
	text := resultText(res)

	if !strings.Contains(text, "PINNED") {
		t.Error("summary mode should include PINNED label")
	}
	if !strings.Contains(text, "Pinned summary content") {
		t.Error("summary mode should include pinned message content")
	}
}

func TestPinnedMessageInAfterIDMode(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("afterid-pin", "After ID pin test", "", "", "", "", "")
	id1, _ := cs.postMessage("afterid-pin", "Claude", "Pinned overview", "decision", 0)
	cs.postMessage("afterid-pin", "Gemini", "Second msg", "message", 0)
	id3, _ := cs.postMessage("afterid-pin", "Claude", "Third msg", "action", 0)

	cs.pinMessage("afterid-pin", id1)

	// Read after id2 — should still include pinned message at top
	res, _, err := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID:  "afterid-pin",
		AfterID: fmt.Sprintf("%d", id3-1),
	})
	if err != nil {
		t.Fatalf("handleReadTranscript error: %v", err)
	}
	text := resultText(res)

	if !strings.Contains(text, "PINNED") {
		t.Error("after_id mode should include PINNED message for context")
	}
	if !strings.Contains(text, "Pinned overview") {
		t.Error("after_id mode should include pinned message content")
	}
}

// ========== update_message tests ==========

func TestHandleUpdateMessage(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("update-room", "Update test", "", "", "", "", "")
	id, _ := cs.postMessage("update-room", "Claude", "Original content", "message", 0)

	res, _, err := cs.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: fmt.Sprintf("%d", id),
		Content:   "Updated content",
	})
	if err != nil {
		t.Fatalf("handleUpdateMessage error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "updated") {
		t.Errorf("expected 'updated' in result, got: %s", text)
	}

	// Verify content changed
	msgs, _ := cs.getMessagesByIDs([]int64{id})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "Updated content" {
		t.Errorf("expected 'Updated content', got '%s'", msgs[0].Content)
	}
}

func TestHandleUpdateMessageWithType(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("type-room", "Type test", "", "", "", "", "")
	id, _ := cs.postMessage("type-room", "Claude", "Original", "message", 0)

	res, _, err := cs.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID:   fmt.Sprintf("%d", id),
		Content:     "Updated decision",
		MessageType: "decision",
	})
	if err != nil {
		t.Fatalf("handleUpdateMessage error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "decision") {
		t.Errorf("expected 'decision' type in result, got: %s", text)
	}

	msgs, _ := cs.getMessagesByIDs([]int64{id})
	if msgs[0].MessageType != "decision" {
		t.Errorf("expected type 'decision', got '%s'", msgs[0].MessageType)
	}
}

func TestHandleUpdateMessagePreservesFields(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("preserve-room", "Preserve test", "", "", "", "", "")
	id, _ := cs.postMessage("preserve-room", "OriginalAuthor", "Original", "review", 42)

	cs.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: fmt.Sprintf("%d", id),
		Content:   "Updated",
	})

	msgs, _ := cs.getMessagesByIDs([]int64{id})
	m := msgs[0]
	if m.Author != "OriginalAuthor" {
		t.Errorf("author changed: expected 'OriginalAuthor', got '%s'", m.Author)
	}
	if m.RoomID != "preserve-room" {
		t.Errorf("room_id changed: expected 'preserve-room', got '%s'", m.RoomID)
	}
	if m.MessageType != "review" {
		t.Errorf("message_type changed: expected 'review', got '%s'", m.MessageType)
	}
	if m.ReplyTo != 42 {
		t.Errorf("reply_to changed: expected 42, got %d", m.ReplyTo)
	}
}

func TestHandleUpdateMessageNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: "99999",
		Content:   "Won't work",
	})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' error, got: %s", text)
	}
}

func TestHandleUpdateMessageInvalidType(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("bad-type-room", "Test", "", "", "", "", "")
	id, _ := cs.postMessage("bad-type-room", "Claude", "Test", "message", 0)

	res, _, _ := cs.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID:   fmt.Sprintf("%d", id),
		Content:     "Updated",
		MessageType: "invalid_type",
	})
	text := resultText(res)
	if !strings.Contains(text, "invalid message_type") {
		t.Errorf("expected invalid type error, got: %s", text)
	}
}

func TestHandleUpdateMessageMissingFields(t *testing.T) {
	cs := setupTestServer(t)

	// Missing message_id
	res, _, _ := cs.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{Content: "x"})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing message_id")
	}

	// Missing content
	res, _, _ = cs.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{MessageID: "1"})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing content")
	}

	// Invalid message_id
	res, _, _ = cs.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{MessageID: "abc", Content: "x"})
	if !strings.Contains(resultText(res), "not a valid") {
		t.Error("expected error for invalid message_id")
	}
}

// ========== delete_messages dry_run tests ==========

func TestHandleDeleteMessagesDryRun(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("dry-room", "Dry run test", "", "", "", "", "")
	id1, _ := cs.postMessage("dry-room", "Claude", "Message one", "message", 0)
	id2, _ := cs.postMessage("dry-room", "Gemini", "Message two", "thought", 0)

	res, _, err := cs.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
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
	msgs, _ := cs.getMessagesByIDs([]int64{id1, id2})
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages to still exist, got %d", len(msgs))
	}
}

func TestHandleDeleteMessagesDryRunNotFound(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("dry-nf", "Dry run not found", "", "", "", "", "")
	id, _ := cs.postMessage("dry-nf", "Claude", "Exists", "message", 0)

	res, _, _ := cs.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
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
	cs := setupTestServer(t)
	cs.createRoom("dry-false", "Dry run false", "", "", "", "", "")
	id, _ := cs.postMessage("dry-false", "Claude", "Delete me", "message", 0)

	// dry_run=false should still delete
	res, _, _ := cs.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: fmt.Sprintf("%d", id),
		DryRun:     "false",
	})
	text := resultText(res)
	if !strings.Contains(text, "Deleted 1 message") {
		t.Errorf("expected deletion confirmation, got: %s", text)
	}

	msgs, _ := cs.getMessagesByIDs([]int64{id})
	if len(msgs) != 0 {
		t.Error("message should be deleted when dry_run=false")
	}
}

func TestHandleDeleteMessagesDryRunOmitted(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("dry-omit", "Dry run omitted", "", "", "", "", "")
	id, _ := cs.postMessage("dry-omit", "Claude", "Delete me too", "message", 0)

	// No dry_run param — should delete (backward compatible)
	res, _, _ := cs.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: fmt.Sprintf("%d", id),
	})
	text := resultText(res)
	if !strings.Contains(text, "Deleted 1 message") {
		t.Errorf("expected deletion confirmation, got: %s", text)
	}
}

func TestHandleDeleteMessagesDryRunExcerptTruncation(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("dry-trunc", "Truncation test", "", "", "", "", "")
	longContent := strings.Repeat("A", 200)
	id, _ := cs.postMessage("dry-trunc", "Claude", longContent, "message", 0)

	res, _, _ := cs.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
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

// ========== DB-level pinMessage tests ==========

func TestPinMessageDB(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("db-pin", "DB pin test", "", "", "", "", "")
	id, _ := cs.postMessage("db-pin", "Claude", "Pin me", "message", 0)

	pinned, err := cs.pinMessage("db-pin", id)
	if err != nil {
		t.Fatalf("pinMessage error: %v", err)
	}
	if !pinned {
		t.Error("expected pinned=true")
	}

	// Verify
	msg, _ := cs.getPinnedMessage("db-pin")
	if msg == nil || msg.ID != id {
		t.Error("getPinnedMessage should return the pinned message")
	}
}

func TestGetPinnedMessageNone(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("no-pin", "No pin", "", "", "", "", "")

	msg, err := cs.getPinnedMessage("no-pin")
	if err != nil {
		t.Fatalf("getPinnedMessage error: %v", err)
	}
	if msg != nil {
		t.Error("expected nil when no message is pinned")
	}
}

// ========== DB-level updateMessage tests ==========

func TestUpdateMessageDB(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("db-update", "DB update test", "", "", "", "", "")
	id, _ := cs.postMessage("db-update", "Claude", "Original", "message", 0)

	m, err := cs.updateMessage(id, "Updated", "")
	if err != nil {
		t.Fatalf("updateMessage error: %v", err)
	}
	if m.Content != "Updated" {
		t.Errorf("expected 'Updated', got '%s'", m.Content)
	}
	if m.MessageType != "message" {
		t.Errorf("type should be preserved as 'message', got '%s'", m.MessageType)
	}
}

func TestUpdateMessageDBWithType(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("db-uptype", "DB update type test", "", "", "", "", "")
	id, _ := cs.postMessage("db-uptype", "Claude", "Original", "message", 0)

	m, err := cs.updateMessage(id, "Now a decision", "decision")
	if err != nil {
		t.Fatalf("updateMessage error: %v", err)
	}
	if m.MessageType != "decision" {
		t.Errorf("expected type 'decision', got '%s'", m.MessageType)
	}
}

func TestUpdateMessageDBNotFound(t *testing.T) {
	cs := setupTestServer(t)

	_, err := cs.updateMessage(99999, "Nope", "")
	if err == nil {
		t.Error("expected error for nonexistent message")
	}
}
