package council

import "testing"

func TestMarkAllReadSetsLatestIDInEveryRoom(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "ma-a")
	mustCreateRoom(t, s, "ma-b")
	mustCreateRoom(t, s, "ma-empty")
	mustPost(t, s, "ma-a", "alice", "one")
	latestA := mustPost(t, s, "ma-a", "alice", "two")
	latestB := mustPost(t, s, "ma-b", "bob", "hello")

	updated, skipped, err := s.MarkAllRead("carol")
	if err != nil {
		t.Fatal(err)
	}
	if updated != 2 || skipped != 1 {
		t.Fatalf("updated=%d skipped=%d, want 2 and 1", updated, skipped)
	}
	for room, want := range map[string]string{"ma-a": latestA, "ma-b": latestB, "ma-empty": ""} {
		got, err := s.GetCursor("carol", room)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("cursor in %s = %q, want %q", room, got, want)
		}
	}
	// Other agents are untouched.
	if got, _ := s.GetCursor("alice", "ma-a"); got != "" {
		t.Errorf("alice cursor changed: %q", got)
	}
}

func TestMarkAllReadOverwritesExistingCursor(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "ma-over")
	first := mustPost(t, s, "ma-over", "alice", "one")
	if err := s.MarkRead("carol", "ma-over", first); err != nil {
		t.Fatal(err)
	}
	second := mustPost(t, s, "ma-over", "alice", "two")
	if _, _, err := s.MarkAllRead("carol"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetCursor("carol", "ma-over"); got != second {
		t.Errorf("cursor = %q, want %q", got, second)
	}
}
