package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// setupIntegrationTest creates a Registry with all tools registered, then wires up an
// in-memory MCP client-server pair so tests can exercise the full dispatch path:
// JSON arguments → schema validation → deserialization → handler → response.
func setupIntegrationTest(t *testing.T) (*mcp.ClientSession, *Registry) {
	t.Helper()

	s := setupTestServer(t)
	reg := &Registry{Server: s}
	reg.RegisterTools()
	reg.RegisterResources()

	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()

	if _, err := s.MCP.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })

	return cs, reg
}

// callTool calls a tool through the MCP dispatch path and returns the result.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return result
}

// ========== integration tests (one per tool, exercising full MCP dispatch) ==========

func TestIntegration_CreateRoom(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	result := callTool(t, cs, "create_room", map[string]any{
		"id": "integ-create", "topic": "Integration test room",
	})
	if !strings.Contains(resultText(result), "created") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_GetOrCreateRoom(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	result := callTool(t, cs, "get_or_create_room", map[string]any{
		"id": "integ-get-or-create", "topic": "test",
	})
	text := resultText(result)
	if !strings.Contains(text, "integ-get-or-create") {
		t.Errorf("unexpected: %s", text)
	}
}

func TestIntegration_PostToRoom(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-post")
	result := callTool(t, cs, "post_to_room", map[string]any{
		"room_id": "integ-post",
		"author":  "TestAgent",
		"message": "Hello from integration test",
	})
	if !strings.Contains(resultText(result), "integ-post") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_SignalStatus(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-status")
	result := callTool(t, cs, "signal_status", map[string]any{
		"room_id": "integ-status",
		"status":  "paused",
	})
	if !strings.Contains(resultText(result), "paused") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_BulkStatusUpdate(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-bulk-a")
	mustCreateRoom(t, reg.Server, "integ-bulk-b")
	result := callTool(t, cs, "bulk_status_update", map[string]any{
		"room_ids": "integ-bulk-a,integ-bulk-b",
		"status":   "resolved",
	})
	if !strings.Contains(resultText(result), "resolved") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_ListRooms(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-list")
	result := callTool(t, cs, "list_rooms", map[string]any{})
	if !strings.Contains(resultText(result), "integ-list") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_UpdateRoom(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-update")
	result := callTool(t, cs, "update_room", map[string]any{
		"room_id": "integ-update",
		"topic":   "Updated topic",
	})
	if !strings.Contains(resultText(result), "updated") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_ReadRoom(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-read-room", withDescription("Read room test"))
	result := callTool(t, cs, "read_room", map[string]any{
		"room_id": "integ-read-room",
	})
	if !strings.Contains(resultText(result), "integ-read-room") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_DeleteRoom(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-delete")
	result := callTool(t, cs, "delete_room", map[string]any{
		"room_id": "integ-delete",
	})
	if !strings.Contains(resultText(result), "deleted") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_SearchMessages(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-search")
	mustPost(t, reg.Server, "integ-search", "Claude", "uniqueintegrationterm")
	result := callTool(t, cs, "search_messages", map[string]any{
		"query": "uniqueintegrationterm",
	})
	if !strings.Contains(resultText(result), "uniqueintegrationterm") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_GetMessages(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-get-msgs")
	mustPost(t, reg.Server, "integ-get-msgs", "Claude", "hello")
	result := callTool(t, cs, "get_messages", map[string]any{
		"room_id": "integ-get-msgs",
		"last_n":  "5",
	})
	if !strings.Contains(resultText(result), "hello") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_RoomStats(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-stats")
	mustPost(t, reg.Server, "integ-stats", "Claude", "msg")
	result := callTool(t, cs, "room_stats", map[string]any{
		"room_id": "integ-stats",
	})
	if !strings.Contains(resultText(result), "integ-stats") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_UpdateMessage(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-update-msg")
	id := mustPost(t, reg.Server, "integ-update-msg", "Claude", "original")
	result := callTool(t, cs, "update_message", map[string]any{
		"message_id": id,
		"content":    "updated content",
	})
	if !strings.Contains(resultText(result), "updated") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_PinMessage(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-pin")
	id := mustPost(t, reg.Server, "integ-pin", "Claude", "pin me")
	result := callTool(t, cs, "pin_message", map[string]any{
		"message_id": id,
		"room_id":    "integ-pin",
	})
	if !strings.Contains(resultText(result), "integ-pin") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_DeleteMessages(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-del-msgs")
	id := mustPost(t, reg.Server, "integ-del-msgs", "Claude", "delete me")
	result := callTool(t, cs, "delete_messages", map[string]any{
		"message_ids": id,
	})
	if !strings.Contains(strings.ToLower(resultText(result)), "deleted") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_ArchiveRoom(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-archive")
	mustPost(t, reg.Server, "integ-archive", "Claude", "archive content")
	result := callTool(t, cs, "archive_room", map[string]any{
		"room_id": "integ-archive",
	})
	if !strings.Contains(resultText(result), "archived") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_ReadTranscript(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-transcript")
	mustPost(t, reg.Server, "integ-transcript", "Claude", "transcript content")
	result := callTool(t, cs, "read_transcript", map[string]any{
		"room_id": "integ-transcript",
	})
	if !strings.Contains(resultText(result), "transcript content") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_GetDigest(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-digest", withProject("integ-proj"))
	mustPost(t, reg.Server, "integ-digest", "Claude", "digest content")
	result := callTool(t, cs, "get_digest", map[string]any{
		"since":   "2020-01-01T00:00:00",
		"project": "integ-proj",
	})
	if !strings.Contains(resultText(result), "integ-digest") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_ListArchives(t *testing.T) {
	cs, _ := setupIntegrationTest(t)
	// No archives — should return empty message without error.
	result := callTool(t, cs, "list_archives", map[string]any{})
	text := resultText(result)
	// Either "No archives found" or a listing — either way no error.
	if text == "" {
		t.Error("expected non-empty response from list_archives")
	}
}

func TestIntegration_ReadArchive(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-read-archive")
	mustPost(t, reg.Server, "integ-read-archive", "Claude", "archived message")
	callTool(t, cs, "archive_room", map[string]any{"room_id": "integ-read-archive"})

	result := callTool(t, cs, "read_archive", map[string]any{
		"room_id": "integ-read-archive",
	})
	if !strings.Contains(resultText(result), "archived message") {
		t.Errorf("unexpected: %s", resultText(result))
	}
}

func TestIntegration_GetMessagesAfterID(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-after-id")
	firstID := mustPost(t, reg.Server, "integ-after-id", "Claude", "first message")
	mustPost(t, reg.Server, "integ-after-id", "Claude", "second message")

	result := callTool(t, cs, "get_messages", map[string]any{
		"room_id":  "integ-after-id",
		"after_id": firstID,
	})
	text := resultText(result)
	if !strings.Contains(text, "second message") {
		t.Errorf("expected second message in after_id result: %s", text)
	}
	if strings.Contains(text, "first message") {
		t.Errorf("should not contain first message in after_id result: %s", text)
	}
}

func TestIntegration_ReadTranscriptWorkItems(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-work-items")
	mustPostTyped(t, reg.Server, "integ-work-items", "Claude", "thought content", "thought")
	mustPostTyped(t, reg.Server, "integ-work-items", "Claude", "deploy the service", "action")
	mustPostTyped(t, reg.Server, "integ-work-items", "Claude", "use postgres not sqlite", "decision")

	result := callTool(t, cs, "read_transcript", map[string]any{
		"room_id": "integ-work-items",
		"mode":    "work_items",
	})
	text := resultText(result)
	if !strings.Contains(text, "deploy the service") {
		t.Errorf("expected action in work_items: %s", text)
	}
	if !strings.Contains(text, "use postgres not sqlite") {
		t.Errorf("expected decision in work_items: %s", text)
	}
	if strings.Contains(text, "thought content") {
		t.Errorf("should not contain thought in work_items: %s", text)
	}
}

func TestIntegration_ArchiveRoomEpitaph(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	mustCreateRoom(t, reg.Server, "integ-epitaph")
	mustPostTyped(t, reg.Server, "integ-epitaph", "Claude", "decided to use Redis", "decision")
	mustPostTyped(t, reg.Server, "integ-epitaph", "Claude", "deployed to production", "action")

	callTool(t, cs, "archive_room", map[string]any{"room_id": "integ-epitaph"})

	result := callTool(t, cs, "read_archive", map[string]any{"room_id": "integ-epitaph"})
	text := resultText(result)
	if !strings.Contains(text, "## Summary") {
		t.Errorf("expected epitaph ## Summary block in archive: %s", text)
	}
	if !strings.Contains(text, "decided to use Redis") {
		t.Errorf("expected last decision in epitaph: %s", text)
	}
	if !strings.Contains(text, "deployed to production") {
		t.Errorf("expected last action in epitaph: %s", text)
	}
}

func TestIntegration_KnowledgeLint(t *testing.T) {
	cs, reg := setupIntegrationTest(t)

	// Room with a decision but no synthesis — should be flagged
	mustCreateRoom(t, reg.Server, "integ-lint-flag")
	mustPostTyped(t, reg.Server, "integ-lint-flag", "Claude", "We chose Postgres", "decision")

	// Room with a decision AND synthesis — should NOT be flagged
	mustCreateRoom(t, reg.Server, "integ-lint-ok")
	mustPostTyped(t, reg.Server, "integ-lint-ok", "Claude", "We chose Redis", "decision")
	mustPostTyped(t, reg.Server, "integ-lint-ok", "Claude", "Compiled: Redis chosen for caching", "synthesis")

	result := callTool(t, cs, "knowledge_lint", map[string]any{})
	text := resultText(result)

	if !strings.Contains(text, "integ-lint-flag") {
		t.Errorf("expected integ-lint-flag in linter results: %s", text)
	}
	if strings.Contains(text, "integ-lint-ok") {
		t.Errorf("integ-lint-ok should not be flagged (has synthesis): %s", text)
	}
	if !strings.Contains(text, "Needs synthesis") {
		t.Errorf("expected 'Needs synthesis' label in output: %s", text)
	}
}

func TestIntegration_KnowledgeLintAllClear(t *testing.T) {
	cs, _ := setupIntegrationTest(t)

	// No rooms with decisions — should report all clear
	result := callTool(t, cs, "knowledge_lint", map[string]any{})
	text := resultText(result)

	if !strings.Contains(text, "All clear") {
		t.Errorf("expected 'All clear' with no rooms, got: %s", text)
	}
}
