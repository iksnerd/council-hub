package council

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// ========== MoveMessages ==========

func TestMoveMessages(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("src-room", "Source", "", "", "", "", "")
	s.CreateRoom("dst-room", "Destination", "", "", "", "", "")

	id1, _ := s.PostMessage("src-room", "Claude", "Move me", "decision", "")
	id2, _ := s.PostMessage("src-room", "Gemini", "Move me too", "action", "")
	_, _ = s.PostMessage("src-room", "Claude", "Stay here", "message", "")

	moved, err := s.MoveMessages([]string{id1, id2}, "dst-room")
	if err != nil {
		t.Fatalf("MoveMessages failed: %v", err)
	}
	if moved != 2 {
		t.Errorf("expected 2 moved, got %d", moved)
	}

	// Verify messages are in dst-room
	msgs, err := s.GetTranscript("dst-room")
	if err != nil {
		t.Fatalf("GetTranscript dst-room failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages in dst-room, got %d", len(msgs))
	}

	// Verify src-room has only 1 message remaining
	srcMsgs, err := s.GetTranscript("src-room")
	if err != nil {
		t.Fatalf("GetTranscript src-room failed: %v", err)
	}
	if len(srcMsgs) != 1 {
		t.Errorf("expected 1 message remaining in src-room, got %d", len(srcMsgs))
	}
}

func TestMoveMessagesTargetNotFound(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mv-src", "Source", "", "", "", "", "")
	id, _ := s.PostMessage("mv-src", "Claude", "Hello", "message", "")

	_, err := s.MoveMessages([]string{id}, "nonexistent-room")
	if err == nil {
		t.Error("expected error for nonexistent target room, got nil")
	}
}

func TestMoveMessagesEmpty(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mv-empty-dst", "Dst", "", "", "", "", "")

	moved, err := s.MoveMessages([]string{}, "mv-empty-dst")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved != 0 {
		t.Errorf("expected 0 moved for empty IDs, got %d", moved)
	}
}

