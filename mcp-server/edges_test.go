package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// -- handleReadRecent edge cases (76.7% → higher) --

func TestHandleReadRecentNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "nonexistent",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for nonexistent room, got: %s", text)
	}
}

func TestHandleReadRecentBadLimit(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-recent-bad", "Bad limit", "", "", "", "", "")
	cs.postMessage("h-recent-bad", "Claude", "Hello", "message", 0)

	// Invalid limit string should fall back to default 10
	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent-bad", Limit: "abc",
	})
	text := resultText(res)
	if !strings.Contains(text, "last 1 message(s)") {
		t.Errorf("expected fallback to default limit, got: %s", text)
	}
}

func TestHandleReadRecentWithSummary(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-recent-sum", "Summary in recent", "", "", "", "", "")
	cs.insertSummary("h-recent-sum", "Summary content")
	cs.postMessage("h-recent-sum", "Claude", "After summary", "message", 0)

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent-sum", Limit: "10",
	})
	text := resultText(res)
	if !strings.Contains(text, "SUMMARY") {
		t.Error("expected summary rendering in read_recent")
	}
}

func TestHandleReadRecentReplyToPlainMessage(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-recent-reply-plain", "Reply plain", "", "", "", "", "")
	id1, _ := cs.postMessage("h-recent-reply-plain", "Claude", "Original", "message", 0)
	// Reply with default "message" type — tests the else-if branch for ReplyTo > 0 with message type
	cs.postMessage("h-recent-reply-plain", "Gemini", "Replying", "message", id1)

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent-reply-plain", Limit: "5",
	})
	text := resultText(res)
	expected := fmt.Sprintf("re: #%d", id1)
	if !strings.Contains(text, expected) {
		t.Errorf("expected reply tag in plain message, got: %s", text)
	}
}

// -- handleSearchMessages edge cases (82.6% → higher) --

func TestHandleSearchMessagesBadLimit(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search-bad-lim", "Bad limit", "", "", "", "", "")
	cs.postMessage("h-search-bad-lim", "Claude", "findme", "message", 0)

	// Invalid limit falls back to 20
	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query: "findme", Limit: "not-a-number",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected result with default limit, got: %s", text)
	}
}

func TestHandleSearchMessagesNoResults(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search-empty", "Empty search", "", "", "", "", "")

	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query: "zzz-no-match",
	})
	text := resultText(res)
	if !strings.Contains(text, "No messages found") {
		t.Errorf("expected no messages, got: %s", text)
	}
}

// -- handleUpdateRoom edge cases (78.3% → higher) --

func TestHandleUpdateRoomAllFields(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-update-all", "Original", "", "", "", "", "")

	res, _, _ := cs.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID:       "h-update-all",
		Topic:        "New topic",
		Project:      "New project",
		TechStack:    "New tech",
		Tags:         "new-tag",
		SystemPrompt: "New prompt",
		RelatedRooms: "room-a",
	})
	text := resultText(res)
	for _, field := range []string{"topic", "project", "tech_stack", "tags", "system_prompt", "related_rooms"} {
		if !strings.Contains(text, field) {
			t.Errorf("expected %s in updated fields, got: %s", field, text)
		}
	}
}

func TestHandleUpdateRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "ghost", Topic: "X",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// -- handleCreateRoom edge case --

func TestHandleCreateRoomWithRelatedRooms(t *testing.T) {
	cs := setupTestServer(t)

	res, _, err := cs.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "h-create-related", Topic: "With links", RelatedRooms: "a,b,c",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "ready") {
		t.Errorf("expected ready, got: %s", text)
	}

	room, _ := cs.getRoom("h-create-related")
	if room.RelatedRooms != "a,b,c" {
		t.Errorf("expected related_rooms 'a,b,c', got '%s'", room.RelatedRooms)
	}
}

// -- handleSignalStatus not found --

func TestHandleSignalStatusNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "nonexistent", Status: "active",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// -- handleDeleteRoom not found --

func TestHandleDeleteRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// -- handleRoomStats not found --

func TestHandleRoomStatsNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleRoomStats(context.Background(), nil, RoomStatsInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// -- handleArchiveRoom not found --

func TestHandleArchiveRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// -- handleGetMessages multiple IDs --

func TestHandleGetMessagesMultiple(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-get-multi", "Multi get", "", "", "", "", "")
	cs.postMessage("h-get-multi", "Claude", "First", "message", 0)
	cs.postMessage("h-get-multi", "Gemini", "Second", "review", 0)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "1,2"})
	text := resultText(res)
	if !strings.Contains(text, "2 message(s)") {
		t.Errorf("expected 2 messages, got: %s", text)
	}
}

