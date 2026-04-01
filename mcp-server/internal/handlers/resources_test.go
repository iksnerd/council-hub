package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandleTranscriptResource(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.RegisterResources()
	mustCreateRoom(t, reg.Server, "res-room", withProject("proj"), withTechStack("Go"), withTags("tag"), withSystemPrompt("Be helpful"), withRelatedRooms("related-a"))
	mustPostTyped(t, reg.Server, "res-room", "Claude", "Hello", "thought")

	result, err := reg.handleTranscript(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "council://room/res-room/transcript"},
	})
	if err != nil {
		t.Fatalf("handleTranscript error: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Contents))
	}
	text := result.Contents[0].Text
	if !strings.Contains(text, "COUNCIL ROOM: res-room") {
		t.Error("missing room header")
	}
	if !strings.Contains(text, "Hello") {
		t.Error("missing message content")
	}
}

func TestHandleTranscriptResourceNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	result, err := reg.handleTranscript(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "council://room/nonexistent/transcript"},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	text := result.Contents[0].Text
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found message, got: %s", text)
	}
}

func TestHandleTranscriptResourceEmptyURI(t *testing.T) {
	reg := setupHandlerTest(t)

	_, err := reg.handleTranscript(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "council://room//transcript"},
	})
	if err == nil {
		t.Fatal("expected error for empty room_id")
	}
}
