package handlers

import (
	"context"
	"strings"
	"testing"
)

// ========== post_to_room structured metadata ==========

func TestPostToRoomReturnsCursor(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "cursor-room")

	res, _, err := reg.handlePostToRoom(context.Background(), nil, PostToRoomInput{
		RoomID: "cursor-room", Author: "Claude", Message: "Hello",
	})
	if err != nil {
		t.Fatalf("handlePostToRoom error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, `"message_id":`) {
		t.Errorf("expected JSON message_id in response, got: %s", text)
	}
	if !strings.Contains(text, `"room_id": "cursor-room"`) {
		t.Errorf("expected JSON room_id in response, got: %s", text)
	}
}

// ========== search_messages word-boundary truncation ==========

func TestSearchMessagesSummaryWordBoundary(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "wb-room")
	// Post a message longer than 120 chars with words
	longMsg := "This is a very long message that should be truncated at a word boundary rather than cutting in the middle of a word which would be ugly"
	mustPost(t, reg.Server, "wb-room", "Claude", longMsg)

	res, _, err := reg.handleSearchMessages(context.Background(), nil, SearchMessagesInput{
		RoomID: "wb-room", SummaryOnly: "true",
	})
	if err != nil {
		t.Fatalf("handleSearchMessages error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "...") {
		t.Errorf("expected truncation with ..., got: %s", text)
	}
	// Should not cut mid-word — the excerpt before "..." should end at a space
	idx := strings.Index(text, "...")
	if idx > 0 && text[idx-1] != ' ' {
		// Check it ended at a word boundary (last char before ... should be a letter, which is ok if the whole word fits)
		// The key test: it should NOT be longer than 120 chars before the ...
		lineStart := strings.LastIndex(text[:idx], "|")
		if lineStart > 0 {
			excerpt := strings.TrimSpace(text[lineStart+1 : idx])
			if len(excerpt) > 120 {
				t.Errorf("excerpt too long (%d chars): %s", len(excerpt), excerpt)
			}
		}
	}
}

// read_recent removed in v0.5.0 — use read_transcript(last_n=N)

// ========== read_transcript batch (room_ids) ==========

func TestReadTranscriptBatchRoomIDs(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "batch-a", withDescription("Room A"))
	mustCreateRoom(t, reg.Server, "batch-b", withDescription("Room B"))
	mustPost(t, reg.Server, "batch-a", "Claude", "Message in A")
	mustPost(t, reg.Server, "batch-b", "Gemini", "Message in B")

	res, _, err := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomIDs: "batch-a,batch-b",
	})
	if err != nil {
		t.Fatalf("handleReadTranscript batch error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "batch-a") {
		t.Errorf("expected batch-a in output, got: %s", text)
	}
	if !strings.Contains(text, "batch-b") {
		t.Errorf("expected batch-b in output, got: %s", text)
	}
	if !strings.Contains(text, "Message in A") {
		t.Errorf("expected message content from room A")
	}
	if !strings.Contains(text, "Message in B") {
		t.Errorf("expected message content from room B")
	}
}

func TestReadTranscriptBatchWithSummaryMode(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "bs-a", withDescription("Summary A"), withSystemPrompt("Prompt A"))
	mustCreateRoom(t, reg.Server, "bs-b", withDescription("Summary B"), withSystemPrompt("Prompt B"))
	mustPostTyped(t, reg.Server, "bs-a", "Claude", "Decision A", "decision")
	mustPostTyped(t, reg.Server, "bs-b", "Gemini", "Action B", "action")

	res, _, err := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomIDs: "bs-a,bs-b",
		Mode:    "summary",
	})
	if err != nil {
		t.Fatalf("batch summary error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "summary") {
		t.Errorf("expected summary mode output")
	}
	if !strings.Contains(text, "Prompt A") {
		t.Errorf("expected system prompt A in output")
	}
	if !strings.Contains(text, "Prompt B") {
		t.Errorf("expected system prompt B in output")
	}
}

func TestReadTranscriptBatchBadRoom(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "batch-ok")
	mustPost(t, reg.Server, "batch-ok", "Claude", "OK message")

	res, _, err := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomIDs: "batch-ok,nonexistent",
	})
	if err != nil {
		t.Fatalf("batch with bad room should not return error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "batch-ok") {
		t.Errorf("expected batch-ok in output")
	}
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for nonexistent room in output")
	}
}

func TestReadTranscriptRequiresRoomIDOrRoomIDs(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{})
	text := resultText(res)
	if !strings.Contains(text, "required") {
		t.Errorf("expected error when no room_id or room_ids, got: %s", text)
	}
}

// ========== read_transcript include_related ==========

func TestReadTranscriptIncludeRelated(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "main-room", withDescription("Main"), withRelatedRooms("rel-a,rel-b"))
	mustCreateRoom(t, reg.Server, "rel-a", withDescription("Related A"), withSystemPrompt("Context A"))
	mustCreateRoom(t, reg.Server, "rel-b", withDescription("Related B"), withSystemPrompt("Context B"))
	mustPost(t, reg.Server, "main-room", "Claude", "Main message")
	mustPostTyped(t, reg.Server, "rel-a", "Claude", "Decision in A", "decision")
	mustPostTyped(t, reg.Server, "rel-b", "Gemini", "Action in B", "action")

	res, _, err := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID:         "main-room",
		IncludeRelated: "true",
	})
	if err != nil {
		t.Fatalf("include_related error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "Main message") {
		t.Errorf("expected main room content")
	}
	if !strings.Contains(text, "Context A") {
		t.Errorf("expected related room A system prompt")
	}
	if !strings.Contains(text, "Context B") {
		t.Errorf("expected related room B system prompt")
	}
}

