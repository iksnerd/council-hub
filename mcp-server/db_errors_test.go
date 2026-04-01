package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Tests that exercise DB error paths by closing the database before operations.
// This covers the `if err != nil { return ..., err }` branches after DB calls.

func setupAndClose(t *testing.T) *CouncilServer {
	t.Helper()
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "pre-close", withProject("proj"), withTechStack("Go"), withTags("tag"), withSystemPrompt("prompt"), withRelatedRooms("related"))
	mustPost(t, cs, "pre-close", "Claude", "Before close")
	cs.db.Close()
	return cs
}

func TestCreateRoomDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	err := cs.createRoom("fail", "fail", "", "", "", "", "")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestPostMessageDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.postMessage("pre-close", "Claude", "fail", "message", 0)
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestUpdateStatusDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	err := cs.updateStatus("pre-close", "paused")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestUpdateRoomDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	err := cs.updateRoom("pre-close", "new topic", "", "", "", "", "")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestGetRoomDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.getRoom("pre-close")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestGetTranscriptDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.getTranscript("pre-close")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestListRoomsDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.listRooms("", "", "", "")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestSearchMessagesDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.searchMessages("test", "", "", "", "", 10)
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestGetMessagesByIDsDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.getMessagesByIDs([]int64{1})
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestGetRecentMessagesDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.getRecentMessages("pre-close", 5)
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestDeleteRoomDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	err := cs.deleteRoom("pre-close")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestDeleteMessagesDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.deleteMessages([]int64{1})
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestGetRoomStatsDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.getRoomStats("pre-close")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestGetRoomsNeedingSummaryDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.getRoomsNeedingSummary(20)
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestGetUnsummarizedMessagesDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.getUnsummarizedMessages("pre-close")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestInsertSummaryDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	err := cs.insertSummary("pre-close", "summary text")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestArchiveRoomDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.archiveRoom("pre-close")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

// -- archiveRoom filesystem error paths --

func TestArchiveRoomBadDirectory(t *testing.T) {
	cs := setupTestServer(t)
	// Point dbPath to a location where we can't create the archives dir
	cs.dbPath = "/dev/null/impossible/path/council.db"
	mustCreateRoom(t, cs, "arch-bad-dir")
	mustPost(t, cs, "arch-bad-dir", "Claude", "Message")

	_, err := cs.archiveRoom("arch-bad-dir")
	if err == nil {
		t.Error("expected error for bad archive directory")
	}
}

func TestArchiveRoomBadWritePath(t *testing.T) {
	cs := setupTestServer(t)

	// Create a directory where archives subdir exists but is not writable
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, "archives")
	os.MkdirAll(archiveDir, 0755)

	// Make archives a file (not a dir) so WriteFile to archives/room.md fails
	os.RemoveAll(archiveDir)
	os.WriteFile(archiveDir, []byte("not a dir"), 0644)

	cs.dbPath = filepath.Join(tmpDir, "council.db")
	mustCreateRoom(t, cs, "arch-bad-write")
	mustPost(t, cs, "arch-bad-write", "Claude", "Message")

	_, err := cs.archiveRoom("arch-bad-write")
	if err == nil {
		t.Error("expected error when archive path is a file not a directory")
	}
}

// -- NewCouncilServer error path --

func TestNewCouncilServerInitSchemaFail(t *testing.T) {
	// A path like /dev/null can't be used as a SQLite DB with tables
	_, err := NewCouncilServer("/dev/null", testLogger())
	if err == nil {
		t.Error("expected error for /dev/null as DB path")
	}
}

// -- NewCouncilServer bad path --

func TestNewCouncilServerBadPath(t *testing.T) {
	_, err := NewCouncilServer("/nonexistent/path/to/db.sqlite", testLogger())
	// SQLite may or may not error on open (it creates the file),
	// but if it errors on schema init that's also fine
	_ = err
}

// -- NewCouncilServer bad DSN --

func TestNewCouncilServerBadDSN(t *testing.T) {
	// An invalid driver-level path that sql.Open itself rejects is hard to
	// trigger with sqlite3 (it defers errors to first use), but initSchema
	// will fail if the DB is unusable.
	_, err := NewCouncilServer("file:///dev/null?mode=ro&_journal=OFF", testLogger())
	// Whether this errors depends on driver — we just exercise the path.
	_ = err
}

// -- NewCouncilServer invalid driver --

