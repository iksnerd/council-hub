package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ========== bulk_visibility ==========

func TestHandleBulkVisibilityAll(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "v1")
	mustCreateRoom(t, reg.Server, "v2")

	res, _, _ := reg.handleBulkVisibility(context.Background(), nil, BulkVisibilityInput{
		Visibility: "private", All: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "Set 2 room(s) to **private**") {
		t.Errorf("expected 2 rooms set private, got: %s", text)
	}
	for _, id := range []string{"v1", "v2"} {
		room, _ := reg.Server.GetRoom(id)
		if room.Visibility != "private" {
			t.Errorf("room '%s' should be private, got '%s'", id, room.Visibility)
		}
	}
}

func TestHandleBulkVisibilityByProject(t *testing.T) {
	reg := setupHandlerTest(t)
	_ = reg.Server.CreateRoom("p1", "r", "alpha", "", "", "", "")
	_ = reg.Server.CreateRoom("p2", "r", "beta", "", "", "", "")

	res, _, _ := reg.handleBulkVisibility(context.Background(), nil, BulkVisibilityInput{
		Visibility: "private", Project: "alpha",
	})
	if !strings.Contains(resultText(res), "project 'alpha'") {
		t.Errorf("unexpected message: %s", resultText(res))
	}
	if r, _ := reg.Server.GetRoom("p1"); r.Visibility != "private" {
		t.Errorf("p1 should be private, got %s", r.Visibility)
	}
	if r, _ := reg.Server.GetRoom("p2"); r.Visibility != "public" {
		t.Errorf("p2 should stay public, got %s", r.Visibility)
	}
}

func TestHandleBulkVisibilityValidation(t *testing.T) {
	reg := setupHandlerTest(t)

	// Bad visibility value.
	res, _, _ := reg.handleBulkVisibility(context.Background(), nil, BulkVisibilityInput{
		Visibility: "secret", All: "true",
	})
	if !strings.Contains(resultText(res), "must be 'public' or 'private'") {
		t.Errorf("expected visibility validation error, got: %s", resultText(res))
	}

	// No target.
	res, _, _ = reg.handleBulkVisibility(context.Background(), nil, BulkVisibilityInput{
		Visibility: "private",
	})
	if !strings.Contains(resultText(res), "exactly one target") {
		t.Errorf("expected no-target error, got: %s", resultText(res))
	}

	// More than one target.
	res, _, _ = reg.handleBulkVisibility(context.Background(), nil, BulkVisibilityInput{
		Visibility: "private", All: "true", Project: "alpha",
	})
	if !strings.Contains(resultText(res), "only one target") {
		t.Errorf("expected multi-target error, got: %s", resultText(res))
	}
}

// ========== bulk_status_update ==========

func TestHandleBulkStatusUpdate(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.RegisterTools()
	mustCreateRoom(t, reg.Server, "bulk-1")
	mustCreateRoom(t, reg.Server, "bulk-2")
	mustCreateRoom(t, reg.Server, "bulk-3")

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "bulk-1,bulk-2,bulk-3", Status: "resolved",
	})
	text := resultText(res)

	if !strings.Contains(text, "Updated 3 room(s)") {
		t.Errorf("expected 3 updated, got: %s", text)
	}
	for _, id := range []string{"bulk-1", "bulk-2", "bulk-3"} {
		room, _ := reg.Server.GetRoom(id)
		if room.Status != "resolved" {
			t.Errorf("room '%s' should be resolved, got '%s'", id, room.Status)
		}
	}
}

func TestHandleBulkStatusUpdatePartialFailure(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "bulk-ok")

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
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
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "x", Status: "invalid",
	})
	text := resultText(res)
	if !strings.Contains(text, "Invalid status") {
		t.Errorf("expected invalid status error, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateMissingIDs(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "", Status: "active",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing room_ids, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateEmptyIDs(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: ",,,", Status: "active",
	})
	text := resultText(res)
	if !strings.Contains(text, "No valid room IDs") {
		t.Errorf("expected no valid IDs message, got: %s", text)
	}
}

