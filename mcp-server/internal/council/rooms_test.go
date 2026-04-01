package council

import (
	"os"
	"strings"
	"testing"
)

func TestCreateRoom(t *testing.T) {
	s := setupTestServer(t)

	if err := s.CreateRoom("test-room", "A test room", "", "", "", "", ""); err != nil {
		t.Fatalf("createRoom failed: %v", err)
	}

	room, err := s.GetRoom("test-room")
	if err != nil {
		t.Fatalf("getRoom failed: %v", err)
	}
	if room.ID != "test-room" {
		t.Errorf("expected room ID 'test-room', got '%s'", room.ID)
	}
	if room.Description != "A test room" {
		t.Errorf("expected description 'A test room', got '%s'", room.Description)
	}
	if room.Status != "active" {
		t.Errorf("expected status 'active', got '%s'", room.Status)
	}
}

func TestCreateRoomDuplicate(t *testing.T) {
	s := setupTestServer(t)

	if err := s.CreateRoom("dup-room", "First", "", "", "", "", ""); err != nil {
		t.Fatalf("first createRoom failed: %v", err)
	}
	if err := s.CreateRoom("dup-room", "Second", "", "", "", "", ""); err != nil {
		t.Fatalf("duplicate createRoom failed: %v", err)
	}

	room, _ := s.GetRoom("dup-room")
	if room.Description != "First" {
		t.Errorf("expected original description 'First', got '%s'", room.Description)
	}
}

func TestCreateRoomWithMetadata(t *testing.T) {
	s := setupTestServer(t)

	err := s.CreateRoom("auth-api", "JWT refactoring", "llm-memory", "Go, SQLite, MCP SDK", "auth,security", "You are reviewing for security issues.", "")
	if err != nil {
		t.Fatalf("createRoom failed: %v", err)
	}

	room, err := s.GetRoom("auth-api")
	if err != nil {
		t.Fatalf("getRoom failed: %v", err)
	}
	if room.Project != "llm-memory" {
		t.Errorf("expected project 'llm-memory', got '%s'", room.Project)
	}
	if room.TechStack != "Go, SQLite, MCP SDK" {
		t.Errorf("expected tech_stack 'Go, SQLite, MCP SDK', got '%s'", room.TechStack)
	}
	if room.Tags != "auth,security" {
		t.Errorf("expected tags 'auth,security', got '%s'", room.Tags)
	}
	if room.SystemPrompt != "You are reviewing for security issues." {
		t.Errorf("expected system_prompt, got '%s'", room.SystemPrompt)
	}
}

func TestCreateRoomWithRelatedRooms(t *testing.T) {
	s := setupTestServer(t)
	err := s.CreateRoom("bep44-room", "BEP 44 analysis", "weightless", "Go", "dht,bep44", "", "bep46-room,provenance-room")
	if err != nil {
		t.Fatalf("createRoom failed: %v", err)
	}

	room, err := s.GetRoom("bep44-room")
	if err != nil {
		t.Fatalf("getRoom failed: %v", err)
	}
	if room.RelatedRooms != "bep46-room,provenance-room" {
		t.Errorf("expected related_rooms 'bep46-room,provenance-room', got '%s'", room.RelatedRooms)
	}
}

func TestBidirectionalRelatedRoomsOnCreate(t *testing.T) {
	s := setupTestServer(t)
	// Create target room first
	s.CreateRoom("target-room", "Target", "", "", "", "", "")
	// Create source room linking to target
	s.CreateRoom("source-room", "Source", "", "", "", "", "target-room")

	// Verify source has target
	src, _ := s.GetRoom("source-room")
	if src.RelatedRooms != "target-room" {
		t.Errorf("expected source related_rooms 'target-room', got '%s'", src.RelatedRooms)
	}

	// Verify target now has reverse link to source
	tgt, _ := s.GetRoom("target-room")
	if tgt.RelatedRooms != "source-room" {
		t.Errorf("expected target related_rooms 'source-room', got '%s'", tgt.RelatedRooms)
	}
}

func TestBidirectionalRelatedRoomsOnUpdate(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("upd-src", "Source", "", "", "", "", "")
	s.CreateRoom("upd-tgt", "Target", "", "", "", "", "")

	// Update source to link to target
	s.UpdateRoom("upd-src", "", "", "", "", "", "upd-tgt")

	// Verify reverse link
	tgt, _ := s.GetRoom("upd-tgt")
	if tgt.RelatedRooms != "upd-src" {
		t.Errorf("expected reverse link 'upd-src', got '%s'", tgt.RelatedRooms)
	}
}

