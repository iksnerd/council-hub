package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func resultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	if tc, ok := r.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

// -- handleCreateRoom --

func TestHandleCreateRoom(t *testing.T) {
	cs := setupTestServer(t)
	registerTools(cs)

	res, _, err := cs.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "h-room", Topic: "Handler test", Project: "proj",
	})
	if err != nil {
		t.Fatalf("handleCreateRoom error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "h-room") || !strings.Contains(text, "ready") {
		t.Errorf("unexpected result: %s", text)
	}

	room, _ := cs.getRoom("h-room")
	if room.Project != "proj" {
		t.Errorf("expected project 'proj', got '%s'", room.Project)
	}
}

func TestHandleCreateRoomMissingID(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleCreateRoom(context.Background(), nil, CreateRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing ID, got: %s", text)
	}
}

// -- handlePostToRoom --

func TestHandlePostToRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-post", "Post test", "", "", "", "", "")

	res, _, err := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post", Author: "Claude", Message: "Hello", MessageType: "thought",
	})
	if err != nil {
		t.Fatalf("handlePostToRoom error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("unexpected result: %s", text)
	}
}

func TestHandlePostToRoomMissingFields(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing fields, got: %s", text)
	}
}

func TestHandlePostToRoomInvalidType(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-post-bad", "Bad type test", "", "", "", "", "")

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post-bad", Author: "Claude", Message: "Hello", MessageType: "invalid",
	})
	text := resultText(res)
	if !strings.Contains(text, "Invalid message_type") {
		t.Errorf("expected invalid type error, got: %s", text)
	}
}

func TestHandlePostToRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "nonexistent", Author: "Claude", Message: "Hello",
	})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found error, got: %s", text)
	}
}

func TestHandlePostToRoomDefaultType(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-post-default", "Default type", "", "", "", "", "")

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-post-default", Author: "Claude", Message: "Hello",
	})
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("expected success, got: %s", text)
	}
}

func TestHandlePostToRoomWithReplyTo(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-reply", "Reply test", "", "", "", "", "")
	id, _ := cs.postMessage("h-reply", "Claude", "Original", "message", 0)

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-reply", Author: "Gemini", Message: "Reply",
		ReplyTo: "1",
	})
	text := resultText(res)
	if !strings.Contains(text, "posted") {
		t.Errorf("expected success, got: %s", text)
	}

	msgs, _ := cs.getRecentMessages("h-reply", 2)
	// The reply should be the second message
	found := false
	for _, m := range msgs {
		if m.Author == "Gemini" && m.ReplyTo == id {
			found = true
		}
	}
	if !found {
		t.Error("reply_to not preserved through handler")
	}
}

func TestHandlePostToRoomInvalidReplyTo(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-reply-bad", "Bad reply", "", "", "", "", "")

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "h-reply-bad", Author: "Claude", Message: "Hello",
		ReplyTo: "not-a-number",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for bad reply_to, got: %s", text)
	}
}

// -- handleSignalStatus --

func TestHandleSignalStatus(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-status", "Status test", "", "", "", "", "")

	res, _, _ := cs.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "h-status", Status: "resolved",
	})
	text := resultText(res)
	if !strings.Contains(text, "resolved") {
		t.Errorf("expected resolved, got: %s", text)
	}
}

func TestHandleSignalStatusInvalid(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "whatever", Status: "invalid",
	})
	text := resultText(res)
	if !strings.Contains(text, "Invalid status") {
		t.Errorf("expected invalid status error, got: %s", text)
	}
}

// -- handleListRooms --

func TestHandleListRooms(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-list-1", "Room 1", "proj-a", "", "tag1", "", "")
	cs.createRoom("h-list-2", "Room 2", "proj-b", "", "tag2", "", "related-room")

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if !strings.Contains(text, "2 room(s)") {
		t.Errorf("expected 2 rooms, got: %s", text)
	}
	if !strings.Contains(text, "h-list-1") || !strings.Contains(text, "h-list-2") {
		t.Error("list missing room IDs")
	}
	if !strings.Contains(text, "Related: related-room") {
		t.Error("list missing related rooms")
	}
}

func TestHandleListRoomsEmpty(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if !strings.Contains(text, "No rooms found") {
		t.Errorf("expected no rooms, got: %s", text)
	}
}

