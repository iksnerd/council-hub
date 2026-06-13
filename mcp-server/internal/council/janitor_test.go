package council

import (
	"strings"
	"testing"
)

// --- Coherence linter: live contradictions ---

func TestLintIncoherentFlagsLiveContradiction(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-contra")
	a := mustPostTyped(t, cs, "lint-contra", "Claude", "We ship on Friday", "decision")
	b := mustPostTyped(t, cs, "lint-contra", "Gemini", "We ship on Monday", "decision")
	if _, err := cs.CreateLink(b, a, "contradicts", "Gemini"); err != nil {
		t.Fatal(err)
	}

	cs.lintIncoherent()

	room, _ := cs.GetRoom("lint-contra")
	if !hasTag(room.Tags, "incoherent") {
		t.Errorf("expected 'incoherent' tag for live contradiction, got '%s'", room.Tags)
	}

	msgs, _ := cs.GetRecentMessages("lint-contra", 5)
	found := false
	for _, m := range msgs {
		if m.Author == "system" && strings.Contains(m.Content, "coherence problem") {
			found = true
		}
	}
	if !found {
		t.Error("expected coherence linter system message in room")
	}
}

func TestLintIncoherentSkipsReconciledContradiction(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-contra-synth")
	a := mustPostTyped(t, cs, "lint-contra-synth", "Claude", "Option A", "decision")
	b := mustPostTyped(t, cs, "lint-contra-synth", "Gemini", "Option B", "decision")
	if _, err := cs.CreateLink(b, a, "contradicts", "Gemini"); err != nil {
		t.Fatal(err)
	}
	// A synthesis posted after the contradiction reconciles it.
	mustPostTyped(t, cs, "lint-contra-synth", "Claude", "We go with A, here's why", "synthesis")

	cs.lintIncoherent()

	room, _ := cs.GetRoom("lint-contra-synth")
	if hasTag(room.Tags, "incoherent") {
		t.Errorf("a contradiction with a later synthesis should not be flagged, got '%s'", room.Tags)
	}
}

func TestLintIncoherentSkipsSupersededContradiction(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-contra-super")
	a := mustPostTyped(t, cs, "lint-contra-super", "Claude", "Old call", "decision")
	b := mustPostTyped(t, cs, "lint-contra-super", "Gemini", "Competing call", "decision")
	if _, err := cs.CreateLink(b, a, "contradicts", "Gemini"); err != nil {
		t.Fatal(err)
	}
	// Superseding one side declares a winner — the conflict is resolved.
	if _, err := cs.PostMessageWithRefs("lint-contra-super", "Claude", "Settled call", "decision", "", "", b); err != nil {
		t.Fatal(err)
	}

	cs.lintIncoherent()

	room, _ := cs.GetRoom("lint-contra-super")
	if hasTag(room.Tags, "incoherent") {
		t.Errorf("a contradiction with a superseded endpoint should not be flagged, got '%s'", room.Tags)
	}
}

func TestLintIncoherentIgnoresCrossRoomContradiction(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "contra-room-a")
	mustCreateRoom(t, cs, "contra-room-b")
	a := mustPostTyped(t, cs, "contra-room-a", "Claude", "Stance A", "decision")
	b := mustPostTyped(t, cs, "contra-room-b", "Gemini", "Stance B", "decision")
	if _, err := cs.CreateLink(b, a, "contradicts", "Gemini"); err != nil {
		t.Fatal(err)
	}

	cs.lintIncoherent()

	// Coherence is scoped per-room (a room is a topic); cross-room contradictions
	// aren't flagged by this check.
	for _, id := range []string{"contra-room-a", "contra-room-b"} {
		room, _ := cs.GetRoom(id)
		if hasTag(room.Tags, "incoherent") {
			t.Errorf("cross-room contradiction should not flag %s, got '%s'", id, room.Tags)
		}
	}
}

// --- Coherence linter: duplicate syntheses ---

func TestLintIncoherentFlagsDuplicateSynthesesBothRooms(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "dup-room-a")
	mustCreateRoom(t, cs, "dup-room-b")
	synthA := mustPostTyped(t, cs, "dup-room-a", "Claude", "How auth works", "synthesis")
	synthB := mustPostTyped(t, cs, "dup-room-b", "Gemini", "How auth works (again)", "synthesis")
	if _, err := cs.CreateLink(synthB, synthA, "duplicates", "Gemini"); err != nil {
		t.Fatal(err)
	}

	cs.lintIncoherent()

	for _, id := range []string{"dup-room-a", "dup-room-b"} {
		room, _ := cs.GetRoom(id)
		if !hasTag(room.Tags, "incoherent") {
			t.Errorf("expected 'incoherent' tag on %s for duplicate syntheses, got '%s'", id, room.Tags)
		}
	}
}

