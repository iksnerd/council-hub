package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== create_room ==========

func TestHandleCreateRoom(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "h-room", Topic: "Handler test", Project: "proj",
	})
	if err != nil {
		t.Fatalf("handleCreateRoom error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "h-room") || !strings.Contains(text, "created") {
		t.Errorf("unexpected result: %s", text)
	}

	room, _ := reg.Server.GetRoom("h-room")
	if room.Project != "proj" {
		t.Errorf("expected project 'proj', got '%s'", room.Project)
	}
}

func TestHandleCreateRoomPrivateVisibility(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "secret-room", Topic: "node-local", Visibility: "private",
	})
	if err != nil {
		t.Fatalf("handleCreateRoom error: %v", err)
	}
	if text := resultText(res); !strings.Contains(text, "private") {
		t.Errorf("expected visibility note in output, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("secret-room")
	if room.Visibility != "private" {
		t.Errorf("expected visibility 'private', got '%s'", room.Visibility)
	}

	// read_room surfaces the private flag.
	readRes, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "secret-room"})
	if text := resultText(readRes); !strings.Contains(text, "private") {
		t.Errorf("expected read_room to show private visibility, got: %s", text)
	}
}

func TestHandleCreateRoomWithRepo(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "repo-room", Topic: "commit links", Repo: "iksnerd/council-hub",
	})
	if err != nil {
		t.Fatalf("handleCreateRoom error: %v", err)
	}
	if text := resultText(res); !strings.Contains(text, "iksnerd/council-hub") {
		t.Errorf("expected repo note in output, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("repo-room")
	if room.Repo != "iksnerd/council-hub" {
		t.Errorf("expected repo persisted, got '%s'", room.Repo)
	}

	// read_room surfaces the repo.
	readRes, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "repo-room"})
	if text := resultText(readRes); !strings.Contains(text, "iksnerd/council-hub") {
		t.Errorf("expected read_room to show repo, got: %s", text)
	}
}

func TestHandleUpdateRoomRepo(t *testing.T) {
	reg := setupHandlerTest(t)

	if _, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{ID: "repo-update", Topic: "t"}); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, _, err := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{RoomID: "repo-update", Repo: "owner/widget"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	room, _ := reg.Server.GetRoom("repo-update")
	if room.Repo != "owner/widget" {
		t.Errorf("expected repo 'owner/widget' after update, got '%s'", room.Repo)
	}
}

func TestHandleUpdateRoomVisibility(t *testing.T) {
	reg := setupHandlerTest(t)

	if _, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{ID: "toggle-room", Topic: "t"}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if _, _, err := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{RoomID: "toggle-room", Visibility: "private"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	room, _ := reg.Server.GetRoom("toggle-room")
	if room.Visibility != "private" {
		t.Errorf("expected 'private' after update, got '%s'", room.Visibility)
	}

	// Re-expose to the cluster.
	if _, _, err := reg.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{RoomID: "toggle-room", Visibility: "public"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	room, _ = reg.Server.GetRoom("toggle-room")
	if room.Visibility != "public" {
		t.Errorf("expected 'public' after update, got '%s'", room.Visibility)
	}
}

func TestHandleCreateRoomMissingID(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing ID, got: %s", text)
	}
}

func TestHandleCreateRoomWithRelatedRooms(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "h-create-related", Topic: "With links", RelatedRooms: "a,b,c",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "created") {
		t.Errorf("expected created, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("h-create-related")
	if room.RelatedRooms != "a,b,c" {
		t.Errorf("expected related_rooms 'a,b,c', got '%s'", room.RelatedRooms)
	}
}

func TestHandleCreateRoomDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	_, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{ID: "fail"})
	if err == nil {
		t.Error("expected error")
	}
}

// ========== duplicate room detection ==========

func TestHandleCreateRoomDuplicateWarning(t *testing.T) {
	reg := setupHandlerTest(t)
	// Create an existing room with overlapping tags.
	mustCreateRoom(t, reg.Server, "existing-auth", withProject("myapp"), withTags("go,auth,api"))

	// Create a new room with overlapping tags — should get a warning.
	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID:      "new-auth-service",
		Topic:   "Authentication service",
		Project: "myapp",
		Tags:    "go,auth,backend",
	})
	text := resultText(res)
	if !strings.Contains(text, "new-auth-service") {
		t.Errorf("expected new room in response, got: %s", text)
	}
	if !strings.Contains(text, "Similar room") {
		t.Errorf("expected duplicate warning, got: %s", text)
	}
	if !strings.Contains(text, "existing-auth") {
		t.Errorf("expected existing room ID in warning, got: %s", text)
	}
}

