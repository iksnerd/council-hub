package council

import (
	"testing"
)

func TestReactToMessageAdd(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "react-room")
	msgID := mustPost(t, s, "react-room", "Claude", "React to me")

	reactions, added, err := s.ReactToMessage(msgID, "👍", "Gemini")
	if err != nil {
		t.Fatalf("ReactToMessage error: %v", err)
	}
	if !added {
		t.Error("expected added=true on first reaction")
	}
	authors := reactions["👍"]
	if len(authors) != 1 || authors[0] != "Gemini" {
		t.Errorf("expected ['Gemini'], got %v", authors)
	}
}

func TestReactToMessageToggleOff(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "react-toggle-room")
	msgID := mustPost(t, s, "react-toggle-room", "Claude", "Toggle me")

	// Add reaction
	_, _, err := s.ReactToMessage(msgID, "👍", "Gemini")
	if err != nil {
		t.Fatalf("first ReactToMessage error: %v", err)
	}

	// Remove reaction (toggle off)
	reactions, added, err := s.ReactToMessage(msgID, "👍", "Gemini")
	if err != nil {
		t.Fatalf("second ReactToMessage error: %v", err)
	}
	if added {
		t.Error("expected added=false on toggle off")
	}
	if _, ok := reactions["👍"]; ok {
		t.Error("expected 👍 key removed after last author toggled off")
	}
}

func TestReactToMessageMultipleAuthors(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "react-multi-room")
	msgID := mustPost(t, s, "react-multi-room", "Claude", "Multiple reactions")

	s.ReactToMessage(msgID, "👍", "Gemini") //nolint:errcheck
	reactions, _, err := s.ReactToMessage(msgID, "👍", "GPT")
	if err != nil {
		t.Fatalf("ReactToMessage error: %v", err)
	}
	if len(reactions["👍"]) != 2 {
		t.Errorf("expected 2 authors, got %d", len(reactions["👍"]))
	}
}

func TestReactToMessageMultipleEmojis(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "react-emoji-room")
	msgID := mustPost(t, s, "react-emoji-room", "Claude", "Multiple emojis")

	s.ReactToMessage(msgID, "👍", "Gemini") //nolint:errcheck
	reactions, _, err := s.ReactToMessage(msgID, "🎉", "Gemini")
	if err != nil {
		t.Fatalf("ReactToMessage error: %v", err)
	}
	if len(reactions) != 2 {
		t.Errorf("expected 2 emoji keys, got %d", len(reactions))
	}
}

func TestReactToMessageNotFound(t *testing.T) {
	s := setupTestServer(t)

	_, _, err := s.ReactToMessage("nonexistent-id", "👍", "Gemini")
	if err == nil {
		t.Fatal("expected error for nonexistent message")
	}
}

func TestReactToMessageRemoveOneOfMany(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "react-remove-room")
	msgID := mustPost(t, s, "react-remove-room", "Claude", "Remove one")

	// Two authors add same emoji
	s.ReactToMessage(msgID, "👍", "Gemini") //nolint:errcheck
	s.ReactToMessage(msgID, "👍", "GPT")    //nolint:errcheck

	// Remove one
	reactions, added, err := s.ReactToMessage(msgID, "👍", "Gemini")
	if err != nil {
		t.Fatalf("ReactToMessage error: %v", err)
	}
	if added {
		t.Error("expected added=false")
	}
	// Key should still exist (GPT is still there)
	if len(reactions["👍"]) != 1 || reactions["👍"][0] != "GPT" {
		t.Errorf("expected ['GPT'] remaining, got %v", reactions["👍"])
	}
}

func TestReactToMessageExistingReactions(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "react-existing-room")
	msgID := mustPost(t, s, "react-existing-room", "Claude", "Has reactions")

	// Seed two emojis
	s.ReactToMessage(msgID, "👍", "A") //nolint:errcheck
	s.ReactToMessage(msgID, "🎉", "B") //nolint:errcheck

	// Adding a new emoji should preserve existing
	reactions, _, err := s.ReactToMessage(msgID, "🔥", "C")
	if err != nil {
		t.Fatalf("ReactToMessage error: %v", err)
	}
	if len(reactions) != 3 {
		t.Errorf("expected 3 emoji keys, got %d", len(reactions))
	}
}
