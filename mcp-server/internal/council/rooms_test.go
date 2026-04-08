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
	s.UpdateRoom("upd-src", "", "", "", "", "", "", "", "upd-tgt")

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
	s.UpdateRoom("dup-b", "", "", "", "", "", "", "", "dup-a")

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
	if err := s.UpdateRoom("update-room", "", "new-project", "", "new-tag", "", "", "", ""); err != nil {
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

	if err := s.UpdateRoom("link-room", "", "", "", "", "", "", "", "room-a,room-b"); err != nil {
		t.Fatalf("updateRoom failed: %v", err)
	}

	room, _ := s.GetRoom("link-room")
	if room.RelatedRooms != "room-a,room-b" {
		t.Errorf("expected related_rooms 'room-a,room-b', got '%s'", room.RelatedRooms)
	}
}

func TestUpdateRoomNotFound(t *testing.T) {
	s := setupTestServer(t)

	err := s.UpdateRoom("nonexistent", "topic", "", "", "", "", "", "", "")
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
	rooms, err := s.ListRooms("project-alpha", "", "", "", 100, 0)
	if err != nil {
		t.Fatalf("listRooms failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms for project-alpha, got %d", len(rooms))
	}

	// Filter by tag
	rooms, _ = s.ListRooms("", "auth", "", "", 100, 0)
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms with tag 'auth', got %d", len(rooms))
	}

	// Filter by tag that only one room has
	rooms, _ = s.ListRooms("", "frontend", "", "", 100, 0)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room with tag 'frontend', got %d", len(rooms))
	}

	// No filter — all rooms
	rooms, _ = s.ListRooms("", "", "", "", 100, 0)
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms total, got %d", len(rooms))
	}

	// Filter by project + tag
	rooms, _ = s.ListRooms("project-alpha", "api", "", "", 100, 0)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room for project-alpha+api, got %d", len(rooms))
	}
}

func TestListRoomsByStatus(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("active-room", "Active", "", "", "", "", "")
	s.CreateRoom("paused-room", "Paused", "", "", "", "", "")
	s.UpdateStatus("paused-room", "paused")

	rooms, _ := s.ListRooms("", "", "paused", "", 100, 0)
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

	err := s.UpdateRoom("upd-all", "New Topic", "New Proj", "New Tech", "New Tags", "", "", "New Prompt", "New Related")
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

	rooms, _ := s.ListRooms("", "", "active", "", 100, 0)
	if len(rooms) != 1 || rooms[0].ID != "ls-active" {
		t.Errorf("expected only active room, got %d rooms", len(rooms))
	}

	rooms, _ = s.ListRooms("", "", "resolved", "", 100, 0)
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

// ========== ListRooms multi-word search ==========

func TestListRoomsMultiWordSearch(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "council-hub-multi", withDescription("Multi agent collaboration platform"))

	// Both words match room ID ("council" and "hub" in "council-hub-multi")
	rooms, err := s.ListRooms("", "", "", "council hub", 100, 0)
	if err != nil {
		t.Fatalf("ListRooms multi-word search failed: %v", err)
	}
	if len(rooms) != 1 {
		t.Errorf("expected 1 room matching 'council hub', got %d", len(rooms))
	}

	// Both words match description ("agent" and "platform")
	rooms, _ = s.ListRooms("", "", "", "agent platform", 100, 0)
	if len(rooms) != 1 {
		t.Errorf("expected 1 room matching 'agent platform', got %d", len(rooms))
	}

	// AND logic: both words must match somewhere, "nonexistent" matches nothing
	rooms, _ = s.ListRooms("", "", "", "nonexistent xyz", 100, 0)
	if len(rooms) != 0 {
		t.Errorf("expected 0 rooms matching 'nonexistent xyz', got %d", len(rooms))
	}
}

// ========== ListRooms keyword search ==========

func TestListRoomsSearch(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "jwt-auth", withDescription("JWT authentication refactor"), withTags("security"))
	mustCreateRoom(t, s, "db-migration", withDescription("Database migration"))
	mustCreateRoom(t, s, "jwt-tokens", withDescription("Token validation"), withTags("jwt"))

	// Match by description keyword
	rooms, err := s.ListRooms("", "", "", "JWT", 100, 0)
	if err != nil {
		t.Fatalf("ListRooms search failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms matching 'JWT', got %d", len(rooms))
	}

	// Match by room ID
	rooms, _ = s.ListRooms("", "", "", "db-migration", 100, 0)
	if len(rooms) != 1 {
		t.Errorf("expected 1 room matching ID 'db-migration', got %d", len(rooms))
	}

	// Match by tag content
	rooms, _ = s.ListRooms("", "", "", "security", 100, 0)
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

// -- normalizeProject --

