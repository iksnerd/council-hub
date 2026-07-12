package council

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"
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

// -- formatTranscript resolves {sha:...} commit tokens when the room has a repo --

func TestFormatTranscriptResolvesCommitRefs(t *testing.T) {
	room := Room{ID: "sha-room", Description: "Test", Status: "active", Repo: "iksnerd/council-hub"}
	msgs := []Message{
		{ID: "uuid-0001", Author: "Claude", Content: "Shipped {sha:89cfaf1abc} — done", MessageType: "action"},
	}
	transcript := FormatTranscript(room, msgs)
	if !strings.Contains(transcript, "[`89cfaf1`](https://github.com/iksnerd/council-hub/commit/89cfaf1abc)") {
		t.Errorf("expected resolved commit link in transcript, got: %s", transcript)
	}
	if strings.Contains(transcript, "{sha:") {
		t.Errorf("raw {sha:...} token should not survive rendering, got: %s", transcript)
	}
}

func TestFormatTranscriptCommitRefsNoRepoFallsBack(t *testing.T) {
	room := Room{ID: "sha-room-norepo", Description: "Test", Status: "active"}
	msgs := []Message{
		{ID: "uuid-0001", Author: "Claude", Content: "Shipped {sha:89cfaf1abc}", MessageType: "action"},
	}
	transcript := FormatTranscript(room, msgs)
	if !strings.Contains(transcript, "`89cfaf1`") {
		t.Errorf("expected bare short-SHA code span when no repo set, got: %s", transcript)
	}
	if strings.Contains(transcript, "https://") {
		t.Errorf("no link should be produced without a repo, got: %s", transcript)
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
	mustPostTyped(t, cs, "lint-synth", "Claude", "We should use Go", "decision")
	mustPostTyped(t, cs, "lint-synth", "Claude", "We should use Docker", "decision")
	cs.DB.Exec(`UPDATE rooms SET created_at = datetime('now', '-2 days') WHERE id = 'lint-synth'`)

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
	mustPostTyped(t, cs, "lint-idem", "Claude", "Decision 1", "decision")
	mustPostTyped(t, cs, "lint-idem", "Claude", "Decision 2", "decision")
	mustPostTyped(t, cs, "lint-idem", "Claude", "Decision 3", "decision")
	cs.DB.Exec(`UPDATE rooms SET created_at = datetime('now', '-2 days') WHERE id = 'lint-idem'`)

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

	// Backdate the message to 8 days ago and room to bypass grace period
	cs.DB.Exec(`UPDATE messages SET timestamp = datetime('now', '-8 days') WHERE room_id = 'lint-stale'`)
	cs.DB.Exec(`UPDATE rooms SET created_at = datetime('now', '-2 days') WHERE id = 'lint-stale'`)

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

func TestLintStalePins(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-pin")
	pinID := mustPostTyped(t, cs, "lint-pin", "Claude", "Old TL;DR", "synthesis")
	if _, err := cs.PinMessage("lint-pin", pinID); err != nil {
		t.Fatal(err)
	}
	// 5 decision/action messages land after the pin.
	for i := 0; i < 5; i++ {
		mustPostTyped(t, cs, "lint-pin", "Claude", "Update", "decision")
	}

	cs.lintStalePins()

	room, _ := cs.GetRoom("lint-pin")
	if !hasTag(room.Tags, "stale-pin") {
		t.Errorf("expected 'stale-pin' tag, got '%s'", room.Tags)
	}

	// A system note should have been posted.
	msgs, _ := cs.GetRecentMessages("lint-pin", 10)
	found := false
	for _, m := range msgs {
		if m.Author == "system" && strings.Contains(m.Content, "pinned summary") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected stale-pin linter system message in room")
	}
}

func TestLintStalePinsSkipsFreshPin(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-fresh-pin")
	// Updates land first, then the pin — nothing qualifying is newer than the pin.
	for i := 0; i < 5; i++ {
		mustPostTyped(t, cs, "lint-fresh-pin", "Claude", "Update", "decision")
	}
	pinID := mustPostTyped(t, cs, "lint-fresh-pin", "Claude", "Fresh TL;DR", "synthesis")
	if _, err := cs.PinMessage("lint-fresh-pin", pinID); err != nil {
		t.Fatal(err)
	}

	cs.lintStalePins()

	room, _ := cs.GetRoom("lint-fresh-pin")
	if hasTag(room.Tags, "stale-pin") {
		t.Errorf("fresh pin should not be flagged, got '%s'", room.Tags)
	}
}

func TestLintStalePinsBelowThreshold(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-pin-few")
	pinID := mustPostTyped(t, cs, "lint-pin-few", "Claude", "TL;DR", "synthesis")
	if _, err := cs.PinMessage("lint-pin-few", pinID); err != nil {
		t.Fatal(err)
	}
	// Only 4 updates after the pin — under the threshold of 5.
	for i := 0; i < 4; i++ {
		mustPostTyped(t, cs, "lint-pin-few", "Claude", "Update", "decision")
	}

	cs.lintStalePins()

	room, _ := cs.GetRoom("lint-pin-few")
	if hasTag(room.Tags, "stale-pin") {
		t.Errorf("below-threshold room should not be flagged, got '%s'", room.Tags)
	}
}

func TestLintStalePinsIdempotent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-pin-idem")
	pinID := mustPostTyped(t, cs, "lint-pin-idem", "Claude", "TL;DR", "synthesis")
	if _, err := cs.PinMessage("lint-pin-idem", pinID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		mustPostTyped(t, cs, "lint-pin-idem", "Claude", "Update", "action")
	}

	cs.lintStalePins()
	cs.lintStalePins() // run twice

	room, _ := cs.GetRoom("lint-pin-idem")
	count := 0
	for _, tag := range strings.Split(room.Tags, ",") {
		if strings.TrimSpace(tag) == "stale-pin" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'stale-pin' tag, got %d in '%s'", count, room.Tags)
	}
}

func TestLintStalePlans(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-plan")
	mustPostTyped(t, cs, "lint-plan", "Claude", "Here's the plan to execute", "plan")

	cs.lintStalePlans()

	room, _ := cs.GetRoom("lint-plan")
	if !hasTag(room.Tags, "stale-plan") {
		t.Errorf("expected 'stale-plan' tag, got '%s'", room.Tags)
	}
	msgs, _ := cs.GetRecentMessages("lint-plan", 5)
	found := false
	for _, m := range msgs {
		if m.Author == "system" && strings.Contains(m.Content, "no") && strings.Contains(m.Content, "action") {
			found = true
		}
	}
	if !found {
		t.Error("expected stale-plan linter system message in room")
	}
}

func TestLintStalePlansSkipsWhenActionFollows(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-plan-done")
	mustPostTyped(t, cs, "lint-plan-done", "Claude", "the plan", "plan")
	mustPostTyped(t, cs, "lint-plan-done", "Gemini", "shipped it", "action")

	cs.lintStalePlans()

	room, _ := cs.GetRoom("lint-plan-done")
	if hasTag(room.Tags, "stale-plan") {
		t.Errorf("a plan with a following action should not be flagged, got '%s'", room.Tags)
	}
}

func TestLintStalePlansReopensOnNewerPlan(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-plan-reopen")
	mustPostTyped(t, cs, "lint-plan-reopen", "Claude", "plan v1", "plan")
	mustPostTyped(t, cs, "lint-plan-reopen", "Gemini", "did v1", "action")
	// A newer plan with no action after it — the latest handoff is unexecuted.
	mustPostTyped(t, cs, "lint-plan-reopen", "Claude", "plan v2", "plan")

	cs.lintStalePlans()

	room, _ := cs.GetRoom("lint-plan-reopen")
	if !hasTag(room.Tags, "stale-plan") {
		t.Errorf("a newer plan with no following action should re-flag, got '%s'", room.Tags)
	}
}

func TestPostActionClearsStalePlan(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "plan-clear", withTags("stale-plan,important"))
	mustPostTyped(t, cs, "plan-clear", "Claude", "shipped", "action")

	room, _ := cs.GetRoom("plan-clear")
	if hasTag(room.Tags, "stale-plan") {
		t.Errorf("posting an action should clear 'stale-plan', got '%s'", room.Tags)
	}
	if !hasTag(room.Tags, "important") {
		t.Errorf("non-linter tags should survive, got '%s'", room.Tags)
	}
}

