package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ========== read_transcript (basic) ==========

func TestHandleReadTranscript(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript")
	mustPost(t, cs, "h-transcript", "Claude", "Hello world")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{RoomID: "h-transcript"})
	text := resultText(res)
	if !strings.Contains(text, "COUNCIL ROOM: h-transcript") {
		t.Error("missing room header")
	}
	if !strings.Contains(text, "Hello world") {
		t.Error("missing message content")
	}
}

func TestHandleReadTranscriptMissing(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{})
	if !strings.Contains(resultText(res), "Error") {
		t.Error("expected error")
	}
}

func TestHandleReadTranscriptNotFound(t *testing.T) {
	cs := setupTestServer(t)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{RoomID: "ghost"})
	if !strings.Contains(resultText(res), "not found") {
		t.Error("expected not found")
	}
}

func TestHandleTranscriptResourceDBError(t *testing.T) {
	cs := setupHandlerServer(t)
	cs.db.Exec("DROP TABLE messages")

	_, err := cs.handleTranscript(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "council://room/hdb-room/transcript"},
	})
	if err == nil {
		t.Error("expected error when messages table is missing")
	}
}

// ========== read_transcript (last_n) ==========

func TestHandleReadTranscriptWithLastN(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-ln", withProject("proj"), withSystemPrompt("Be concise."))
	for i := 0; i < 10; i++ {
		mustPost(t, cs, "h-transcript-ln", "Claude", fmt.Sprintf("Message %d", i))
	}

	// last_n=3 should return only the last 3 messages but still include room header and system prompt
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-ln", LastN: "3",
	})
	text := resultText(res)

	// Room header must be present
	if !strings.Contains(text, "COUNCIL ROOM: h-transcript-ln") {
		t.Error("missing room header")
	}
	// System prompt must be present
	if !strings.Contains(text, "Instructions: Be concise.") {
		t.Error("missing system prompt in last_n transcript")
	}
	// Should have Message 7, 8, 9 but not 0-6
	if !strings.Contains(text, "Message 7") {
		t.Error("expected Message 7 in last 3")
	}
	if !strings.Contains(text, "Message 9") {
		t.Error("expected Message 9 in last 3")
	}
	if strings.Contains(text, "Message 6") {
		t.Error("Message 6 should not be in last 3")
	}
	if strings.Contains(text, "Message 0") {
		t.Error("Message 0 should not be in last 3")
	}
}

func TestHandleReadTranscriptLastNPreservesSummaries(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-sum-ln")

	for i := 0; i < 5; i++ {
		mustPost(t, cs, "h-transcript-sum-ln", "Claude", "Old msg")
	}
	cs.insertSummary("h-transcript-sum-ln", "Summary of 5 old messages")
	for i := 0; i < 5; i++ {
		mustPost(t, cs, "h-transcript-sum-ln", "Gemini", fmt.Sprintf("New msg %d", i))
	}

	// last_n=2 should keep summaries + last 2 regular messages
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-sum-ln", LastN: "2",
	})
	text := resultText(res)

	if !strings.Contains(text, "SUMMARY") {
		t.Error("summary should be preserved with last_n")
	}
	if !strings.Contains(text, "New msg 4") {
		t.Error("expected last message in last_n=2")
	}
	if !strings.Contains(text, "New msg 3") {
		t.Error("expected second-to-last message in last_n=2")
	}
	if strings.Contains(text, "New msg 0") {
		t.Error("New msg 0 should not be in last_n=2")
	}
}

func TestHandleReadTranscriptLastNOmittedReturnsAll(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-all")
	for i := 0; i < 5; i++ {
		mustPost(t, cs, "h-transcript-all", "Claude", fmt.Sprintf("Msg %d", i))
	}

	// No last_n — should return all 5 messages
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-all",
	})
	text := resultText(res)
	if !strings.Contains(text, "Msg 0") || !strings.Contains(text, "Msg 4") {
		t.Error("expected all messages when last_n is omitted")
	}
}

