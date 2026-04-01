package main

import (
	"context"
	"strings"
	"testing"
)

// -- db.go: getPinnedMessage error path --
func TestGetPinnedMessageError(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "pin-err")
	// Corrupt messages table to make the query fail
	cs.db.Exec("DROP TABLE messages")

	_, err := cs.getPinnedMessage("pin-err")
	if err == nil {
		t.Error("expected error for missing messages table")
	}
}

// -- db.go: scan error paths for list functions --
func TestScanErrorsDB(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "scan-err")
	mustPost(t, cs, "scan-err", "Claude", "Msg")

	// Corrupt messages schema for scan failures
	cs.db.Exec("ALTER TABLE messages RENAME TO messages_old")
	cs.db.Exec("CREATE TABLE messages (id INTEGER PRIMARY KEY, room_id TEXT)") // missing columns

	if _, err := cs.getMessagesAfterID("scan-err", 0); err == nil {
		t.Error("expected scan error in getMessagesAfterID")
	}

	if _, err := cs.getLatestPerType("scan-err"); err == nil {
		t.Error("expected scan error in getLatestPerType")
	}

	// Trigger scan error in getRoomStats author query
	cs.db.Exec("ALTER TABLE messages RENAME TO messages_old2")
	cs.db.Exec("CREATE TABLE messages (id INTEGER PRIMARY KEY, room_id TEXT, author INTEGER)") // incompatible author type
	cs.db.Exec("INSERT INTO messages (room_id, author) VALUES ('scan-err', 'not-an-int')")
	if _, err := cs.getRoomStats("scan-err"); err == nil {
		t.Error("expected scan error in getRoomStats")
	}
}

// -- db.go: getRoomStats scan error in type query --
func TestGetRoomStatsTypeScanError(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "type-scan-err")
	mustPost(t, cs, "type-scan-err", "Claude", "Msg")

	// Corrupt messages for type query scan error
	cs.db.Exec("ALTER TABLE messages RENAME TO messages_old")
	cs.db.Exec("CREATE TABLE messages (id INTEGER PRIMARY KEY, room_id TEXT, author TEXT, message_type INTEGER, is_summary BOOLEAN)")
	cs.db.Exec("INSERT INTO messages (room_id, author, message_type, is_summary) VALUES ('type-scan-err', 'Claude', 'not-an-int', 0)")

	if _, err := cs.getRoomStats("type-scan-err"); err == nil {
		t.Error("expected scan error in getRoomStats type query")
	}
}

// -- db.go: getMessageCounts scan error path --
func TestGetMessageCountsDBClose(t *testing.T) {
	cs := setupTestServer(t)
	cs.db.Close()
	counts := cs.getMessageCounts()
	if len(counts) != 0 {
		t.Error("expected empty counts for closed DB")
	}
}

// -- tools.go: handleGetOrCreateRoom edge cases --
func TestHandleGetOrCreateRoomEdgeCases(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-upsert-edges")

	// LastN > 50
	res, _, _ := cs.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "h-upsert-edges", LastN: "100",
	})
	if !strings.Contains(resultText(res), "Found") {
		t.Error("expected Found")
	}

	// LastN <= 0
	cs.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "h-upsert-edges", LastN: "0",
	})

	// Non-default message type in recent list
	mustPostTyped(t, cs, "h-upsert-edges", "Gemini", "A thought", "thought")
	res, _, _ = cs.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "h-upsert-edges",
	})
	if !strings.Contains(resultText(res), "(thought)") {
		t.Error("expected message type in recent list")
	}
}

func TestHandleGetOrCreateRoomCreateFail(t *testing.T) {
	cs := setupTestServer(t)
	cs.db.Close()
	_, _, err := cs.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{ID: "fail"})
	if err == nil {
		t.Error("expected error for closed DB")
	}
}

// -- tools.go: handleUpdateMessage error paths --
func TestHandleUpdateMessageDBError(t *testing.T) {
	cs := setupHandlerServer(t)
	cs.db.Close()

	_, _, err := cs.handleUpdateMessage(context.Background(), nil, UpdateMessageInput{
		MessageID: "1", Content: "Fail",
	})
	if err == nil {
		t.Error("expected error for closed DB")
	}
}

// -- tools.go: handleReadTranscript mode=summary empty room --
func TestHandleReadTranscriptSummaryEmpty(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "sum-empty")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "sum-empty", Mode: "summary",
	})
	if !strings.Contains(resultText(res), "No messages yet") {
		t.Error("expected 'No messages yet' in empty summary")
	}
}

// -- janitor.go: more error paths --
func TestJanitorSweepErrorPaths(t *testing.T) {
	cs := setupTestServer(t)
	cs.db.Close()
	cs.janitorSweep() // Should log error and return
}

func TestJanitorSweepUnsummarizedError(t *testing.T) {
	cs := setupTestServer(t)
	setupRoomWithMessages(t, cs, "j-unsum-err", 25)

	// Corrupt messages table so getUnsummarizedMessages fails but getRoomsNeedingSummary works
	cs.db.Exec("ALTER TABLE messages RENAME TO messages_old")
	cs.db.Exec("CREATE TABLE messages (room_id TEXT, is_summary BOOLEAN, author TEXT)")
	for i := 0; i < 21; i++ {
		cs.db.Exec("INSERT INTO messages (room_id, is_summary) VALUES ('j-unsum-err', 0)")
	}

	cs.janitorSweep() // Should hit "failed to get unsummarized messages"
}

// -- tools.go: handleDeleteMessages error paths --
func TestHandleDeleteMessagesMoreErrors(t *testing.T) {
	cs := setupHandlerServer(t)

	// Dry run DB error
	cs.db.Close()
	_, _, err := cs.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{
		MessageIDs: "1", DryRun: "true",
	})
	if err == nil {
		t.Error("expected dry run DB error")
	}
}

// -- tools.go: handlePinMessage error path --
func TestHandlePinMessageDBError(t *testing.T) {
	cs := setupHandlerServer(t)
	cs.db.Close()
	res, _, err := cs.handlePinMessage(context.Background(), nil, PinMessageInput{
		RoomID: "x", MessageID: "1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected Error in result text for closed DB")
	}
}

// -- tools.go: handleReadTranscript error paths --
func TestHandleReadTranscriptErrors(t *testing.T) {
	cs := setupHandlerServer(t)

	// Summary mode DB error
	cs.db.Close()
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "hdb-room", Mode: "summary",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected summary mode error for closed DB")
	}

	// Changelog mode error
	cs2 := setupHandlerServer(t)
	cs2.db.Close()
	res, _, _ = cs2.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "hdb-room", Mode: "changelog",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected changelog mode error for closed DB")
	}

	// after_id mode error
	cs3 := setupHandlerServer(t)
	cs3.db.Close()
	res, _, _ = cs3.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "hdb-room", AfterID: "1",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected after_id mode error for closed DB")
	}

	// Full transcript mode error
	cs4 := setupHandlerServer(t)
	cs4.db.Close()
	res, _, _ = cs4.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "hdb-room",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected full transcript error for closed DB")
	}
}

func TestHandleReadTranscriptAfterIDInvalidEdge(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "after-bad")
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "after-bad", AfterID: "not-a-number",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for invalid after_id")
	}
}

// -- tools.go: handleGetMessages more coverage --
func TestHandleGetMessagesMore(t *testing.T) {
	cs := setupHandlerServer(t)

	// IDs branch DB error returns real error
	cs.db.Close()
	_, _, err := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		MessageIDs: "1",
	})
	if err == nil {
		t.Error("expected IDs branch error for closed DB")
	}
}
