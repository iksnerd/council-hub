package council

import (
	"testing"
)

// seedNotebookProject creates two rooms in project "nb-proj" (one with a repo)
// plus an unrelated room, and posts a mix of typed messages. Returns the IDs
// of the typed messages in posting order.
func seedNotebookProject(t *testing.T, s *Server) []string {
	t.Helper()
	mustCreateRoom(t, s, "nb-room-a", withProject("nb-proj"))
	mustCreateRoom(t, s, "nb-room-b", withProject("nb-proj"))
	mustCreateRoom(t, s, "nb-other", withProject("other-proj"))
	if err := s.SetRepo("nb-room-a", "alice/widgets"); err != nil {
		t.Fatalf("SetRepo failed: %v", err)
	}

	var ids []string
	ids = append(ids, mustPostTyped(t, s, "nb-room-a", "claude", "decided to use SQLite", "decision"))
	mustPost(t, s, "nb-room-a", "claude", "just chatting") // type "message" — excluded by default
	ids = append(ids, mustPostTyped(t, s, "nb-room-b", "gemini", "shipped the parser {sha:abc1234}", "action"))
	ids = append(ids, mustPostTyped(t, s, "nb-room-a", "claude", "compiled findings", "synthesis"))
	mustPostTyped(t, s, "nb-other", "claude", "other-project decision", "decision")
	return ids
}

func TestGetNotebookEntriesAcrossProject(t *testing.T) {
	s := setupTestServer(t)
	ids := seedNotebookProject(t, s)

	entries, err := s.GetNotebookEntries("nb-proj", nil, "", "", "", 0)
	if err != nil {
		t.Fatalf("GetNotebookEntries failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Chronological order across rooms (UUIDv7 IDs ascending)
	for i, want := range ids {
		if entries[i].ID != want {
			t.Errorf("entry %d: expected ID %s, got %s", i, want, entries[i].ID)
		}
	}

	// Per-room repo carried on each entry
	if entries[0].Repo != "alice/widgets" {
		t.Errorf("expected repo 'alice/widgets' on room-a entry, got %q", entries[0].Repo)
	}
	if entries[1].Repo != "" {
		t.Errorf("expected empty repo on room-b entry, got %q", entries[1].Repo)
	}

	// Other project excluded
	for _, e := range entries {
		if e.RoomID == "nb-other" {
			t.Error("entry from another project leaked into notebook")
		}
	}
}

func TestGetNotebookEntriesTypeFilter(t *testing.T) {
	s := setupTestServer(t)
	seedNotebookProject(t, s)

	entries, err := s.GetNotebookEntries("nb-proj", []string{"action"}, "", "", "", 0)
	if err != nil {
		t.Fatalf("GetNotebookEntries failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 action entry, got %d", len(entries))
	}
	if entries[0].MessageType != "action" {
		t.Errorf("expected action, got %s", entries[0].MessageType)
	}
}

func TestGetNotebookEntriesAfterID(t *testing.T) {
	s := setupTestServer(t)
	ids := seedNotebookProject(t, s)

	entries, err := s.GetNotebookEntries("nb-proj", nil, "", "", ids[0], 0)
	if err != nil {
		t.Fatalf("GetNotebookEntries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after first ID, got %d", len(entries))
	}
	if entries[0].ID != ids[1] || entries[1].ID != ids[2] {
		t.Errorf("delta read returned wrong entries: %s, %s", entries[0].ID, entries[1].ID)
	}
}

func TestGetNotebookEntriesLimitKeepsMostRecent(t *testing.T) {
	s := setupTestServer(t)
	ids := seedNotebookProject(t, s)

	entries, err := s.GetNotebookEntries("nb-proj", nil, "", "", "", 2)
	if err != nil {
		t.Fatalf("GetNotebookEntries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Most recent two, still chronological
	if entries[0].ID != ids[1] || entries[1].ID != ids[2] {
		t.Errorf("limit should keep most recent entries in order, got %s, %s", entries[0].ID, entries[1].ID)
	}
}

func TestGetNotebookEntriesSinceUntil(t *testing.T) {
	s := setupTestServer(t)
	seedNotebookProject(t, s)

	// All messages were just posted — a future "since" excludes everything,
	// and a future "until" includes everything. Also exercises the T separator.
	entries, err := s.GetNotebookEntries("nb-proj", nil, "2099-01-01T00:00:00", "", "", 0)
	if err != nil {
		t.Fatalf("GetNotebookEntries failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries with future since, got %d", len(entries))
	}

	entries, err = s.GetNotebookEntries("nb-proj", nil, "", "2099-01-01T00:00:00", "", 0)
	if err != nil {
		t.Fatalf("GetNotebookEntries failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries with future until, got %d", len(entries))
	}
}

func TestGetNotebookEntriesProjectNormalized(t *testing.T) {
	s := setupTestServer(t)
	seedNotebookProject(t, s)

	// "NB Proj" normalizes to "nb-proj"
	entries, err := s.GetNotebookEntries("NB Proj", nil, "", "", "", 0)
	if err != nil {
		t.Fatalf("GetNotebookEntries failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries for normalized project name, got %d", len(entries))
	}
}

func TestGetNotebookEntriesRequiresProject(t *testing.T) {
	s := setupTestServer(t)

	if _, err := s.GetNotebookEntries("", nil, "", "", "", 0); err == nil {
		t.Error("expected error for empty project")
	}
}

func TestCountRoomsInProject(t *testing.T) {
	s := setupTestServer(t)
	seedNotebookProject(t, s)

	count, err := s.CountRoomsInProject("nb-proj")
	if err != nil {
		t.Fatalf("CountRoomsInProject failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rooms, got %d", count)
	}

	count, err = s.CountRoomsInProject("no-such-project")
	if err != nil {
		t.Fatalf("CountRoomsInProject failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rooms, got %d", count)
	}
}