func TestHandleReadRoomFoldsInSummary(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "rr-summary")
	pinID := mustPostTyped(t, reg.Server, "rr-summary", "Claude", "the living abstract", "synthesis")
	if _, err := reg.Server.PinMessage("rr-summary", pinID); err != nil {
		t.Fatalf("pin failed: %v", err)
	}
	mustPostTyped(t, reg.Server, "rr-summary", "Gemini", "shipped the fix", "action")

	// No include_last_n: read_room should orient with content, not just a header.
	res, _, err := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "rr-summary"})
	if err != nil {
		t.Fatalf("handleReadRoom error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "the living abstract") {
		t.Errorf("expected pinned content folded in, got: %s", text)
	}
	if !strings.Contains(text, "shipped the fix") {
		t.Errorf("expected latest-per-type content folded in, got: %s", text)
	}
	if !strings.Contains(text, "📌 Pinned") {
		t.Errorf("expected pinned marker, got: %s", text)
	}
}

func TestHandleGetOrCreateRoomRepoProjectHint(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "lens-core", withProject("weightless"))
	if err := reg.Server.SetRepo("lens-core", "iksnerd/lens"); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}

	// New room, same repo, no project — should suggest the repo's existing project.
	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "lens-review", Topic: "Lens review", Repo: "iksnerd/lens",
	})
	text := resultText(res)
	if !strings.Contains(text, "weightless") || !strings.Contains(text, "keep this repo") {
		t.Errorf("expected repo→project hint, got: %s", text)
	}

	// A matching project should NOT trigger the hint.
	res2, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "lens-spec", Topic: "Lens spec", Repo: "iksnerd/lens", Project: "weightless",
	})
	if strings.Contains(resultText(res2), "keep this repo") {
		t.Errorf("no hint expected when project already matches, got: %s", resultText(res2))
	}
}

func TestHandleReadRoomShowsHealthTagHint(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "flagged-room")
	if _, err := reg.Server.DB.Exec(`UPDATE rooms SET tags='needs-synthesis' WHERE id='flagged-room'`); err != nil {
		t.Fatalf("flag: %v", err)
	}
	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "flagged-room"})
	text := resultText(res)
	if !strings.Contains(text, "Health flags") || !strings.Contains(text, "needs-synthesis") {
		t.Errorf("expected actionable health-flag hint, got: %s", text)
	}
}

func TestHandlePostToRoomClearsThenNoStaleHint(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "stale-then-active")
	if _, err := reg.Server.DB.Exec(`UPDATE rooms SET tags='stale' WHERE id='stale-then-active'`); err != nil {
		t.Fatalf("flag: %v", err)
	}
	// A normal post clears `stale`, so the response should NOT nudge about it.
	res, _, _ := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "stale-then-active", Author: "Claude", Message: "back to work", MessageType: "thought",
	})
	if strings.Contains(resultText(res), "Health flags") {
		t.Errorf("stale flag was cleared by the post; should not hint, got: %s", resultText(res))
	}
}

func TestHandleReadRoomEmptyShowsNoMessages(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "rr-empty")
	res, _, _ := reg.handleReadRoom(context.Background(), nil, ReadRoomInput{RoomID: "rr-empty"})
	if !strings.Contains(resultText(res), "No messages yet") {
		t.Errorf("expected empty-room hint, got: %s", resultText(res))
	}
}

func TestHandleSimilarRoomsCarryExcerpt(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "existing-cache", withProject("perf"), withTags("redis,caching,backend"))
	pinID := mustPostTyped(t, reg.Server, "existing-cache", "Claude", "decided on write-through caching", "synthesis")
	if _, err := reg.Server.PinMessage("existing-cache", pinID); err != nil {
		t.Fatalf("pin failed: %v", err)
	}

	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "new-cache", Topic: "Cache design", Project: "perf", Tags: "redis,caching,go",
	})
	text := resultText(res)
	if !strings.Contains(text, "existing-cache") {
		t.Fatalf("expected similar-room note, got: %s", text)
	}
	if !strings.Contains(text, "msgs") {
		t.Errorf("expected message count in similar-room note, got: %s", text)
	}
	if !strings.Contains(text, "decided on write-through caching") {
		t.Errorf("expected pinned excerpt in similar-room note, got: %s", text)
	}
}