func TestNormalizeProject(t *testing.T) {
	cases := []struct{ in, want string }{
		{"council-hub", "council-hub"},
		{"Council-Hub", "council-hub"},
		{"COUNCIL_HUB", "council-hub"},
		{"My Project", "my-project"},
		{"  spaces  ", "spaces"},
		{"a--b", "a-b"},
		{"special!@#chars", "specialchars"},
		{"", ""},
		{"---", ""},
		{"foo_bar_baz", "foo-bar-baz"},
	}
	for _, tc := range cases {
		got := normalizeProject(tc.in)
		if got != tc.want {
			t.Errorf("normalizeProject(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCreateRoomNormalizesProject(t *testing.T) {
	s := setupTestServer(t)
	if err := s.CreateRoom("norm-room", "desc", "Council-Hub", "", "", "", ""); err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}
	room, _ := s.GetRoom("norm-room")
	if room.Project != "council-hub" {
		t.Errorf("expected project 'council-hub', got '%s'", room.Project)
	}
}

func TestUpdateRoomNormalizesProject(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("upd-norm-room", "desc", "old-project", "", "", "", "")
	if err := s.UpdateRoom("upd-norm-room", "", "NEW_PROJECT", "", "", "", "", "", ""); err != nil {
		t.Fatalf("UpdateRoom failed: %v", err)
	}
	room, _ := s.GetRoom("upd-norm-room")
	if room.Project != "new-project" {
		t.Errorf("expected project 'new-project', got '%s'", room.Project)
	}
}

func TestListRoomsNormalizesProjectFilter(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("filter-room", "desc", "My Project", "", "", "", "")

	// Verify stored value is normalized
	room, _ := s.GetRoom("filter-room")
	if room.Project != "my-project" {
		t.Fatalf("expected stored project 'my-project', got '%s'", room.Project)
	}

	// Filter with different casing/format — should still find it
	rooms, err := s.ListRooms("MY_PROJECT", "", "", "", 100, 0)
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}
	if len(rooms) != 1 {
		t.Errorf("expected 1 room, got %d", len(rooms))
	}
}

// -- cascade-clean related_rooms on deletion --

func TestDeleteRoomCleansRelatedRooms(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-a", "A", "", "", "", "", "")
	s.CreateRoom("room-b", "B", "", "", "", "", "room-a")
	s.CreateRoom("room-c", "C", "", "", "", "", "room-a")

	if err := s.DeleteRoom("room-a"); err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}

	roomB, _ := s.GetRoom("room-b")
	if strings.Contains(roomB.RelatedRooms, "room-a") {
		t.Errorf("room-b still references deleted room-a: %q", roomB.RelatedRooms)
	}

	roomC, _ := s.GetRoom("room-c")
	if strings.Contains(roomC.RelatedRooms, "room-a") {
		t.Errorf("room-c still references deleted room-a: %q", roomC.RelatedRooms)
	}
}

func TestDeleteRoomCleansReverseLinks(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("target", "Target", "", "", "", "", "")
	// Creating source with related_rooms=target triggers syncReverseLinks,
	// so target gets a reverse link back to source.
	s.CreateRoom("source", "Source", "", "", "", "", "target")

	// Verify bidirectional link was established
	target, _ := s.GetRoom("target")
	if !strings.Contains(target.RelatedRooms, "source") {
		t.Fatalf("reverse link not established; target.RelatedRooms=%q", target.RelatedRooms)
	}

	// Delete source — should clean the reverse link from target
	if err := s.DeleteRoom("source"); err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}

	target, _ = s.GetRoom("target")
	if strings.Contains(target.RelatedRooms, "source") {
		t.Errorf("target still references deleted source: %q", target.RelatedRooms)
	}
}

func TestDeleteRoomNoFalsePositiveCleanup(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room", "Short name", "", "", "", "", "")
	s.CreateRoom("room-extra", "Longer name", "", "", "", "", "")
	s.CreateRoom("holder", "Holds both", "", "", "", "", "room, room-extra")

	if err := s.DeleteRoom("room"); err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}

	holder, _ := s.GetRoom("holder")
	if strings.Contains(holder.RelatedRooms, "room-extra") == false {
		t.Errorf("room-extra was incorrectly removed from holder: %q", holder.RelatedRooms)
	}
	if strings.Contains(holder.RelatedRooms, "room") && !strings.Contains(holder.RelatedRooms, "room-extra") {
		// This would mean "room" is still there but "room-extra" is gone — wrong
		t.Errorf("cleanup removed too much: %q", holder.RelatedRooms)
	}
	// Exact check: only "room-extra" should remain
	got := strings.TrimSpace(holder.RelatedRooms)
	if got != "room-extra" {
		t.Errorf("expected holder.RelatedRooms = 'room-extra', got %q", got)
	}
}