func TestHandleListRoomsCompact(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-compact-1", "Short topic", "proj-a", "Go", "tag1", "", "")
	cs.createRoom("h-compact-2", "Another topic that is a bit longer and should be shown", "proj-b", "", "tag2", "", "related-x")

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)

	// Should be compact format
	if !strings.Contains(text, "2 room(s)") {
		t.Errorf("expected 2 rooms, got: %s", text)
	}
	// Compact should have pipe-separated format
	if !strings.Contains(text, "h-compact-1") || !strings.Contains(text, "h-compact-2") {
		t.Error("missing room IDs in compact output")
	}
	if !strings.Contains(text, "proj-a") {
		t.Error("missing project in compact output")
	}
	// Compact should NOT include verbose fields like Tech, Related
	if strings.Contains(text, "Tech:") {
		t.Error("compact mode should not include Tech field")
	}
	if strings.Contains(text, "Related:") {
		t.Error("compact mode should not include Related field")
	}
}

func TestHandleListRoomsCompactNoProject(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-compact-noproj", "No project room", "", "", "", "", "")

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)

	// Room with no project should show "-"
	if !strings.Contains(text, "- |") {
		t.Error("expected dash for empty project in compact mode")
	}
}

func TestHandleListRoomsCompactLongTopic(t *testing.T) {
	cs := setupTestServer(t)
	longTopic := strings.Repeat("A", 80)
	cs.createRoom("h-compact-long", longTopic, "proj", "", "", "", "")

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)

	// Topic should be truncated to 60 chars + "..."
	if !strings.Contains(text, "...") {
		t.Error("expected truncated topic in compact mode")
	}
	if strings.Contains(text, strings.Repeat("A", 80)) {
		t.Error("full 80-char topic should not appear in compact mode")
	}
}

func TestHandleListRoomsNonCompact(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-verbose-1", "Full detail room", "proj", "Go, Docker", "auth", "", "related-a")

	// Without compact=true, should include verbose fields
	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)

	if !strings.Contains(text, "Tech: Go, Docker") {
		t.Error("non-compact should include Tech field")
	}
	if !strings.Contains(text, "Related: related-a") {
		t.Error("non-compact should include Related field")
	}
}

func TestHandleListRoomsFiltered(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-filt-1", "Room 1", "proj-a", "", "", "", "")
	cs.createRoom("h-filt-2", "Room 2", "proj-b", "", "", "", "")

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Project: "proj-a"})
	text := resultText(res)
	if !strings.Contains(text, "1 room(s)") {
		t.Errorf("expected 1 room, got: %s", text)
	}
}

// -- handleUpdateRoom --

func TestHandleUpdateRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-update", "Original", "", "", "", "", "")

	res, _, _ := cs.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "h-update", Topic: "Updated topic", RelatedRooms: "room-x",
	})
	text := resultText(res)
	if !strings.Contains(text, "topic") || !strings.Contains(text, "related_rooms") {
		t.Errorf("expected updated fields listed, got: %s", text)
	}

	room, _ := cs.getRoom("h-update")
	if room.Description != "Updated topic" {
		t.Errorf("expected 'Updated topic', got '%s'", room.Description)
	}
	if room.RelatedRooms != "room-x" {
		t.Errorf("expected related_rooms 'room-x', got '%s'", room.RelatedRooms)
	}
}

func TestHandleUpdateRoomMissingID(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleUpdateRoomNoFields(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{RoomID: "x"})
	text := resultText(res)
	if !strings.Contains(text, "at least one field") {
		t.Errorf("expected field error, got: %s", text)
	}
}

// -- handleReadRoom --

func TestHandleReadRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-read", "Read test", "proj", "Go", "tag1", "prompt", "related-a")

	res, _, _ := cs.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "h-read"})
	text := resultText(res)
	if !strings.Contains(text, "Read test") {
		t.Error("missing topic")
	}
	if !strings.Contains(text, "proj") {
		t.Error("missing project")
	}
	if !strings.Contains(text, "Related Rooms:** related-a") {
		t.Error("missing related rooms")
	}
}

func TestHandleReadRoomMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadRoom(context.Background(), nil, ReadRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleReadRoomNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found, got: %s", text)
	}
}