func TestHandleReadTranscriptLastNInvalid(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-bad-ln")
	for i := 0; i < 3; i++ {
		mustPost(t, cs, "h-transcript-bad-ln", "Claude", fmt.Sprintf("Msg %d", i))
	}

	// Invalid last_n should return all (graceful fallback)
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-bad-ln", LastN: "abc",
	})
	text := resultText(res)
	if !strings.Contains(text, "Msg 0") || !strings.Contains(text, "Msg 2") {
		t.Error("invalid last_n should return all messages")
	}
}

func TestHandleReadTranscriptLastNLargerThanTotal(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-big-ln")
	mustPost(t, cs, "h-transcript-big-ln", "Claude", "Only message")

	// last_n=100 with only 1 message should return all
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-big-ln", LastN: "100",
	})
	text := resultText(res)
	if !strings.Contains(text, "Only message") {
		t.Error("expected the single message")
	}
}

// ========== read_transcript (after_id) ==========

func TestHandleReadTranscriptAfterID(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-after")
	mustPost(t, cs, "h-transcript-after", "Claude", "First")
	id2 := mustPostTyped(t, cs, "h-transcript-after", "Gemini", "Second", "thought")
	mustPostTyped(t, cs, "h-transcript-after", "Claude", "Third", "decision")

	// Get messages after the second one — should return only "Third"
	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-after", AfterID: fmt.Sprintf("%d", id2),
	})
	text := resultText(res)

	if !strings.Contains(text, "1 message(s) after") {
		t.Errorf("expected 1 message after header, got: %s", text)
	}
	if !strings.Contains(text, "Third") {
		t.Error("expected 'Third' in after_id result")
	}
	if strings.Contains(text, "First") || strings.Contains(text, "Second") {
		t.Error("should not contain messages before after_id")
	}
}

func TestHandleReadTranscriptAfterIDNoNewMessages(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-after-empty")
	id1 := mustPost(t, cs, "h-transcript-after-empty", "Claude", "Only one")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-after-empty", AfterID: fmt.Sprintf("%d", id1),
	})
	text := resultText(res)
	if !strings.Contains(text, "0 message(s) after") {
		t.Errorf("expected 0 messages, got: %s", text)
	}
}

func TestHandleReadTranscriptAfterIDInvalid(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-after-bad")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-after-bad", AfterID: "not-a-number",
	})
	text := resultText(res)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error for invalid after_id, got: %s", text)
	}
}

func TestHandleReadTranscriptAfterIDWithTypedMessages(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-after-typed")
	id1 := mustPost(t, cs, "h-transcript-after-typed", "Claude", "Base")
	mustPostTyped(t, cs, "h-transcript-after-typed", "Gemini", "A thought", "thought")
	cs.postMessage("h-transcript-after-typed", "Claude", "A decision", "decision", id1)

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-after-typed", AfterID: fmt.Sprintf("%d", id1),
	})
	text := resultText(res)
	if !strings.Contains(text, "thought") {
		t.Error("expected thought type in after_id result")
	}
	if !strings.Contains(text, "decision") {
		t.Error("expected decision type in after_id result")
	}
}

func TestHandleReadTranscriptAfterIDLatestID(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-afterid-latest")
	id1 := mustPost(t, cs, "h-transcript-afterid-latest", "Claude", "First")
	mustPostTyped(t, cs, "h-transcript-afterid-latest", "Gemini", "Second", "thought")
	id3 := mustPostTyped(t, cs, "h-transcript-afterid-latest", "Claude", "Third", "decision")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-afterid-latest", AfterID: fmt.Sprintf("%d", id1),
	})
	text := resultText(res)

	// Header should include latest ID
	expected := fmt.Sprintf("(latest: #%d)", id3)
	if !strings.Contains(text, expected) {
		t.Errorf("expected latest_id in header, got: %s", text)
	}
	if !strings.Contains(text, "2 message(s) after") {
		t.Errorf("expected 2 messages, got: %s", text)
	}
}