func TestLintStalePlansIdempotent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-plan-idem")
	mustPostTyped(t, cs, "lint-plan-idem", "Claude", "the plan", "plan")

	cs.lintStalePlans()
	cs.lintStalePlans()

	room, _ := cs.GetRoom("lint-plan-idem")
	count := 0
	for _, tag := range strings.Split(room.Tags, ",") {
		if strings.TrimSpace(tag) == "stale-plan" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'stale-plan' tag, got %d in '%s'", count, room.Tags)
	}
}

func TestPinMessageClearsStalePin(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "repin-room", withTags("stale-pin,important"))
	freshID := mustPostTyped(t, cs, "repin-room", "Claude", "New TL;DR", "synthesis")

	if _, err := cs.PinMessage("repin-room", freshID); err != nil {
		t.Fatal(err)
	}

	room, _ := cs.GetRoom("repin-room")
	if hasTag(room.Tags, "stale-pin") {
		t.Errorf("re-pin should clear 'stale-pin', got '%s'", room.Tags)
	}
	if !hasTag(room.Tags, "important") {
		t.Errorf("non-linter tags should survive re-pin, got '%s'", room.Tags)
	}
}

func TestJanitorSweepRunsBothLinters(t *testing.T) {
	cs := setupTestServer(t)

	// Room needing synthesis: 3 decisions, older than 24h grace period
	mustCreateRoom(t, cs, "sweep-synth")
	mustPostTyped(t, cs, "sweep-synth", "Claude", "Decision 1", "decision")
	mustPostTyped(t, cs, "sweep-synth", "Claude", "Decision 2", "decision")
	mustPostTyped(t, cs, "sweep-synth", "Claude", "Decision 3", "decision")
	cs.DB.Exec(`UPDATE rooms SET created_at = datetime('now', '-2 days') WHERE id = 'sweep-synth'`)

	// Stale room: message older than 7 days, room older than 24h
	mustCreateRoom(t, cs, "sweep-stale")
	mustPost(t, cs, "sweep-stale", "Claude", "Old msg")
	cs.DB.Exec(`UPDATE messages SET timestamp = datetime('now', '-10 days') WHERE room_id = 'sweep-stale'`)
	cs.DB.Exec(`UPDATE rooms SET created_at = datetime('now', '-2 days') WHERE id = 'sweep-stale'`)

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

func TestJanitorSweepRunsIntegrityCheck(t *testing.T) {
	cs := setupTestServer(t)

	before := cs.LastIntegrityCheck
	cs.JanitorSweep()
	after := cs.LastIntegrityCheck

	if !after.After(before) {
		t.Errorf("expected LastIntegrityCheck to advance after JanitorSweep (before=%v, after=%v)", before, after)
	}
}

// ========== buildEpitaph ==========

func TestBuildEpitaph(t *testing.T) {
	msgs := []Message{
		{ID: "1", MessageType: "thought", Author: "Claude", Content: "a thought"},
		{ID: "2", MessageType: "decision", Author: "Claude", Content: "use postgres"},
		{ID: "3", MessageType: "action", Author: "Gemini", Content: "deployed to prod"},
		{ID: "4", MessageType: "decision", Author: "Gemini", Content: "switch to redis"},
	}

	out := buildEpitaph(msgs)

	if !strings.Contains(out, "## Summary") {
		t.Error("expected ## Summary header")
	}
	if !strings.Contains(out, "switch to redis") {
		t.Errorf("expected last decision, got: %s", out)
	}
	if !strings.Contains(out, "deployed to prod") {
		t.Errorf("expected last action, got: %s", out)
	}
	// First decision should be superseded
	if strings.Contains(out, "use postgres") {
		t.Errorf("earlier decision should not appear, got: %s", out)
	}
}

func TestBuildEpitaphNoMessages(t *testing.T) {
	out := buildEpitaph([]Message{})
	if out != "" {
		t.Errorf("expected empty epitaph with no messages, got: %s", out)
	}
}

func TestBuildEpitaphDecisionOnly(t *testing.T) {
	msgs := []Message{
		{ID: "1", MessageType: "decision", Author: "Claude", Content: "chose kafka"},
	}
	out := buildEpitaph(msgs)
	if !strings.Contains(out, "chose kafka") {
		t.Errorf("expected decision in epitaph, got: %s", out)
	}
	if strings.Contains(out, "Last action") {
		t.Errorf("should not include Last action when none exists, got: %s", out)
	}
}

// ========== ListArchives / ReadArchive ==========

func TestListArchivesEmpty(t *testing.T) {
	s := setupTestServer(t)
	// Ensure the archive dir doesn't exist for this test
	t.Cleanup(func() { _ = os.RemoveAll(s.archiveDir()) })
	_ = os.RemoveAll(s.archiveDir())

	archives, err := s.ListArchives()
	if err != nil {
		t.Fatalf("ListArchives error: %v", err)
	}
	if len(archives) != 0 {
		t.Errorf("expected no archives, got %d", len(archives))
	}
}

func TestListArchivesAndReadArchive(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "arc-room")
	mustPostTyped(t, s, "arc-room", "Claude", "we decided X", "decision")

	path, err := s.ArchiveRoom("arc-room")
	if err != nil {
		t.Fatalf("ArchiveRoom error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty archive path")
	}

	archives, err := s.ListArchives()
	if err != nil {
		t.Fatalf("ListArchives error: %v", err)
	}
	found := false
	for _, a := range archives {
		if a.RoomID == "arc-room" {
			found = true
			if a.Size == 0 {
				t.Error("expected non-zero archive size")
			}
		}
	}
	if !found {
		t.Errorf("arc-room not found in archives: %v", archives)
	}

	content, err := s.ReadArchive("arc-room")
	if err != nil {
		t.Fatalf("ReadArchive error: %v", err)
	}
	if !strings.Contains(content, "we decided X") {
		t.Errorf("expected message content in archive, got: %s", content)
	}
}

