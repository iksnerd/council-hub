package council

import (
	"database/sql"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// ========== TruncateRunes ==========

func TestTruncateRunesWithinLimit(t *testing.T) {
	if got := TruncateRunes("short", 10, " ", 5); got != "short" {
		t.Errorf("expected no-op within limit, got %q", got)
	}
}

func TestTruncateRunesBoundaryBackup(t *testing.T) {
	// ASCII: a space after minBoundary runes backs the cut up to it.
	s := strings.Repeat("a", 50) + " " + strings.Repeat("b", 50)
	got := TruncateRunes(s, 60, " ", 40)
	if got != strings.Repeat("a", 50)+"..." {
		t.Errorf("expected cut backed up to the space at rune 50, got %q", got)
	}
}

func TestTruncateRunesCyrillicMinBoundaryIsRunes(t *testing.T) {
	// minBoundary counts RUNES, but strings.LastIndex returns a BYTE offset — for
	// 2-byte Cyrillic a space at rune 21 sits at byte 42, which the buggy byte
	// comparison read as "past minBoundary=40" and cut a 21-rune excerpt.
	s := strings.Repeat("ж", 21) + " " + strings.Repeat("д", 50)
	got := TruncateRunes(s, 60, " ", 40)
	if !utf8.ValidString(got) {
		t.Fatalf("result is not valid UTF-8: %q", got)
	}
	if runes := utf8.RuneCountInString(got); runes != 63 { // hard cut at 60 + "..."
		t.Errorf("expected hard cut at 60 runes (63 with ellipsis), got %d runes: %q", runes, got)
	}
}

// ========== DisplayContent ==========

func TestDisplayContentTombstone(t *testing.T) {
	live := Message{Content: "hello"}
	if got := DisplayContent(live); got != "hello" {
		t.Errorf("expected live content unchanged, got %q", got)
	}

	retracted := Message{Content: "secret", RetractedAt: sql.NullTime{Time: time.Now(), Valid: true}}
	if got := DisplayContent(retracted); got != "_[retracted]_" {
		t.Errorf("expected anonymous tombstone, got %q", got)
	}

	retracted.RetractedBy = "Claude"
	if got := DisplayContent(retracted); got != "_[retracted by Claude]_" {
		t.Errorf("expected attributed tombstone, got %q", got)
	}
}
