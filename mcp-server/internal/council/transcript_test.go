package council

import (
	"strings"
	"testing"
)

func TestTranscriptFormatting(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "fmt-room", withDescription("Formatting test"))
	mustPost(t, cs, "fmt-room", "Claude", "First message")
	mustPost(t, cs, "fmt-room", "Gemini", "Second message")

	room, _ := cs.GetRoom("fmt-room")
	msgs, _ := cs.GetTranscript("fmt-room")
	transcript := FormatTranscript(room, msgs)

	if !strings.Contains(transcript, "# COUNCIL ROOM: fmt-room") {
		t.Error("transcript missing room header")
	}
	if !strings.Contains(transcript, "**Topic:** Formatting test") {
		t.Error("transcript missing topic")
	}
	if !strings.Contains(transcript, "**Status:** active") {
		t.Error("transcript missing status")
	}
	if !strings.Contains(transcript, "Claude") || !strings.Contains(transcript, "Gemini") {
		t.Error("transcript missing authors")
	}
	if !strings.Contains(transcript, "post_to_room") {
		t.Error("transcript missing system instruction")
	}
}

func TestTranscriptWithFullMetadata(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "rich-room", withDescription("JWT refactoring"), withProject("llm-memory"), withTechStack("Go, SQLite"), withTags("auth,security"), withSystemPrompt("Focus on token handling."))
	mustPostTyped(t, cs, "rich-room", "Claude", "I think we should use RS256", "thought")
	mustPostTyped(t, cs, "rich-room", "Gemini", "Agreed, let's proceed", "decision")

	room, _ := cs.GetRoom("rich-room")
	msgs, _ := cs.GetTranscript("rich-room")
	transcript := FormatTranscript(room, msgs)

	if !strings.Contains(transcript, "**Project:** llm-memory") {
		t.Error("transcript missing project")
	}
	if !strings.Contains(transcript, "**Tech Stack:** Go, SQLite") {
		t.Error("transcript missing tech stack")
	}
	if !strings.Contains(transcript, "**Tags:** auth,security") {
		t.Error("transcript missing tags")
	}
	if !strings.Contains(transcript, "*Instructions: Focus on token handling.*") {
		t.Error("transcript missing system prompt")
	}
	if !strings.Contains(transcript, "Claude (thought)") {
		t.Error("transcript missing message type for thought")
	}
	if !strings.Contains(transcript, "Gemini (decision)") {
		t.Error("transcript missing message type for decision")
	}
}

func TestTranscriptWithSummary(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "sum-room")
	for i := 0; i < 5; i++ {
		mustPost(t, cs, "sum-room", "Claude", "Old message")
	}

	cs.InsertSummary("sum-room", "Summary of 5 old messages")

	mustPost(t, cs, "sum-room", "Gemini", "New message after summary")

	msgs, err := cs.GetTranscript("sum-room")
	if err != nil {
		t.Fatalf("getTranscript failed: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (summary + new), got %d", len(msgs))
	}

	if !msgs[0].IsSummary {
		t.Error("first message should be the summary")
	}
	if msgs[1].Author != "Gemini" {
		t.Errorf("second message should be from Gemini, got '%s'", msgs[1].Author)
	}
}

func TestTranscriptWithRelatedRooms(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "linked-room", withProject("proj"), withRelatedRooms("other-room,another-room"))
	mustPost(t, cs, "linked-room", "Claude", "Test")

	room, _ := cs.GetRoom("linked-room")
	msgs, _ := cs.GetTranscript("linked-room")
	transcript := FormatTranscript(room, msgs)

	if !strings.Contains(transcript, "**Related Rooms:** other-room,another-room") {
		t.Error("transcript missing related rooms")
	}
}



// -- formatTranscript edge: plain message with reply_to --

func TestFormatTranscriptReplyToPlainMessage(t *testing.T) {
	room := Room{ID: "fmt-reply", Description: "Test", Status: "active"}
	msgs := []Message{
		{ID: "uuid-0001", Author: "Claude", Content: "Original", MessageType: "message", ReplyTo: ""},
		{ID: "uuid-0002", Author: "Gemini", Content: "Reply", MessageType: "message", ReplyTo: "uuid-0001"},
	}

	transcript := FormatTranscript(room, msgs)
	if !strings.Contains(transcript, "Gemini (re: #uuid-000") {
		t.Errorf("expected plain message reply rendering, got: %s", transcript)
	}
}

// -- formatTranscript with summary rendering --

func TestFormatTranscriptWithSummary(t *testing.T) {
	room := Room{ID: "sum-fmt", Description: "Test", Status: "active"}
	msgs := []Message{
		{ID: "uuid-0001", Author: "System", Content: "Summary of prior discussion", IsSummary: true},
		{ID: "uuid-0002", Author: "Claude", Content: "New point", MessageType: "message"},
	}
	transcript := FormatTranscript(room, msgs)
	if !strings.Contains(transcript, "SUMMARY") {
		t.Error("missing summary in transcript")
	}
	if !strings.Contains(transcript, "Summary of prior discussion") {
		t.Error("missing summary content")
	}
}

