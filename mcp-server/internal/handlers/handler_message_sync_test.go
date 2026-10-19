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
