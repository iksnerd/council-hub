package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ========== search_messages ==========

func TestHandleSearchMessagesNoFilter(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for no filters, got: %s", text)
	}
}

func TestHandleSearchMessagesWithLimit(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-lim")
	for i := 0; i < 5; i++ {
		mustPost(t, reg.Server, "h-search-lim", "Claude", "keyword match")
	}

	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query: "keyword", Limit: "2",
	})
	text := resultText(res)
	if !strings.Contains(text, "2 message(s)") {
		t.Errorf("expected 2 results with limit, got: %s", text)
	}
}

func TestHandleSearchMessagesSnippetTruncation(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-trunc")
	longContent := "keyword " + strings.Repeat("X", 500)
	mustPost(t, reg.Server, "h-search-trunc", "Claude", longContent)

	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query: "keyword",
	})
	text := resultText(res)
	if !strings.Contains(text, "...") {
		t.Errorf("expected truncated snippet with ..., got:\n%s", text)
	}
}

func TestHandleSearchMessagesBadLimit(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-bad-lim")
	mustPost(t, reg.Server, "h-search-bad-lim", "Claude", "findme")

	// Invalid limit falls back to 20
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query: "findme", Limit: "not-a-number",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected result with default limit, got: %s", text)
	}
}

func TestHandleSearchMessagesNoResults(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-empty")

	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query: "zzz-no-match",
	})
	text := resultText(res)
	if !strings.Contains(text, "No messages found") {
		t.Errorf("expected no messages, got: %s", text)
	}
}

func TestHandleSearchByAuthorOnly(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-auth")
	mustPost(t, reg.Server, "h-search-auth", "Claude", "hello")
	mustPost(t, reg.Server, "h-search-auth", "Gemini", "world")

	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Author: "Gemini",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message from Gemini, got: %s", text)
	}
}

func TestHandleSearchByTypeOnly(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-type")
	mustPostTyped(t, reg.Server, "h-search-type", "Claude", "thought1", "thought")
	mustPostTyped(t, reg.Server, "h-search-type", "Claude", "decision1", "decision")

	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		MessageType: "decision",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 decision, got: %s", text)
	}
}

func TestHandleSearchByRoomOnly(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-room-only")
	mustPost(t, reg.Server, "h-search-room-only", "Claude", "in room")

	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-room-only",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message, got: %s", text)
	}
}

func TestHandleSearchMessagesSummaryOnly(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-sumonly")
	longContent := "This is a long message " + strings.Repeat("that goes on and on ", 10)
	mustPostTyped(t, reg.Server, "h-search-sumonly", "Claude", longContent, "thought")
	mustPostTyped(t, reg.Server, "h-search-sumonly", "Gemini", "Short reply", "review")

	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-sumonly", SummaryOnly: "true",
	})
	text := resultText(res)

	// Should be compact pipe-separated format
	if !strings.Contains(text, "|") {
		t.Error("expected pipe-separated compact output")
	}
	// Content should be truncated at 120 chars
	if strings.Contains(text, strings.Repeat("that goes on and on ", 10)) {
		t.Error("full content should not appear in summary_only mode")
	}
	// Should still show results
	if !strings.Contains(text, "2 message(s)") {
		t.Errorf("expected 2 messages, got: %s", text)
	}
}

func TestHandleSearchMessagesSummaryOnlyVsDefault(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-compare")
	mustPost(t, reg.Server, "h-search-compare", "Claude", "Test content here")

	// Default (non-summary) should use bold formatting
	res1, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-compare",
	})
	text1 := resultText(res1)

	// Summary mode should NOT use bold formatting
	res2, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-compare", SummaryOnly: "true",
	})
	text2 := resultText(res2)

	if !strings.Contains(text1, "**#") {
		t.Error("default mode should use bold formatting")
	}
	if strings.Contains(text2, "**#") {
		t.Error("summary_only mode should not use bold formatting")
	}
}

func TestHandleSearchMessagesProjectFilter(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-proj-a", withProject("alpha"))
	mustCreateRoom(t, reg.Server, "h-search-proj-b", withProject("beta"))
	mustPost(t, reg.Server, "h-search-proj-a", "Claude", "keyword match")
	mustPost(t, reg.Server, "h-search-proj-b", "Gemini", "keyword match")

	// Search with project filter should only return results from "alpha"
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query: "keyword", Project: "alpha",
	})
	text := resultText(res)

	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message from alpha project, got: %s", text)
	}
	if !strings.Contains(text, "h-search-proj-a") {
		t.Error("expected result from room in alpha project")
	}
	if strings.Contains(text, "h-search-proj-b") {
		t.Error("should not contain result from beta project")
	}
}

func TestHandleSearchMessagesProjectFilterOnly(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-projonly", withProject("gamma"))
	mustPost(t, reg.Server, "h-search-projonly", "Claude", "Hello world")

	// Using project as the only filter should work
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Project: "gamma",
	})
	text := resultText(res)

	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message with project-only filter, got: %s", text)
	}
}