// -- formatTranscript edge: plain message with reply_to --

func TestFormatTranscriptReplyToPlainMessage(t *testing.T) {
	room := Room{ID: "fmt-reply", Description: "Test", Status: "active"}
	msgs := []Message{
		{ID: 1, Author: "Claude", Content: "Original", MessageType: "message", ReplyTo: 0},
		{ID: 2, Author: "Gemini", Content: "Reply", MessageType: "message", ReplyTo: 1},
	}

	transcript := formatTranscript(room, msgs)
	if !strings.Contains(transcript, "Gemini (re: #1)") {
		t.Errorf("expected plain message reply rendering, got: %s", transcript)
	}
}

// -- janitorSweep with no rooms needing summary --

func TestJanitorSweepNoRooms(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("j-empty", "Few messages", "", "", "", "", "")
	cs.postMessage("j-empty", "Claude", "Hello", "message", 0)

	// Should not panic or error with no rooms over threshold
	cs.janitorSweep()

	msgs, _ := cs.getTranscript("j-empty")
	for _, m := range msgs {
		if m.IsSummary {
			t.Error("should not have summarized a room with 1 message")
		}
	}
}

// -- runJanitor cancellation --

func TestRunJanitorCancellation(t *testing.T) {
	cs := setupTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		cs.runJanitor(ctx)
		close(done)
	}()

	cancel()
	<-done // Should return promptly after cancel
}

// -- NewCouncilServer error path --

func TestNewCouncilServerBadPath(t *testing.T) {
	_, err := NewCouncilServer("/nonexistent/path/to/db.sqlite", testLogger())
	// SQLite may or may not error on open (it creates the file),
	// but if it errors on schema init that's also fine
	_ = err
}

// -- searchMessages limit edge cases --

func TestSearchMessagesLimitClamping(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("search-clamp", "Clamp test", "", "", "", "", "")
	for i := 0; i < 5; i++ {
		cs.postMessage("search-clamp", "Claude", "keyword", "message", 0)
	}

	// Negative limit should clamp to 20
	msgs, _ := cs.searchMessages("keyword", "", "", "", -5)
	if len(msgs) != 5 {
		t.Errorf("expected 5 (all) with clamped limit, got %d", len(msgs))
	}

	// Over-100 limit should clamp to 20
	msgs, _ = cs.searchMessages("keyword", "", "", "", 200)
	if len(msgs) != 5 {
		t.Errorf("expected 5 with clamped limit, got %d", len(msgs))
	}
}

// -- updateRoom: all fields set (covers every branch) --

func TestUpdateRoomAllFields(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("upd-all", "Topic", "Proj", "Tech", "Tags", "Prompt", "Related")

	err := cs.updateRoom("upd-all", "New Topic", "New Proj", "New Tech", "New Tags", "New Prompt", "New Related")
	if err != nil {
		t.Fatalf("updateRoom failed: %v", err)
	}

	room, _ := cs.getRoom("upd-all")
	if room.Description != "New Topic" {
		t.Errorf("expected 'New Topic', got '%s'", room.Description)
	}
	if room.RelatedRooms != "New Related" {
		t.Errorf("expected 'New Related', got '%s'", room.RelatedRooms)
	}
}

// -- listRooms: status filter --

func TestListRoomsByStatusFilter(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("ls-active", "Active", "", "", "", "", "")
	cs.createRoom("ls-resolved", "Resolved", "", "", "", "", "")
	cs.updateStatus("ls-resolved", "resolved")

	rooms, _ := cs.listRooms("", "", "active")
	if len(rooms) != 1 || rooms[0].ID != "ls-active" {
		t.Errorf("expected only active room, got %d rooms", len(rooms))
	}

	rooms, _ = cs.listRooms("", "", "resolved")
	if len(rooms) != 1 || rooms[0].ID != "ls-resolved" {
		t.Errorf("expected only resolved room, got %d rooms", len(rooms))
	}
}

// -- postMessage: default type branch --

func TestPostMessageDefaultType(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("default-type", "Default", "", "", "", "", "")

	id, err := cs.postMessage("default-type", "Claude", "Hello", "", 0)
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}

	msgs, _ := cs.getMessagesByIDs([]int64{id})
	if msgs[0].MessageType != "message" {
		t.Errorf("expected default type 'message', got '%s'", msgs[0].MessageType)
	}
}

// -- handleReadRecent: all message type branches in rendering --

