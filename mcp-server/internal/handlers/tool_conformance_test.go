package handlers

import (
	"context"
	"testing"
)

// drenskicorp RFC 0013 section 7: an argument problem over MCP is a tool
// execution error (isError: true), so a client can tell failure from success
// without reading the text.
func TestErrorResultsAreFlaggedIsError(t *testing.T) {
	res, _, _ := textResult("Error: room_id is required")
	if !res.IsError {
		t.Error("an Error: result must set IsError")
	}
	ok, _, _ := textResult("Posted to room 'x'")
	if ok.IsError {
		t.Error("a success result must not set IsError")
	}
}

func TestIntegration_BadMessageTypeIsToolError(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	callTool(t, cs, "create_room", map[string]any{"id": "iserr", "topic": "x"})
	res := callTool(t, cs, "post_to_room", map[string]any{"room_id": "iserr", "author": "a", "message": "m", "message_type": "bogus"})
	if !res.IsError {
		t.Errorf("invalid message_type should be a tool error: %s", resultText(res))
	}
}

// RFC 0013 section 3: title is optional, but every tool here has one so a client
// can show a name for people rather than the snake_case identifier.
func TestEveryToolHasATitle(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range list.Tools {
		if tool.Title == "" {
			t.Errorf("tool %s has no title", tool.Name)
		}
	}
}