func TestHandleBulkStatusUpdateWithMessage(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "bulk-msg-1")
	mustCreateRoom(t, reg.Server, "bulk-msg-2")

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "bulk-msg-1,bulk-msg-2",
		Status:  "resolved",
		Message: "Closed: all issues fixed in PR #42.",
		Author:  "Claude",
	})
	text := resultText(res)
	if !strings.Contains(text, "Updated 2 room(s)") {
		t.Errorf("expected 2 updated, got: %s", text)
	}

	msgs1, _ := reg.Server.GetRecentMessages("bulk-msg-1", 1)
	if len(msgs1) != 1 || !strings.Contains(msgs1[0].Content, "PR #42") {
		t.Error("expected closing message in room 1")
	}
	if msgs1[0].MessageType != "decision" {
		t.Errorf("expected decision type, got '%s'", msgs1[0].MessageType)
	}
}

func TestHandleBulkStatusUpdateMessageWithoutAuthor(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "x", Status: "resolved", Message: "Closing",
	})
	text := resultText(res)
	if !strings.Contains(text, "author is required") {
		t.Errorf("expected author required error, got: %s", text)
	}
}

// ========== get_or_create_room ==========

func TestHandleGetOrCreateRoomNew(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-new", Topic: "New room", Project: "proj", SystemPrompt: "Be helpful.",
	})
	text := resultText(res)

	if !strings.Contains(text, "Created") {
		t.Errorf("expected 'Created', got: %s", text)
	}
	if !strings.Contains(text, "Be helpful.") {
		t.Error("expected system prompt in response")
	}

	room, err := reg.Server.GetRoom("upsert-new")
	if err != nil {
		t.Fatalf("room should exist: %v", err)
	}
	if room.Project != "proj" {
		t.Errorf("expected project 'proj', got '%s'", room.Project)
	}
}

func TestHandleGetOrCreateRoomExisting(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "upsert-existing", withDescription("Already here"), withProject("proj"), withSystemPrompt("Prompt text."))
	mustPostTyped(t, reg.Server, "upsert-existing", "Claude", "First message", "decision")
	mustPostTyped(t, reg.Server, "upsert-existing", "Gemini", "Second message", "action")

	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-existing",
	})
	text := resultText(res)

	if !strings.Contains(text, "Found") {
		t.Errorf("expected 'Found', got: %s", text)
	}
	if !strings.Contains(text, "Recent messages") {
		t.Error("expected recent messages for existing room")
	}
	if !strings.Contains(text, "First message") || !strings.Contains(text, "Second message") {
		t.Error("expected both messages in response")
	}
}

func TestHandleGetOrCreateRoomExistingEmpty(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "upsert-empty")

	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-empty",
	})
	text := resultText(res)
	if !strings.Contains(text, "Found") {
		t.Error("expected Found")
	}
	if !strings.Contains(text, "No messages yet") {
		t.Errorf("expected 'No messages yet', got: %s", text)
	}
}

func TestHandleGetOrCreateRoomMissingID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing id, got: %s", text)
	}
}

func TestHandleGetOrCreateRoomCustomLastN(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "upsert-lastn")
	for i := 0; i < 10; i++ {
		mustPost(t, reg.Server, "upsert-lastn", "Claude", fmt.Sprintf("Msg %d", i))
	}

	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-lastn", LastN: "2",
	})
	text := resultText(res)
	if !strings.Contains(text, "Recent messages (2)") {
		t.Errorf("expected 2 recent messages, got: %s", text)
	}
	if !strings.Contains(text, "Msg 9") {
		t.Error("expected last message")
	}
	if strings.Contains(text, "Msg 0") {
		t.Error("should not contain old messages with last_n=2")
	}
}