func TestHandleSearchMessagesProjectFilterNoResults(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-proj-nr", withProject("delta"))
	mustPost(t, reg.Server, "h-search-proj-nr", "Claude", "Hello")

	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Project: "nonexistent-project",
	})
	text := resultText(res)

	if !strings.Contains(text, "No messages found") {
		t.Errorf("expected no results for nonexistent project, got: %s", text)
	}
}

func TestHandleSearchMessagesSince(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-since")
	mustPost(t, reg.Server, "h-search-since", "Claude", "old message")

	// Since far future — nothing should match
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-since", Since: "2099-01-01T00:00:00",
	})
	text := resultText(res)
	if !strings.Contains(text, "No messages found") {
		t.Errorf("expected no results with future since, got: %s", text)
	}

	// Since past — message should match
	res2, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-since", Since: "2000-01-01T00:00:00",
	})
	text2 := resultText(res2)
	if !strings.Contains(text2, "1 message(s)") {
		t.Errorf("expected 1 message with past since, got: %s", text2)
	}
}

func TestHandleSearchMessagesUntil(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-until")
	mustPost(t, reg.Server, "h-search-until", "Claude", "a message")

	// Until far future — message should match
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-until", Until: "2099-01-01T00:00:00",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message with future until, got: %s", text)
	}

	// Until far past — nothing should match
	res2, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-until", Until: "2000-01-01T00:00:00",
	})
	text2 := resultText(res2)
	if !strings.Contains(text2, "No messages found") {
		t.Errorf("expected no results with past until, got: %s", text2)
	}
}

func TestHandleSearchMessagesSinceAndUntil(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-search-range")
	mustPost(t, reg.Server, "h-search-range", "Claude", "a message")

	// Window that includes now — should match
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-range",
		Since:  "2000-01-01T00:00:00",
		Until:  "2099-01-01T00:00:00",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 message in valid range, got: %s", text)
	}

	// Window entirely in the past — should not match
	res2, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "h-search-range",
		Since:  "2000-01-01T00:00:00",
		Until:  "2000-12-31T23:59:59",
	})
	text2 := resultText(res2)
	if !strings.Contains(text2, "No messages found") {
		t.Errorf("expected no results with past window, got: %s", text2)
	}
}

func TestHandleSearchMessagesSemanticClusterWide(t *testing.T) {
	// semantic=true + cluster_wide=true must NOT go through the cluster fan-out path
	// (Phoenix fan-out uses LIKE search, not sqlite-vec). It should attempt local
	// semantic search and return a user-friendly error when no embedder is configured
	// (test environment has no Ollama), rather than silently doing keyword search.
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "sem-cluster-room")
	mustPost(t, reg.Server, "sem-cluster-room", "Claude", "Using Postgres for persistence")

	res, _, err := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query:       "Postgres",
		Semantic:    "true",
		ClusterWide: "true",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(res)
	// In tests no embedder is configured, so we expect a graceful error message
	// (not a panic or nil pointer — the cluster fan-out path must NOT be taken).
	if !strings.Contains(text, "Error") && !strings.Contains(text, "local-only") {
		t.Errorf("expected error or local-only note for semantic+cluster_wide, got: %s", text)
	}
}

func TestHandleSearchMessagesSemanticClusterWideMissingQuery(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Semantic:    "true",
		ClusterWide: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing query, got: %s", text)
	}
}

func TestHandleSearchMessagesDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	_, _, err := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{Query: "test"})
	if err == nil {
		t.Error("expected error")
	}
}

// ========== room_stats ==========

func TestHandleRoomStatsMissing(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleRoomStats(context.Background(), nil, RoomStatsInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error")
	}
}