func TestReadArchiveNotFound(t *testing.T) {
	s := setupTestServer(t)
	_, err := s.ReadArchive("nonexistent-room")
	if err == nil {
		t.Error("expected error for nonexistent archive")
	}
}

// ========== RunJanitor ==========

func TestRunJanitorStopsOnContextCancel(t *testing.T) {
	s := setupTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.RunJanitor(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// passed
	case <-time.After(2 * time.Second):
		t.Error("RunJanitor did not stop after context cancellation")
	}
}

func TestBuildEpitaphRetractedDecisionTombstone(t *testing.T) {
	msgs := []Message{
		{ID: "1", MessageType: "decision", Author: "Claude", Content: "the withdrawn call",
			RetractedAt: sql.NullTime{Time: time.Now(), Valid: true}, RetractedBy: "Claude"},
	}
	out := buildEpitaph(msgs)
	// Archives are permanent — a retracted decision must render as its tombstone,
	// never leak the withdrawn body.
	if strings.Contains(out, "the withdrawn call") {
		t.Errorf("retracted content leaked into epitaph: %s", out)
	}
	if !strings.Contains(out, "[retracted by Claude]") {
		t.Errorf("expected tombstone in epitaph, got: %s", out)
	}
}

func TestBuildEpitaphLongContentTruncated(t *testing.T) {
	longContent := strings.Repeat("word ", 100) // 500 chars
	msgs := []Message{
		{ID: "1", MessageType: "decision", Author: "Claude", Content: longContent},
	}
	out := buildEpitaph(msgs)
	if !strings.Contains(out, "...") {
		t.Errorf("expected long content to be truncated with ..., got: %s", out)
	}
}
