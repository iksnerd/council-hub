package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ========== load_resources tool handler ==========

type loadResourcesArgs = struct {
	URI string `json:"uri"`
}

func TestHandleLoadResourcesNoURI(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleLoadResources(context.Background(), nil, loadResourcesArgs{})
	if err != nil {
		t.Fatalf("handleLoadResources error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "Available Resources") {
		t.Errorf("expected resource listing, got: %s", text)
	}
	if !strings.Contains(text, "council://guide") {
		t.Errorf("expected council://guide in listing, got: %s", text)
	}
	if !strings.Contains(text, "council://message-types") {
		t.Errorf("expected council://message-types in listing, got: %s", text)
	}
	if !strings.Contains(text, "council://workflows") {
		t.Errorf("expected council://workflows in listing, got: %s", text)
	}
	if !strings.Contains(text, "council://janitor") {
		t.Errorf("expected council://janitor in listing, got: %s", text)
	}
	if !strings.Contains(text, "council://room/{room_id}/transcript") {
		t.Errorf("expected dynamic transcript template in listing, got: %s", text)
	}
}

func TestHandleLoadResourcesGuide(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleLoadResources(context.Background(), nil, loadResourcesArgs{URI: "council://guide"})
	text := resultText(res)
	if !strings.Contains(text, "Usage Guide") && !strings.Contains(text, "Council Hub") {
		t.Errorf("expected usage guide content, got: %s", text)
	}
}

func TestHandleLoadResourcesMessageTypes(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleLoadResources(context.Background(), nil, loadResourcesArgs{URI: "council://message-types"})
	text := resultText(res)
	if text == "" {
		t.Error("expected non-empty message types content")
	}
}

func TestHandleLoadResourcesWorkflows(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleLoadResources(context.Background(), nil, loadResourcesArgs{URI: "council://workflows"})
	text := resultText(res)
	if text == "" {
		t.Error("expected non-empty workflows content")
	}
}

func TestHandleLoadResourcesJanitor(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleLoadResources(context.Background(), nil, loadResourcesArgs{URI: "council://janitor"})
	text := resultText(res)
	if !strings.Contains(text, "Janitor") || !strings.Contains(text, "needs-synthesis") {
		t.Errorf("expected janitor playbook content, got: %s", text)
	}
}

func TestHandleLoadResourcesInvalidURI(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleLoadResources(context.Background(), nil, loadResourcesArgs{URI: "council://nonexistent"})
	if !res.IsError {
		t.Error("expected IsError=true for unknown URI")
	}
	text := resultText(res)
	if !strings.Contains(text, "Unknown resource URI") {
		t.Errorf("expected unknown resource error message, got: %s", text)
	}
}

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