func TestHandleRoomStatsNotFound(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleRoomStats(context.Background(), nil, RoomStatsInput{RoomID: "ghost"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

func TestHandleRoomStatsLatestMessageID(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-stats-latest")
	mustPostTyped(t, reg.Server, "h-stats-latest", "Claude", "First", "thought")
	id2 := mustPostTyped(t, reg.Server, "h-stats-latest", "Gemini", "Second", "decision")

	res, _, _ := reg.handleRoomStats(context.Background(), nil, RoomStatsInput{RoomID: "h-stats-latest"})
	text := resultText(res)

	expected := fmt.Sprintf("Latest message ID:** %s", id2[:8])
	if !strings.Contains(text, expected) {
		t.Errorf("expected latest_message_id %s, got: %s", id2[:8], text)
	}
}

func TestHandleRoomStatsTypeCounts(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-stats-types")
	mustPostTyped(t, reg.Server, "h-stats-types", "Claude", "T1", "thought")
	mustPostTyped(t, reg.Server, "h-stats-types", "Claude", "T2", "thought")
	mustPostTyped(t, reg.Server, "h-stats-types", "Gemini", "D1", "decision")
	mustPostTyped(t, reg.Server, "h-stats-types", "Claude", "A1", "action")

	res, _, _ := reg.handleRoomStats(context.Background(), nil, RoomStatsInput{RoomID: "h-stats-types"})
	text := resultText(res)

	if !strings.Contains(text, "Types:") {
		t.Error("expected Types field in room_stats")
	}
	if !strings.Contains(text, "thought: 2") {
		t.Errorf("expected thought: 2, got: %s", text)
	}
	if !strings.Contains(text, "decision: 1") {
		t.Errorf("expected decision: 1, got: %s", text)
	}
	if !strings.Contains(text, "action: 1") {
		t.Errorf("expected action: 1, got: %s", text)
	}
}

func TestHandleRoomStatsEmptyNoLatestID(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "h-stats-empty")

	res, _, _ := reg.handleRoomStats(context.Background(), nil, RoomStatsInput{RoomID: "h-stats-empty"})
	text := resultText(res)

	if strings.Contains(text, "Latest message ID") {
		t.Error("empty room should not show Latest message ID")
	}
	if strings.Contains(text, "Types:") {
		t.Error("empty room should not show Types")
	}
}

func TestHandleRoomStatsDBError(t *testing.T) {
	reg := setupHandlerServer(t)
	reg.Server.DB.Close()

	res, _, _ := reg.handleRoomStats(context.Background(), nil, RoomStatsInput{RoomID: "hdb-room"})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error, got: %s", text)
	}
}

// ========== search_messages include_related ==========

func TestHandleSearchMessagesIncludeRelated(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "main-room", withRelatedRooms("linked-room"))
	mustCreateRoom(t, reg.Server, "linked-room")
	mustPost(t, reg.Server, "main-room", "Claude", "concept in main room")
	mustPost(t, reg.Server, "linked-room", "Gemini", "concept in linked room")

	// Without include_related, only finds message in main-room
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query:  "concept",
		RoomID: "main-room",
	})
	if !strings.Contains(resultText(res), "1 message(s)") {
		t.Errorf("expected 1 result without include_related, got: %s", resultText(res))
	}

	// With include_related, finds messages in both rooms
	res, _, _ = reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query:          "concept",
		RoomID:         "main-room",
		IncludeRelated: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "2 message(s)") {
		t.Errorf("expected 2 results with include_related, got: %s", text)
	}
	if !strings.Contains(text, "searched") {
		t.Errorf("expected scope note in response, got: %s", text)
	}
}

func TestHandleSearchMessagesIncludeRelatedNoRelated(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "solo-room")
	mustPost(t, reg.Server, "solo-room", "Claude", "isolated concept")

	// Room has no related rooms — include_related should be a no-op (no error)
	res, _, _ := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		Query:          "isolated",
		RoomID:         "solo-room",
		IncludeRelated: "true",
	})
	text := resultText(res)
	if !strings.Contains(text, "1 message(s)") {
		t.Errorf("expected 1 result, got: %s", text)
	}
}

// ========== move_messages ==========

func TestHandleMoveMessages(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "mv-src-h")
	mustCreateRoom(t, reg.Server, "mv-dst-h")
	id1 := mustPost(t, reg.Server, "mv-src-h", "Claude", "move this")
	id2 := mustPost(t, reg.Server, "mv-src-h", "Gemini", "move this too")

	res, _, _ := reg.handleMoveMessages(context.Background(), nil, MoveMessagesInput{
		MessageIDs:   id1 + "," + id2,
		TargetRoomID: "mv-dst-h",
	})
	text := resultText(res)
	if !strings.Contains(text, "Moved 2 message(s)") {
		t.Errorf("expected success message, got: %s", text)
	}
}

func TestHandleMoveMessagesTargetNotFound(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "mv-src-notfound")
	id := mustPost(t, reg.Server, "mv-src-notfound", "Claude", "hello")

	res, _, _ := reg.handleMoveMessages(context.Background(), nil, MoveMessagesInput{
		MessageIDs:   id,
		TargetRoomID: "does-not-exist",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for missing target room, got: %s", text)
	}
}

func TestHandleMoveMessagesMissingIDs(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleMoveMessages(context.Background(), nil, MoveMessagesInput{
		TargetRoomID: "some-room",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Errorf("expected error for missing message_ids")
	}
}

func TestHandleMoveMessagesMissingTarget(t *testing.T) {
	reg := setupHandlerTest(t)
	res, _, _ := reg.handleMoveMessages(context.Background(), nil, MoveMessagesInput{
		MessageIDs: "abc123",
	})
	if !strings.Contains(resultText(res), "Error") {
		t.Errorf("expected error for missing target_room_id")
	}
}