func TestLintIncoherentSkipsNonSynthesisDuplicates(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "dup-thoughts")
	a := mustPostTyped(t, cs, "dup-thoughts", "Claude", "stray idea", "thought")
	b := mustPostTyped(t, cs, "dup-thoughts", "Gemini", "same idea", "thought")
	if _, err := cs.CreateLink(b, a, "duplicates", "Gemini"); err != nil {
		t.Fatal(err)
	}

	cs.lintIncoherent()

	room, _ := cs.GetRoom("dup-thoughts")
	if hasTag(room.Tags, "incoherent") {
		t.Errorf("duplicate non-synthesis messages should not be flagged, got '%s'", room.Tags)
	}
}

func TestLintIncoherentSkipsSupersededDuplicate(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "dup-resolved")
	mustCreateRoom(t, cs, "dup-canonical")
	dupSynth := mustPostTyped(t, cs, "dup-resolved", "Claude", "redundant article", "synthesis")
	canonical := mustPostTyped(t, cs, "dup-canonical", "Gemini", "canonical article", "synthesis")
	if _, err := cs.CreateLink(dupSynth, canonical, "duplicates", "Gemini"); err != nil {
		t.Fatal(err)
	}
	// Consolidating: supersede the redundant synthesis with the canonical one.
	if _, err := cs.PostMessageWithRefs("dup-resolved", "Claude", "see the canonical version", "synthesis", "", "", dupSynth); err != nil {
		t.Fatal(err)
	}

	cs.lintIncoherent()

	room, _ := cs.GetRoom("dup-canonical")
	if hasTag(room.Tags, "incoherent") {
		t.Errorf("a superseded duplicate should not be flagged, got '%s'", room.Tags)
	}
}

// --- Idempotency and self-healing ---

func TestLintIncoherentIdempotent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "lint-contra-idem")
	a := mustPostTyped(t, cs, "lint-contra-idem", "Claude", "A", "decision")
	b := mustPostTyped(t, cs, "lint-contra-idem", "Gemini", "B", "decision")
	if _, err := cs.CreateLink(b, a, "contradicts", "Gemini"); err != nil {
		t.Fatal(err)
	}

	cs.lintIncoherent()
	cs.lintIncoherent() // run twice

	room, _ := cs.GetRoom("lint-contra-idem")
	count := 0
	for _, tag := range strings.Split(room.Tags, ",") {
		if strings.TrimSpace(tag) == "incoherent" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'incoherent' tag, got %d in '%s'", count, room.Tags)
	}
}

func TestPostSynthesisClearsIncoherent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "incoherent-clear-synth", withTags("incoherent,important"))
	mustPostTyped(t, cs, "incoherent-clear-synth", "Claude", "reconciliation", "synthesis")

	room, _ := cs.GetRoom("incoherent-clear-synth")
	if hasTag(room.Tags, "incoherent") {
		t.Errorf("posting a synthesis should clear 'incoherent', got '%s'", room.Tags)
	}
	if !hasTag(room.Tags, "important") {
		t.Errorf("non-linter tags should survive, got '%s'", room.Tags)
	}
}

func TestPostSupersedesClearsIncoherent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "incoherent-clear-super", withTags("incoherent,important"))
	target := mustPostTyped(t, cs, "incoherent-clear-super", "Claude", "old", "decision")
	if _, err := cs.PostMessageWithRefs("incoherent-clear-super", "Claude", "winner", "decision", "", "", target); err != nil {
		t.Fatal(err)
	}

	room, _ := cs.GetRoom("incoherent-clear-super")
	if hasTag(room.Tags, "incoherent") {
		t.Errorf("a superseding post should clear 'incoherent', got '%s'", room.Tags)
	}
	if !hasTag(room.Tags, "important") {
		t.Errorf("non-linter tags should survive, got '%s'", room.Tags)
	}
}