// -- handleDeleteRoom --

func TestHandleDeleteRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-del", "Delete test", "", "", "", "", "")

	res, _, _ := cs.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{RoomID: "h-del"})
	text := resultText(res)
	if !strings.Contains(text, "permanently deleted") {
		t.Errorf("expected deleted, got: %s", text)
	}
}

func TestHandleDeleteRoomMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleDeleteRoom(context.Background(), nil, DeleteRoomInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing room_id")
	}
}

// -- handleSearchMessages --

func TestHandleSearchMessages(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search", "Search test", "", "", "", "", "")
	cs.postMessage("h-search", "Claude", "JWT token is broken", "thought", 0)
	cs.postMessage("h-search", "Gemini", "Agreed about JWT", "review", 0)

	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "JWT"})
	text := resultText(res)
	if !strings.Contains(text, "2 message(s)") {
		t.Errorf("expected 2 results, got: %s", text)
	}
}

func TestHandleSearchMessagesNoFilter(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for no filters, got: %s", text)
	}
}

func TestHandleSearchMessagesWithLimit(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search-lim", "Limit test", "", "", "", "", "")
	for i := 0; i < 5; i++ {
		cs.postMessage("h-search-lim", "Claude", "keyword match", "message", 0)
	}

	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query: "keyword", Limit: "2",
	})
	text := resultText(res)
	if !strings.Contains(text, "2 message(s)") {
		t.Errorf("expected 2 results with limit, got: %s", text)
	}
}

func TestHandleSearchMessagesSnippetTruncation(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search-trunc", "Trunc test", "", "", "", "", "")
	longContent := strings.Repeat("X", 500)
	cs.postMessage("h-search-trunc", "Claude", longContent, "message", 0)

	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query: "XXX",
	})
	text := resultText(res)
	if !strings.Contains(text, "...") {
		t.Error("expected truncated snippet with ...")
	}
}

// -- handleGetMessages --

func TestHandleGetMessages(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-get", "Get test", "", "", "", "", "")
	cs.postMessage("h-get", "Claude", "Full content here", "message", 0)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "1"})
	text := resultText(res)
	if !strings.Contains(text, "Full content here") {
		t.Errorf("expected full content, got: %s", text)
	}
}

func TestHandleGetMessagesByRoomID(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-get-room", "Browse test", "", "", "", "", "")
	for i := 0; i < 5; i++ {
		cs.postMessage("h-get-room", "Claude", fmt.Sprintf("Msg %d", i), "message", 0)
	}

	// Browse by room_id + last_n
	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "h-get-room", LastN: "3",
	})
	text := resultText(res)
	if !strings.Contains(text, "3 message(s)") {
		t.Errorf("expected 3 messages, got: %s", text)
	}
	if !strings.Contains(text, "Msg 4") {
		t.Error("expected last message in browse result")
	}
	if strings.Contains(text, "Msg 0") {
		t.Error("Msg 0 should not be in last 3")
	}
}

func TestHandleGetMessagesByRoomIDDefaultLimit(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-get-room-def", "Default browse", "", "", "", "", "")
	for i := 0; i < 15; i++ {
		cs.postMessage("h-get-room-def", "Claude", fmt.Sprintf("Msg %d", i), "message", 0)
	}

	// room_id without last_n should default to 10
	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "h-get-room-def",
	})
	text := resultText(res)
	if !strings.Contains(text, "10 message(s)") {
		t.Errorf("expected 10 messages with default limit, got: %s", text)
	}
}

func TestHandleGetMessagesByRoomIDNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "nonexistent",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for nonexistent room, got: %s", text)
	}
}

func TestHandleGetMessagesNoParams(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{})
	text := resultText(res)
	if !strings.Contains(text, "provide either message_ids or room_id") {
		t.Errorf("expected parameter error, got: %s", text)
	}
}

func TestHandleGetMessagesByRoomIDBadLimit(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-get-bad-lim", "Bad limit", "", "", "", "", "")
	cs.postMessage("h-get-bad-lim", "Claude", "Hello", "message", 0)

	// Invalid last_n should fall back to 10
	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		RoomID: "h-get-bad-lim", LastN: "xyz",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message with fallback limit, got: %s", text)
	}
}