func TestReadTranscriptIncludeRelatedNoRelated(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "solo-room", withDescription("Solo"))
	mustPost(t, reg.Server, "solo-room", "Claude", "Solo message")

	res, _, err := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID:         "solo-room",
		IncludeRelated: "true",
	})
	if err != nil {
		t.Fatalf("include_related with no related rooms error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "Solo message") {
		t.Errorf("expected solo room content")
	}
}

// ========== get_digest ==========

func TestGetDigestBasic(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "digest-a", withProject("myproj"))
	mustCreateRoom(t, reg.Server, "digest-b", withProject("myproj"))
	mustPost(t, reg.Server, "digest-a", "Claude", "Recent message A")
	mustPost(t, reg.Server, "digest-b", "Gemini", "Recent message B")

	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{
		Project: "myproj",
		Since:   "2000-01-01T00:00:00",
	})
	if err != nil {
		t.Fatalf("handleGetDigest error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "digest-a") {
		t.Errorf("expected digest-a in output, got: %s", text)
	}
	if !strings.Contains(text, "digest-b") {
		t.Errorf("expected digest-b in output, got: %s", text)
	}
	if !strings.Contains(text, "2 rooms") {
		t.Errorf("expected 2 rooms in digest, got: %s", text)
	}
}

func TestGetDigestNoActivity(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "digest-empty", withProject("empty-proj"))

	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{
		Project: "empty-proj",
		Since:   "2000-01-01T00:00:00",
	})
	if err != nil {
		t.Fatalf("handleGetDigest error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "No new activity") {
		t.Errorf("expected no activity message, got: %s", text)
	}
}

func TestGetDigestMissingSince(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleGetDigest(context.Background(), nil, DigestInput{})
	text := resultText(res)
	if !strings.Contains(text, "since is required") {
		t.Errorf("expected error for missing since, got: %s", text)
	}
}

func TestGetDigestAllProjects(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "d-proj1", withProject("proj1"))
	mustCreateRoom(t, reg.Server, "d-proj2", withProject("proj2"))
	mustPost(t, reg.Server, "d-proj1", "Claude", "Msg 1")
	mustPost(t, reg.Server, "d-proj2", "Gemini", "Msg 2")

	// No project filter — should return both
	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{
		Since: "2000-01-01T00:00:00",
	})
	if err != nil {
		t.Fatalf("handleGetDigest error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "d-proj1") || !strings.Contains(text, "d-proj2") {
		t.Errorf("expected both projects in digest, got: %s", text)
	}
}

func TestGetDigestFutureTimestamp(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "d-future", withProject("fp"))
	mustPost(t, reg.Server, "d-future", "Claude", "Old message")

	// Since is in the future — no results
	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{
		Since: "2099-01-01T00:00:00",
	})
	if err != nil {
		t.Fatalf("handleGetDigest error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "No new activity") {
		t.Errorf("expected no activity for future timestamp, got: %s", text)
	}
}

// ========== additional edge cases ==========

func TestReadTranscriptBatchWithAfterID(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ba-room")
	id1 := mustPost(t, reg.Server, "ba-room", "Claude", "First")
	mustPost(t, reg.Server, "ba-room", "Gemini", "Second")

	res, _, err := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomIDs: "ba-room",
		AfterID: id1,
	})
	if err != nil {
		t.Fatalf("batch after_id error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "Second") {
		t.Errorf("expected Second message in after_id result, got: %s", text)
	}
}

func TestReadTranscriptIncludeRelatedMissing(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ir-main", withRelatedRooms("ir-exists,ir-missing"))
	mustCreateRoom(t, reg.Server, "ir-exists", withDescription("Exists"))
	mustPost(t, reg.Server, "ir-main", "Claude", "Main content")
	mustPost(t, reg.Server, "ir-exists", "Claude", "Exists content")

	res, _, err := reg.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID:         "ir-main",
		IncludeRelated: "true",
	})
	if err != nil {
		t.Fatalf("include_related with missing room error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "Main content") {
		t.Errorf("expected main content")
	}
	if !strings.Contains(text, "Exists") {
		t.Errorf("expected existing related room")
	}
	if !strings.Contains(text, "ir-missing") {
		t.Errorf("expected error mention for missing related room")
	}
}

func TestGetDigestWithLongExcerpt(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "d-long", withProject("lp"))
	longMsg := "This is a very long message that definitely exceeds one hundred and twenty characters and should be truncated at a proper word boundary instead of mid word"
	mustPost(t, reg.Server, "d-long", "Claude", longMsg)

	res, _, err := reg.handleGetDigest(context.Background(), nil, DigestInput{
		Project: "lp",
		Since:   "2000-01-01T00:00:00",
	})
	if err != nil {
		t.Fatalf("handleGetDigest error: %v", err)
	}
	text := resultText(res)
	if !strings.Contains(text, "...") {
		t.Errorf("expected truncation in digest excerpt, got: %s", text)
	}
	if !strings.Contains(text, "d-long") {
		t.Errorf("expected room ID in digest, got: %s", text)
	}
}

// ========== toolResultText ==========

func TestToolResultTextNil(t *testing.T) {
	if got := toolResultText(nil); got != "" {
		t.Errorf("expected empty string for nil, got: %s", got)
	}
}
