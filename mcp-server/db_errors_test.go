package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Tests that exercise DB error paths by closing the database before operations.
// This covers the `if err != nil { return ..., err }` branches after DB calls.

func setupAndClose(t *testing.T) *CouncilServer {
	t.Helper()
	cs := setupTestServer(t)
	cs.createRoom("pre-close", "Room before close", "proj", "Go", "tag", "prompt", "related")
	cs.postMessage("pre-close", "Claude", "Before close", "message", 0)
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
	_, err := cs.listRooms("", "", "")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestSearchMessagesDBClosed(t *testing.T) {
	cs := setupAndClose(t)
	_, err := cs.searchMessages("test", "", "", "", 10)
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
	cs.createRoom("arch-bad-dir", "Archive test", "", "", "", "", "")
	cs.postMessage("arch-bad-dir", "Claude", "Message", "message", 0)

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
	cs.createRoom("arch-bad-write", "Archive test", "", "", "", "", "")
	cs.postMessage("arch-bad-write", "Claude", "Message", "message", 0)

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
