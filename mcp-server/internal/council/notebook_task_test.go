package council

import "testing"

func TestAddTaskEntryStartsOpen(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)

	id, err := s.AddOutlineEntry("ol-nb", "task", "", "E3 — query_skills_registry", "")
	if err != nil {
		t.Fatalf("AddOutlineEntry(task) failed: %v", err)
	}

	_, entries, _ := s.GetOutline("ol-nb")
	var got OutlineEntry
	for _, e := range entries {
		if e.ID == id {
			got = e
		}
	}
	if got.Kind != "task" {
		t.Fatalf("expected kind task, got %q", got.Kind)
	}
	if got.Status != "open" {
		t.Errorf("a new task should start 'open', got %q", got.Status)
	}
	if got.Prose != "E3 — query_skills_registry" {
		t.Errorf("task label wrong: %q", got.Prose)
	}
}

func TestAddTaskEntryRequiresLabel(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)

	if _, err := s.AddOutlineEntry("ol-nb", "task", "", "   ", ""); err == nil {
		t.Error("expected error for a task with an empty label")
	}
}

func TestSetTaskStatusTransitions(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)
	id, _ := s.AddOutlineEntry("ol-nb", "task", "", "ship E4", "")

	for _, status := range []string{"doing", "done", "open"} {
		if err := s.SetTaskStatus(id, status); err != nil {
			t.Fatalf("SetTaskStatus(%s) failed: %v", status, err)
		}
		_, entries, _ := s.GetOutline("ol-nb")
		if entries[len(entries)-1].Status != status {
			t.Errorf("expected status %q, got %q", status, entries[len(entries)-1].Status)
		}
	}
}

func TestSetTaskStatusRejectsInvalid(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)
	id, _ := s.AddOutlineEntry("ol-nb", "task", "", "x", "")

	if err := s.SetTaskStatus(id, "blocked"); err == nil {
		t.Error("expected error for an invalid status")
	}
}

func TestSetTaskStatusRejectsNonTask(t *testing.T) {
	s := setupTestServer(t)
	msgID := seedOutline(t, s)
	proseID, _ := s.AddOutlineEntry("ol-nb", "prose", "", "intro", "")
	refID, _ := s.AddOutlineEntry("ol-nb", "ref", msgID, "", "")

	if err := s.SetTaskStatus(proseID, "done"); err == nil {
		t.Error("expected error setting status on a prose entry")
	}
	if err := s.SetTaskStatus(refID, "done"); err == nil {
		t.Error("expected error setting status on a ref entry")
	}
	if err := s.SetTaskStatus("ghost", "done"); err == nil {
		t.Error("expected error for unknown entry")
	}
}

func TestUpdateTaskLabel(t *testing.T) {
	s := setupTestServer(t)
	seedOutline(t, s)
	id, _ := s.AddOutlineEntry("ol-nb", "task", "", "old label", "")
	if err := s.SetTaskStatus(id, "doing"); err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateOutlineEntry(id, "new label"); err != nil {
		t.Fatalf("UpdateOutlineEntry on a task failed: %v", err)
	}

	_, entries, _ := s.GetOutline("ol-nb")
	last := entries[len(entries)-1]
	if last.Prose != "new label" {
		t.Errorf("task label not updated: %q", last.Prose)
	}
	if last.Status != "doing" {
		t.Errorf("editing the label should not change status, got %q", last.Status)
	}
}
