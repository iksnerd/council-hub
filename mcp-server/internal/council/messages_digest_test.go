package council

import (
	"strings"
	"testing"
	"time"
)

// ========== GetMessagesAfterID ==========

func TestGetMessagesAfterID(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "after-room")
	id1 := mustPost(t, s, "after-room", "Claude", "First")
	id2 := mustPost(t, s, "after-room", "Gemini", "Second")
	id3 := mustPost(t, s, "after-room", "Claude", "Third")

	msgs, err := s.GetMessagesAfterID("after-room", id1)
	if err != nil {
		t.Fatalf("GetMessagesAfterID failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after id %s, got %d", id1, len(msgs))
	}
	if msgs[0].ID != id2 {
		t.Errorf("expected first result id %s, got %s", id2, msgs[0].ID)
	}

	// After the last message — empty
	msgs, err = s.GetMessagesAfterID("after-room", id3)
	if err != nil {
		t.Fatalf("GetMessagesAfterID failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

// ========== GetLatestPerType ==========

func TestGetLatestPerType(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "latest-type")
	mustPostTyped(t, s, "latest-type", "Claude", "thought 1", "thought")
	mustPostTyped(t, s, "latest-type", "Claude", "thought 2", "thought")
	mustPostTyped(t, s, "latest-type", "Claude", "thought 3", "thought")
	mustPostTyped(t, s, "latest-type", "Gemini", "decision 1", "decision")
	mustPostTyped(t, s, "latest-type", "Gemini", "decision 2", "decision")
	mustPostTyped(t, s, "latest-type", "Claude", "code block", "code")

	msgs, err := s.GetLatestPerType("latest-type")
	if err != nil {
		t.Fatalf("GetLatestPerType failed: %v", err)
	}
	// Up to 2 per type: thought(2), decision(2), code(1) = 5
	if len(msgs) < 3 || len(msgs) > 6 {
		t.Fatalf("expected 3-6 messages (up to 2 per type), got %d", len(msgs))
	}
	types := map[string]int{}
	for _, m := range msgs {
		types[m.MessageType]++
	}
	if types["thought"] != 2 {
		t.Errorf("expected 2 thought messages, got %d", types["thought"])
	}
	if types["decision"] != 2 {
		t.Errorf("expected 2 decision messages, got %d", types["decision"])
	}
	if types["code"] != 1 {
		t.Errorf("expected 1 code message, got %d", types["code"])
	}
}

func TestGetLatestPerTypeEmpty(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "latest-empty")
	msgs, err := s.GetLatestPerType("latest-empty")
	if err != nil {
		t.Fatalf("GetLatestPerType failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

// ========== GetMessageCounts ==========

func TestGetMessageCounts(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "count-a")
	mustCreateRoom(t, s, "count-b")
	mustPost(t, s, "count-a", "Claude", "msg 1")
	mustPost(t, s, "count-a", "Claude", "msg 2")
	mustPost(t, s, "count-b", "Gemini", "msg 1")

	counts := s.GetMessageCounts()
	if counts["count-a"] != 2 {
		t.Errorf("expected 2 for count-a, got %d", counts["count-a"])
	}
	if counts["count-b"] != 1 {
		t.Errorf("expected 1 for count-b, got %d", counts["count-b"])
	}
}

func TestGetMessageCountsEmpty(t *testing.T) {
	s := setupTestServer(t)
	counts := s.GetMessageCounts()
	if len(counts) != 0 {
		t.Errorf("expected empty map, got %d entries", len(counts))
	}
}

// ========== GetDigest ==========

func TestGetDigest(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "digest-a", withProject("proj-x"))
	mustCreateRoom(t, s, "digest-b", withProject("proj-x"))
	mustCreateRoom(t, s, "digest-c", withProject("proj-y"))
	mustPost(t, s, "digest-a", "Claude", "msg a1")
	mustPost(t, s, "digest-a", "Claude", "msg a2")
	mustPost(t, s, "digest-b", "Gemini", "msg b1")
	mustPost(t, s, "digest-c", "Amp", "msg c1")

	since := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")

	entries, err := s.GetDigest("", since)
	if err != nil {
		t.Fatalf("GetDigest failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 digest entries, got %d", len(entries))
	}

	// Filter by project
	entries, err = s.GetDigest("proj-x", since)
	if err != nil {
		t.Fatalf("GetDigest project filter failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for proj-x, got %d", len(entries))
	}
	for _, e := range entries {
		if e.NewMessages == 0 || e.LatestAuthor == "" {
			t.Errorf("entry missing fields: %+v", e)
		}
	}
}

func TestGetDigestRetractedHeadTombstone(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "digest-retract", withProject("proj-r"))
	mustPost(t, s, "digest-retract", "Claude", "earlier msg")
	id := mustPost(t, s, "digest-retract", "Claude", "the withdrawn latest")
	if _, err := s.RetractMessages([]string{id}, "Claude"); err != nil {
		t.Fatalf("RetractMessages: %v", err)
	}

	since := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	entries, err := s.GetDigest("proj-r", since)
	if err != nil {
		t.Fatalf("GetDigest failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// The latest head is retracted: the excerpt must render as a tombstone,
	// never broadcast the withdrawn content.
	if strings.Contains(entries[0].LatestExcerpt, "withdrawn latest") {
		t.Errorf("retracted content leaked into digest excerpt: %q", entries[0].LatestExcerpt)
	}
	if !strings.Contains(entries[0].LatestExcerpt, "[retracted by Claude]") {
		t.Errorf("expected tombstone excerpt, got %q", entries[0].LatestExcerpt)
	}
	if entries[0].LatestAuthor != "Claude" {
		t.Errorf("expected latest author preserved, got %q", entries[0].LatestAuthor)
	}
}

func TestGetDigestNoResults(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "digest-empty")
	entries, err := s.GetDigest("", "2099-01-01 00:00:00")
	if err != nil {
		t.Fatalf("GetDigest failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestGetDigestTNormalization(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "digest-t")
	mustPost(t, s, "digest-t", "Claude", "msg")
	since := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02T15:04:05")
	entries, err := s.GetDigest("", since)
	if err != nil {
		t.Fatalf("GetDigest T-normalization failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

// ========== GetPinnedExcerpts ==========

func TestGetPinnedExcerpts(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "pin-excerpt-1")
	mustCreateRoom(t, s, "pin-excerpt-2")

	// Pin a message in room 1 only
	id1 := mustPost(t, s, "pin-excerpt-1", "Claude", "This is pinned content")
	s.PinMessage("pin-excerpt-1", id1)
	mustPost(t, s, "pin-excerpt-2", "Gemini", "Not pinned")

	excerpts := s.GetPinnedExcerpts([]string{"pin-excerpt-1", "pin-excerpt-2"})
	if _, ok := excerpts["pin-excerpt-1"]; !ok {
		t.Error("expected excerpt for pin-excerpt-1")
	}
	if _, ok := excerpts["pin-excerpt-2"]; ok {
		t.Error("expected no excerpt for pin-excerpt-2")
	}

	// Test truncation: pin a message with 100+ chars
	longContent := strings.Repeat("word ", 25) // 125 chars
	idLong := mustPost(t, s, "pin-excerpt-1", "Claude", longContent)
	s.PinMessage("pin-excerpt-1", idLong) // replaces existing pin

	excerpts = s.GetPinnedExcerpts([]string{"pin-excerpt-1"})
	excerpt := excerpts["pin-excerpt-1"]
	if !strings.HasSuffix(excerpt, "...") {
		t.Errorf("expected truncated excerpt ending with '...', got: %s", excerpt)
	}
	// Excerpt should be at most ~63 chars (60 + "...")
	if len(excerpt) > 65 {
		t.Errorf("expected excerpt <= 65 chars, got %d: %s", len(excerpt), excerpt)
	}
}
