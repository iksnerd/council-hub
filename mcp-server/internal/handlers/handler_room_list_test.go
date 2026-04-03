package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== signal_status ==========

func TestHandleSignalStatus(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-status")

	res, _, _ := reg.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "h-status", Status: "resolved",
	})
	text := resultText(res)
	if !strings.Contains(text, "resolved") {
		t.Errorf("expected resolved, got: %s", text)
	}
}

func TestHandleSignalStatusInvalid(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "whatever", Status: "invalid",
	})
	text := resultText(res)
	if !strings.Contains(text, "Invalid status") {
		t.Errorf("expected invalid status error, got: %s", text)
	}
}

func TestHandleSignalStatusNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "nonexistent", Status: "active",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleSignalStatusDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	res, _, _ := reg.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "hdb-room", Status: "paused",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// ========== list_rooms ==========

func TestHandleListRooms(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-list-1", withProject("proj-a"), withTags("tag1"))
	mustCreateRoom(t, reg.Server, "h-list-2", withProject("proj-b"), withTags("tag2"), withRelatedRooms("related-room"))

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if !strings.Contains(text, "2 room(s)") {
		t.Errorf("expected 2 rooms, got: %s", text)
	}
	if !strings.Contains(text, "h-list-1") || !strings.Contains(text, "h-list-2") {
		t.Error("list missing room IDs")
	}
	// default output is compact — no Related field; use verbose=true to see it
}

func TestHandleListRoomsEmpty(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if !strings.Contains(text, "No rooms found") {
		t.Errorf("expected no rooms, got: %s", text)
	}
}

func TestHandleListRoomsCompact(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-compact-1", withDescription("Short topic"), withProject("proj-a"), withTechStack("Go"), withTags("tag1"))
	mustCreateRoom(t, reg.Server, "h-compact-2", withDescription("Another topic that is a bit longer and should be shown"), withProject("proj-b"), withTags("tag2"), withRelatedRooms("related-x"))

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)

	if !strings.Contains(text, "2 room(s)") {
		t.Errorf("expected 2 rooms, got: %s", text)
	}
	if !strings.Contains(text, "h-compact-1") || !strings.Contains(text, "h-compact-2") {
		t.Error("missing room IDs in compact output")
	}
	if !strings.Contains(text, "proj-a") {
		t.Error("missing project in compact output")
	}
	if strings.Contains(text, "Tech:") {
		t.Error("compact mode should not include Tech field")
	}
	if strings.Contains(text, "Related:") {
		t.Error("compact mode should not include Related field")
	}
}

func TestHandleListRoomsCompactNoProject(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-compact-noproj")

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)
	if !strings.Contains(text, "- |") {
		t.Error("expected dash for empty project in compact mode")
	}
}

func TestHandleListRoomsCompactLongTopic(t *testing.T) {
	reg := setupHandlerTest(t)
	longTopic := strings.Repeat("A", 80)
	mustCreateRoom(t, reg.Server, "h-compact-long", withDescription(longTopic), withProject("proj"))

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)
	if !strings.Contains(text, "...") {
		t.Error("expected truncated topic in compact mode")
	}
	if strings.Contains(text, strings.Repeat("A", 80)) {
		t.Error("full 80-char topic should not appear in compact mode")
	}
}

func TestHandleListRoomsNonCompact(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-verbose-1", withDescription("Full detail room"), withProject("proj"), withTechStack("Go, Docker"), withTags("auth"), withRelatedRooms("related-a"))

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Verbose: "true"})
	text := resultText(res)
	if !strings.Contains(text, "Tech: Go, Docker") {
		t.Error("verbose mode should include Tech field")
	}
	if !strings.Contains(text, "Related: related-a") {
		t.Error("verbose mode should include Related field")
	}
}

func TestHandleListRoomsFiltered(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-filt-1", withProject("proj-a"))
	mustCreateRoom(t, reg.Server, "h-filt-2", withProject("proj-b"))

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Project: "proj-a"})
	text := resultText(res)
	if !strings.Contains(text, "1 room(s)") {
		t.Errorf("expected 1 room, got: %s", text)
	}
}

