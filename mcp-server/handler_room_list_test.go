package main

import (
	"context"
	"strings"
	"testing"
)

// ========== signal_status ==========

func TestHandleSignalStatus(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-status")

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

func TestHandleSignalStatusNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "nonexistent", Status: "active",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleSignalStatusDBError(t *testing.T) {
	cs := setupHandlerServer(t)
	cs.db.Close()

	res, _, _ := cs.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "hdb-room", Status: "paused",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// ========== list_rooms ==========

func TestHandleListRooms(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-list-1", withProject("proj-a"), withTags("tag1"))
	mustCreateRoom(t, cs, "h-list-2", withProject("proj-b"), withTags("tag2"), withRelatedRooms("related-room"))

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

func TestHandleListRoomsCompact(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-compact-1", withDescription("Short topic"), withProject("proj-a"), withTechStack("Go"), withTags("tag1"))
	mustCreateRoom(t, cs, "h-compact-2", withDescription("Another topic that is a bit longer and should be shown"), withProject("proj-b"), withTags("tag2"), withRelatedRooms("related-x"))

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
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
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-compact-noproj")

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)
	if !strings.Contains(text, "- |") {
		t.Error("expected dash for empty project in compact mode")
	}
}

func TestHandleListRoomsCompactLongTopic(t *testing.T) {
	cs := setupTestServer(t)
	longTopic := strings.Repeat("A", 80)
	mustCreateRoom(t, cs, "h-compact-long", withDescription(longTopic), withProject("proj"))

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)
	if !strings.Contains(text, "...") {
		t.Error("expected truncated topic in compact mode")
	}
	if strings.Contains(text, strings.Repeat("A", 80)) {
		t.Error("full 80-char topic should not appear in compact mode")
	}
}

func TestHandleListRoomsNonCompact(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-verbose-1", withDescription("Full detail room"), withProject("proj"), withTechStack("Go, Docker"), withTags("auth"), withRelatedRooms("related-a"))

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if !strings.Contains(text, "Tech: Go, Docker") {
		t.Error("non-compact should include Tech field")
	}
	if !strings.Contains(text, "Related: related-a") {
		t.Error("non-compact should include Related field")
	}
}

func TestHandleListRoomsFiltered(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-filt-1", withProject("proj-a"))
	mustCreateRoom(t, cs, "h-filt-2", withProject("proj-b"))

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Project: "proj-a"})
	text := resultText(res)
	if !strings.Contains(text, "1 room(s)") {
		t.Errorf("expected 1 room, got: %s", text)
	}
}

func TestHandleListRoomsWithTechStack(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-list-tech", withTechStack("Go, SQLite"))

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if !strings.Contains(text, "Tech: Go, SQLite") {
		t.Errorf("expected tech stack in listing, got: %s", text)
	}
}

func TestHandleListRoomsCompactMessageCount(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-compact-cnt1", withProject("proj"))
	mustCreateRoom(t, cs, "h-compact-cnt2", withProject("proj"))
	mustPost(t, cs, "h-compact-cnt1", "Claude", "M1")
	mustPostTyped(t, cs, "h-compact-cnt1", "Gemini", "M2", "thought")
	mustPostTyped(t, cs, "h-compact-cnt1", "Claude", "M3", "decision")

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "true"})
	text := resultText(res)
	if !strings.Contains(text, "3 msgs") {
		t.Errorf("expected '3 msgs' in compact output, got: %s", text)
	}
	if !strings.Contains(text, "0 msgs") {
		t.Errorf("expected '0 msgs' for empty room, got: %s", text)
	}
}

func TestHandleListRoomsSearch(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "auth-migration", withDescription("JWT auth migration"), withProject("proj-a"), withTags("auth,security"))
	mustCreateRoom(t, cs, "frontend-ui", withDescription("React dashboard"), withProject("proj-b"), withTags("ui,react"))
	mustCreateRoom(t, cs, "auth-review", withDescription("Auth code review"), withProject("proj-a"), withTags("auth"))

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Search: "auth"})
	text := resultText(res)
	if !strings.Contains(text, "2 room(s)") {
		t.Errorf("expected 2 rooms matching 'auth', got: %s", text)
	}

	res, _, _ = cs.handleListRooms(context.Background(), nil, ListRoomsInput{Search: "React"})
	text = resultText(res)
	if !strings.Contains(text, "1 room(s)") {
		t.Errorf("expected 1 room matching 'React', got: %s", text)
	}

	res, _, _ = cs.handleListRooms(context.Background(), nil, ListRoomsInput{Search: "security"})
	text = resultText(res)
	if !strings.Contains(text, "1 room(s)") {
		t.Errorf("expected 1 room matching tag 'security', got: %s", text)
	}

	res, _, _ = cs.handleListRooms(context.Background(), nil, ListRoomsInput{Search: "zzz-nope"})
	text = resultText(res)
	if !strings.Contains(text, "No rooms found") {
		t.Errorf("expected no rooms, got: %s", text)
	}
}

func TestHandleListRoomsSearchWithCompact(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "search-compact-1", withDescription("Bug tracker"), withProject("proj"), withTags("bug"))
	mustCreateRoom(t, cs, "search-compact-2", withDescription("Feature design"), withProject("proj"), withTags("feature"))
	mustPost(t, cs, "search-compact-1", "Claude", "A message")

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{
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

func TestHandleListRoomsDBError(t *testing.T) {
	cs := setupHandlerServer(t)
	cs.db.Close()

	_, _, err := cs.handleListRooms(context.Background(), nil, ListRoomsInput{})
	if err == nil {
		t.Error("expected error")
	}
}