func TestHandleGetOrCreateRoomLastNOverLimit(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "upsert-overlimit")
	for i := 0; i < 10; i++ {
		mustPost(t, reg.Server, "upsert-overlimit", "Claude", fmt.Sprintf("Msg %d", i))
	}
	// last_n > 50 should clamp to 50
	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-overlimit", LastN: "999",
	})
	text := resultText(res)
	// All 10 messages should be present (clamped to 50, but only 10 exist)
	if !strings.Contains(text, "Msg 0") || !strings.Contains(text, "Msg 9") {
		t.Error("expected all messages within clamped limit")
	}
}

// ========== health tag stripping via bulk_status_update ==========

func TestHandleBulkStatusUpdate_ClearsHealthTagsOnResolve(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.Server.CreateRoom("htag-1", "Test", "", "", "needs-synthesis,sprint", "", "") //nolint:errcheck
	reg.Server.CreateRoom("htag-2", "Test", "", "", "stale,backlog", "", "")          //nolint:errcheck

	res, _, _ := reg.handleBulkStatusUpdate(context.Background(), nil, BulkStatusInput{
		RoomIDs: "htag-1,htag-2", Status: "resolved",
	})
	if !strings.Contains(resultText(res), "Updated 2") {
		t.Errorf("expected 2 updated, got: %s", resultText(res))
	}

	r1, _ := reg.Server.GetRoom("htag-1")
	if strings.Contains(r1.Tags, "needs-synthesis") {
		t.Errorf("expected 'needs-synthesis' stripped on resolve, got tags: %s", r1.Tags)
	}
	if !strings.Contains(r1.Tags, "sprint") {
		t.Errorf("expected 'sprint' tag preserved, got tags: %s", r1.Tags)
	}

	r2, _ := reg.Server.GetRoom("htag-2")
	if strings.Contains(r2.Tags, "stale") {
		t.Errorf("expected 'stale' stripped on resolve, got tags: %s", r2.Tags)
	}
	if !strings.Contains(r2.Tags, "backlog") {
		t.Errorf("expected 'backlog' tag preserved, got tags: %s", r2.Tags)
	}
}

// ========== tag normalization via UpdateRoom handler ==========

func TestHandleUpdateRoom_NormalizesTags(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "norm-update-room")

	res, _, _ := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "norm-update-room",
		Tags:   `["auth","mtls","tls"]`,
	})
	if strings.Contains(resultText(res), "Error") {
		t.Fatalf("unexpected error: %s", resultText(res))
	}

	room, _ := reg.Server.GetRoom("norm-update-room")
	for _, unwanted := range []string{"[", "]", `"`} {
		if strings.Contains(room.Tags, unwanted) {
			t.Errorf("tag normalization failed — found %q in tags: %s", unwanted, room.Tags)
		}
	}
	for _, want := range []string{"auth", "mtls", "tls"} {
		if !strings.Contains(room.Tags, want) {
			t.Errorf("expected tag %q in tags: %s", want, room.Tags)
		}
	}
}

// ========== knowledge_lint alias removed ==========

func TestKnowledgeLintAliasRemoved(t *testing.T) {
	// knowledge_lint alias was removed in v0.17.0 — check_room_health is the canonical name.
	// Verify that calling it returns an unknown-tool error, not a result.
	cs, _ := setupIntegrationTest(t)

	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "knowledge_lint"})
	if err == nil {
		t.Error("expected error when calling removed 'knowledge_lint' alias, got nil")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected 'unknown tool' error, got: %v", err)
	}
}

func TestHandleGetOrCreateRoomLastNInvalidString(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "upsert-badlastn")
	mustPost(t, reg.Server, "upsert-badlastn", "Claude", "A message")

	// Invalid last_n string defaults to 5
	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "upsert-badlastn", LastN: "abc",
	})
	if !strings.Contains(resultText(res), "Found") {
		t.Error("expected successful result with defaulted last_n")
	}
}