func TestHandleListRoomsWithTechStack(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-list-tech", withTechStack("Go, SQLite"))

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Verbose: "true"})
	text := resultText(res)
	if !strings.Contains(text, "Tech: Go, SQLite") {
		t.Errorf("expected tech stack in verbose listing, got: %s", text)
	}
}

func TestHandleListRoomsCompactMessageCount(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-compact-cnt1", withProject("proj"))
	mustCreateRoom(t, reg.Server, "h-compact-cnt2", withProject("proj"))
	mustPost(t, reg.Server, "h-compact-cnt1", "Claude", "M1")
	mustPostTyped(t, reg.Server, "h-compact-cnt1", "Gemini", "M2", "thought")
	mustPostTyped(t, reg.Server, "h-compact-cnt1", "Claude", "M3", "decision")

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)
	if !strings.Contains(text, "3 msgs") {
		t.Errorf("expected '3 msgs' in compact output, got: %s", text)
	}
	if !strings.Contains(text, "0 msgs") {
		t.Errorf("expected '0 msgs' for empty room, got: %s", text)
	}
}

func TestHandleListRoomsSearch(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "auth-migration", withDescription("JWT auth migration"), withProject("proj-a"), withTags("auth,security"))
	mustCreateRoom(t, reg.Server, "frontend-ui", withDescription("React dashboard"), withProject("proj-b"), withTags("ui,react"))
	mustCreateRoom(t, reg.Server, "auth-review", withDescription("Auth code review"), withProject("proj-a"), withTags("auth"))

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{Search: "auth"})
	text := resultText(res)
	if !strings.Contains(text, "2 room(s)") {
		t.Errorf("expected 2 rooms matching 'auth', got: %s", text)
	}

	res, _, _ = reg.handleListRooms(context.Background(), nil, ListRoomsInput{Search: "React"})
	text = resultText(res)
	if !strings.Contains(text, "1 room(s)") {
		t.Errorf("expected 1 room matching 'React', got: %s", text)
	}

	res, _, _ = reg.handleListRooms(context.Background(), nil, ListRoomsInput{Search: "security"})
	text = resultText(res)
	if !strings.Contains(text, "1 room(s)") {
		t.Errorf("expected 1 room matching tag 'security', got: %s", text)
	}

	res, _, _ = reg.handleListRooms(context.Background(), nil, ListRoomsInput{Search: "zzz-nope"})
	text = resultText(res)
	if !strings.Contains(text, "No rooms found") {
		t.Errorf("expected no rooms, got: %s", text)
	}
}

func TestHandleListRoomsSearchWithCompact(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "search-compact-1", withDescription("Bug tracker"), withProject("proj"), withTags("bug"))
	mustCreateRoom(t, reg.Server, "search-compact-2", withDescription("Feature design"), withProject("proj"), withTags("feature"))
	mustPost(t, reg.Server, "search-compact-1", "Claude", "A message")

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{
		Search: "Bug", Compact: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 room(s)") {
		t.Errorf("expected 1 room, got: %s", text)
	}
	if !strings.Contains(text, "1 msgs") {
		t.Error("compact search should include message count")
	}
}

// ========== list_rooms with pinned excerpts ==========

func TestHandleListRoomsWithPinnedExcerpt(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-pin-room")
	id := mustPost(t, reg.Server, "h-pin-room", "Claude", "Important decision about architecture")
	reg.Server.PinMessage("h-pin-room", id)

	// Default (compact) mode
	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if !strings.Contains(text, "\xf0\x9f\x93\x8c") { // 📌 in UTF-8
		t.Errorf("expected pinned emoji in compact output, got: %s", text)
	}
	if !strings.Contains(text, "Important decision") {
		t.Errorf("expected pinned content excerpt, got: %s", text)
	}
}

func TestHandleListRoomsNoPinned(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-nopin-room")
	mustPost(t, reg.Server, "h-nopin-room", "Claude", "Just a regular message")

	res, _, _ := reg.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if strings.Contains(text, "\xf0\x9f\x93\x8c") { // 📌 in UTF-8
		t.Errorf("expected no pinned emoji in output, got: %s", text)
	}
}

func TestHandleListRoomsDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	_, _, err := reg.handleListRooms(context.Background(), nil, ListRoomsInput{})
	if err == nil {
		t.Error("expected error")
	}
}
