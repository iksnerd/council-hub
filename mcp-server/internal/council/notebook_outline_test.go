package council

import (
	"strings"
	"testing"
)

// seedOutline creates a notebook with one room/message to reference.
// Returns the referenced message ID.
func seedOutline(t *testing.T, s *Server) string {
	t.Helper()
	mustCreateRoom(t, s, "ol-room", withProject("ol-proj"))
	if err := s.SetRepo("ol-room", "alice/widgets"); err != nil {
		t.Fatalf("SetRepo failed: %v", err)
	}
	msgID := mustPostTyped(t, s, "ol-room", "claude", "the key decision {sha:abc1234}", "decision")
	if err := s.CreateNotebook("ol-nb", "ol-proj", "Release Notes"); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	return msgID
}

func TestCreateNotebookDuplicate(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)

	err := s.CreateNotebook("ol-nb", "ol-proj", "Again")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected already-exists error, got %v", err)
	}
}

func TestCreateNotebookValidation(t *testing.T) {
	s := setupTestServer(t)

	if err := s.CreateNotebook("", "proj", "t"); err == nil {
		t.Error("expected error for empty id")
	}
	if err := s.CreateNotebook("nb", "", "t"); err == nil {
		t.Error("expected error for empty project")
	}
}

func TestListNotebooks(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)
	if err := s.CreateNotebook("other-nb", "other-proj", "Other"); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	all, err := s.ListNotebooks("")
	if err != nil {
		t.Fatalf("ListNotebooks failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 notebooks, got %d", len(all))
	}

	scoped, err := s.ListNotebooks("ol-proj")
	if err != nil {
		t.Fatalf("ListNotebooks failed: %v", err)
	}
	if len(scoped) != 1 || scoped[0].ID != "ol-nb" {
		t.Errorf("expected only ol-nb for ol-proj, got %+v", scoped)
	}
}

func TestAddOutlineEntryOrderAndInsertAfter(t *testing.T) {
	s := setupTestServer(t)
	msgID := seedOutline(t, s)

	e1, err := s.AddOutlineEntry("ol-nb", "prose", "", "## Section A", "")
	if err != nil {
		t.Fatalf("AddOutlineEntry failed: %v", err)
	}
	e2, err := s.AddOutlineEntry("ol-nb", "ref", msgID, "", "")
	if err != nil {
		t.Fatalf("AddOutlineEntry failed: %v", err)
	}
	// Insert between e1 and e2
	e3, err := s.AddOutlineEntry("ol-nb", "prose", "", "intro text", e1)
	if err != nil {
		t.Fatalf("AddOutlineEntry (after) failed: %v", err)
	}

	_, entries, err := s.GetOutline("ol-nb")
	if err != nil {
		t.Fatalf("GetOutline failed: %v", err)
	}
	got := []string{entries[0].ID, entries[1].ID, entries[2].ID}
	want := []string{e1, e3, e2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch at %d: got %v want %v", i, got, want)
		}
	}
	// Positions are dense 1..n
	for i, e := range entries {
		if e.Position != i+1 {
			t.Errorf("entry %d has position %d", i, e.Position)
		}
	}
}

func TestAddOutlineEntryValidation(t *testing.T) {
	s := setupTestServer(t)
	msgID := seedOutline(t, s)

	if _, err := s.AddOutlineEntry("no-such-nb", "prose", "", "x", ""); err == nil {
		t.Error("expected error for unknown notebook")
	}
	if _, err := s.AddOutlineEntry("ol-nb", "heading", "", "x", ""); err == nil {
		t.Error("expected error for invalid kind")
	}
	if _, err := s.AddOutlineEntry("ol-nb", "ref", "no-such-msg", "", ""); err == nil {
		t.Error("expected error for missing ref message")
	}
	if _, err := s.AddOutlineEntry("ol-nb", "prose", "", "  ", ""); err == nil {
		t.Error("expected error for empty prose")
	}
	if _, err := s.AddOutlineEntry("ol-nb", "prose", "", "x", "ghost-entry"); err == nil {
		t.Error("expected error for unknown after_entry_id")
	}
	if _, err := s.AddOutlineEntry("ol-nb", "ref", msgID, "", ""); err != nil {
		t.Errorf("valid ref add failed: %v", err)
	}
}

func TestGetOutlineTranscludesRefs(t *testing.T) {
	s := setupTestServer(t)
	msgID := seedOutline(t, s)

	if _, err := s.AddOutlineEntry("ol-nb", "ref", msgID, "", ""); err != nil {
		t.Fatalf("AddOutlineEntry failed: %v", err)
	}

	n, entries, err := s.GetOutline("ol-nb")
	if err != nil {
		t.Fatalf("GetOutline failed: %v", err)
	}
	if n.Title != "Release Notes" || n.EntryCount != 1 {
		t.Errorf("notebook metadata wrong: %+v", n)
	}
	e := entries[0]
	if !e.RefFound {
		t.Fatal("ref should resolve")
	}
	if e.RefRoomID != "ol-room" || e.RefAuthor != "claude" || e.RefType != "decision" {
		t.Errorf("transclusion fields wrong: %+v", e)
	}
	if e.RefContent != "the key decision {sha:abc1234}" {
		t.Errorf("content wrong: %q", e.RefContent)
	}
	if e.RefRepo != "alice/widgets" {
		t.Errorf("repo wrong: %q", e.RefRepo)
	}
}