func TestHandleReadTranscriptAfterIDNoMessagesNoLatest(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-afterid-empty")
	id1 := mustPost(t, cs, "h-transcript-afterid-empty", "Claude", "Only one")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-afterid-empty", AfterID: fmt.Sprintf("%d", id1),
	})
	text := resultText(res)

	// With no messages after, should not show "latest:"
	if strings.Contains(text, "latest:") {
		t.Errorf("should not show latest_id when no new messages, got: %s", text)
	}
}

// ========== read_transcript (summary mode) ==========

func TestHandleReadTranscriptSummaryMode(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-summode", withSystemPrompt("Focus on security."))
	mustPostTyped(t, cs, "h-transcript-summode", "Claude", "Old thought", "thought")
	mustPostTyped(t, cs, "h-transcript-summode", "Claude", "Latest thought", "thought")
	mustPostTyped(t, cs, "h-transcript-summode", "Gemini", "A decision was made", "decision")
	mustPostTyped(t, cs, "h-transcript-summode", "Claude", "Do this action", "action")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-summode", Mode: "summary",
	})
	text := resultText(res)

	// Should include system prompt
	if !strings.Contains(text, "Focus on security.") {
		t.Error("summary mode should include system prompt")
	}
	// Should have summary header
	if !strings.Contains(text, "summary") {
		t.Error("expected summary header")
	}
	// Should include latest per type, not the old thought
	if !strings.Contains(text, "Latest thought") {
		t.Error("expected latest thought")
	}
	if strings.Contains(text, "Old thought") {
		t.Error("should not contain old thought, only latest per type")
	}
	if !strings.Contains(text, "A decision was made") {
		t.Error("expected the decision")
	}
	if !strings.Contains(text, "Do this action") {
		t.Error("expected the action")
	}
}

func TestHandleReadTranscriptSummaryModeEmpty(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-summode-empty")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-summode-empty", Mode: "summary",
	})
	text := resultText(res)
	if !strings.Contains(text, "No messages yet") {
		t.Errorf("expected 'No messages yet', got: %s", text)
	}
}

func TestHandleReadTranscriptSummaryModeTruncatesLongContent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-summode-long")
	longContent := strings.Repeat("X", 300)
	mustPostTyped(t, cs, "h-transcript-summode-long", "Claude", longContent, "thought")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-summode-long", Mode: "summary",
	})
	text := resultText(res)
	if !strings.Contains(text, "...") {
		t.Error("expected truncated content in summary mode")
	}
	if strings.Contains(text, strings.Repeat("X", 300)) {
		t.Error("full 300-char content should not appear in summary mode")
	}
}

// ========== read_transcript (changelog mode) ==========

func TestHandleReadTranscriptChangelog(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-cl")
	mustPostTyped(t, cs, "h-transcript-cl", "Claude", "Thinking about options", "thought")
	mustPostTyped(t, cs, "h-transcript-cl", "Claude", "Let's use RS256", "decision")
	mustPostTyped(t, cs, "h-transcript-cl", "Gemini", "Reviewing approach", "review")
	mustPostTyped(t, cs, "h-transcript-cl", "Claude", "Implemented RS256 in auth.go", "action")
	mustPostTyped(t, cs, "h-transcript-cl", "Gemini", "Some critique", "critique")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-cl", Mode: "changelog",
	})
	text := resultText(res)

	if !strings.Contains(text, "changelog") {
		t.Error("expected changelog header")
	}
	// Should contain decision and action
	if !strings.Contains(text, "Let's use RS256") {
		t.Error("expected decision in changelog")
	}
	if !strings.Contains(text, "Implemented RS256") {
		t.Error("expected action in changelog")
	}
	// Should NOT contain thought, review, critique
	if strings.Contains(text, "Thinking about") {
		t.Error("changelog should not contain thoughts")
	}
	if strings.Contains(text, "Reviewing approach") {
		t.Error("changelog should not contain reviews")
	}
	if strings.Contains(text, "Some critique") {
		t.Error("changelog should not contain critiques")
	}
}