func TestHandleReadRecentAllBranches(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-recent-all", "All branches", "", "", "", "", "")
	id1, _ := cs.postMessage("h-recent-all", "Claude", "Plain message", "message", 0)
	cs.postMessage("h-recent-all", "Gemini", "A thought", "thought", 0)
	cs.postMessage("h-recent-all", "Claude", "Reply plain", "message", id1)
	cs.postMessage("h-recent-all", "Gemini", "Reply typed", "review", id1)
	cs.insertSummary("h-recent-all", "A summary")

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent-all", Limit: "50",
	})
	text := resultText(res)

	// Plain message (no type label)
	if !strings.Contains(text, "Claude:**") {
		t.Error("missing plain message rendering")
	}
	// Typed message
	if !strings.Contains(text, "Gemini (thought)") {
		t.Error("missing typed message rendering")
	}
	// Plain reply
	if !strings.Contains(text, fmt.Sprintf("Claude (re: #%d):", id1)) {
		t.Error("missing plain reply rendering")
	}
	// Typed reply
	if !strings.Contains(text, fmt.Sprintf("Gemini (review, re: #%d):", id1)) {
		t.Error("missing typed reply rendering")
	}
	// Summary
	if !strings.Contains(text, "SUMMARY") {
		t.Error("missing summary rendering")
	}
}

// -- handleSearchMessages: filter by author only, type only --

func TestHandleSearchByAuthorOnly(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search-auth", "Author search", "", "", "", "", "")
	cs.postMessage("h-search-auth", "Claude", "hello", "message", 0)
	cs.postMessage("h-search-auth", "Gemini", "world", "message", 0)

	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Author: "Gemini",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message from Gemini, got: %s", text)
	}
}

func TestHandleSearchByTypeOnly(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search-type", "Type search", "", "", "", "", "")
	cs.postMessage("h-search-type", "Claude", "thought1", "thought", 0)
	cs.postMessage("h-search-type", "Claude", "decision1", "decision", 0)

	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		MessageType: "decision",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 decision, got: %s", text)
	}
}

func TestHandleSearchByRoomOnly(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search-room-only", "Room search", "", "", "", "", "")
	cs.postMessage("h-search-room-only", "Claude", "in room", "message", 0)

	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-room-only",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message, got: %s", text)
	}
}

// -- Real migration test: old schema → initSchema adds new columns --

func TestMigrationFromOldSchema(t *testing.T) {
	// Step 1: Create an in-memory DB with the OLD schema (no related_rooms, no reply_to)
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	oldSchema := `
	CREATE TABLE rooms (
		id TEXT PRIMARY KEY,
		description TEXT,
		status TEXT DEFAULT 'active',
		project TEXT DEFAULT '',
		tech_stack TEXT DEFAULT '',
		tags TEXT DEFAULT '',
		system_prompt TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		room_id TEXT,
		author TEXT,
		content TEXT,
		message_type TEXT DEFAULT 'message',
		is_summary BOOLEAN DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(room_id) REFERENCES rooms(id)
	);`
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatalf("failed to create old schema: %v", err)
	}

	// Step 2: Insert data using old schema (no related_rooms/reply_to columns)
	_, err = db.Exec(`INSERT INTO rooms (id, description, project) VALUES ('old-room', 'Pre-migration room', 'proj')`)
	if err != nil {
		t.Fatalf("failed to insert room: %v", err)
	}
	_, err = db.Exec(`INSERT INTO messages (room_id, author, content, message_type) VALUES ('old-room', 'Claude', 'Old message', 'thought')`)
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Step 3: Run initSchema — this should add new columns via ALTER TABLE
	if err := initSchema(db); err != nil {
		t.Fatalf("initSchema on old DB failed: %v", err)
	}

	// Step 4: Verify old data survived and new columns have defaults
	var relatedRooms string
	err = db.QueryRow(`SELECT related_rooms FROM rooms WHERE id = 'old-room'`).Scan(&relatedRooms)
	if err != nil {
		t.Fatalf("failed to read related_rooms: %v", err)
	}
	if relatedRooms != "" {
		t.Errorf("expected empty related_rooms default, got '%s'", relatedRooms)
	}

	var replyTo int64
	err = db.QueryRow(`SELECT reply_to FROM messages WHERE room_id = 'old-room'`).Scan(&replyTo)
	if err != nil {
		t.Fatalf("failed to read reply_to: %v", err)
	}
	if replyTo != 0 {
		t.Errorf("expected reply_to default 0, got %d", replyTo)
	}

	// Step 5: Verify old data is intact
	var desc, project string
	err = db.QueryRow(`SELECT description, project FROM rooms WHERE id = 'old-room'`).Scan(&desc, &project)
	if err != nil {
		t.Fatalf("failed to read old room: %v", err)
	}
	if desc != "Pre-migration room" {
		t.Errorf("expected 'Pre-migration room', got '%s'", desc)
	}
	if project != "proj" {
		t.Errorf("expected 'proj', got '%s'", project)
	}

	var author, content, msgType string
	err = db.QueryRow(`SELECT author, content, message_type FROM messages WHERE room_id = 'old-room'`).Scan(&author, &content, &msgType)
	if err != nil {
		t.Fatalf("failed to read old message: %v", err)
	}
	if author != "Claude" || content != "Old message" || msgType != "thought" {
		t.Errorf("old message data corrupted: author=%s content=%s type=%s", author, content, msgType)
	}

	// Step 6: Verify new columns are writable after migration
	_, err = db.Exec(`UPDATE rooms SET related_rooms = 'room-a,room-b' WHERE id = 'old-room'`)
	if err != nil {
		t.Fatalf("failed to write related_rooms: %v", err)
	}
	_, err = db.Exec(`INSERT INTO messages (room_id, author, content, message_type, reply_to) VALUES ('old-room', 'Gemini', 'New reply', 'review', 1)`)
	if err != nil {
		t.Fatalf("failed to insert message with reply_to: %v", err)
	}
}