func TestGetOutlineDanglingRef(t *testing.T) {
	s := setupTestServer(t)
	msgID := seedOutline(t, s)

	if _, err := s.AddOutlineEntry("ol-nb", "ref", msgID, "", ""); err != nil {
		t.Fatalf("AddOutlineEntry failed: %v", err)
	}
	if _, err := s.DeleteMessages([]string{msgID}); err != nil {
		t.Fatalf("DeleteMessages failed: %v", err)
	}

	_, entries, err := s.GetOutline("ol-nb")
	if err != nil {
		t.Fatalf("GetOutline failed: %v", err)
	}
	if entries[0].RefFound {
		t.Error("deleted ref should resolve as not found")
	}
}

func TestUpdateOutlineEntry(t *testing.T) {
	s := setupTestServer(t)
	msgID := seedOutline(t, s)

	proseID, _ := s.AddOutlineEntry("ol-nb", "prose", "", "old text", "")
	refID, _ := s.AddOutlineEntry("ol-nb", "ref", msgID, "", "")

	if err := s.UpdateOutlineEntry(proseID, "new text"); err != nil {
		t.Fatalf("UpdateOutlineEntry failed: %v", err)
	}
	_, entries, _ := s.GetOutline("ol-nb")
	if entries[0].Prose != "new text" {
		t.Errorf("prose not updated: %q", entries[0].Prose)
	}

	if err := s.UpdateOutlineEntry(refID, "x"); err == nil {
		t.Error("expected error updating a ref entry")
	}
	if err := s.UpdateOutlineEntry("ghost", "x"); err == nil {
		t.Error("expected error for unknown entry")
	}
}

func TestRemoveOutlineEntryRenumbers(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)

	e1, _ := s.AddOutlineEntry("ol-nb", "prose", "", "one", "")
	e2, _ := s.AddOutlineEntry("ol-nb", "prose", "", "two", "")
	e3, _ := s.AddOutlineEntry("ol-nb", "prose", "", "three", "")
	_ = e2

	if err := s.RemoveOutlineEntry(e2); err != nil {
		t.Fatalf("RemoveOutlineEntry failed: %v", err)
	}

	_, entries, _ := s.GetOutline("ol-nb")
	if len(entries) != 2 || entries[0].ID != e1 || entries[1].ID != e3 {
		t.Fatalf("wrong entries after remove: %+v", entries)
	}
	if entries[0].Position != 1 || entries[1].Position != 2 {
		t.Errorf("positions not renumbered: %d, %d", entries[0].Position, entries[1].Position)
	}

	if err := s.RemoveOutlineEntry("ghost"); err == nil {
		t.Error("expected error for unknown entry")
	}
}

func TestMoveOutlineEntry(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)

	e1, _ := s.AddOutlineEntry("ol-nb", "prose", "", "one", "")
	e2, _ := s.AddOutlineEntry("ol-nb", "prose", "", "two", "")
	e3, _ := s.AddOutlineEntry("ol-nb", "prose", "", "three", "")

	// Move e3 to the top
	if err := s.MoveOutlineEntry(e3, ""); err != nil {
		t.Fatalf("MoveOutlineEntry to top failed: %v", err)
	}
	_, entries, _ := s.GetOutline("ol-nb")
	if entries[0].ID != e3 || entries[1].ID != e1 || entries[2].ID != e2 {
		t.Fatalf("move to top produced wrong order: %+v", []string{entries[0].ID, entries[1].ID, entries[2].ID})
	}

	// Move e3 after e2 (to the end)
	if err := s.MoveOutlineEntry(e3, e2); err != nil {
		t.Fatalf("MoveOutlineEntry after failed: %v", err)
	}
	_, entries, _ = s.GetOutline("ol-nb")
	if entries[2].ID != e3 {
		t.Errorf("move after produced wrong order")
	}

	if err := s.MoveOutlineEntry(e3, e3); err == nil {
		t.Error("expected error moving entry after itself")
	}
	if err := s.MoveOutlineEntry("ghost", ""); err == nil {
		t.Error("expected error for unknown entry")
	}
}

func TestDeleteNotebook(t *testing.T) {
	s := setupTestServer(t)
	msgID := seedOutline(t, s)

	if _, err := s.AddOutlineEntry("ol-nb", "ref", msgID, "", ""); err != nil {
		t.Fatalf("AddOutlineEntry failed: %v", err)
	}
	if err := s.DeleteNotebook("ol-nb"); err != nil {
		t.Fatalf("DeleteNotebook failed: %v", err)
	}
	if _, err := s.GetNotebook("ol-nb"); err == nil {
		t.Error("notebook should be gone")
	}
	// Referenced message survives — refs are pointers, not copies
	if _, err := s.GetMessageByID(msgID); err != nil {
		t.Errorf("referenced message should survive notebook deletion: %v", err)
	}
	if err := s.DeleteNotebook("ghost"); err == nil {
		t.Error("expected error for unknown notebook")
	}
}