func TestHandleGetMessagesIDsTakePrecedence(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-get-prio", "Priority test", "", "", "", "", "")
	cs.postMessage("h-get-prio", "Claude", "Message one", "message", 0)
	cs.postMessage("h-get-prio", "Gemini", "Message two", "review", 0)

	// If both message_ids and room_id are provided, message_ids takes precedence
	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{
		MessageIDs: "1", RoomID: "h-get-prio",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message (by ID), got: %s", text)
	}
}

func TestHandleGetMessagesMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{})
	if !strings.Contains(resultText(res), "provide either") {
		t.Error("expected error for missing params")
	}
}

func TestHandleGetMessagesInvalidID(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "abc"})
	if !strings.Contains(resultText(res), "not a valid") {
		t.Error("expected invalid ID error")
	}
}

func TestHandleGetMessagesNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{MessageIDs: "99999"})
	if !strings.Contains(resultText(res), "No messages found") {
		t.Error("expected no messages found")
	}
}

// -- handleReadRecent --

func TestHandleReadRecent(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-recent", "Recent test", "", "", "", "", "")
	cs.postMessage("h-recent", "Claude", "Message 1", "message", 0)
	cs.postMessage("h-recent", "Gemini", "Message 2", "review", 0)

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{
		RoomID: "h-recent", Limit: "1",
	})
	text := resultText(res)
	if !strings.Contains(text, "last 1 message(s)") {
		t.Errorf("expected 1 message, got: %s", text)
	}
	if !strings.Contains(text, "Message 2") {
		t.Error("expected most recent message")
	}
}

func TestHandleReadRecentMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadRecent(context.Background(), nil, ReadRecentInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing room_id")
	}
}

// -- handleRoomStats --

func TestHandleRoomStats(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-stats", "Stats test", "", "", "", "", "")
	cs.postMessage("h-stats", "Claude", "M1", "thought", 0)

	res, _, _ := cs.handleRoomStats(context.Background(), nil, RoomStatsInput{RoomID: "h-stats"})
	text := resultText(res)
	if !strings.Contains(text, "Messages:** 1") {
		t.Errorf("expected 1 message, got: %s", text)
	}
}

func TestHandleRoomStatsMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleRoomStats(context.Background(), nil, RoomStatsInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error")
	}
}

// -- handleDeleteMessages --

func TestHandleDeleteMessages(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-delmsg", "Delete msg", "", "", "", "", "")
	cs.postMessage("h-delmsg", "Claude", "Delete me", "message", 0)

	res, _, _ := cs.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{MessageIDs: "1"})
	text := resultText(res)
	if !strings.Contains(text, "Deleted 1") {
		t.Errorf("expected 1 deleted, got: %s", text)
	}
}

func TestHandleDeleteMessagesMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error")
	}
}

func TestHandleDeleteMessagesInvalidID(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleDeleteMessages(context.Background(), nil, DeleteMessagesInput{MessageIDs: "abc"})
	if !strings.Contains(resultText(res), "not a valid") {
		t.Error("expected invalid ID error")
	}
}

// -- handleArchiveRoom --

func TestHandleArchiveRoom(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-archive", "Archive test", "", "", "", "", "")
	cs.postMessage("h-archive", "Claude", "Archive me", "message", 0)

	res, _, _ := cs.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{RoomID: "h-archive"})
	text := resultText(res)
	if !strings.Contains(text, "archived") {
		t.Errorf("expected archived, got: %s", text)
	}
}

func TestHandleArchiveRoomAndDelete(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-archive-del", "Archive+delete", "", "", "", "", "")
	cs.postMessage("h-archive-del", "Claude", "Gone", "message", 0)

	res, _, _ := cs.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{
		RoomID: "h-archive-del", Delete: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "deleted") {
		t.Errorf("expected deleted, got: %s", text)
	}

	_, err := cs.getRoom("h-archive-del")
	if err == nil {
		t.Error("room should be deleted after archive+delete")
	}
}

func TestHandleArchiveRoomMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleArchiveRoom(context.Background(), nil, ArchiveRoomInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error")
	}
}

// -- handleReadTranscript --

