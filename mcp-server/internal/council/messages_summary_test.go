package council

import (
	"database/sql"
	"testing"
)

// ========== UUID migration ==========

func TestMigrateMessagesToUUIDs(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// 1. Create old integer-ID schema
	db.Exec(`CREATE TABLE rooms (id TEXT PRIMARY KEY, description TEXT, status TEXT DEFAULT 'active',
		project TEXT DEFAULT '', tech_stack TEXT DEFAULT '', tags TEXT DEFAULT '',
		system_prompt TEXT DEFAULT '', related_rooms TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	db.Exec(`CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, room_id TEXT, author TEXT,
		content TEXT, message_type TEXT DEFAULT 'message', is_summary BOOLEAN DEFAULT 0,
		reply_to INTEGER DEFAULT 0, pinned BOOLEAN DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(room_id) REFERENCES rooms(id))`)

	// 2. Insert rooms
	db.Exec(`INSERT INTO rooms (id, description) VALUES ('room-a', 'Test Room A')`)
	db.Exec(`INSERT INTO rooms (id, description) VALUES ('room-b', 'Test Room B')`)

	// 3. Insert messages with integer IDs (including reply_to cross-references)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (1, 'room-a', 'Claude', 'First message', 'message')`)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (2, 'room-a', 'Gemini', 'Second message', 'thought')`)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type, reply_to) VALUES (3, 'room-a', 'Claude', 'Reply to first', 'message', 1)`)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type, pinned) VALUES (4, 'room-b', 'Amp', 'Pinned in room-b', 'decision', 1)`)
	db.Exec(`INSERT INTO messages (id, room_id, author, content, message_type, is_summary) VALUES (5, 'room-b', 'Claude', 'Summary', 'message', 1)`)

	// 4. Run the migration
	if err := migrateMessagesToUUIDs(db); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// 5. Verify count — no data lost
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count)
	if count != 5 {
		t.Errorf("expected 5 messages after migration, got %d", count)
	}

	// 6. Load migrated messages in insertion order
	type migratedMsg struct {
		id, author, replyTo string
		pinned, isSummary   bool
	}
	rows, err := db.Query(`SELECT id, author, reply_to, pinned, is_summary FROM messages ORDER BY rowid ASC`)
	if err != nil {
		t.Fatalf("query after migration: %v", err)
	}
	var msgs []migratedMsg
	for rows.Next() {
		var m migratedMsg
		rows.Scan(&m.id, &m.author, &m.replyTo, &m.pinned, &m.isSummary)
		msgs = append(msgs, m)
	}
	rows.Close()
	if len(msgs) != 5 {
		t.Fatalf("expected 5 migrated messages, got %d", len(msgs))
	}

	// 7. All IDs are valid UUID v7 strings
	for _, m := range msgs {
		if !isValidUUIDv7(m.id) {
			t.Errorf("id %q is not a valid UUID v7", m.id)
		}
	}

	// 8. Insertion order preserved (Claude first, Gemini second, etc.)
	if msgs[0].author != "Claude" {
		t.Errorf("expected Claude first, got %s", msgs[0].author)
	}
	if msgs[1].author != "Gemini" {
		t.Errorf("expected Gemini second, got %s", msgs[1].author)
	}

	// 9. UUID v7 time ordering: later messages have lexicographically greater IDs
	if msgs[1].id <= msgs[0].id {
		t.Errorf("expected uuid[1] > uuid[0], got %s <= %s", msgs[1].id, msgs[0].id)
	}
	if msgs[2].id <= msgs[1].id {
		t.Errorf("expected uuid[2] > uuid[1], got %s <= %s", msgs[2].id, msgs[1].id)
	}

	// 10. reply_to is correctly translated (message 3 replied to message 1)
	replyMsg := msgs[2] // third message (was id=3, reply_to=1)
	if replyMsg.replyTo == "" {
		t.Error("reply_to should have been translated to UUID, got empty string")
	}
	if replyMsg.replyTo != msgs[0].id {
		t.Errorf("reply_to should point to first message UUID %s, got %s", msgs[0].id, replyMsg.replyTo)
	}

	// 11. Pinned flag preserved
	if !msgs[3].pinned {
		t.Error("pinned flag not preserved after migration")
	}

	// 12. is_summary flag preserved
	if !msgs[4].isSummary {
		t.Error("is_summary flag not preserved after migration")
	}

	// 13. Rooms table untouched
	var roomCount int
	db.QueryRow(`SELECT COUNT(*) FROM rooms`).Scan(&roomCount)
	if roomCount != 2 {
		t.Errorf("expected 2 rooms, got %d", roomCount)
	}

	// 14. Idempotency: running migration again is a no-op
	if err := migrateMessagesToUUIDs(db); err != nil {
		t.Errorf("second migration run failed: %v", err)
	}
	db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count)
	if count != 5 {
		t.Errorf("idempotency failed: count changed to %d", count)
	}
}

func isValidUUIDv7(s string) bool {
	if len(s) != 36 {
		return false
	}
	// UUID format: 8-4-4-4-12 hex chars with dashes, version nibble = 7
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	// Version nibble at position 14 must be '7'
	return s[14] == '7'
}

func TestPostMessageAutoClearSynthesis(t *testing.T) {
	s := setupTestServer(t)
	err := s.CreateRoom("synth-room", "", "", "", "foo,needs-synthesis,bar", "", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.PostMessage("synth-room", "Author", "Synthesis text", "synthesis", "")
	if err != nil {
		t.Fatal(err)
	}

	room, err := s.GetRoom("synth-room")
	if err != nil {
		t.Fatal(err)
	}
	if room.Tags != "foo,bar" {
		t.Errorf("Expected tags to be 'foo,bar', got '%s'", room.Tags)
	}
}

// ========== GetUnsummarizedMessages ==========

func TestGetUnsummarizedMessages(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "unsumm-room")
	mustPost(t, s, "unsumm-room", "Claude", "first")
	mustPost(t, s, "unsumm-room", "Claude", "second")

	msgs, err := s.GetUnsummarizedMessages("unsumm-room")
	if err != nil {
		t.Fatalf("GetUnsummarizedMessages error: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 unsummarized messages, got %d", len(msgs))
	}
}

func TestGetUnsummarizedMessagesAfterSummary(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "summ-then-more")
	mustPost(t, s, "summ-then-more", "Claude", "before summary")
	if err := s.InsertSummary("summ-then-more", "Summary of above"); err != nil {
		t.Fatalf("InsertSummary error: %v", err)
	}
	mustPost(t, s, "summ-then-more", "Claude", "after summary")

	msgs, err := s.GetUnsummarizedMessages("summ-then-more")
	if err != nil {
		t.Fatalf("GetUnsummarizedMessages error: %v", err)
	}
	// Only message after summary should be returned
	if len(msgs) != 1 {
		t.Errorf("expected 1 unsummarized message after summary, got %d", len(msgs))
	}
	if msgs[0].Content != "after summary" {
		t.Errorf("expected 'after summary', got %q", msgs[0].Content)
	}
}

func TestGetUnsummarizedMessagesEmpty(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "all-summarized")
	mustPost(t, s, "all-summarized", "Claude", "msg")
	if err := s.InsertSummary("all-summarized", "all caught up"); err != nil {
		t.Fatalf("InsertSummary error: %v", err)
	}

	msgs, err := s.GetUnsummarizedMessages("all-summarized")
	if err != nil {
		t.Fatalf("GetUnsummarizedMessages error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 unsummarized messages after summary covers all, got %d", len(msgs))
	}
}

// ========== GetRoomsNeedingSummary ==========

func TestGetRoomsNeedingSummary(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "needs-summ")
	for i := 0; i < 6; i++ {
		mustPost(t, s, "needs-summ", "Claude", "msg")
	}
	mustCreateRoom(t, s, "below-threshold")
	mustPost(t, s, "below-threshold", "Claude", "just one")

	rooms, err := s.GetRoomsNeedingSummary(5)
	if err != nil {
		t.Fatalf("GetRoomsNeedingSummary error: %v", err)
	}
	found := false
	for _, id := range rooms {
		if id == "needs-summ" {
			found = true
		}
		if id == "below-threshold" {
			t.Errorf("below-threshold should not appear (only 1 msg, threshold=5)")
		}
	}
	if !found {
		t.Errorf("needs-summ should appear (6 msgs > threshold 5), got: %v", rooms)
	}
}

func TestGetRoomsNeedingSummaryAfterSummaryInserted(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "caught-up")
	for i := 0; i < 6; i++ {
		mustPost(t, s, "caught-up", "Claude", "msg")
	}
	if err := s.InsertSummary("caught-up", "all summarized"); err != nil {
		t.Fatalf("InsertSummary error: %v", err)
	}

	rooms, err := s.GetRoomsNeedingSummary(5)
	if err != nil {
		t.Fatalf("GetRoomsNeedingSummary error: %v", err)
	}
	for _, id := range rooms {
		if id == "caught-up" {
			t.Errorf("caught-up should not appear after summary inserted")
		}
	}
}

func TestGetRoomsNeedingSummaryEmpty(t *testing.T) {
	s := setupTestServer(t)
	rooms, err := s.GetRoomsNeedingSummary(5)
	if err != nil {
		t.Fatalf("GetRoomsNeedingSummary error: %v", err)
	}
	if len(rooms) != 0 {
		t.Errorf("expected no rooms needing summary, got: %v", rooms)
	}
}
