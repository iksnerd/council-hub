package main

import (
	"os"
	"strings"
	"testing"
)

func TestCreateRoom(t *testing.T) {
	cs := setupTestServer(t)

	if err := cs.createRoom("test-room", "A test room", "", "", "", "", ""); err != nil {
		t.Fatalf("createRoom failed: %v", err)
	}

	room, err := cs.getRoom("test-room")
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
	cs := setupTestServer(t)

	if err := cs.createRoom("dup-room", "First", "", "", "", "", ""); err != nil {
		t.Fatalf("first createRoom failed: %v", err)
	}
	if err := cs.createRoom("dup-room", "Second", "", "", "", "", ""); err != nil {
		t.Fatalf("duplicate createRoom failed: %v", err)
	}

	room, _ := cs.getRoom("dup-room")
	if room.Description != "First" {
		t.Errorf("expected original description 'First', got '%s'", room.Description)
	}
}

func TestCreateRoomWithMetadata(t *testing.T) {
	cs := setupTestServer(t)

	err := cs.createRoom("auth-api", "JWT refactoring", "llm-memory", "Go, SQLite, MCP SDK", "auth,security", "You are reviewing for security issues.", "")
	if err != nil {
		t.Fatalf("createRoom failed: %v", err)
	}

	room, err := cs.getRoom("auth-api")
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
	cs := setupTestServer(t)
	err := cs.createRoom("bep44-room", "BEP 44 analysis", "weightless", "Go", "dht,bep44", "", "bep46-room,provenance-room")
	if err != nil {
		t.Fatalf("createRoom failed: %v", err)
	}

	room, err := cs.getRoom("bep44-room")
	if err != nil {
		t.Fatalf("getRoom failed: %v", err)
	}
	if room.RelatedRooms != "bep46-room,provenance-room" {
		t.Errorf("expected related_rooms 'bep46-room,provenance-room', got '%s'", room.RelatedRooms)
	}
}

func TestSignalStatus(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("status-room", "Status test", "", "", "", "", "")

	if err := cs.updateStatus("status-room", "paused"); err != nil {
		t.Fatalf("updateStatus failed: %v", err)
	}

	room, _ := cs.getRoom("status-room")
	if room.Status != "paused" {
		t.Errorf("expected status 'paused', got '%s'", room.Status)
	}
}

func TestUpdateStatusNonexistentRoom(t *testing.T) {
	cs := setupTestServer(t)

	err := cs.updateStatus("nonexistent", "active")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestUpdateRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("update-room", "Original topic", "old-project", "Go", "old-tag", "Old prompt", "")

	// Update only project and tags
	if err := cs.updateRoom("update-room", "", "new-project", "", "new-tag", "", ""); err != nil {
		t.Fatalf("updateRoom failed: %v", err)
	}

	room, _ := cs.getRoom("update-room")
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
	cs := setupTestServer(t)
	cs.createRoom("link-room", "Link test", "", "", "", "", "")

	if err := cs.updateRoom("link-room", "", "", "", "", "", "room-a,room-b"); err != nil {
		t.Fatalf("updateRoom failed: %v", err)
	}

	room, _ := cs.getRoom("link-room")
	if room.RelatedRooms != "room-a,room-b" {
		t.Errorf("expected related_rooms 'room-a,room-b', got '%s'", room.RelatedRooms)
	}
}

func TestUpdateRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	err := cs.updateRoom("nonexistent", "topic", "", "", "", "", "")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestDeleteRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("del-room", "To be deleted", "", "", "", "", "")
	cs.postMessage("del-room", "Claude", "Message 1", "message", 0)
	cs.postMessage("del-room", "Gemini", "Message 2", "message", 0)

	if err := cs.deleteRoom("del-room"); err != nil {
		t.Fatalf("deleteRoom failed: %v", err)
	}

	// Room should be gone
	_, err := cs.getRoom("del-room")
	if err == nil {
		t.Error("expected error getting deleted room")
	}

	// Messages should be gone
	msgs, _ := cs.getTranscript("del-room")
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after delete, got %d", len(msgs))
	}
}

func TestDeleteRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	err := cs.deleteRoom("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestListRooms(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("room-a", "Auth work", "project-alpha", "Go", "auth,api", "", "")
	cs.createRoom("room-b", "Frontend", "project-beta", "React, TypeScript", "frontend", "", "")
	cs.createRoom("room-c", "More auth", "project-alpha", "Go", "auth", "", "")

	// Filter by project
	rooms, err := cs.listRooms("project-alpha", "", "", "")
	if err != nil {
		t.Fatalf("listRooms failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms for project-alpha, got %d", len(rooms))
	}

	// Filter by tag
	rooms, _ = cs.listRooms("", "auth", "", "")
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms with tag 'auth', got %d", len(rooms))
	}

	// Filter by tag that only one room has
	rooms, _ = cs.listRooms("", "frontend", "", "")
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room with tag 'frontend', got %d", len(rooms))
	}

	// No filter — all rooms
	rooms, _ = cs.listRooms("", "", "", "")
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms total, got %d", len(rooms))
	}

	// Filter by project + tag
	rooms, _ = cs.listRooms("project-alpha", "api", "", "")
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room for project-alpha+api, got %d", len(rooms))
	}
}

func TestListRoomsByStatus(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("active-room", "Active", "", "", "", "", "")
	cs.createRoom("paused-room", "Paused", "", "", "", "", "")
	cs.updateStatus("paused-room", "paused")

	rooms, _ := cs.listRooms("", "", "paused", "")
	if len(rooms) != 1 {
		t.Fatalf("expected 1 paused room, got %d", len(rooms))
	}
	if rooms[0].ID != "paused-room" {
		t.Errorf("expected 'paused-room', got '%s'", rooms[0].ID)
	}
}