func TestBidirectionalNoDuplicateLinks(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("dup-a", "Room A", "", "", "", "", "")
	s.CreateRoom("dup-b", "Room B", "", "", "", "", "dup-a")

	// Update again with same link — should not duplicate
	s.UpdateRoom("dup-b", "", "", "", "", "", "dup-a")

	a, _ := s.GetRoom("dup-a")
	if a.RelatedRooms != "dup-b" {
		t.Errorf("expected 'dup-b', got '%s'", a.RelatedRooms)
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

func TestUpdateRoom(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("update-room", "Original topic", "old-project", "Go", "old-tag", "Old prompt", "")

	// Update only project and tags
	if err := s.UpdateRoom("update-room", "", "new-project", "", "new-tag", "", ""); err != nil {
		t.Fatalf("updateRoom failed: %v", err)
	}

	room, _ := s.GetRoom("update-room")
	if room.Project != "new-project" {
		t.Errorf("expected project 'new-project', got '%s'", room.Project)
	}
	if room.Tags != "new-tag" {
		t.Errorf("expected tags 'new-tag', got '%s'", room.Tags)
	}
	// Unchanged fields should remain
	if room.Description != "Original topic" {
		t.Errorf("expected description 'Original topic', got '%s'", room.Description)
	}
	if room.TechStack != "Go" {
		t.Errorf("expected tech_stack 'Go', got '%s'", room.TechStack)
	}
	if room.SystemPrompt != "Old prompt" {
		t.Errorf("expected system_prompt 'Old prompt', got '%s'", room.SystemPrompt)
	}
}

func TestUpdateRoomRelatedRooms(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("link-room", "Link test", "", "", "", "", "")

	if err := s.UpdateRoom("link-room", "", "", "", "", "", "room-a,room-b"); err != nil {
		t.Fatalf("updateRoom failed: %v", err)
	}

	room, _ := s.GetRoom("link-room")
	if room.RelatedRooms != "room-a,room-b" {
		t.Errorf("expected related_rooms 'room-a,room-b', got '%s'", room.RelatedRooms)
	}
}

func TestUpdateRoomNotFound(t *testing.T) {
	s := setupTestServer(t)

	err := s.UpdateRoom("nonexistent", "topic", "", "", "", "", "")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestDeleteRoom(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("del-room", "To be deleted", "", "", "", "", "")
	s.PostMessage("del-room", "Claude", "Message 1", "message", "")
	s.PostMessage("del-room", "Gemini", "Message 2", "message", "")

	if err := s.DeleteRoom("del-room"); err != nil {
		t.Fatalf("deleteRoom failed: %v", err)
	}

	// Room should be gone
	_, err := s.GetRoom("del-room")
	if err == nil {
		t.Error("expected error getting deleted room")
	}

	// Messages should be gone
	msgs, _ := s.GetTranscript("del-room")
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after delete, got %d", len(msgs))
	}
}

func TestDeleteRoomNotFound(t *testing.T) {
	s := setupTestServer(t)

	err := s.DeleteRoom("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestListRooms(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-a", "Auth work", "project-alpha", "Go", "auth,api", "", "")
	s.CreateRoom("room-b", "Frontend", "project-beta", "React, TypeScript", "frontend", "", "")
	s.CreateRoom("room-c", "More auth", "project-alpha", "Go", "auth", "", "")

	// Filter by project
	rooms, err := s.ListRooms("project-alpha", "", "", "")
	if err != nil {
		t.Fatalf("listRooms failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms for project-alpha, got %d", len(rooms))
	}

	// Filter by tag
	rooms, _ = s.ListRooms("", "auth", "", "")
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms with tag 'auth', got %d", len(rooms))
	}

	// Filter by tag that only one room has
	rooms, _ = s.ListRooms("", "frontend", "", "")
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room with tag 'frontend', got %d", len(rooms))
	}

	// No filter — all rooms
	rooms, _ = s.ListRooms("", "", "", "")
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms total, got %d", len(rooms))
	}

	// Filter by project + tag
	rooms, _ = s.ListRooms("project-alpha", "api", "", "")
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room for project-alpha+api, got %d", len(rooms))
	}
}

func TestListRoomsByStatus(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("active-room", "Active", "", "", "", "", "")
	s.CreateRoom("paused-room", "Paused", "", "", "", "", "")
	s.UpdateStatus("paused-room", "paused")

	rooms, _ := s.ListRooms("", "", "paused", "")
	if len(rooms) != 1 {
		t.Fatalf("expected 1 paused room, got %d", len(rooms))
	}
	if rooms[0].ID != "paused-room" {
		t.Errorf("expected 'paused-room', got '%s'", rooms[0].ID)
	}
}

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

func TestNewColumnsUsable(t *testing.T) {
	s := setupTestServer(t)

	s.CreateRoom("migrate-room", "Migration test", "", "", "", "", "related-a")
	room, _ := s.GetRoom("migrate-room")
	if room.RelatedRooms != "related-a" {
		t.Errorf("expected related_rooms 'related-a', got '%s'", room.RelatedRooms)
	}

	id, _ := s.PostMessage("migrate-room", "Test", "msg", "message", "some-parent-id")
	msgs, _ := s.GetMessagesByIDs([]string{id})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].ReplyTo != "some-parent-id" {
		t.Errorf("expected reply_to 'some-parent-id', got '%s'", msgs[0].ReplyTo)
	}
}

// -- updateRoom: all fields set (covers every branch) --

