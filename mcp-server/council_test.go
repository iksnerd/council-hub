package main

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func init() {
	// Clean up any leftover test archives
	os.RemoveAll("archives")
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

func setupTestServer(t *testing.T) *CouncilServer {
	t.Helper()
	cs, err := NewCouncilServer(":memory:", testLogger())
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	t.Cleanup(func() { cs.db.Close() })
	return cs
}

func TestCreateRoom(t *testing.T) {
	cs := setupTestServer(t)

	if err := cs.createRoom("test-room", "A test room", "", "", "", ""); err != nil {
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

	if err := cs.createRoom("dup-room", "First", "", "", "", ""); err != nil {
		t.Fatalf("first createRoom failed: %v", err)
	}
	if err := cs.createRoom("dup-room", "Second", "", "", "", ""); err != nil {
		t.Fatalf("duplicate createRoom failed: %v", err)
	}

	room, _ := cs.getRoom("dup-room")
	if room.Description != "First" {
		t.Errorf("expected original description 'First', got '%s'", room.Description)
	}
}

func TestCreateRoomWithMetadata(t *testing.T) {
	cs := setupTestServer(t)

	err := cs.createRoom("auth-api", "JWT refactoring", "llm-memory", "Go, SQLite, MCP SDK", "auth,security", "You are reviewing for security issues.")
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

func TestPostMessage(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("msg-room", "Message test", "", "", "", "")

	id1, err := cs.postMessage("msg-room", "Claude", "Hello from Claude", "message")
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}
	id2, err := cs.postMessage("msg-room", "Gemini", "Hello from Gemini", "message")
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}

	if id2 <= id1 {
		t.Errorf("expected id2 > id1, got id1=%d id2=%d", id1, id2)
	}

	msgs, err := cs.getTranscript("msg-room")
	if err != nil {
		t.Fatalf("getTranscript failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Author != "Claude" || msgs[1].Author != "Gemini" {
		t.Errorf("unexpected message order: %s, %s", msgs[0].Author, msgs[1].Author)
	}
}

func TestMessageType(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("type-room", "Type test", "", "", "", "")

	cs.postMessage("type-room", "Claude", "I think we should...", "thought")
	cs.postMessage("type-room", "Gemini", "Let's go with RS256", "decision")
	cs.postMessage("type-room", "Claude", "func main() {}", "code")

	msgs, _ := cs.getTranscript("type-room")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].MessageType != "thought" {
		t.Errorf("expected 'thought', got '%s'", msgs[0].MessageType)
	}
	if msgs[1].MessageType != "decision" {
		t.Errorf("expected 'decision', got '%s'", msgs[1].MessageType)
	}
	if msgs[2].MessageType != "code" {
		t.Errorf("expected 'code', got '%s'", msgs[2].MessageType)
	}
}

func TestSignalStatus(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("status-room", "Status test", "", "", "", "")

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

func TestTranscriptFormatting(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("fmt-room", "Formatting test", "", "", "", "")
	cs.postMessage("fmt-room", "Claude", "First message", "message")
	cs.postMessage("fmt-room", "Gemini", "Second message", "message")

	room, _ := cs.getRoom("fmt-room")
	msgs, _ := cs.getTranscript("fmt-room")
	transcript := formatTranscript(room, msgs)

	if !strings.Contains(transcript, "# COUNCIL ROOM: fmt-room") {
		t.Error("transcript missing room header")
	}
	if !strings.Contains(transcript, "**Topic:** Formatting test") {
		t.Error("transcript missing topic")
	}
	if !strings.Contains(transcript, "**Status:** active") {
		t.Error("transcript missing status")
	}
	if !strings.Contains(transcript, "Claude") || !strings.Contains(transcript, "Gemini") {
		t.Error("transcript missing authors")
	}
	if !strings.Contains(transcript, "post_to_room") {
		t.Error("transcript missing system instruction")
	}
}

func TestTranscriptWithFullMetadata(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("rich-room", "JWT refactoring", "llm-memory", "Go, SQLite", "auth,security", "Focus on token handling.")
	cs.postMessage("rich-room", "Claude", "I think we should use RS256", "thought")
	cs.postMessage("rich-room", "Gemini", "Agreed, let's proceed", "decision")

	room, _ := cs.getRoom("rich-room")
	msgs, _ := cs.getTranscript("rich-room")
	transcript := formatTranscript(room, msgs)

	if !strings.Contains(transcript, "**Project:** llm-memory") {
		t.Error("transcript missing project")
	}
	if !strings.Contains(transcript, "**Tech Stack:** Go, SQLite") {
		t.Error("transcript missing tech stack")
	}
	if !strings.Contains(transcript, "**Tags:** auth,security") {
		t.Error("transcript missing tags")
	}
	if !strings.Contains(transcript, "*Instructions: Focus on token handling.*") {
		t.Error("transcript missing system prompt")
	}
	if !strings.Contains(transcript, "Claude (thought)") {
		t.Error("transcript missing message type for thought")
	}
	if !strings.Contains(transcript, "Gemini (decision)") {
		t.Error("transcript missing message type for decision")
	}
}

func TestTranscriptWithSummary(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("sum-room", "Summary test", "", "", "", "")

	for i := 0; i < 5; i++ {
		cs.postMessage("sum-room", "Claude", "Old message", "message")
	}

	cs.insertSummary("sum-room", "Summary of 5 old messages")

	cs.postMessage("sum-room", "Gemini", "New message after summary", "message")

	msgs, err := cs.getTranscript("sum-room")
	if err != nil {
		t.Fatalf("getTranscript failed: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (summary + new), got %d", len(msgs))
	}

	if !msgs[0].IsSummary {
		t.Error("first message should be the summary")
	}
	if msgs[1].Author != "Gemini" {
		t.Errorf("second message should be from Gemini, got '%s'", msgs[1].Author)
	}
}

func TestListRooms(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("room-a", "Auth work", "project-alpha", "Go", "auth,api", "")
	cs.createRoom("room-b", "Frontend", "project-beta", "React, TypeScript", "frontend", "")
	cs.createRoom("room-c", "More auth", "project-alpha", "Go", "auth", "")

	// Filter by project
	rooms, err := cs.listRooms("project-alpha", "", "")
	if err != nil {
		t.Fatalf("listRooms failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms for project-alpha, got %d", len(rooms))
	}

	// Filter by tag
	rooms, _ = cs.listRooms("", "auth", "")
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms with tag 'auth', got %d", len(rooms))
	}

	// Filter by tag that only one room has
	rooms, _ = cs.listRooms("", "frontend", "")
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room with tag 'frontend', got %d", len(rooms))
	}

	// No filter — all rooms
	rooms, _ = cs.listRooms("", "", "")
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms total, got %d", len(rooms))
	}

	// Filter by project + tag
	rooms, _ = cs.listRooms("project-alpha", "api", "")
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room for project-alpha+api, got %d", len(rooms))
	}
}

