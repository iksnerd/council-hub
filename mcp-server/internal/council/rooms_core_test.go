package council

import (
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

func TestCreateRoomDefaultVisibilityPublic(t *testing.T) {
	s := setupTestServer(t)

	if err := s.CreateRoom("vis-room", "A room", "", "", "", "", ""); err != nil {
		t.Fatalf("createRoom failed: %v", err)
	}
	room, _ := s.GetRoom("vis-room")
	if room.Visibility != "public" {
		t.Errorf("expected default visibility 'public', got '%s'", room.Visibility)
	}
}

func TestSetVisibility(t *testing.T) {
	s := setupTestServer(t)

	if err := s.CreateRoom("priv-room", "A room", "", "", "", "", ""); err != nil {
		t.Fatalf("createRoom failed: %v", err)
	}

	if err := s.SetVisibility("priv-room", "private"); err != nil {
		t.Fatalf("SetVisibility failed: %v", err)
	}
	room, _ := s.GetRoom("priv-room")
	if room.Visibility != "private" {
		t.Errorf("expected visibility 'private', got '%s'", room.Visibility)
	}

	// Unknown / empty values normalize to public.
	if err := s.SetVisibility("priv-room", "bogus"); err != nil {
		t.Fatalf("SetVisibility failed: %v", err)
	}
	room, _ = s.GetRoom("priv-room")
	if room.Visibility != "public" {
		t.Errorf("expected normalized visibility 'public', got '%s'", room.Visibility)
	}

	// Missing room is an error.
	if err := s.SetVisibility("nope", "private"); err == nil {
		t.Error("expected error setting visibility on missing room")
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
