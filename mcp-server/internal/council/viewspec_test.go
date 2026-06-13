package council

import (
	"strings"
	"testing"
)

func TestParseViewSpec(t *testing.T) {
	// Empty show → everything on.
	v := ParseViewSpec("", "")
	if !(v.ShowIDs && v.ShowAuthor && v.ShowTimestamps && v.ShowReactions) {
		t.Errorf("empty show should enable all metadata, got %+v", v)
	}
	if v.TruncateLineOne {
		t.Error("default truncate should be off")
	}

	// Explicit selection → only those on.
	v = ParseViewSpec("author", "line-one")
	if !v.ShowAuthor {
		t.Error("expected ShowAuthor")
	}
	if v.ShowIDs || v.ShowTimestamps || v.ShowReactions {
		t.Errorf("only author should be shown, got %+v", v)
	}
	if !v.TruncateLineOne {
		t.Error("expected TruncateLineOne for 'line-one'")
	}

	// Aliases + multiple tokens.
	v = ParseViewSpec("ids, ts", "")
	if !v.ShowIDs || !v.ShowTimestamps {
		t.Errorf("expected ids+ts, got %+v", v)
	}
	if v.ShowAuthor {
		t.Error("author should be off")
	}
}

func renderRoom(t *testing.T, s *Server, roomID string, v ViewSpec) string {
	t.Helper()
	room, err := s.GetRoom(roomID)
	if err != nil {
		t.Fatalf("GetRoom: %v", err)
	}
	msgs, err := s.GetTranscript(roomID)
	if err != nil {
		t.Fatalf("GetTranscript: %v", err)
	}
	return FormatTranscriptView(room, msgs, v)
}

func TestFormatTranscriptViewHidesMetadata(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "vs-meta")
	id := mustPostTyped(t, s, "vs-meta", "Claude", "the body text", "decision")

	// show=author hides ids + timestamps but keeps author and content.
	out := renderRoom(t, s, "vs-meta", ParseViewSpec("author", ""))
	if strings.Contains(out, "#"+id[:8]) {
		t.Errorf("expected message ID hidden, got:\n%s", out)
	}
	if !strings.Contains(out, "Claude") || !strings.Contains(out, "the body text") {
		t.Errorf("expected author + content shown, got:\n%s", out)
	}

	// Default still shows the ID.
	def := renderRoom(t, s, "vs-meta", DefaultViewSpec())
	if !strings.Contains(def, "#"+id[:8]) {
		t.Errorf("default view should show the message ID, got:\n%s", def)
	}
}

func TestFormatTranscriptViewTruncate(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "vs-trunc")
	mustPostTyped(t, s, "vs-trunc", "Claude", "first line\nsecond line\nthird", "thought")

	out := renderRoom(t, s, "vs-trunc", ParseViewSpec("", "line-one"))
	if !strings.Contains(out, "first line") {
		t.Errorf("expected first line, got:\n%s", out)
	}
	if strings.Contains(out, "second line") {
		t.Errorf("line-one truncate should drop later lines, got:\n%s", out)
	}
}

func TestFilterMessages(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "filt-room")
	mustPostTyped(t, s, "filt-room", "Claude Code (Opus)", "a decision", "decision")
	mustPostTyped(t, s, "filt-room", "Gemini CLI", "an action", "action")
	mustPostTyped(t, s, "filt-room", "Claude Code (Opus)", "a thought", "thought")

	msgs, _ := s.GetTranscript("filt-room")

	// Empty filter → unchanged.
	if got := FilterMessages(msgs, "", "", "", ""); len(got) != len(msgs) {
		t.Errorf("empty filter should pass all %d, got %d", len(msgs), len(got))
	}

	// Author substring (case-insensitive).
	byAuthor := FilterMessages(msgs, "claude", "", "", "")
	if len(byAuthor) != 2 {
		t.Errorf("expected 2 Claude messages, got %d", len(byAuthor))
	}

	// Type exact.
	byType := FilterMessages(msgs, "", "action", "", "")
	if len(byType) != 1 || byType[0].Author != "Gemini CLI" {
		t.Errorf("expected 1 action from Gemini, got %+v", byType)
	}

	// Combined author + type.
	combined := FilterMessages(msgs, "claude", "thought", "", "")
	if len(combined) != 1 || combined[0].Content != "a thought" {
		t.Errorf("expected 1 Claude thought, got %+v", combined)
	}
}

func TestFilterMessagesByTime(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "filt-time")
	old := mustPost(t, s, "filt-time", "Claude", "old")
	mustPost(t, s, "filt-time", "Claude", "new")
	// Backdate the first message well into the past.
	s.DB.Exec(`UPDATE messages SET timestamp = '2020-01-01 00:00:00' WHERE id = ?`, old)

	msgs, _ := s.GetTranscript("filt-time")
	recent := FilterMessages(msgs, "", "", "2021-01-01", "")
	if len(recent) != 1 || recent[0].Content != "new" {
		t.Errorf("since filter should keep only the recent message, got %+v", recent)
	}
}

func TestFormatTranscriptViewHidesReactions(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "vs-react")
	id := mustPost(t, s, "vs-react", "Claude", "react to me")
	if _, _, err := s.ReactToMessage(id, "👍", "Gemini"); err != nil {
		t.Fatal(err)
	}

	// Default shows reactions; show=author hides them.
	def := renderRoom(t, s, "vs-react", DefaultViewSpec())
	if !strings.Contains(def, "Reactions:") {
		t.Errorf("default should show reactions, got:\n%s", def)
	}
	hidden := renderRoom(t, s, "vs-react", ParseViewSpec("author", ""))
	if strings.Contains(hidden, "Reactions:") {
		t.Errorf("show=author should hide reactions, got:\n%s", hidden)
	}
}
