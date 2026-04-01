package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ========== v0.5.0: bidirectional related_rooms via handlers ==========

func TestHandleCreateRoomBidirectionalLinks(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "bidir-target")

	res, _, _ := cs.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "bidir-source", Topic: "Source room", RelatedRooms: "bidir-target",
	})
	text := resultText(res)
	if !strings.Contains(text, "bidirectional") {
		t.Errorf("expected bidirectional mention in response, got: %s", text)
	}

	// Verify reverse link was created
	tgt, _ := cs.getRoom("bidir-target")
	if !strings.Contains(tgt.RelatedRooms, "bidir-source") {
		t.Errorf("expected reverse link to bidir-source, got: '%s'", tgt.RelatedRooms)
	}
}

func TestHandleUpdateRoomBidirectionalLinks(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "upd-bidir-a")
	mustCreateRoom(t, cs, "upd-bidir-b")

	cs.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "upd-bidir-a", RelatedRooms: "upd-bidir-b",
	})

	b, _ := cs.getRoom("upd-bidir-b")
	if !strings.Contains(b.RelatedRooms, "upd-bidir-a") {
		t.Errorf("expected reverse link, got: '%s'", b.RelatedRooms)
	}
}

// ========== v0.5.0: enriched create_room response ==========

func TestCreateRoomResponseIncludesMetadata(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleCreateRoom(context.Background(), nil, CreateRoomInput{
		ID: "rich-create", Topic: "My topic", Project: "my-proj", Tags: "alpha,beta",
	})
	text := resultText(res)

	if !strings.Contains(text, "**Topic:** My topic") {
		t.Error("expected topic in response")
	}
	if !strings.Contains(text, "**Project:** my-proj") {
		t.Error("expected project in response")
	}
	if !strings.Contains(text, "**Tags:** alpha,beta") {
		t.Error("expected tags in response")
	}
}

// ========== v0.5.0: enriched signal_status response ==========

func TestSignalStatusResponseIncludesContext(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "status-ctx", withDescription("Bug fix room"), withProject("my-proj"))

	res, _, _ := cs.handleSignalStatus(context.Background(), nil, SignalStatusInput{
		RoomID: "status-ctx", Status: "resolved",
	})
	text := resultText(res)

	if !strings.Contains(text, "**resolved**") {
		t.Errorf("expected bold status, got: %s", text)
	}
	if !strings.Contains(text, "Bug fix room") {
		t.Error("expected topic in response")
	}
	if !strings.Contains(text, "my-proj") {
		t.Error("expected project in response")
	}
}

// ========== v0.5.0: enriched update_room response ==========

func TestUpdateRoomResponseIncludesState(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "upd-state", withProject("proj-a"), withTags("v1"))

	res, _, _ := cs.handleUpdateRoom(context.Background(), nil, UpdateRoomInput{
		RoomID: "upd-state", Tags: "v2,released",
	})
	text := resultText(res)

	if !strings.Contains(text, "Current state") {
		t.Error("expected current state section")
	}
	if !strings.Contains(text, "v2,released") {
		t.Error("expected updated tags in state")
	}
	if !strings.Contains(text, "proj-a") {
		t.Error("expected project in state")
	}
}

// ========== v0.5.1: compact as default for list_rooms ==========

func TestListRoomsDefaultIsCompact(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "default-compact-1", withTechStack("Go"), withRelatedRooms("x"))

	// No args — should be compact (no Tech/Related fields)
	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{})
	text := resultText(res)
	if strings.Contains(text, "Tech:") {
		t.Error("default list should be compact — no Tech field")
	}
	if strings.Contains(text, "Related:") {
		t.Error("default list should be compact — no Related field")
	}
	if !strings.Contains(text, "default-compact-1") {
		t.Error("room ID should appear in compact output")
	}
}

func TestListRoomsVerboseFlag(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "verbose-flag-room", withTechStack("Go, SQLite"), withRelatedRooms("other-room"))

	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Verbose: "true"})
	text := resultText(res)
	if !strings.Contains(text, "Tech: Go, SQLite") {
		t.Errorf("verbose=true should show Tech field, got: %s", text)
	}
	if !strings.Contains(text, "Related: other-room") {
		t.Error("verbose=true should show Related field")
	}
}

func TestListRoomsLegacyCompactFalseIsVerbose(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "legacy-compact-false", withTechStack("Elixir"))

	// compact=false should still give verbose output for backwards compat
	res, _, _ := cs.handleListRooms(context.Background(), nil, ListRoomsInput{Compact: "false"})
	text := resultText(res)
	if !strings.Contains(text, "Tech: Elixir") {
		t.Errorf("compact=false should show verbose output, got: %s", text)
	}
}