func TestRoomStats(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("stats-room", "Stats test", "", "", "", "", "")
	cs.postMessage("stats-room", "Claude", "Message 1", "thought", 0)
	cs.postMessage("stats-room", "Claude", "Message 2", "decision", 0)
	cs.postMessage("stats-room", "Gemini", "Message 3", "review", 0)

	stats, err := cs.getRoomStats("stats-room")
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
	cs := setupTestServer(t)
	cs.createRoom("empty-stats", "Empty room", "", "", "", "", "")

	stats, err := cs.getRoomStats("empty-stats")
	if err != nil {
		t.Fatalf("getRoomStats failed: %v", err)
	}
	if stats.MessageCount != 0 {
		t.Errorf("expected 0 messages, got %d", stats.MessageCount)
	}
}

func TestRoomStatsNotFound(t *testing.T) {
	cs := setupTestServer(t)

	_, err := cs.getRoomStats("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestArchiveRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("archive-room", "Archive test", "proj", "Go", "test", "Be helpful", "")
	cs.postMessage("archive-room", "Claude", "Test message", "message", 0)

	archivePath, err := cs.archiveRoom("archive-room")
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
	cs := setupTestServer(t)

	_, err := cs.archiveRoom("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestNewColumnsUsable(t *testing.T) {
	cs := setupTestServer(t)

	cs.createRoom("migrate-room", "Migration test", "", "", "", "", "related-a")
	room, _ := cs.getRoom("migrate-room")
	if room.RelatedRooms != "related-a" {
		t.Errorf("expected related_rooms 'related-a', got '%s'", room.RelatedRooms)
	}

	id, _ := cs.postMessage("migrate-room", "Test", "msg", "message", 42)
	msgs, _ := cs.getMessagesByIDs([]int64{id})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].ReplyTo != 42 {
		t.Errorf("expected reply_to 42, got %d", msgs[0].ReplyTo)
	}
}

// -- updateRoom: all fields set (covers every branch) --

func TestUpdateRoomAllFields(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "upd-all", withDescription("Topic"), withProject("Proj"), withTechStack("Tech"), withTags("Tags"), withSystemPrompt("Prompt"), withRelatedRooms("Related"))

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
	mustCreateRoom(t, cs, "ls-active")
	mustCreateRoom(t, cs, "ls-resolved")
	cs.updateStatus("ls-resolved", "resolved")

	rooms, _ := cs.listRooms("", "", "active", "")
	if len(rooms) != 1 || rooms[0].ID != "ls-active" {
		t.Errorf("expected only active room, got %d rooms", len(rooms))
	}

	rooms, _ = cs.listRooms("", "", "resolved", "")
	if len(rooms) != 1 || rooms[0].ID != "ls-resolved" {
		t.Errorf("expected only resolved room, got %d rooms", len(rooms))
	}
}

// -- NewCouncilServer with file DB to exercise non-memory DSN path --

func TestNewCouncilServerFileDSN(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	cs, err := NewCouncilServer(dbPath, testLogger())
	if err != nil {
		t.Fatalf("NewCouncilServer failed: %v", err)
	}
	defer cs.db.Close()

	// Verify it works
	mustCreateRoom(t, cs, "file-room")
}

// -- archiveRoom with :memory: DB (different archive dir logic) --

func TestArchiveRoomMemoryDB(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "mem-archive")
	mustPost(t, cs, "mem-archive", "Claude", "test")

	path, err := cs.archiveRoom("mem-archive")
	if err != nil {
		t.Fatalf("archiveRoom failed: %v", err)
	}
	if !strings.Contains(path, "archives") {
		t.Errorf("expected archives in path, got: %s", path)
	}
}

// -- deleteRoom: error wrapping includes room ID context --

func TestDeleteRoomErrorWrapping(t *testing.T) {
	cs := setupTestServer(t)

	err := cs.deleteRoom("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should contain room ID, got: %s", err)
	}
}

// -- deleteRoom: message cleanup error path (drop messages table to trigger) --

func TestDeleteRoomMessageCleanupError(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "del-msg-err")
	mustPost(t, cs, "del-msg-err", "Claude", "msg")

	// Drop messages table so DELETE FROM messages fails
	cs.db.Exec("DROP TABLE messages")

	err := cs.deleteRoom("del-msg-err")
	if err == nil {
		t.Fatal("expected error when messages table is missing")
	}
	if !strings.Contains(err.Error(), "delete messages") {
		t.Errorf("expected 'delete messages' in error, got: %s", err)
	}
}

// -- deleteRoom: room DELETE itself fails (closed DB) --

func TestDeleteRoomExecError(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "del-exec-err")
	cs.db.Close()

	err := cs.deleteRoom("del-exec-err")
	if err == nil {
		t.Fatal("expected error on closed DB")
	}
	if !strings.Contains(err.Error(), "delete room") {
		t.Errorf("expected 'delete room' in error, got: %s", err)
	}
}

// -- archiveRoom: getTranscript fails --

func TestArchiveRoomTranscriptError(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "arch-transcript-err")
	mustPost(t, cs, "arch-transcript-err", "Claude", "msg")
	// Corrupt messages table so getTranscript fails
	cs.db.Exec("ALTER TABLE messages RENAME TO messages_old")
	cs.db.Exec("CREATE TABLE messages AS SELECT id, room_id FROM messages_old")

	_, err := cs.archiveRoom("arch-transcript-err")
	if err == nil {
		t.Error("expected transcript error during archive")
	}
}
