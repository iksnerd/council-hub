package council

import (
	"os"
	"strings"
	"testing"
)

func TestRoomStats(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("stats-room", "Stats test", "", "", "", "", "")
	s.PostMessage("stats-room", "Claude", "Message 1", "thought", "")
	s.PostMessage("stats-room", "Claude", "Message 2", "decision", "")
	s.PostMessage("stats-room", "Gemini", "Message 3", "review", "")

	stats, err := s.GetRoomStats("stats-room")
	if err != nil {
		t.Fatalf("getRoomStats failed: %v", err)
	}

	if stats.MessageCount != 3 {
		t.Errorf("expected 3 messages, got %d", stats.MessageCount)
	}
	if stats.Participants["Claude"] != 2 {
		t.Errorf("expected 2 messages from Claude, got %d", stats.Participants["Claude"])
	}
	if stats.Participants["Gemini"] != 1 {
		t.Errorf("expected 1 message from Gemini, got %d", stats.Participants["Gemini"])
	}
	if stats.Status != "active" {
		t.Errorf("expected status 'active', got '%s'", stats.Status)
	}
}

func TestRoomStatsEmpty(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("empty-stats", "Empty room", "", "", "", "", "")

	stats, err := s.GetRoomStats("empty-stats")
	if err != nil {
		t.Fatalf("getRoomStats failed: %v", err)
	}
	if stats.MessageCount != 0 {
		t.Errorf("expected 0 messages, got %d", stats.MessageCount)
	}
}

func TestRoomStatsNotFound(t *testing.T) {
	s := setupTestServer(t)

	_, err := s.GetRoomStats("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestArchiveRoom(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("archive-room", "Archive test", "proj", "Go", "test", "Be helpful", "")
	s.PostMessage("archive-room", "Claude", "Test message", "message", "")

	archivePath, err := s.ArchiveRoom("archive-room")
	if err != nil {
		t.Fatalf("archiveRoom failed: %v", err)
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read archive: %v", err)
	}
	if !strings.Contains(string(data), "COUNCIL ROOM: archive-room") {
		t.Error("archive missing room header")
	}
	if !strings.Contains(string(data), "Test message") {
		t.Error("archive missing message content")
	}

	os.RemoveAll("archives")
}

func TestArchiveRoomNotFound(t *testing.T) {
	s := setupTestServer(t)

	_, err := s.ArchiveRoom("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestArchivePathTraversalRejected(t *testing.T) {
	s := setupTestServer(t)

	// Room IDs come from untrusted MCP input and are used as archive filenames.
	// Traversal attempts must be rejected, not resolved to a path outside archiveDir.
	bad := []string{
		"../../etc/passwd",
		"..",
		"foo/bar",
		`foo\bar`,
		"a/../../b",
	}
	for _, id := range bad {
		if _, err := s.ReadArchive(id); err == nil {
			t.Errorf("ReadArchive(%q) should be rejected, got nil error", id)
		}
		// ArchiveRoom validates the path before any room lookup matters; a traversal
		// ID must never produce a write path outside the archive directory.
		s.CreateRoom(id, "t", "p", "Go", "t", "", "")
		if _, err := s.ArchiveRoom(id); err == nil {
			t.Errorf("ArchiveRoom(%q) should be rejected, got nil error", id)
		}
	}
}

func TestRoomLifecycle(t *testing.T) {
	s := setupTestServer(t)

	// Create two linked rooms
	s.CreateRoom("design", "System design", "council-hub", "Go, SQLite", "architecture", "Focus on modularity.", "")
	s.CreateRoom("impl", "Implementation", "council-hub", "Go", "code", "", "design")

	// Bidirectional link established automatically
	design, _ := s.GetRoom("design")
	if design.RelatedRooms != "impl" {
		t.Errorf("expected reverse link 'impl', got '%s'", design.RelatedRooms)
	}

	// Post various message types
	id1, _ := s.PostMessage("design", "Claude", "Proposal: split into internal packages", "thought", "")
	id2, _ := s.PostMessage("design", "Gemini", "Agreed — use internal/council and internal/handlers", "decision", "")
	s.PostMessage("design", "Claude", "type Server struct { DB *sql.DB }", "code", id2)
	s.PostMessage("impl", "Claude", "Starting refactor", "action", "")

	// Pin, update, check transcript reflects both
	s.PinMessage("design", id1)
	s.UpdateMessage(id1, "Proposal: split into internal packages (revised)", "")
	s.UpdateStatus("design", "resolved")

	msgs, _ := s.GetTranscript("design")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	room, _ := s.GetRoom("design")
	transcript := FormatTranscript(room, msgs)
	if !strings.Contains(transcript, "PINNED") {
		t.Error("expected pinned marker in transcript")
	}
	if !strings.Contains(transcript, "resolved") {
		t.Error("expected resolved status")
	}

	// Delta reads
	after, _ := s.GetMessagesAfterID("design", id2)
	if len(after) != 1 {
		t.Errorf("expected 1 message after id %s, got %d", id2, len(after))
	}

	// Summary mode
	latest, _ := s.GetLatestPerType("design")
	types := map[string]bool{}
	for _, m := range latest {
		types[m.MessageType] = true
	}
	if !types["thought"] || !types["decision"] || !types["code"] {
		t.Errorf("expected thought/decision/code in latest per type, got %v", types)
	}

	// Stats
	stats, _ := s.GetRoomStats("design")
	if stats.MessageCount != 3 || stats.Status != "resolved" {
		t.Errorf("unexpected stats: count=%d status=%s", stats.MessageCount, stats.Status)
	}

	// Search across rooms
	results, _ := s.SearchMessages("refactor", "", "", "", "", "", "", 20)
	if len(results) == 0 {
		t.Error("expected search results for 'refactor'")
	}

	// Message counts
	counts := s.GetMessageCounts()
	if counts["design"] != 3 || counts["impl"] != 1 {
		t.Errorf("unexpected message counts: %v", counts)
	}

	// Archive then delete
	archivePath, err := s.ArchiveRoom("design")
	if err != nil || !strings.Contains(archivePath, "design") {
		t.Errorf("archive failed: path=%s err=%v", archivePath, err)
	}
	s.DeleteRoom("design")
	if _, err := s.GetRoom("design"); err == nil {
		t.Error("expected error for deleted room")
	}
}

func TestSignalStatus(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("status-room", "Status test", "", "", "", "", "")

	if err := s.UpdateStatus("status-room", "paused"); err != nil {
		t.Fatalf("updateStatus failed: %v", err)
	}

	room, _ := s.GetRoom("status-room")
	if room.Status != "paused" {
		t.Errorf("expected status 'paused', got '%s'", room.Status)
	}
}

func TestUpdateStatusNonexistentRoom(t *testing.T) {
	s := setupTestServer(t)

	err := s.UpdateStatus("nonexistent", "active")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}
