package handlers

import (
	"context"
	"strings"
	"testing"
)

func TestMarkReadWarnsOnSubstringNeighbour(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "mr-room")
	id := mustPost(t, reg.Server, "mr-room", "x", "hi")
	if err := reg.Server.MarkRead("claude-code", "mr-room", id); err != nil {
		t.Fatal(err)
	}
	res, _, err := reg.handleMarkRead(context.Background(), nil, MarkReadInput{RoomID: "mr-room", Cursor: id, Agent: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(res)
	if !strings.Contains(text, "claude-code") || !strings.Contains(strings.ToLower(text), "substring") {
		t.Errorf("expected neighbour warning, got: %s", text)
	}
	// Second call by the same, now-established agent stays quiet.
	res, _, _ = reg.handleMarkRead(context.Background(), nil, MarkReadInput{RoomID: "mr-room", Cursor: id, Agent: "claude"})
	if strings.Contains(strings.ToLower(resultText(res)), "substring") {
		t.Errorf("warning should fire once per new name: %s", resultText(res))
	}
}

func TestMarkReadNoWarningForDistinctNames(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "mr-room2")
	id := mustPost(t, reg.Server, "mr-room2", "x", "hi")
	_ = reg.Server.MarkRead("gemini", "mr-room2", id)
	res, _, _ := reg.handleMarkRead(context.Background(), nil, MarkReadInput{RoomID: "mr-room2", Cursor: id, Agent: "claude"})
	if strings.Contains(strings.ToLower(resultText(res)), "substring") {
		t.Errorf("unexpected warning: %s", resultText(res))
	}
}

func TestMarkReadAllRooms(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "mra-a")
	mustCreateRoom(t, reg.Server, "mra-b")
	mustCreateRoom(t, reg.Server, "mra-empty")
	mustPost(t, reg.Server, "mra-a", "x", "old")
	latestA := mustPost(t, reg.Server, "mra-a", "x", "newer")
	latestB := mustPost(t, reg.Server, "mra-b", "x", "hi")

	res, _, err := reg.handleMarkRead(context.Background(), nil, MarkReadInput{RoomID: "*", Agent: "newbie"})
	if err != nil {
		t.Fatal(err)
	}
	text := resultText(res)
	if !strings.Contains(text, "2 rooms") || !strings.Contains(text, "1 had no messages") {
		t.Errorf("unexpected reply: %s", text)
	}
	if c, _ := reg.Server.GetCursor("newbie", "mra-a"); c != latestA {
		t.Errorf("cursor a = %q, want %q", c, latestA)
	}
	if c, _ := reg.Server.GetCursor("newbie", "mra-b"); c != latestB {
		t.Errorf("cursor b = %q, want %q", c, latestB)
	}

	// Digest is empty right after.
	res, _, _ = reg.handleGetDigest(context.Background(), nil, DigestInput{UnreadOnly: "true", Agent: "newbie"})
	if !strings.Contains(resultText(res), "No unread rooms") {
		t.Errorf("expected empty unread digest, got: %s", resultText(res))
	}

	// A new post shows up, and only that room.
	mustPost(t, reg.Server, "mra-b", "x", "fresh news")
	res, _, _ = reg.handleGetDigest(context.Background(), nil, DigestInput{UnreadOnly: "true", Agent: "newbie"})
	text = resultText(res)
	if !strings.Contains(text, "mra-b") || strings.Contains(text, "mra-a") {
		t.Errorf("expected only mra-b unread, got: %s", text)
	}
}

func TestMarkReadAllRoomsRejectsCursor(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "mra-c")
	id := mustPost(t, reg.Server, "mra-c", "x", "hi")
	res, _, _ := reg.handleMarkRead(context.Background(), nil, MarkReadInput{RoomID: "*", Cursor: id, Agent: "newbie"})
	text := resultText(res)
	if !strings.HasPrefix(text, "Error:") || !strings.Contains(text, "not allowed") {
		t.Errorf("expected cursor rejection, got: %s", text)
	}
	if c, _ := reg.Server.GetCursor("newbie", "mra-c"); c != "" {
		t.Errorf("no cursor should be written, got %q", c)
	}
}

func TestMarkReadSingleRoomStillRequiresCursor(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "mra-d")
	res, _, _ := reg.handleMarkRead(context.Background(), nil, MarkReadInput{RoomID: "mra-d", Agent: "newbie"})
	if !strings.Contains(resultText(res), "cursor (message ID) is required") {
		t.Errorf("got: %s", resultText(res))
	}
	id := mustPost(t, reg.Server, "mra-d", "x", "hi")
	res, _, _ = reg.handleMarkRead(context.Background(), nil, MarkReadInput{RoomID: "mra-d", Cursor: id, Agent: "newbie"})
	if !strings.Contains(resultText(res), "Cursor saved for agent 'newbie' in room 'mra-d'") {
		t.Errorf("single-room reply changed: %s", resultText(res))
	}
}