func TestHandleReadTranscriptChangelogEmpty(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "h-transcript-cl-empty")
	mustPostTyped(t, cs, "h-transcript-cl-empty", "Claude", "Just a thought", "thought")

	res, _, _ := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "h-transcript-cl-empty", Mode: "changelog",
	})
	text := resultText(res)
	if !strings.Contains(text, "No decisions or actions") {
		t.Errorf("expected empty changelog message, got: %s", text)
	}
}

// ========== read_transcript (pinned messages) ==========

func TestPinnedMessageInTranscript(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "transcript-pin")
	mustPost(t, cs, "transcript-pin", "Claude", "Regular message 1")
	id2 := mustPostTyped(t, cs, "transcript-pin", "Gemini", "This is the TL;DR", "decision")
	mustPost(t, cs, "transcript-pin", "Claude", "Regular message 3")

	cs.pinMessage("transcript-pin", id2)

	room, _ := cs.getRoom("transcript-pin")
	msgs, _ := cs.getTranscript("transcript-pin")
	transcript := formatTranscript(room, msgs)

	// Pinned message should appear before regular messages
	pinnedIdx := strings.Index(transcript, "PINNED")
	msg1Idx := strings.Index(transcript, "Regular message 1")
	msg3Idx := strings.Index(transcript, "Regular message 3")

	if pinnedIdx == -1 {
		t.Fatal("PINNED label not found in transcript")
	}
	if pinnedIdx > msg1Idx {
		t.Error("pinned message should appear before first regular message")
	}
	if pinnedIdx > msg3Idx {
		t.Error("pinned message should appear before last regular message")
	}

	// Pinned content should appear only once (not duplicated)
	count := strings.Count(transcript, "This is the TL;DR")
	if count != 1 {
		t.Errorf("pinned message content should appear exactly once, appeared %d times", count)
	}
}

func TestPinnedMessageInSummaryMode(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "summary-pin", withSystemPrompt("Test instructions"))
	id := mustPostTyped(t, cs, "summary-pin", "Claude", "Pinned summary content", "decision")
	mustPostTyped(t, cs, "summary-pin", "Gemini", "A thought", "thought")

	cs.pinMessage("summary-pin", id)

	res, _, err := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID: "summary-pin", Mode: "summary",
	})
	if err != nil {
		t.Fatalf("handleReadTranscript error: %v", err)
	}
	text := resultText(res)

	if !strings.Contains(text, "PINNED") {
		t.Error("summary mode should include PINNED label")
	}
	if !strings.Contains(text, "Pinned summary content") {
		t.Error("summary mode should include pinned message content")
	}
}

func TestPinnedMessageInAfterIDMode(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "afterid-pin")
	id1 := mustPostTyped(t, cs, "afterid-pin", "Claude", "Pinned overview", "decision")
	mustPost(t, cs, "afterid-pin", "Gemini", "Second msg")
	id3 := mustPostTyped(t, cs, "afterid-pin", "Claude", "Third msg", "action")

	cs.pinMessage("afterid-pin", id1)

	// Read after id2 — should still include pinned message at top
	res, _, err := cs.handleReadTranscript(context.Background(), nil, ReadTranscriptInput{
		RoomID:  "afterid-pin",
		AfterID: fmt.Sprintf("%d", id3-1),
	})
	if err != nil {
		t.Fatalf("handleReadTranscript error: %v", err)
	}
	text := resultText(res)

	if !strings.Contains(text, "PINNED") {
		t.Error("after_id mode should include PINNED message for context")
	}
	if !strings.Contains(text, "Pinned overview") {
		t.Error("after_id mode should include pinned message content")
	}
}