func TestMoveMessagesPreservesMetadata(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mv-meta-src", "Source", "", "", "", "", "")
	s.CreateRoom("mv-meta-dst", "Destination", "", "", "", "", "")

	id, _ := s.PostMessage("mv-meta-src", "Gemini", "Important decision", "decision", "")

	_, err := s.MoveMessages([]string{id}, "mv-meta-dst")
	if err != nil {
		t.Fatalf("MoveMessages failed: %v", err)
	}

	msgs, err := s.GetTranscript("mv-meta-dst")
	if err != nil {
		t.Fatalf("GetTranscript failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	m := msgs[0]
	if m.Author != "Gemini" {
		t.Errorf("expected author Gemini, got %s", m.Author)
	}
	if m.MessageType != "decision" {
		t.Errorf("expected type decision, got %s", m.MessageType)
	}
	if m.RoomID != "mv-meta-dst" {
		t.Errorf("expected room mv-meta-dst, got %s", m.RoomID)
	}
}

// ========== GetMentions ==========

func TestGetMentions(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mention-room", "Mention tests", "", "", "", "", "")

	// Post with mentions — one message mentioning claude, one mentioning both
	_, err := s.PostMessageWithMentions("mention-room", "gemini-cli", "Hey @claude, please review this", "thought", "", "claude")
	if err != nil {
		t.Fatalf("PostMessageWithMentions failed: %v", err)
	}
	_, err = s.PostMessageWithMentions("mention-room", "amp", "Pinging @claude and @gemini-cli", "action", "", "claude,gemini-cli")
	if err != nil {
		t.Fatalf("PostMessageWithMentions failed: %v", err)
	}
	// Post without mentions — should not appear
	s.PostMessage("mention-room", "gemini-cli", "No mentions here", "message", "")

	msgs, err := s.GetMentions("claude", "", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 mentions of claude, got %d", len(msgs))
	}
}

// TestGetMentionsCollapsesToLiveHeads guards the v0.46.4 fix: mentions is a
// discovery surface, so an edited mention must show once (the head, not a stale
// duplicate) and a retracted mention must not ping at all.
func TestGetMentionsCollapsesToLiveHeads(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mention-live", "Mention live-head tests", "", "", "", "", "")

	edited, _ := s.PostMessageWithMentions("mention-live", "gemini-cli", "v1 @claude take a look", "thought", "", "claude")
	if _, err := s.UpdateMessage(edited, "v2 @claude take a look", ""); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	gone, _ := s.PostMessageWithMentions("mention-live", "amp", "@claude this will be retracted", "thought", "", "claude")
	if _, err := s.RetractMessages([]string{gone}, "amp"); err != nil {
		t.Fatalf("retract failed: %v", err)
	}

	msgs, err := s.GetMentions("claude", "", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	// Only the edited message's head should remain — no stale revision, no retracted.
	if len(msgs) != 1 {
		t.Fatalf("expected 1 live mention, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Content != "v2 @claude take a look" {
		t.Errorf("expected the head revision, got %q", msgs[0].Content)
	}
}

func TestGetMentionsNotFound(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mention-empty", "Empty mention room", "", "", "", "", "")
	s.PostMessage("mention-empty", "gemini-cli", "No mentions here", "message", "")

	msgs, err := s.GetMentions("claude", "", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 mentions, got %d", len(msgs))
	}
}

func TestGetMentionsBoundary(t *testing.T) {
	// "claude" fuzzy-matches "claude-sonnet" and "claude" — both should be returned.
	// Fuzzy matching is intentional: "claude" matches "Claude Code (Opus)", "claude-code", etc.
	s := setupTestServer(t)
	s.CreateRoom("boundary-room", "Boundary test", "", "", "", "", "")

	s.PostMessageWithMentions("boundary-room", "system", "For claude-sonnet only", "message", "", "claude-sonnet")
	s.PostMessageWithMentions("boundary-room", "system", "For claude only", "message", "", "claude")

	msgs, err := s.GetMentions("claude", "", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 fuzzy matches for 'claude' (claude + claude-sonnet), got %d", len(msgs))
	}
}

func TestGetMentionsProjectFilter(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("proj-a-room", "Project A", "alpha", "", "", "", "")
	s.CreateRoom("proj-b-room", "Project B", "beta", "", "", "", "")

	s.PostMessageWithMentions("proj-a-room", "bot", "@claude in alpha", "message", "", "claude")
	s.PostMessageWithMentions("proj-b-room", "bot", "@claude in beta", "message", "", "claude")

	all, err := s.GetMentions("claude", "", 20)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 mentions across all projects, got %d", len(all))
	}

	scoped, err := s.GetMentions("claude", "alpha", 20)
	if err != nil {
		t.Fatalf("GetMentions(project) failed: %v", err)
	}
	if len(scoped) != 1 {
		t.Fatalf("expected 1 mention scoped to 'alpha', got %d", len(scoped))
	}
	if scoped[0].RoomID != "proj-a-room" {
		t.Errorf("expected mention from 'proj-a-room', got '%s'", scoped[0].RoomID)
	}
}

func TestGetMentionsLimit(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("mention-limit", "Limit test", "", "", "", "", "")

	for i := 0; i < 5; i++ {
		s.PostMessageWithMentions("mention-limit", "bot", "ping", "message", "", "claude")
	}

	msgs, err := s.GetMentions("claude", "", 3)
	if err != nil {
		t.Fatalf("GetMentions failed: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 (limit), got %d", len(msgs))
	}
}

// ========== UpdateMessageWithExpected (optimistic concurrency) ==========

func TestUpdateMessageWithExpectedMatch(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("occ-room", "OCC test", "", "", "", "", "")
	id, _ := s.PostMessage("occ-room", "Claude", "original", "message", "")

	m, err := s.UpdateMessageWithExpected(id, "updated", "", "original", "")
	if err != nil {
		t.Fatalf("UpdateMessageWithExpected failed: %v", err)
	}
	if m.Content != "updated" {
		t.Errorf("expected content 'updated', got '%s'", m.Content)
	}
	// Append-only: the edit returns a NEW head node revising the original.
	if m.ID == id {
		t.Error("expected a new revision node, got the same ID (in-place overwrite)")
	}
	if m.Revises != id {
		t.Errorf("expected revision to point back at original %s, got revises=%q", id, m.Revises)
	}
}

func TestUpdateMessageWithExpectedMismatch(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("occ-mismatch", "OCC mismatch", "", "", "", "", "")
	id, _ := s.PostMessage("occ-mismatch", "Claude", "original", "message", "")

	_, err := s.UpdateMessageWithExpected(id, "updated", "", "stale", "")
	if err == nil {
		t.Fatal("expected error for content mismatch, got nil")
	}
	changed, ok := err.(*ErrContentChanged)
	if !ok {
		t.Fatalf("expected *ErrContentChanged, got %T: %v", err, err)
	}
	if changed.CurrentContent != "original" {
		t.Errorf("expected current content 'original', got '%s'", changed.CurrentContent)
	}
}

func TestUpdateMessageWithExpectedEmpty(t *testing.T) {
	// Empty expected_content skips the optimistic check (still append-only).
	s := setupTestServer(t)
	s.CreateRoom("occ-empty", "OCC empty", "", "", "", "", "")
	id, _ := s.PostMessage("occ-empty", "Claude", "original", "message", "")

	m, err := s.UpdateMessageWithExpected(id, "blind update", "", "", "")
	if err != nil {
		t.Fatalf("UpdateMessageWithExpected with empty expected failed: %v", err)
	}
	if m.Content != "blind update" {
		t.Errorf("expected 'blind update', got '%s'", m.Content)
	}
}

// TestEditAppendsImmutableRevision is the core OHS/Journal property: an edit never
// overwrites — it appends a new head node, preserves the prior version, collapses
// reads to the head, and keeps the version history walkable in the link graph.
func TestEditAppendsImmutableRevision(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("rev-room", "Revision test", "", "", "", "", "")
	orig, _ := s.PostMessage("rev-room", "claude", "deploy on friady", "decision", "")

	head, err := s.UpdateMessageWithExpected(orig, "deploy on Friday", "", "", "gemini")
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}

	// New head: new ID, new content, points back at the original, attributed to the editor.
	if head.ID == orig {
		t.Fatal("edit overwrote in place instead of appending a revision")
	}
	if head.Revises != orig || head.Author != "gemini" || head.Content != "deploy on Friday" {
		t.Errorf("unexpected head: revises=%s author=%s content=%q", head.Revises, head.Author, head.Content)
	}

	// Original is preserved verbatim and flagged as a non-head revision.
	old, _ := s.GetMessagesByIDs([]string{orig})
	if old[0].Content != "deploy on friady" || !old[0].Revised {
		t.Errorf("original not preserved/flagged: content=%q revised=%v", old[0].Content, old[0].Revised)
	}

	// Reads collapse to the single head.
	recent, _ := s.GetRecentMessages("rev-room", 10)
	if len(recent) != 1 || recent[0].ID != head.ID {
		t.Fatalf("expected reads to collapse to the head, got %d messages", len(recent))
	}

	// The version history is walkable: head --revises--> orig, orig <--revised_by-- head.
	out, _, _ := s.GetLinks(head.ID)
	if !hasEdge(out, head.ID, orig, "revises") {
		t.Errorf("expected revises edge from head, got %+v", out)
	}
	_, in, _ := s.GetLinks(orig)
	if !hasEdge(in, head.ID, orig, "revised_by") {
		t.Errorf("expected revised_by backlink on original, got %+v", in)
	}

	// Editing the stale original is refused and points at the head.
	_, err = s.UpdateMessageWithExpected(orig, "fork attempt", "", "", "claude")
	already, ok := err.(*ErrAlreadyRevised)
	if !ok || already.HeadID != head.ID {
		t.Errorf("expected ErrAlreadyRevised pointing at %s, got %v", head.ID, err)
	}

	// The transcript shows the head marked edited, never the old text.
	msgs, _ := s.GetTranscript("rev-room")
	room, _ := s.GetRoom("rev-room")
	tr := FormatTranscript(room, msgs)
	if !strings.Contains(tr, "✎ edited") || strings.Contains(tr, "friady") {
		t.Errorf("transcript should show edited head, not old content:\n%s", tr)
	}
}

// TestRetractTombstonesNode verifies retraction preserves the node and its links
// while rendering a tombstone — the immutable counterpart to deletion.
func TestRetractTombstonesNode(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("tomb-room", "Retract test", "", "", "", "", "")
	a, _ := s.PostMessage("tomb-room", "claude", "keep", "message", "")
	b, _ := s.PostMessage("tomb-room", "claude", "secret typo to retract", "message", "")
	s.CreateLink(a, b, "relates", "claude")

	if _, err := s.RetractMessages([]string{b}, "claude"); err != nil {
		t.Fatalf("retract failed: %v", err)
	}

	// Node survives and the link is intact (graph never dangles).
	msgs, _ := s.GetTranscript("tomb-room")
	if len(msgs) != 2 {
		t.Fatalf("retracted node should survive, got %d messages", len(msgs))
	}
	out, _, _ := s.GetLinks(a)
	if !hasEdge(out, a, b, "relates") {
		t.Errorf("retract must not sever links, got %+v", out)
	}

	// It renders as a tombstone, not its content.
	room, _ := s.GetRoom("tomb-room")
	tr := FormatTranscript(room, msgs)
	if !strings.Contains(tr, "[retracted by claude]") || strings.Contains(tr, "secret typo") {
		t.Errorf("expected tombstone, not content:\n%s", tr)
	}
}

// TestDisplayContent covers the shared masking helper every read path routes
// through: live content passes verbatim, a retracted node reads as a tombstone
// (with attribution when recorded).
func TestDisplayContent(t *testing.T) {
	if got := DisplayContent(Message{Content: "hello"}); got != "hello" {
		t.Errorf("live content should pass through, got %q", got)
	}
	retracted := Message{Content: "secret", RetractedAt: sql.NullTime{Valid: true}, RetractedBy: "claude"}
	if got := DisplayContent(retracted); got != "_[retracted by claude]_" {
		t.Errorf("expected attributed tombstone, got %q", got)
	}
	if strings.Contains(DisplayContent(retracted), "secret") {
		t.Error("retracted content must not leak through DisplayContent")
	}
	anon := Message{Content: "secret", RetractedAt: sql.NullTime{Valid: true}}
	if got := DisplayContent(anon); got != "_[retracted]_" {
		t.Errorf("expected anonymous tombstone, got %q", got)
	}
}

// TestRestoreMessages verifies retraction is reversible (only purge is final).
func TestRestoreMessages(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("restore-room", "Restore test", "", "", "", "", "")
	id, _ := s.PostMessage("restore-room", "claude", "back from the dead", "message", "")

	if _, err := s.RetractMessages([]string{id}, "claude"); err != nil {
		t.Fatalf("retract failed: %v", err)
	}
	n, err := s.RestoreMessages([]string{id})
	if err != nil || n != 1 {
		t.Fatalf("expected 1 restored, got %d err=%v", n, err)
	}

	m, _ := s.GetMessagesByIDs([]string{id})
	if m[0].RetractedAt.Valid || m[0].RetractedBy != "" {
		t.Errorf("expected retraction cleared, got valid=%v by=%q", m[0].RetractedAt.Valid, m[0].RetractedBy)
	}
	// Restoring a live message is a no-op.
	if n, _ := s.RestoreMessages([]string{id}); n != 0 {
		t.Errorf("restoring a live message should affect 0 rows, got %d", n)
	}
}

// TestGetRevisionHistory walks the append-only edit chain from any node.
func TestGetRevisionHistory(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("hist-room", "History test", "", "", "", "", "")
	v1, _ := s.PostMessage("hist-room", "claude", "v1", "draft", "")
	h2, _ := s.UpdateMessage(v1, "v2", "")
	h3, _ := s.UpdateMessage(h2.ID, "v3", "")

	// History is identical regardless of which node in the chain you ask about.
	for _, from := range []string{v1, h2.ID, h3.ID} {
		chain, err := s.GetRevisionHistory(from)
		if err != nil {
			t.Fatalf("history(%s) failed: %v", from, err)
		}
		if len(chain) != 3 {
			t.Fatalf("expected 3 versions from %s, got %d", from, len(chain))
		}
		if chain[0].Content != "v1" || chain[1].Content != "v2" || chain[2].Content != "v3" {
			t.Errorf("expected v1→v2→v3, got %q→%q→%q", chain[0].Content, chain[1].Content, chain[2].Content)
		}
		if chain[2].ID != h3.ID {
			t.Errorf("expected head %s last, got %s", h3.ID, chain[2].ID)
		}
	}

	// A never-edited message returns a single version.
	solo, _ := s.PostMessage("hist-room", "claude", "only", "message", "")
	chain, _ := s.GetRevisionHistory(solo)
	if len(chain) != 1 {
		t.Errorf("expected single version for un-edited message, got %d", len(chain))
	}
}

func hasEdge(links []MessageLink, from, to, relation string) bool {
	for _, l := range links {
		if l.FromID == from && l.ToID == to && l.Relation == relation {
			return true
		}
	}
	return false
}

func TestPostMessageWithMentionsStoredAndRetrievable(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("pmwm-room", "Test", "", "", "", "", "")

	id, err := s.PostMessageWithMentions("pmwm-room", "agent-a", "content", "thought", "", "agent-b,agent-c")
	if err != nil {
		t.Fatalf("PostMessageWithMentions: %v", err)
	}

	msgs, err := s.GetMessagesByIDs([]string{id})
	if err != nil || len(msgs) == 0 {
		t.Fatalf("GetMessagesByIDs failed: %v", err)
	}
	if msgs[0].Mentions != "agent-b,agent-c" {
		t.Errorf("expected mentions 'agent-b,agent-c', got '%s'", msgs[0].Mentions)
	}
}

func TestMarkReadAndGetCursor(t *testing.T) {
	s := setupTestServer(t)

	// GetCursor returns empty string when no cursor exists
	cursor, err := s.GetCursor("claude", "some-room")
	if err != nil {
		t.Fatalf("GetCursor on missing cursor: %v", err)
	}
	if cursor != "" {
		t.Errorf("expected empty cursor, got %q", cursor)
	}

	// MarkRead stores a cursor
	if err := s.MarkRead("claude", "room-a", "msg-001"); err != nil {
		t.Fatalf("MarkRead failed: %v", err)
	}
	cursor, err = s.GetCursor("claude", "room-a")
	if err != nil {
		t.Fatalf("GetCursor after MarkRead: %v", err)
	}
	if cursor != "msg-001" {
		t.Errorf("expected cursor 'msg-001', got %q", cursor)
	}

	// MarkRead overwrites with a newer cursor
	if err := s.MarkRead("claude", "room-a", "msg-002"); err != nil {
		t.Fatalf("MarkRead overwrite failed: %v", err)
	}
	cursor, err = s.GetCursor("claude", "room-a")
	if err != nil {
		t.Fatalf("GetCursor after overwrite: %v", err)
	}
	if cursor != "msg-002" {
		t.Errorf("expected cursor 'msg-002', got %q", cursor)
	}

	// Different agents have independent cursors for the same room
	if err := s.MarkRead("gemini", "room-a", "msg-050"); err != nil {
		t.Fatalf("MarkRead (gemini) failed: %v", err)
	}
	claudeCursor, _ := s.GetCursor("claude", "room-a")
	geminiCursor, _ := s.GetCursor("gemini", "room-a")
	if claudeCursor != "msg-002" {
		t.Errorf("claude cursor changed after gemini mark_read: got %q", claudeCursor)
	}
	if geminiCursor != "msg-050" {
		t.Errorf("expected gemini cursor 'msg-050', got %q", geminiCursor)
	}

	// Same agent, different rooms are independent
	if err := s.MarkRead("claude", "room-b", "msg-099"); err != nil {
		t.Fatalf("MarkRead (room-b) failed: %v", err)
	}
	cursorA, _ := s.GetCursor("claude", "room-a")
	cursorB, _ := s.GetCursor("claude", "room-b")
	if cursorA != "msg-002" {
		t.Errorf("room-a cursor changed after room-b mark_read: got %q", cursorA)
	}
	if cursorB != "msg-099" {
		t.Errorf("expected room-b cursor 'msg-099', got %q", cursorB)
	}
}

func TestGetMessagesFromIDInclusive(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("src", "Source room", "", "", "", "", "")

	id1, _ := s.PostMessage("src", "alice", "first", "message", "")
	id2, _ := s.PostMessage("src", "alice", "second", "message", "")
	id3, _ := s.PostMessage("src", "alice", "third", "message", "")

	// From id2 inclusive: should get id2 and id3.
	msgs, err := s.GetMessagesFromIDInclusive("src", id2)
	if err != nil {
		t.Fatalf("GetMessagesFromIDInclusive failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != id2 || msgs[1].ID != id3 {
		t.Errorf("unexpected message IDs: %v", []string{msgs[0].ID, msgs[1].ID})
	}
	_ = id1
}

func TestGetMessageByID(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-x", "Test room", "", "", "", "", "")
	id, _ := s.PostMessage("room-x", "bob", "hello", "thought", "")

	m, err := s.GetMessageByID(id)
	if err != nil {
		t.Fatalf("GetMessageByID failed: %v", err)
	}
	if m.ID != id || m.RoomID != "room-x" || m.Author != "bob" {
		t.Errorf("unexpected message: %+v", m)
	}

	_, err = s.GetMessageByID("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent ID, got nil")
	}
}

func TestResolveMessageIDExactMatch(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-resolve", "Test room", "", "", "", "", "")
	id, _ := s.PostMessage("room-resolve", "bob", "hello", "thought", "")

	resolved, err := s.ResolveMessageID(id)
	if err != nil {
		t.Fatalf("ResolveMessageID failed: %v", err)
	}
	if resolved != id {
		t.Errorf("expected exact match to return %q, got %q", id, resolved)
	}
}

func TestResolveMessageIDUniquePrefix(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-resolve2", "Test room", "", "", "", "", "")
	id, _ := s.PostMessage("room-resolve2", "bob", "hello", "thought", "")

	resolved, err := s.ResolveMessageID(id[:8])
	if err != nil {
		t.Fatalf("ResolveMessageID(prefix) failed: %v", err)
	}
	if resolved != id {
		t.Errorf("expected prefix to resolve to %q, got %q", id, resolved)
	}
}

func TestResolveMessageIDAmbiguousPrefix(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-resolve3", "Test room", "", "", "", "", "")
	id1, _ := s.PostMessage("room-resolve3", "bob", "first", "thought", "")

	// Force a second row sharing id1's 8-char prefix — real collisions come from
	// UUIDv7's shared millisecond-timestamp bits; this constructs the same shape
	// directly against the DB so the test doesn't need real timing collisions.
	sharedPrefix := id1[:8]
	fakeID := sharedPrefix + "-0000-7000-8000-000000000000"
	if _, err := s.DB.Exec(
		`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, ?, ?, ?, ?)`,
		fakeID, "room-resolve3", "alice", "second", "thought",
	); err != nil {
		t.Fatalf("insert collision row: %v", err)
	}

	_, err := s.ResolveMessageID(sharedPrefix)
	if err == nil {
		t.Fatal("expected ambiguous-prefix error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), id1) || !strings.Contains(err.Error(), fakeID) {
		t.Errorf("expected both candidate IDs listed, got: %v", err)
	}
}

func TestResolveMessageIDNotFound(t *testing.T) {
	s := setupTestServer(t)
	_, err := s.ResolveMessageID("totally-nonexistent-prefix")
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

func TestResolveMessageIDShortPrefixRejected(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-resolve-short", "Test room", "", "", "", "", "")
	id, _ := s.PostMessage("room-resolve-short", "bob", "hello", "thought", "")

	// A 6-char prefix would uniquely match, but anything below minResolvePrefixLen
	// never came from our own display output (which prints #%.8s) — reject it.
	_, err := s.ResolveMessageID(id[:6])
	if err == nil {
		t.Fatal("expected error for prefix shorter than 8 chars, got nil")
	}
	if !strings.Contains(err.Error(), "at least 8") {
		t.Errorf("expected error to mention the 8-char minimum, got: %v", err)
	}

	// An exact full ID shorter than 8 chars must still resolve — the exact-match
	// attempt runs before the length check (legacy pre-UUID IDs).
	if _, err := s.DB.Exec(
		`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, ?, ?, ?, ?)`,
		"42", "room-resolve-short", "alice", "legacy", "thought",
	); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	resolved, err := s.ResolveMessageID("42")
	if err != nil {
		t.Fatalf("expected short exact ID to resolve, got: %v", err)
	}
	if resolved != "42" {
		t.Errorf("expected exact match '42', got %q", resolved)
	}
}

func TestResolveMessageIDAmbiguousTruncationExact(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-resolve-many", "Test room", "", "", "", "", "")

	// maxPrefixCandidates+2 rows sharing an 8-char prefix: the error must list
	// exactly maxPrefixCandidates candidates and flag that more exist.
	sharedPrefix := "0aaaaaaa"
	for i := 0; i < maxPrefixCandidates+2; i++ {
		id := fmt.Sprintf("%s-%04d-7000-8000-000000000000", sharedPrefix, i)
		if _, err := s.DB.Exec(
			`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, ?, ?, ?, ?)`,
			id, "room-resolve-many", "alice", "collision", "thought",
		); err != nil {
			t.Fatalf("insert collision row %d: %v", i, err)
		}
	}

	_, err := s.ResolveMessageID(sharedPrefix)
	ambiguous, ok := err.(*ErrAmbiguousMessageID)
	if !ok {
		t.Fatalf("expected *ErrAmbiguousMessageID, got: %v", err)
	}
	if len(ambiguous.Candidates) != maxPrefixCandidates {
		t.Errorf("expected exactly %d candidates listed, got %d", maxPrefixCandidates, len(ambiguous.Candidates))
	}
	if !ambiguous.Truncated {
		t.Error("expected Truncated=true when more matches exist beyond the cap")
	}
	if !strings.Contains(err.Error(), "...and more") {
		t.Errorf("expected '...and more' in error, got: %v", err)
	}

	// Exactly maxPrefixCandidates matches: all listed, NOT flagged as truncated.
	s.CreateRoom("room-resolve-exact", "Test room", "", "", "", "", "")
	sharedPrefix2 := "0bbbbbbb"
	for i := 0; i < maxPrefixCandidates; i++ {
		id := fmt.Sprintf("%s-%04d-7000-8000-000000000000", sharedPrefix2, i)
		if _, err := s.DB.Exec(
			`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, ?, ?, ?, ?)`,
			id, "room-resolve-exact", "alice", "collision", "thought",
		); err != nil {
			t.Fatalf("insert collision row %d: %v", i, err)
		}
	}
	_, err = s.ResolveMessageID(sharedPrefix2)
	ambiguous, ok = err.(*ErrAmbiguousMessageID)
	if !ok {
		t.Fatalf("expected *ErrAmbiguousMessageID, got: %v", err)
	}
	if len(ambiguous.Candidates) != maxPrefixCandidates {
		t.Errorf("expected all %d candidates listed, got %d", maxPrefixCandidates, len(ambiguous.Candidates))
	}
	if ambiguous.Truncated {
		t.Error("expected Truncated=false when the matches fit the cap exactly")
	}
	if strings.Contains(err.Error(), "...and more") {
		t.Errorf("expected no '...and more' when nothing is truncated, got: %v", err)
	}
}

func TestResolveMessageIDRangeBoundaries(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-resolve-range", "Test room", "", "", "", "", "")

	// The range form (id >= prefix AND id < prefix+"g") must match only true
	// prefixes: an adjacent id one hex increment above the prefix ("0000000f" →
	// "00000010") sorts inside a naive open-ended range but is not a match.
	target := "0000000f-0000-7000-8000-000000000001"
	adjacent := "00000010-0000-7000-8000-000000000002"
	for _, id := range []string{target, adjacent} {
		if _, err := s.DB.Exec(
			`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, ?, ?, ?, ?)`,
			id, "room-resolve-range", "alice", "boundary", "thought",
		); err != nil {
			t.Fatalf("insert row %s: %v", id, err)
		}
	}

	resolved, err := s.ResolveMessageID("0000000f")
	if err != nil {
		t.Fatalf("expected hex-boundary prefix to resolve uniquely, got: %v", err)
	}
	if resolved != target {
		t.Errorf("expected %q, got %q", target, resolved)
	}

	// A prefix spanning a dash boundary resolves too ('-' sorts below 'g').
	resolved, err = s.ResolveMessageID(target[:14]) // "0000000f-0000-"
	if err != nil {
		t.Fatalf("expected dash-boundary prefix to resolve, got: %v", err)
	}
	if resolved != target {
		t.Errorf("expected %q, got %q", target, resolved)
	}
}