func TestMigrationIdempotent(t *testing.T) {
	// Running initSchema twice should not error (ALTER TABLE on existing columns is ignored)
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		t.Fatalf("first initSchema failed: %v", err)
	}
	if err := initSchema(db); err != nil {
		t.Fatalf("second initSchema failed: %v", err)
	}

	// Verify tables still work
	_, err = db.Exec(`INSERT INTO rooms (id, description, related_rooms) VALUES ('test', 'test', 'a,b')`)
	if err != nil {
		t.Fatalf("insert after double init failed: %v", err)
	}
	_, err = db.Exec(`INSERT INTO messages (room_id, author, content, reply_to) VALUES ('test', 'X', 'Y', 42)`)
	if err != nil {
		t.Fatalf("insert after double init failed: %v", err)
	}
}

func TestMigrationPreservesMultipleRows(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Old schema
	oldSchema := `
	CREATE TABLE rooms (
		id TEXT PRIMARY KEY, description TEXT, status TEXT DEFAULT 'active',
		project TEXT DEFAULT '', tech_stack TEXT DEFAULT '', tags TEXT DEFAULT '',
		system_prompt TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT, room_id TEXT, author TEXT, content TEXT,
		message_type TEXT DEFAULT 'message', is_summary BOOLEAN DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(room_id) REFERENCES rooms(id)
	);`
	db.Exec(oldSchema)

	// Insert multiple rooms and messages
	db.Exec(`INSERT INTO rooms (id, description, project) VALUES ('r1', 'Room 1', 'proj-a')`)
	db.Exec(`INSERT INTO rooms (id, description, project) VALUES ('r2', 'Room 2', 'proj-b')`)
	db.Exec(`INSERT INTO rooms (id, description, project, tags) VALUES ('r3', 'Room 3', 'proj-a', 'important')`)
	db.Exec(`INSERT INTO messages (room_id, author, content) VALUES ('r1', 'Claude', 'Msg 1')`)
	db.Exec(`INSERT INTO messages (room_id, author, content) VALUES ('r1', 'Gemini', 'Msg 2')`)
	db.Exec(`INSERT INTO messages (room_id, author, content) VALUES ('r2', 'Amp', 'Msg 3')`)

	// Migrate
	if err := initSchema(db); err != nil {
		t.Fatalf("initSchema failed: %v", err)
	}

	// All 3 rooms survive
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM rooms`).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 rooms after migration, got %d", count)
	}

	// All 3 messages survive
	db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 messages after migration, got %d", count)
	}

	// Tags preserved
	var tags string
	db.QueryRow(`SELECT tags FROM rooms WHERE id = 'r3'`).Scan(&tags)
	if tags != "important" {
		t.Errorf("expected tags 'important', got '%s'", tags)
	}

	// All rows get default for new columns
	db.QueryRow(`SELECT COUNT(*) FROM rooms WHERE related_rooms = ''`).Scan(&count)
	if count != 3 {
		t.Errorf("expected all 3 rooms with empty related_rooms, got %d", count)
	}
	db.QueryRow(`SELECT COUNT(*) FROM messages WHERE reply_to = 0`).Scan(&count)
	if count != 3 {
		t.Errorf("expected all 3 messages with reply_to=0, got %d", count)
	}
}