func TestHandleReadTranscript(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript", "Transcript test", "", "", "", "", "")
	cs.postMessage("h-transcript", "Claude", "Hello world", "message", 0)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{RoomID: "h-transcript"})
	text := resultText(res)
	if !strings.Contains(text, "COUNCIL ROOM: h-transcript") {
		t.Error("missing room header")
	}
	if !strings.Contains(text, "Hello world") {
		t.Error("missing message content")
	}
}

func TestHandleReadTranscriptWithLastN(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-ln", "LastN test", "proj", "", "", "Be concise.", "")
	for i := 0; i < 10; i++ {
		cs.postMessage("h-transcript-ln", "Claude", fmt.Sprintf("Message %d", i), "message", 0)
	}

	// last_n=3 should return only the last 3 messages but still include room header and system prompt
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-ln", LastN: "3",
	})
	text := resultText(res)

	// Room header must be present
	if !strings.Contains(text, "COUNCIL ROOM: h-transcript-ln") {
		t.Error("missing room header")
	}
	// System prompt must be present
	if !strings.Contains(text, "Instructions: Be concise.") {
		t.Error("missing system prompt in last_n transcript")
	}
	// Should have Message 7, 8, 9 but not 0-6
	if !strings.Contains(text, "Message 7") {
		t.Error("expected Message 7 in last 3")
	}
	if !strings.Contains(text, "Message 9") {
		t.Error("expected Message 9 in last 3")
	}
	if strings.Contains(text, "Message 6") {
		t.Error("Message 6 should not be in last 3")
	}
	if strings.Contains(text, "Message 0") {
		t.Error("Message 0 should not be in last 3")
	}
}

func TestHandleReadTranscriptLastNPreservesSummaries(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-sum-ln", "Summary+LastN", "", "", "", "", "")

	for i := 0; i < 5; i++ {
		cs.postMessage("h-transcript-sum-ln", "Claude", "Old msg", "message", 0)
	}
	cs.insertSummary("h-transcript-sum-ln", "Summary of 5 old messages")
	for i := 0; i < 5; i++ {
		cs.postMessage("h-transcript-sum-ln", "Gemini", fmt.Sprintf("New msg %d", i), "message", 0)
	}

	// last_n=2 should keep summaries + last 2 regular messages
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-sum-ln", LastN: "2",
	})
	text := resultText(res)

	if !strings.Contains(text, "SUMMARY") {
		t.Error("summary should be preserved with last_n")
	}
	if !strings.Contains(text, "New msg 4") {
		t.Error("expected last message in last_n=2")
	}
	if !strings.Contains(text, "New msg 3") {
		t.Error("expected second-to-last message in last_n=2")
	}
	if strings.Contains(text, "New msg 0") {
		t.Error("New msg 0 should not be in last_n=2")
	}
}

func TestHandleReadTranscriptLastNOmittedReturnsAll(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-all", "All messages", "", "", "", "", "")
	for i := 0; i < 5; i++ {
		cs.postMessage("h-transcript-all", "Claude", fmt.Sprintf("Msg %d", i), "message", 0)
	}

	// No last_n — should return all 5 messages
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-all",
	})
	text := resultText(res)
	if !strings.Contains(text, "Msg 0") || !strings.Contains(text, "Msg 4") {
		t.Error("expected all messages when last_n is omitted")
	}
}

func TestHandleReadTranscriptLastNInvalid(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-bad-ln", "Bad last_n", "", "", "", "", "")
	for i := 0; i < 3; i++ {
		cs.postMessage("h-transcript-bad-ln", "Claude", fmt.Sprintf("Msg %d", i), "message", 0)
	}

	// Invalid last_n should return all (graceful fallback)
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-bad-ln", LastN: "abc",
	})
	text := resultText(res)
	if !strings.Contains(text, "Msg 0") || !strings.Contains(text, "Msg 2") {
		t.Error("invalid last_n should return all messages")
	}
}

func TestHandleReadTranscriptLastNLargerThanTotal(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-big-ln", "Big last_n", "", "", "", "", "")
	cs.postMessage("h-transcript-big-ln", "Claude", "Only message", "message", 0)

	// last_n=100 with only 1 message should return all
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-big-ln", LastN: "100",
	})
	text := resultText(res)
	if !strings.Contains(text, "Only message") {
		t.Error("expected the single message")
	}
}

// -- read_transcript after_id --