func TestUpdateRoomAddRemoveTags(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("tags-room", "", "", "", "foo,bar", "", "")

	// Add a tag
	err := s.UpdateRoom("tags-room", "", "", "", "", "baz", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	room, _ := s.GetRoom("tags-room")
	if !strings.Contains(room.Tags, "foo") || !strings.Contains(room.Tags, "bar") || !strings.Contains(room.Tags, "baz") {
		t.Errorf("Expected tags to contain foo, bar, baz, got '%s'", room.Tags)
	}

	// Remove a tag
	err = s.UpdateRoom("tags-room", "", "", "", "", "", "bar", "", "")
	if err != nil {
		t.Fatal(err)
	}
	room, _ = s.GetRoom("tags-room")
	if strings.Contains(room.Tags, "bar") {
		t.Errorf("Expected tags to NOT contain bar, got '%s'", room.Tags)
	}
	if !strings.Contains(room.Tags, "foo") || !strings.Contains(room.Tags, "baz") {
		t.Errorf("Expected tags to contain foo and baz, got '%s'", room.Tags)
	}

	// Overwrite tags
	err = s.UpdateRoom("tags-room", "", "", "", "new-only", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	room, _ = s.GetRoom("tags-room")
	if room.Tags != "new-only" {
		t.Errorf("Expected tags to be 'new-only', got '%s'", room.Tags)
	}
}

// ========== FindSimilarRooms ==========

func TestFindSimilarRoomsByTag(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "existing-auth", withProject("myapp"), withTags("go,auth,api"))
	mustCreateRoom(t, s, "existing-db", withProject("myapp"), withTags("go,postgres,api"))

	similar, err := s.FindSimilarRooms("new-room", "Auth service implementation", "myapp", "go,auth", 5)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	if len(similar) == 0 {
		t.Fatal("expected at least one similar room")
	}
	if similar[0].ID != "existing-auth" {
		t.Errorf("expected existing-auth as top match, got %s", similar[0].ID)
	}
}

func TestFindSimilarRoomsByDescription(t *testing.T) {
	s := setupTestServer(t)
	// Need score >= 3: use 3 overlapping keywords to reach threshold
	mustCreateRoom(t, s, "auth-service", withDescription("Authentication middleware design patterns"))

	similar, err := s.FindSimilarRooms("new-room", "Authentication middleware design overview", "", "", 5)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	if len(similar) == 0 {
		t.Fatal("expected a match on description keywords (authentication, middleware, design)")
	}
	if similar[0].ID != "auth-service" {
		t.Errorf("expected auth-service as match, got %s", similar[0].ID)
	}
}

func TestFindSimilarRoomsExcludesSelf(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "self-room", withTags("go,auth"))

	similar, err := s.FindSimilarRooms("self-room", "Auth", "", "go,auth", 5)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	for _, r := range similar {
		if r.ID == "self-room" {
			t.Error("FindSimilarRooms should not return the excluded room")
		}
	}
}

func TestFindSimilarRoomsNoSignal(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "some-room", withTags("go"))

	similar, err := s.FindSimilarRooms("new-room", "the a to", "", "", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(similar) != 0 {
		t.Errorf("expected no results with no signal, got %d", len(similar))
	}
}

func TestFindSimilarRoomsLimit(t *testing.T) {
	s := setupTestServer(t)
	for i := 0; i < 5; i++ {
		mustCreateRoom(t, s, strings.Repeat("r", i+1)+"-room", withTags("go,auth,api"))
	}

	similar, err := s.FindSimilarRooms("new", "Auth", "", "go,auth,api", 2)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	if len(similar) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(similar))
	}
}