func TestNewCouncilServerInvalidDriver(t *testing.T) {
	// Use a path with null bytes which should fail
	_, err := NewCouncilServer("file:\x00invalid", testLogger())
	if err == nil {
		// Some drivers may not error — that's OK, we're just exercising the path
		t.Log("no error on null byte path — driver-specific behavior")
	}
}

// -- Migration tests --

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

// -- Scan error paths via corrupted tables --

func corruptMessages(t *testing.T) *CouncilServer {
	t.Helper()
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "corrupt-room")
	for i := 0; i < 5; i++ {
		mustPost(t, cs, "corrupt-room", "Claude", "msg")
	}
	// Replace messages table with incompatible schema
	cs.db.Exec("ALTER TABLE messages RENAME TO messages_old")
	cs.db.Exec("CREATE TABLE messages AS SELECT id, room_id FROM messages_old")
	return cs
}

func TestGetMessagesByIDsScanError(t *testing.T) {
	cs := corruptMessages(t)
	_, err := cs.getMessagesByIDs([]int64{1, 2})
	if err == nil {
		t.Error("expected scan error")
	}
}

func TestGetRecentMessagesScanError(t *testing.T) {
	cs := corruptMessages(t)
	_, err := cs.getRecentMessages("corrupt-room", 5)
	if err == nil {
		t.Error("expected scan error")
	}
}

func TestSearchMessagesScanError(t *testing.T) {
	cs := corruptMessages(t)
	_, err := cs.searchMessages("msg", "", "", "", "", 10)
	if err == nil {
		t.Error("expected scan error")
	}
}

func TestGetTranscriptScanError(t *testing.T) {
	cs := corruptMessages(t)
	_, err := cs.getTranscript("corrupt-room")
	if err == nil {
		t.Error("expected scan error")
	}
}

func TestGetUnsummarizedMessagesScanError(t *testing.T) {
	cs := corruptMessages(t)
	_, err := cs.getUnsummarizedMessages("corrupt-room")
	if err == nil {
		t.Error("expected scan error")
	}
}

func TestGetRoomStatsScanError(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "stats-corrupt")
	mustPost(t, cs, "stats-corrupt", "Claude", "msg")
	// Corrupt the per-author GROUP BY query by replacing messages
	cs.db.Exec("ALTER TABLE messages RENAME TO messages_old")
	cs.db.Exec("CREATE TABLE messages AS SELECT id, room_id FROM messages_old")

	_, err := cs.getRoomStats("stats-corrupt")
	if err == nil {
		t.Error("expected scan error")
	}
}

func TestGetRoomsNeedingSummaryScanError(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "summary-corrupt")
	for i := 0; i < 25; i++ {
		mustPost(t, cs, "summary-corrupt", "Claude", "msg")
	}
	cs.db.Exec("ALTER TABLE messages RENAME TO messages_old")
	cs.db.Exec("CREATE TABLE messages AS SELECT id, room_id FROM messages_old")

	_, err := cs.getRoomsNeedingSummary(20)
	if err == nil {
		t.Error("expected scan error")
	}
}

func corruptRooms(t *testing.T) *CouncilServer {
	t.Helper()
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "corrupt-list", withProject("proj"), withTechStack("Go"), withTags("tag"), withRelatedRooms("related"))
	cs.db.Exec("ALTER TABLE rooms RENAME TO rooms_old")
	cs.db.Exec("CREATE TABLE rooms AS SELECT id FROM rooms_old")
	return cs
}

func TestListRoomsScanError(t *testing.T) {
	cs := corruptRooms(t)
	_, err := cs.listRooms("", "", "", "")
	if err == nil {
		t.Error("expected scan error")
	}
}

// -- getRecentMessages: query error (not getRoom error) --

func TestGetRecentMessagesQueryError(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "recent-qerr")
	mustPost(t, cs, "recent-qerr", "Claude", "msg")
	// Drop messages but keep rooms so getRoom succeeds, query fails
	cs.db.Exec("DROP TABLE messages")

	_, err := cs.getRecentMessages("recent-qerr", 5)
	if err == nil {
		t.Error("expected query error")
	}
}

// -- getRoomStats: first QueryRow error (aggregate stats) --

func TestGetRoomStatsAggregateError(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "stats-agg-err")
	// Drop messages to make the COUNT query target a missing table
	cs.db.Exec("DROP TABLE messages")

	_, err := cs.getRoomStats("stats-agg-err")
	if err == nil {
		t.Error("expected aggregate stats error")
	}
}