func TestUpdateRoomAllFields(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "upd-all", withDescription("Topic"), withProject("Proj"), withTechStack("Tech"), withTags("Tags"), withSystemPrompt("Prompt"), withRelatedRooms("Related"))

	err := s.UpdateRoom("upd-all", "New Topic", "New Proj", "New Tech", "New Tags", "New Prompt", "New Related")
	if err != nil {
		t.Fatalf("updateRoom failed: %v", err)
	}

	room, _ := s.GetRoom("upd-all")
	if room.Description != "New Topic" {
		t.Errorf("expected 'New Topic', got '%s'", room.Description)
	}
	if room.RelatedRooms != "New Related" {
		t.Errorf("expected 'New Related', got '%s'", room.RelatedRooms)
	}
}

// -- listRooms: status filter --

func TestListRoomsByStatusFilter(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "ls-active")
	mustCreateRoom(t, s, "ls-resolved")
	s.UpdateStatus("ls-resolved", "resolved")

	rooms, _ := s.ListRooms("", "", "active", "")
	if len(rooms) != 1 || rooms[0].ID != "ls-active" {
		t.Errorf("expected only active room, got %d rooms", len(rooms))
	}

	rooms, _ = s.ListRooms("", "", "resolved", "")
	if len(rooms) != 1 || rooms[0].ID != "ls-resolved" {
		t.Errorf("expected only resolved room, got %d rooms", len(rooms))
	}
}

// -- NewServer with file DB to exercise non-memory DSN path --

func TestNewServerFileDSN(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.DB"
	s, err := NewServer(dbPath, testLogger())
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer s.DB.Close()

	// Verify it works
	mustCreateRoom(t, s, "file-room")
}

// -- archiveRoom with :memory: DB (different archive dir logic) --

func TestArchiveRoomMemoryDB(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "mem-archive")
	mustPost(t, s, "mem-archive", "Claude", "test")

	path, err := s.ArchiveRoom("mem-archive")
	if err != nil {
		t.Fatalf("archiveRoom failed: %v", err)
	}
	if !strings.Contains(path, "archives") {
		t.Errorf("expected archives in path, got: %s", path)
	}
}

// -- deleteRoom: error wrapping includes room ID context --

func TestDeleteRoomErrorWrapping(t *testing.T) {
	s := setupTestServer(t)

	err := s.DeleteRoom("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should contain room ID, got: %s", err)
	}
}

// -- deleteRoom: message cleanup error path (drop messages table to trigger) --

func TestDeleteRoomMessageCleanupError(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "del-msg-err")
	mustPost(t, s, "del-msg-err", "Claude", "msg")

	// Drop messages table so DELETE FROM messages fails
	s.DB.Exec("DROP TABLE messages")

	err := s.DeleteRoom("del-msg-err")
	if err == nil {
		t.Fatal("expected error when messages table is missing")
	}
	if !strings.Contains(err.Error(), "delete messages") {
		t.Errorf("expected 'delete messages' in error, got: %s", err)
	}
}

// -- deleteRoom: room DELETE itself fails (closed DB) --

func TestDeleteRoomExecError(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "del-exec-err")
	s.DB.Close()

	err := s.DeleteRoom("del-exec-err")
	if err == nil {
		t.Fatal("expected error on closed DB")
	}
	if !strings.Contains(err.Error(), "delete room") {
		t.Errorf("expected 'delete room' in error, got: %s", err)
	}
}

// -- archiveRoom: getTranscript fails --

func TestArchiveRoomTranscriptError(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "arch-transcript-err")
	mustPost(t, s, "arch-transcript-err", "Claude", "msg")
	// Corrupt messages table so getTranscript fails
	s.DB.Exec("ALTER TABLE messages RENAME TO messages_old")
	s.DB.Exec("CREATE TABLE messages AS SELECT id, room_id FROM messages_old")

	_, err := s.ArchiveRoom("arch-transcript-err")
	if err == nil {
		t.Error("expected transcript error during archive")
	}
}

// ========== ListRooms keyword search ==========

func TestListRoomsSearch(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "jwt-auth", withDescription("JWT authentication refactor"), withTags("security"))
	mustCreateRoom(t, s, "db-migration", withDescription("Database migration"))
	mustCreateRoom(t, s, "jwt-tokens", withDescription("Token validation"), withTags("jwt"))

	// Match by description keyword
	rooms, err := s.ListRooms("", "", "", "JWT")
	if err != nil {
		t.Fatalf("ListRooms search failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms matching 'JWT', got %d", len(rooms))
	}

	// Match by room ID
	rooms, _ = s.ListRooms("", "", "", "db-migration")
	if len(rooms) != 1 {
		t.Errorf("expected 1 room matching ID 'db-migration', got %d", len(rooms))
	}

	// Match by tag content
	rooms, _ = s.ListRooms("", "", "", "security")
	if len(rooms) != 1 {
		t.Errorf("expected 1 room matching tag 'security', got %d", len(rooms))
	}
}

// ========== Full lifecycle workflow ==========

// TestRoomLifecycle exercises the complete room lifecycle to catch regressions
// after structural changes like the internal/ package refactor.
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
	results, _ := s.SearchMessages("refactor", "", "", "", "", 20)
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
