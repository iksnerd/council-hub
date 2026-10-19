package council

import "testing"

func TestDeleteNotebookIsSoftAndRestorable(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)
	for _, label := range []string{"a", "b", "c"} {
		if _, err := s.AddOutlineEntry("ol-nb", "task", "", label, ""); err != nil {
			t.Fatal(err)
		}
	}

	n, err := s.DeleteNotebook("ol-nb")
	if err != nil || n != 3 {
		t.Fatalf("DeleteNotebook = (%d, %v), want (3, nil)", n, err)
	}
	if _, err := s.GetNotebook("ol-nb"); err == nil {
		t.Fatal("soft-deleted notebook must not be readable")
	}
	if list, _ := s.ListNotebooks(""); len(list) != 0 {
		t.Errorf("soft-deleted notebook must not be listed, got %d", len(list))
	}
	if err := s.CreateNotebook("ol-nb", "", ""); err == nil {
		t.Error("create over a soft-deleted id must fail, pointing at restore")
	}
	if _, err := s.DeleteNotebook("ol-nb"); err == nil {
		t.Error("deleting an already-deleted notebook should error")
	}

	got, err := s.RestoreNotebook("ol-nb")
	if err != nil || got != 3 {
		t.Fatalf("RestoreNotebook = (%d, %v), want (3, nil)", got, err)
	}
	if nb, err := s.GetNotebook("ol-nb"); err != nil || nb.EntryCount != 3 {
		t.Errorf("restored notebook should have 3 entries, got %+v err=%v", nb, err)
	}
	if _, err := s.RestoreNotebook("ol-nb"); err == nil {
		t.Error("restoring a live notebook should error")
	}
}

func TestPurgeNotebookRequiresSoftDelete(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)
	if _, err := s.AddOutlineEntry("ol-nb", "task", "", "a", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PurgeNotebook("ol-nb"); err == nil {
		t.Fatal("purge of a live notebook must be refused")
	}
	if _, err := s.DeleteNotebook("ol-nb"); err != nil {
		t.Fatal(err)
	}
	n, err := s.PurgeNotebook("ol-nb")
	if err != nil || n != 1 {
		t.Fatalf("PurgeNotebook = (%d, %v), want (1, nil)", n, err)
	}
	if _, err := s.RestoreNotebook("ol-nb"); err == nil {
		t.Error("purged notebook cannot be restored")
	}
	var c int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM notebook_entries WHERE notebook_id = 'ol-nb'`).Scan(&c)
	if c != 0 {
		t.Errorf("purge should remove entries, %d left", c)
	}
}

func TestGetOutlineFilteredPushesDownStatusProjectLimitEntry(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)
	mustCreateRoom(t, s, "alpha-room", withProject("alpha"))
	mustCreateRoom(t, s, "beta-room", withProject("beta"))
	for _, r := range []string{"alpha-room", "beta-room"} {
		if _, err := s.AddOutlineEntry("ol-nb", "room_ref", r, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	tA, _ := s.AddOutlineEntry("ol-nb", "task", "", "alpha: first", "")
	tB, _ := s.AddOutlineEntry("ol-nb", "task", "", "[beta] second", "")
	tC, _ := s.AddOutlineEntry("ol-nb", "task", "", "unrelated", "")
	if err := s.SetTaskStatus(tC, "done"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateStatus("beta-room", "resolved"); err != nil {
		t.Fatal(err)
	}

	// keys are the entry's ref (room) or its id (task)
	keys := func(f OutlineFilter) map[string]bool {
		_, es, err := s.GetOutlineFiltered("ol-nb", f)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]bool{}
		for _, e := range es {
			if e.Kind == "room_ref" {
				m[e.RefID] = true
			} else {
				m[e.ID] = true
			}
		}
		return m
	}

	if got := keys(OutlineFilter{Status: "done"}); len(got) != 2 || !got[tC] || !got["beta-room"] {
		t.Errorf("status=done wrong: %v", got)
	}
	if got := keys(OutlineFilter{Status: "open"}); len(got) != 3 || got[tC] || got["beta-room"] {
		t.Errorf("status=open wrong: %v", got)
	}
	if got := keys(OutlineFilter{Project: "alpha"}); len(got) != 2 || !got[tA] || !got["alpha-room"] {
		t.Errorf("project=alpha wrong: %v", got)
	}
	if got := keys(OutlineFilter{Project: "beta"}); len(got) != 2 || !got[tB] || !got["beta-room"] {
		t.Errorf("project=beta wrong: %v", got)
	}
	if got := keys(OutlineFilter{EntryID: tB}); len(got) != 1 || !got[tB] {
		t.Errorf("entry_id wrong: %v", got)
	}
	_, es, _ := s.GetOutlineFiltered("ol-nb", OutlineFilter{Limit: 2})
	if len(es) != 2 || es[1].ID != tC {
		t.Errorf("limit should keep the last 2 entries, got %d", len(es))
	}
}
