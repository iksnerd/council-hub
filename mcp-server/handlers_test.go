package main

import (
	"context"
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

func TestHandleGetMessagesMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleGetMessages(context.Background(), nil, GetMessagesInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error for missing IDs")
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
