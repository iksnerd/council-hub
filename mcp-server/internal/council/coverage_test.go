package council

import (
	"strings"
	"testing"
)

// -- db.go: getPinnedMessage error path --
func TestGetPinnedMessageError(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "pin-err")
	// Corrupt messages table to make the query fail
	s.DB.Exec("DROP TABLE messages")

	_, err := s.GetPinnedMessage("pin-err")
	if err == nil {
		t.Error("expected error for missing messages table")
	}
}

// -- db.go: scan error paths for list functions --
func TestScanErrorsDB(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "scan-err")
	mustPost(t, s, "scan-err", "Claude", "Msg")

	// Corrupt messages schema for scan failures
	s.DB.Exec("ALTER TABLE messages RENAME TO messages_old")
	s.DB.Exec("CREATE TABLE messages (id INTEGER PRIMARY KEY, room_id TEXT)") // missing columns

	if _, err := s.GetMessagesAfterID("scan-err", 0); err == nil {
		t.Error("expected scan error in GetMessagesAfterID")
	}

	if _, err := s.GetLatestPerType("scan-err"); err == nil {
		t.Error("expected scan error in GetLatestPerType")
	}

	// Trigger scan error in GetRoomStats author query
	s.DB.Exec("ALTER TABLE messages RENAME TO messages_old2")
	s.DB.Exec("CREATE TABLE messages (id INTEGER PRIMARY KEY, room_id TEXT, author INTEGER)") // incompatible author type
	s.DB.Exec("INSERT INTO messages (room_id, author) VALUES ('scan-err', 'not-an-int')")
	if _, err := s.GetRoomStats("scan-err"); err == nil {
		t.Error("expected scan error in GetRoomStats")
	}
}

// -- db.go: getRoomStats scan error in type query --
func TestGetRoomStatsTypeScanError(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "type-scan-err")
	mustPost(t, s, "type-scan-err", "Claude", "Msg")

	// Corrupt messages for type query scan error
	s.DB.Exec("ALTER TABLE messages RENAME TO messages_old")
	s.DB.Exec("CREATE TABLE messages (id INTEGER PRIMARY KEY, room_id TEXT, author TEXT, message_type INTEGER, is_summary BOOLEAN)")
	s.DB.Exec("INSERT INTO messages (room_id, author, message_type, is_summary) VALUES ('type-scan-err', 'Claude', 'not-an-int', 0)")

	if _, err := s.GetRoomStats("type-scan-err"); err == nil {
		t.Error("expected scan error in GetRoomStats type query")
	}
}

// -- db.go: getMessageCounts scan error path --
func TestGetMessageCountsDBClose(t *testing.T) {
	s := setupTestServer(t)
	s.DB.Close()
	counts := s.GetMessageCounts()
	if len(counts) != 0 {
		t.Error("expected empty counts for closed DB")
	}
}

// -- janitor.go: more error paths --
func TestJanitorSweepErrorPaths(t *testing.T) {
	s := setupTestServer(t)
	s.DB.Close()
	s.JanitorSweep() // Should log error and return
}

func TestJanitorSweepUnsummarizedError(t *testing.T) {
	s := setupTestServer(t)
	setupRoomWithMessages(t, s, "j-unsum-err", 25)

	// Corrupt messages table so getUnsummarizedMessages fails but getRoomsNeedingSummary works
	s.DB.Exec("ALTER TABLE messages RENAME TO messages_old")
	s.DB.Exec("CREATE TABLE messages (room_id TEXT, is_summary BOOLEAN, author TEXT)")
	for i := 0; i < 21; i++ {
		s.DB.Exec("INSERT INTO messages (room_id, is_summary) VALUES ('j-unsum-err', 0)")
	}

	s.JanitorSweep() // Should hit "failed to get unsummarized messages"
}