func TestLintIncoherentSelfCorrectsStaleFlag(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "dup-fixed-a")
	mustCreateRoom(t, cs, "dup-fixed-b")
	synthA := mustPostTyped(t, cs, "dup-fixed-a", "Claude", "article", "synthesis")
	synthB := mustPostTyped(t, cs, "dup-fixed-b", "Gemini", "article again", "synthesis")
	if _, err := cs.CreateLink(synthB, synthA, "duplicates", "Gemini"); err != nil {
		t.Fatal(err)
	}

	cs.lintIncoherent()
	if room, _ := cs.GetRoom("dup-fixed-b"); !hasTag(room.Tags, "incoherent") {
		t.Fatalf("precondition: dup-fixed-b should be flagged, got '%s'", room.Tags)
	}

	// Resolve the duplicate by superseding synthA in room A. That clears A's flag via
	// the event-driven path, but B's flag persists until the next sweep reconciles it.
	if _, err := cs.PostMessageWithRefs("dup-fixed-a", "Claude", "canonical now", "synthesis", "", "", synthA); err != nil {
		t.Fatal(err)
	}

	cs.lintIncoherent()

	room, _ := cs.GetRoom("dup-fixed-b")
	if hasTag(room.Tags, "incoherent") {
		t.Errorf("the sweep should self-correct the now-stale 'incoherent' flag on dup-fixed-b, got '%s'", room.Tags)
	}
}

func TestResolveClearsIncoherent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "incoherent-resolve", withTags("incoherent,important"))

	if err := cs.UpdateStatus("incoherent-resolve", "resolved"); err != nil {
		t.Fatal(err)
	}

	room, _ := cs.GetRoom("incoherent-resolve")
	if hasTag(room.Tags, "incoherent") {
		t.Errorf("resolving a room should clear 'incoherent', got '%s'", room.Tags)
	}
	if !hasTag(room.Tags, "important") {
		t.Errorf("non-linter tags should survive resolve, got '%s'", room.Tags)
	}
}

// --- Janitor: no-notebook nudge ---

func TestLintProjectsNeedingNotebook(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "nn-a", withProject("busy-proj"))
	mustCreateRoom(t, cs, "nn-b", withProject("busy-proj"))
	// 8 decision/action messages across the project's rooms, no notebook.
	for i := 0; i < 5; i++ {
		mustPostTyped(t, cs, "nn-a", "Claude", "decided", "decision")
	}
	for i := 0; i < 3; i++ {
		mustPostTyped(t, cs, "nn-b", "Claude", "shipped", "action")
	}

	got := cs.lintProjectsNeedingNotebook()

	found := false
	for _, p := range got {
		if p == "busy-proj" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'busy-proj' to be nudged for a notebook, got %v", got)
	}
}

func TestLintProjectsNeedingNotebookSkipsWhenNotebookExists(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "nn-has", withProject("documented-proj"))
	for i := 0; i < 10; i++ {
		mustPostTyped(t, cs, "nn-has", "Claude", "decided", "decision")
	}
	if err := cs.CreateNotebook("doc-nb", "documented-proj", "Doc"); err != nil {
		t.Fatal(err)
	}

	got := cs.lintProjectsNeedingNotebook()
	for _, p := range got {
		if p == "documented-proj" {
			t.Errorf("a project with a notebook should not be nudged, got %v", got)
		}
	}
}

func TestLintProjectsNeedingNotebookBelowThreshold(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "nn-quiet", withProject("quiet-proj"))
	// Only a few decisions — under notebookNudgeMinDecisions.
	for i := 0; i < 3; i++ {
		mustPostTyped(t, cs, "nn-quiet", "Claude", "decided", "decision")
	}

	got := cs.lintProjectsNeedingNotebook()
	for _, p := range got {
		if p == "quiet-proj" {
			t.Errorf("a project under the threshold should not be nudged, got %v", got)
		}
	}
}

func TestJanitorSweepIncludesIncoherent(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "sweep-incoherent")
	a := mustPostTyped(t, cs, "sweep-incoherent", "Claude", "A", "decision")
	b := mustPostTyped(t, cs, "sweep-incoherent", "Gemini", "B", "decision")
	if _, err := cs.CreateLink(b, a, "contradicts", "Gemini"); err != nil {
		t.Fatal(err)
	}

	result := cs.JanitorSweep()

	found := false
	for _, id := range result.Incoherent {
		if id == "sweep-incoherent" {
			found = true
		}
	}
	if !found {
		t.Errorf("JanitorSweep should report 'sweep-incoherent' in Incoherent, got %v", result.Incoherent)
	}
	room, _ := cs.GetRoom("sweep-incoherent")
	if !hasTag(room.Tags, "incoherent") {
		t.Errorf("JanitorSweep should flag incoherent, got '%s'", room.Tags)
	}
}