func TestHandleReadTranscriptAfterID(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-after", "AfterID test", "", "", "", "", "")
	cs.postMessage("h-transcript-after", "Claude", "First", "message", 0)
	id2, _ := cs.postMessage("h-transcript-after", "Gemini", "Second", "thought", 0)
	cs.postMessage("h-transcript-after", "Claude", "Third", "decision", 0)

	// Get messages after the second one — should return only "Third"
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-after", AfterID: fmt.Sprintf("%d", id2),
	})
	text := resultText(res)

	if !strings.Contains(text, "1 message(s) after") {
		t.Errorf("expected 1 message after header, got: %s", text)
	}
	if !strings.Contains(text, "Third") {
		t.Error("expected 'Third' in after_id result")
	}
	if strings.Contains(text, "First") || strings.Contains(text, "Second") {
		t.Error("should not contain messages before after_id")
	}
}

func TestHandleReadTranscriptAfterIDNoNewMessages(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-after-empty", "No new", "", "", "", "", "")
	id1, _ := cs.postMessage("h-transcript-after-empty", "Claude", "Only one", "message", 0)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-after-empty", AfterID: fmt.Sprintf("%d", id1),
	})
	text := resultText(res)
	if !strings.Contains(text, "0 message(s) after") {
		t.Errorf("expected 0 messages, got: %s", text)
	}
}

func TestHandleReadTranscriptAfterIDInvalid(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-after-bad", "Bad after_id", "", "", "", "", "")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-after-bad", AfterID: "not-a-number",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for invalid after_id, got: %s", text)
	}
}

func TestHandleReadTranscriptAfterIDWithTypedMessages(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-after-typed", "Typed after", "", "", "", "", "")
	id1, _ := cs.postMessage("h-transcript-after-typed", "Claude", "Base", "message", 0)
	cs.postMessage("h-transcript-after-typed", "Gemini", "A thought", "thought", 0)
	cs.postMessage("h-transcript-after-typed", "Claude", "A decision", "decision", id1)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-after-typed", AfterID: fmt.Sprintf("%d", id1),
	})
	text := resultText(res)
	if !strings.Contains(text, "thought") {
		t.Error("expected thought type in after_id result")
	}
	if !strings.Contains(text, "decision") {
		t.Error("expected decision type in after_id result")
	}
}

// -- read_transcript mode=summary --

func TestHandleReadTranscriptSummaryMode(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-summode", "Summary mode test", "", "", "", "Focus on security.", "")
	cs.postMessage("h-transcript-summode", "Claude", "Old thought", "thought", 0)
	cs.postMessage("h-transcript-summode", "Claude", "Latest thought", "thought", 0)
	cs.postMessage("h-transcript-summode", "Gemini", "A decision was made", "decision", 0)
	cs.postMessage("h-transcript-summode", "Claude", "Do this action", "action", 0)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-summode", Mode: "summary",
	})
	text := resultText(res)

	// Should include system prompt
	if !strings.Contains(text, "Focus on security.") {
		t.Error("summary mode should include system prompt")
	}
	// Should have summary header
	if !strings.Contains(text, "summary") {
		t.Error("expected summary header")
	}
	// Should include latest per type, not the old thought
	if !strings.Contains(text, "Latest thought") {
		t.Error("expected latest thought")
	}
	if strings.Contains(text, "Old thought") {
		t.Error("should not contain old thought, only latest per type")
	}
	if !strings.Contains(text, "A decision was made") {
		t.Error("expected the decision")
	}
	if !strings.Contains(text, "Do this action") {
		t.Error("expected the action")
	}
}

func TestHandleReadTranscriptSummaryModeEmpty(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-summode-empty", "Empty summary", "", "", "", "", "")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-summode-empty", Mode: "summary",
	})
	text := resultText(res)
	if !strings.Contains(text, "No messages yet") {
		t.Errorf("expected 'No messages yet', got: %s", text)
	}
}

func TestHandleReadTranscriptSummaryModeTruncatesLongContent(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-transcript-summode-long", "Long summary", "", "", "", "", "")
	longContent := strings.Repeat("X", 300)
	cs.postMessage("h-transcript-summode-long", "Claude", longContent, "thought", 0)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-summode-long", Mode: "summary",
	})
	text := resultText(res)
	if !strings.Contains(text, "...") {
		t.Error("expected truncated content in summary mode")
	}
	if strings.Contains(text, strings.Repeat("X", 300)) {
		t.Error("full 300-char content should not appear in summary mode")
	}
}