// ========== v0.5.1: mode=summary returns top 2 per type ==========

func TestSummaryModeTopTwoPerType(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "sum-top2")
	mustPostTyped(t, cs, "sum-top2", "Claude", "First decision", "decision")
	mustPostTyped(t, cs, "sum-top2", "Gemini", "Second decision", "decision")
	mustPostTyped(t, cs, "sum-top2", "Claude", "Only thought", "thought")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "sum-top2", Mode: "summary",
	})
	text := resultText(res)

	// Both decisions should appear
	if !strings.Contains(text, "Latest decision") {
		t.Error("expected Latest decision label")
	}
	if !strings.Contains(text, "Previous decision") {
		t.Error("expected Previous decision label for second entry")
	}
	if !strings.Contains(text, "First decision") {
		t.Error("expected first decision content")
	}
	if !strings.Contains(text, "Second decision") {
		t.Error("expected second decision content")
	}
	// Thought has only one — no Previous label
	if !strings.Contains(text, "Latest thought") {
		t.Error("expected Latest thought label")
	}
}

func TestSummaryModeOnlyOnePerTypeWhenJustOne(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "sum-single")
	mustPostTyped(t, cs, "sum-single", "Claude", "The only action", "action")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "sum-single", Mode: "summary",
	})
	text := resultText(res)
	if !strings.Contains(text, "Latest action") {
		t.Error("expected Latest action")
	}
	if strings.Contains(text, "Previous action") {
		t.Error("should not have Previous when only one message of type")
	}
}

// ========== v0.5.1: get_digest smarter excerpts ==========

func TestDigestExcerptHeading(t *testing.T) {
	result := digestExcerpt("## My Heading\nSome body text here.")
	if result != "My Heading" {
		t.Errorf("expected heading extraction, got: %s", result)
	}
}

func TestDigestExcerptFirstSentence(t *testing.T) {
	result := digestExcerpt("This is the first sentence. This is the second sentence.")
	if result != "This is the first sentence." {
		t.Errorf("expected first sentence, got: %s", result)
	}
}

func TestDigestExcerptFallbackTruncation(t *testing.T) {
	long := strings.Repeat("word ", 40)
	result := digestExcerpt(long)
	if len(result) > 125 {
		t.Errorf("expected truncation at ~120 chars, got length %d: %s", len(result), result)
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected ellipsis suffix, got: %s", result)
	}
}

func TestDigestExcerptEmpty(t *testing.T) {
	result := digestExcerpt("")
	if result != "" {
		t.Errorf("expected empty for empty input, got: %s", result)
	}
}

// ========== v0.5.1: after_id includes system_prompt ==========

func TestAfterIDIncludesSystemPrompt(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "after-sp", withSystemPrompt("You are a code reviewer."))
	id := mustPost(t, cs, "after-sp", "Claude", "First message")
	mustPost(t, cs, "after-sp", "Gemini", "Second message")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID:  "after-sp",
		AfterID: fmt.Sprintf("%d", id),
	})
	text := resultText(res)
	if !strings.Contains(text, "You are a code reviewer.") {
		t.Errorf("after_id response should include system_prompt, got: %s", text)
	}
	if !strings.Contains(text, "Second message") {
		t.Error("expected new message in delta read")
	}
}

func TestAfterIDNoSystemPromptWhenEmpty(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "after-nosp")
	id := mustPost(t, cs, "after-nosp", "Claude", "First")
	mustPost(t, cs, "after-nosp", "Claude", "Second")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID:  "after-nosp",
		AfterID: fmt.Sprintf("%d", id),
	})
	text := resultText(res)
	if strings.Contains(text, "System Prompt:") {
		t.Error("should not show System Prompt header when system_prompt is empty")
	}
}

// ========== v0.5.0: post_to_room JSON cursor ==========

func TestPostToRoomJSONCursor(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "json-cursor")

	res, _, _ := cs.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "json-cursor", Author: "Claude", Message: "Test",
	})
	text := resultText(res)

	if !strings.Contains(text, `"message_id":`) {
		t.Errorf("expected JSON message_id, got: %s", text)
	}
	if !strings.Contains(text, `"room_id": "json-cursor"`) {
		t.Errorf("expected JSON room_id, got: %s", text)
	}
	if !strings.Contains(text, `"latest_message_id":`) {
		t.Errorf("expected JSON latest_message_id, got: %s", text)
	}
}