// ========== Janitor / Knowledge Linter ==========

func TestHasTag(t *testing.T) {
	if !hasTag("foo,bar,baz", "bar") {
		t.Error("expected hasTag to find 'bar'")
	}
	if hasTag("foo,bar,baz", "ba") {
		t.Error("hasTag should not match substring 'ba'")
	}
	if hasTag("", "foo") {
		t.Error("hasTag should return false for empty tags")
	}
	if !hasTag("needs-synthesis", "needs-synthesis") {
		t.Error("hasTag should match single tag")
	}
	if hasTag("stale-review,other", "stale") {
		t.Error("hasTag should not match prefix 'stale' in 'stale-review'")
	}
}

func TestAppendTag(t *testing.T) {
	if result := appendTag("", "stale"); result != "stale" {
		t.Errorf("expected 'stale', got '%s'", result)
	}
	if result := appendTag("foo,bar", "baz"); result != "foo,bar,baz" {
		t.Errorf("expected 'foo,bar,baz', got '%s'", result)
	}
	if result := appendTag("foo,stale", "stale"); result != "foo,stale" {
		t.Errorf("appendTag should not duplicate, got '%s'", result)
	}
}

func TestLintNeedsSynthesis(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-synth")
	mustPostTyped(t, cs, "lint-synth", "Claude", "We should use Postgres", "decision")

	cs.lintNeedsSynthesis()

	room, _ := cs.GetRoom("lint-synth")
	if !hasTag(room.Tags, "needs-synthesis") {
		t.Errorf("expected 'needs-synthesis' tag, got '%s'", room.Tags)
	}

	// Check that a system message was posted
	msgs, _ := cs.GetRecentMessages("lint-synth", 5)
	found := false
	for _, m := range msgs {
		if m.Author == "system" && strings.Contains(m.Content, "Knowledge Linter") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected linter system message in room")
	}
}

func TestLintNeedsSynthesisSkipsWhenSynthesisExists(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-has-synth")
	mustPostTyped(t, cs, "lint-has-synth", "Claude", "Decision made", "decision")
	mustPostTyped(t, cs, "lint-has-synth", "Claude", "Compiled article", "synthesis")

	cs.lintNeedsSynthesis()

	room, _ := cs.GetRoom("lint-has-synth")
	if hasTag(room.Tags, "needs-synthesis") {
		t.Error("should not flag room that already has synthesis")
	}
}

func TestLintNeedsSynthesisIdempotent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-idem")
	mustPostTyped(t, cs, "lint-idem", "Claude", "A decision", "decision")

	cs.lintNeedsSynthesis()
	cs.lintNeedsSynthesis() // run twice

	room, _ := cs.GetRoom("lint-idem")
	count := 0
	for _, tag := range strings.Split(room.Tags, ",") {
		if strings.TrimSpace(tag) == "needs-synthesis" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'needs-synthesis' tag, got %d in '%s'", count, room.Tags)
	}
}

func TestLintStaleRooms(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-stale")
	mustPost(t, cs, "lint-stale", "Claude", "Old message")

	// Backdate the message to 8 days ago
	cs.DB.Exec(`UPDATE messages SET timestamp = datetime('now', '-8 days') WHERE room_id = 'lint-stale'`)

	cs.lintStaleRooms()

	room, _ := cs.GetRoom("lint-stale")
	if !hasTag(room.Tags, "stale") {
		t.Errorf("expected 'stale' tag, got '%s'", room.Tags)
	}
}

func TestLintStaleSkipsRecentRooms(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-fresh")
	mustPost(t, cs, "lint-fresh", "Claude", "Recent message")

	cs.lintStaleRooms()

	room, _ := cs.GetRoom("lint-fresh")
	if hasTag(room.Tags, "stale") {
		t.Error("should not flag room with recent activity")
	}
}

func TestJanitorSweepRunsBothLinters(t *testing.T) {
	cs := setupTestServer(t)

	// Room needing synthesis
	mustCreateRoom(t, cs, "sweep-synth")
	mustPostTyped(t, cs, "sweep-synth", "Claude", "Decision here", "decision")

	// Stale room
	mustCreateRoom(t, cs, "sweep-stale")
	mustPost(t, cs, "sweep-stale", "Claude", "Old msg")
	cs.DB.Exec(`UPDATE messages SET timestamp = datetime('now', '-10 days') WHERE room_id = 'sweep-stale'`)

	cs.JanitorSweep()

	r1, _ := cs.GetRoom("sweep-synth")
	if !hasTag(r1.Tags, "needs-synthesis") {
		t.Errorf("JanitorSweep should flag needs-synthesis, got '%s'", r1.Tags)
	}

	r2, _ := cs.GetRoom("sweep-stale")
	if !hasTag(r2.Tags, "stale") {
		t.Errorf("JanitorSweep should flag stale, got '%s'", r2.Tags)
	}
}