// -- search_messages summary_only --

func TestHandleSearchMessagesSummaryOnly(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search-sumonly", "SummaryOnly test", "", "", "", "", "")
	longContent := "This is a long message " + strings.Repeat("that goes on and on ", 10)
	cs.postMessage("h-search-sumonly", "Claude", longContent, "thought", 0)
	cs.postMessage("h-search-sumonly", "Gemini", "Short reply", "review", 0)

	res, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-sumonly", SummaryOnly: "true",
	})
	text := resultText(res)

	// Should be compact pipe-separated format
	if !strings.Contains(text, "|") {
		t.Error("expected pipe-separated compact output")
	}
	// Content should be truncated at 120 chars
	if strings.Contains(text, strings.Repeat("that goes on and on ", 10)) {
		t.Error("full content should not appear in summary_only mode")
	}
	// Should still show results
	if !strings.Contains(text, "2 message(s)") {
		t.Errorf("expected 2 messages, got: %s", text)
	}
}

func TestHandleSearchMessagesSummaryOnlyVsDefault(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("h-search-compare", "Compare test", "", "", "", "", "")
	cs.postMessage("h-search-compare", "Claude", "Test content here", "message", 0)

	// Default (non-summary) should use bold formatting
	res1, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-compare",
	})
	text1 := resultText(res1)

	// Summary mode should NOT use bold formatting
	res2, _, _ := cs.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-compare", SummaryOnly: "true",
	})
	text2 := resultText(res2)

	if !strings.Contains(text1, "**#") {
		t.Error("default mode should use bold formatting")
	}
	if strings.Contains(text2, "**#") {
		t.Error("summary_only mode should not use bold formatting")
	}
}

// -- bulk_status_update --

func TestHandleBulkStatusUpdate(t *testing.T) {
	cs := setupTestServer(t)
	registerTools(cs)
	cs.createRoom("bulk-1", "Room 1", "", "", "", "", "")
	cs.createRoom("bulk-2", "Room 2", "", "", "", "", "")
	cs.createRoom("bulk-3", "Room 3", "", "", "", "", "")

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "bulk-1,bulk-2,bulk-3", Status: "resolved",
	})
	text := resultText(res)

	if !strings.Contains(text, "Updated 3 room(s)") {
		t.Errorf("expected 3 updated, got: %s", text)
	}
	if !strings.Contains(text, "resolved") {
		t.Error("expected 'resolved' in result")
	}

	// Verify all rooms are actually resolved
	for _, id := range []string{"bulk-1", "bulk-2", "bulk-3"} {
		room, _ := cs.getRoom(id)
		if room.Status != "resolved" {
			t.Errorf("room '%s' should be resolved, got '%s'", id, room.Status)
		}
	}
}

func TestHandleBulkStatusUpdatePartialFailure(t *testing.T) {
	cs := setupTestServer(t)
	cs.createRoom("bulk-ok", "Exists", "", "", "", "", "")

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "bulk-ok,nonexistent-room", Status: "paused",
	})
	text := resultText(res)

	if !strings.Contains(text, "Updated 1 room(s)") {
		t.Errorf("expected 1 updated, got: %s", text)
	}
	if !strings.Contains(text, "Not found: nonexistent-room") {
		t.Errorf("expected not found for nonexistent room, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateInvalidStatus(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "x", Status: "invalid",
	})
	text := resultText(res)
	if !strings.Contains(text, "Invalid status") {
		t.Errorf("expected invalid status error, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateMissingIDs(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "", Status: "active",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing room_ids, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateEmptyIDs(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: ",,,", Status: "active",
	})
	text := resultText(res)
	if !strings.Contains(text, "No valid room IDs") {
		t.Errorf("expected no valid IDs message, got: %s", text)
	}
}

func TestHandleReadTranscriptMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error")
	}
}

func TestHandleReadTranscriptNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{RoomID: "ghost"})
	if !strings.Contains(resultText(res), "not found") {
		t.Error("expected not found")
	}
}