func TestHandleCreateRoomNoDuplicateWhenUnrelated(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "infra-room", withProject("ops"), withTags("kubernetes,terraform"))

	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID:      "auth-room",
		Topic:   "User authentication",
		Project: "myapp",
		Tags:    "oauth,jwt",
	})
	text := resultText(res)
	if strings.Contains(text, "Similar room") {
		t.Errorf("unexpected duplicate warning for unrelated rooms: %s", text)
	}
}

func TestHandleGetOrCreateRoomDuplicateWarning(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "existing-cache", withProject("perf"), withTags("redis,caching,backend"))

	// get_or_create a new room (different ID) with overlapping tags.
	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID:      "new-cache-layer",
		Topic:   "Cache layer design",
		Project: "perf",
		Tags:    "redis,caching,go",
	})
	text := resultText(res)
	if !strings.Contains(text, "Similar room") {
		t.Errorf("expected duplicate warning on get_or_create, got: %s", text)
	}
	if !strings.Contains(text, "existing-cache") {
		t.Errorf("expected existing room in warning, got: %s", text)
	}
}

func TestHandleGetOrCreateRoomNoWarningOnExisting(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "my-room", withProject("proj"), withTags("go,api"))

	// Fetching an existing room should NOT trigger duplicate warning.
	res, _, _ := reg.handleGetOrCreateRoom(context.Background(), nil, GetOrCreateRoomInput{
		ID: "my-room",
	})
	text := resultText(res)
	if strings.Contains(text, "Similar room") {
		t.Errorf("unexpected duplicate warning when fetching existing room: %s", text)
	}
}

// ========== create_room templates ==========

func TestHandleCreateRoomTemplate(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-room", Template: "decision-log",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "decision-log") {
		t.Errorf("expected template name in response, got: %s", text)
	}

	room, _ := reg.Server.GetRoom("tpl-room")
	if !strings.Contains(room.Tags, "decision") {
		t.Errorf("expected decision tag from template, got: %s", room.Tags)
	}
	if room.SystemPrompt == "" {
		t.Errorf("expected system_prompt from template, got empty")
	}
}

func TestHandleCreateRoomTemplateOverride(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-override", Template: "sprint", Tags: "custom-tag",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resultText(res)

	room, _ := reg.Server.GetRoom("tpl-override")
	if room.Tags != "custom-tag" {
		t.Errorf("explicit tags should override template, got: %s", room.Tags)
	}
	if room.SystemPrompt == "" {
		t.Errorf("template system_prompt should still apply when tags were overridden")
	}
}

func TestHandleCreateRoomTemplateUnknown(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-bad", Template: "nonexistent",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for unknown template, got: %s", text)
	}
	// Should list valid template names
	if !strings.Contains(text, "decision-log") {
		t.Errorf("expected available template names in error, got: %s", text)
	}
}

func TestHandleCreateRoomTemplateInitialMsg(t *testing.T) {
	reg := setupHandlerTest(t)

	_, _, err := reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-msg", Template: "bug",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs, _ := reg.Server.GetRecentMessages("tpl-msg", 10)
	if len(msgs) == 0 {
		t.Fatal("expected initial message to be posted")
	}
	if msgs[0].Author != "system" {
		t.Errorf("expected author 'system', got '%s'", msgs[0].Author)
	}
	if !strings.Contains(msgs[0].Content, "Bug investigation") {
		t.Errorf("unexpected initial message content: %s", msgs[0].Content)
	}
}

func TestHandleCreateRoomTemplateNoInitialMsgIfExists(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "tpl-exists")

	_, _, _ = reg.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "tpl-exists", Template: "bug",
	})

	msgs, _ := reg.Server.GetRecentMessages("tpl-exists", 10)
	if len(msgs) != 0 {
		t.Errorf("expected no initial message for pre-existing room, got %d messages", len(msgs))
	}
}