func TestGetConceptMap(t *testing.T) {
	s := setupTestServer(t)

	// Create a graph:
	// root -> a, b
	// a -> c
	// c -> root (cycle)
	// b -> d
	// e (unconnected)
	// NOTE: CreateRoom triggers syncReverseLinks, so root <-> a, root <-> b, a <-> c, etc.
	s.CreateRoom("root", "Root topic", "proj", "", "tag1", "", "a, b")
	s.CreateRoom("a", "Topic A", "proj", "", "tag2", "", "c")
	s.CreateRoom("b", "Topic B", "proj", "", "tag3", "", "d")
	s.CreateRoom("c", "Topic C", "proj", "", "tag4", "", "root")
	s.CreateRoom("d", "Topic D", "proj", "", "tag5", "", "")
	s.CreateRoom("e", "Topic E", "proj", "", "tag6", "", "")

	// Test 1: Depth 1
	nodes, err := s.GetConceptMap("root", 1)
	if err != nil {
		t.Fatalf("GetConceptMap depth 1 failed: %v", err)
	}
	// Expected: root (0), a (1), b (1), c (1)
	// 'c' is depth 1 because s.CreateRoom("c", ..., "root") created a link root -> c.
	if len(nodes) != 4 {
		t.Errorf("expected 4 nodes at depth 1, got %d", len(nodes))
	}

	// Test 2: Full traversal (Depth 3)
	nodes, err = s.GetConceptMap("root", 3)
	if err != nil {
		t.Fatalf("GetConceptMap depth 3 failed: %v", err)
	}
	// Expected: root (0), a (1), b (1), c (1), d (2)
	// 'e' should be missing.
	if len(nodes) != 5 {
		t.Errorf("expected 5 nodes at depth 3, got %d", len(nodes))
	}

	depthMap := make(map[string]int)
	viaMap := make(map[string]string)
	for _, n := range nodes {
		depthMap[n.Room.ID] = n.Depth
		viaMap[n.Room.ID] = n.Via
	}

	if depthMap["root"] != 0 {
		t.Errorf("expected root depth 0, got %d", depthMap["root"])
	}
	if depthMap["a"] != 1 || viaMap["a"] != "root" {
		t.Errorf("expected a depth 1 via root, got depth %d via %s", depthMap["a"], viaMap["a"])
	}
	if depthMap["c"] != 1 || viaMap["c"] != "root" {
		t.Errorf("expected c depth 1 via root, got depth %d via %s", depthMap["c"], viaMap["c"])
	}
	if depthMap["d"] != 2 || viaMap["d"] != "b" {
		t.Errorf("expected d depth 2 via b, got depth %d via %s", depthMap["d"], viaMap["d"])
	}

	// Test 3: Unconnected
	nodes, _ = s.GetConceptMap("e", 3)
	if len(nodes) != 1 || nodes[0].Room.ID != "e" {
		t.Errorf("expected only 'e' for unconnected start, got %d nodes", len(nodes))
	}

	// Test 4: Max depth enforcement
	nodes, _ = s.GetConceptMap("root", 10) // should be capped to 5
	// Our graph only goes to depth 2, so this should still return 5 nodes
	if len(nodes) != 5 {
		t.Errorf("expected 5 nodes for deep search on shallow graph, got %d", len(nodes))
	}
}

func TestUpdateStatus_ClearsHealthTagsOnResolve(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("health-room", "Test", "", "", "needs-synthesis,important", "", "")

	if err := s.UpdateStatus("health-room", "resolved"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	room, _ := s.GetRoom("health-room")
	if room.Status != "resolved" {
		t.Errorf("expected status 'resolved', got '%s'", room.Status)
	}
	if hasTag(room.Tags, "needs-synthesis") {
		t.Errorf("expected 'needs-synthesis' to be stripped on resolve, got tags: %s", room.Tags)
	}
	if !hasTag(room.Tags, "important") {
		t.Errorf("expected 'important' tag to be preserved, got tags: %s", room.Tags)
	}
}

func TestUpdateStatus_ClearsStaleTagOnResolve(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("stale-room", "Test", "", "", "stale,backlog", "", "")

	if err := s.UpdateStatus("stale-room", "resolved"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	room, _ := s.GetRoom("stale-room")
	if hasTag(room.Tags, "stale") {
		t.Errorf("expected 'stale' to be stripped on resolve, got tags: %s", room.Tags)
	}
	if !hasTag(room.Tags, "backlog") {
		t.Errorf("expected 'backlog' tag to be preserved, got tags: %s", room.Tags)
	}
}

func TestUpdateStatus_NoTagStripOnActiveOrPaused(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("active-room", "Test", "", "", "needs-synthesis,stale", "", "")

	// Pausing should not strip health tags
	if err := s.UpdateStatus("active-room", "paused"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	room, _ := s.GetRoom("active-room")
	if !hasTag(room.Tags, "needs-synthesis") || !hasTag(room.Tags, "stale") {
		t.Errorf("expected health tags preserved on pause, got tags: %s", room.Tags)
	}
}

func TestNormalizeTags(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`["mtls","gateway"]`, "mtls,gateway"},
		{`["single"]`, "single"},
		{`mtls,gateway`, "mtls,gateway"},
		{` mtls , gateway `, "mtls,gateway"},
		{`[]`, ""},
		{``, ""},
		{`["a", "b", "c"]`, "a,b,c"},
	}
	for _, c := range cases {
		got := normalizeTags(c.input)
		if got != c.want {
			t.Errorf("normalizeTags(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestCreateRoom_NormalizesTags(t *testing.T) {
	s := setupTestServer(t)
	if err := s.CreateRoom("norm-room", "test", "", "", `["auth","mtls"]`, "", ""); err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}
	room, _ := s.GetRoom("norm-room")
	if room.Tags != "auth,mtls" {
		t.Errorf("expected tags 'auth,mtls', got %q", room.Tags)
	}
}