func TestJanitorSweep(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("janitor-room", "Janitor test", "", "", "", "")

	for i := 0; i < 25; i++ {
		cs.postMessage("janitor-room", "Claude", "Message content", "message")
	}

	rooms, err := cs.getRoomsNeedingSummary(20)
	if err != nil {
		t.Fatalf("getRoomsNeedingSummary failed: %v", err)
	}
	if len(rooms) != 1 || rooms[0] != "janitor-room" {
		t.Fatalf("expected janitor-room to need summary, got %v", rooms)
	}

	cs.janitorSweep()

	msgs, _ := cs.getTranscript("janitor-room")
	hasSummary := false
	for _, m := range msgs {
		if m.IsSummary {
			hasSummary = true
			break
		}
	}
	if !hasSummary {
		t.Error("expected summary message after janitor sweep")
	}

	rooms, _ = cs.getRoomsNeedingSummary(20)
	if len(rooms) != 0 {
		t.Errorf("expected no rooms needing summary after sweep, got %v", rooms)
	}
}

func TestUpdateRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("update-room", "Original topic", "old-project", "Go", "old-tag", "Old prompt")

	// Update only project and tags
	if err := cs.updateRoom("update-room", "", "new-project", "", "new-tag", ""); err != nil {
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

func TestDeleteRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("del-room", "To be deleted", "", "", "", "")
	cs.postMessage("del-room", "Claude", "Message 1", "message")
	cs.postMessage("del-room", "Gemini", "Message 2", "message")

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

func TestUpdateRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	err := cs.updateRoom("nonexistent", "topic", "", "", "", "")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestSearchMessages(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("search-room-1", "Room 1", "proj", "", "", "")
	cs.createRoom("search-room-2", "Room 2", "proj", "", "", "")
	cs.postMessage("search-room-1", "Claude", "JWT token validation is broken", "thought")
	cs.postMessage("search-room-1", "Gemini", "I agree about the JWT issue", "review")
	cs.postMessage("search-room-2", "Claude", "Database migration complete", "action")

	// Search by keyword
	msgs, err := cs.searchMessages("JWT", "", "", "", 20)
	if err != nil {
		t.Fatalf("searchMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages with 'JWT', got %d", len(msgs))
	}

	// Search by author
	msgs, _ = cs.searchMessages("", "Claude", "", "", 20)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages from Claude, got %d", len(msgs))
	}

	// Search by message type
	msgs, _ = cs.searchMessages("", "", "review", "", 20)
	if len(msgs) != 1 {
		t.Errorf("expected 1 review message, got %d", len(msgs))
	}

	// Search scoped to room
	msgs, _ = cs.searchMessages("", "Claude", "", "search-room-2", 20)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message from Claude in search-room-2, got %d", len(msgs))
	}

	// No results
	msgs, _ = cs.searchMessages("nonexistent", "", "", "", 20)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestRoomStats(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("stats-room", "Stats test", "", "", "", "")
	cs.postMessage("stats-room", "Claude", "Message 1", "thought")
	cs.postMessage("stats-room", "Claude", "Message 2", "decision")
	cs.postMessage("stats-room", "Gemini", "Message 3", "review")

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
	cs.createRoom("empty-stats", "Empty room", "", "", "", "")

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

func TestDeleteMessages(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("delmsg-room", "Delete messages test", "", "", "", "")
	id1, _ := cs.postMessage("delmsg-room", "Claude", "Keep this", "message")
	id2, _ := cs.postMessage("delmsg-room", "Gemini", "Delete this", "message")
	id3, _ := cs.postMessage("delmsg-room", "Claude", "Delete this too", "message")

	count, err := cs.deleteMessages([]int64{id2, id3})
	if err != nil {
		t.Fatalf("deleteMessages failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 deleted, got %d", count)
	}

	msgs, _ := cs.getTranscript("delmsg-room")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(msgs))
	}
	if msgs[0].ID != id1 {
		t.Errorf("expected message %d to remain, got %d", id1, msgs[0].ID)
	}
}

func TestDeleteMessagesNonexistent(t *testing.T) {
	cs := setupTestServer(t)

	count, err := cs.deleteMessages([]int64{99999})
	if err != nil {
		t.Fatalf("deleteMessages failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 deleted, got %d", count)
	}
}

func TestArchiveRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("archive-room", "Archive test", "proj", "Go", "test", "Be helpful")
	cs.postMessage("archive-room", "Claude", "Test message", "message")

	archivePath, err := cs.archiveRoom("archive-room")
	if err != nil {
		t.Fatalf("archiveRoom failed: %v", err)
	}

	// Verify file was created
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

	// Clean up
	os.RemoveAll("archives")
}

func TestArchiveRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	_, err := cs.archiveRoom("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestSummarize(t *testing.T) {
	msgs := []Message{
		{Author: "Claude", Content: "First point", MessageType: "thought"},
		{Author: "Gemini", Content: "Second point", MessageType: "decision"},
	}

	summary := summarize(msgs)

	if !strings.Contains(summary, "2 messages") {
		t.Error("summary should mention message count")
	}
	if !strings.Contains(summary, "Claude") || !strings.Contains(summary, "Gemini") {
		t.Error("summary should mention participants")
	}
}

