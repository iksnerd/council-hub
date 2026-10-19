package handlers

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestEditNotebookDeleteRejectsEntryID(t *testing.T) {
	reg, _ := setupOutlineHandler(t)
	id := mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "keep me"})

	text := editNotebook(t, reg, EditNotebookInput{Action: "delete", NotebookID: "rel-notes", EntryID: id, Confirm: "rel-notes"})
	if !strings.Contains(text, "delete removes the whole notebook; did you mean remove?") {
		t.Fatalf("expected the did-you-mean error, got: %s", text)
	}
	if n, err := reg.Server.GetNotebook("rel-notes"); err != nil || n.EntryCount != 1 {
		t.Fatalf("notebook must be untouched: %+v %v", n, err)
	}
}

func TestEditNotebookDeleteNeedsConfirm(t *testing.T) {
	reg, _ := setupOutlineHandler(t)
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "a"})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "b"})

	text := editNotebook(t, reg, EditNotebookInput{Action: "delete", NotebookID: "rel-notes"})
	if !strings.Contains(text, "confirm=rel-notes") || !strings.Contains(text, "2 entries") {
		t.Fatalf("expected confirm requirement naming id and count, got: %s", text)
	}
	text = editNotebook(t, reg, EditNotebookInput{Action: "delete", NotebookID: "rel-notes", Confirm: "wrong"})
	if !strings.HasPrefix(text, "Error") {
		t.Fatalf("wrong confirm must be refused, got: %s", text)
	}
	if _, err := reg.Server.GetNotebook("rel-notes"); err != nil {
		t.Fatal("notebook must survive a refused delete")
	}

	// entry count is an accepted confirm value
	text = editNotebook(t, reg, EditNotebookInput{Action: "delete", NotebookID: "rel-notes", Confirm: "2"})
	if !strings.Contains(text, "2 entries removed") {
		t.Fatalf("result must state the count removed, got: %s", text)
	}
}

func TestEditNotebookSoftDeleteRestorePurgeAndSnapshot(t *testing.T) {
	reg := setupHandlerTestWithTempDB(t)
	editNotebook(t, reg, EditNotebookInput{Action: "create", NotebookID: "nb"})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "nb", Kind: "task", Prose: "precious task"})

	text := editNotebook(t, reg, EditNotebookInput{Action: "delete", NotebookID: "nb", Confirm: "nb"})
	m := regexp.MustCompile(`Markdown snapshot: (\S+\.md)`).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("expected a snapshot path, got: %s", text)
	}
	body, err := os.ReadFile(m[1])
	if err != nil || !strings.Contains(string(body), "precious task") {
		t.Fatalf("snapshot should hold the notebook markdown: %v %q", err, body)
	}
	if filepath.Base(filepath.Dir(m[1])) != "notebooks" {
		t.Errorf("snapshot should live under archives/notebooks, got %s", m[1])
	}

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{NotebookID: "nb"})
	if !strings.Contains(resultText(res), "restore") {
		t.Errorf("read of a deleted notebook should point at restore, got: %s", resultText(res))
	}

	if text = editNotebook(t, reg, EditNotebookInput{Action: "purge", NotebookID: "nb"}); !strings.Contains(text, "confirm") {
		t.Errorf("purge must demand confirm, got: %s", text)
	}
	if text = editNotebook(t, reg, EditNotebookInput{Action: "restore", NotebookID: "nb"}); !strings.Contains(text, "restored with 1 entries") {
		t.Fatalf("restore failed: %s", text)
	}
	if text = editNotebook(t, reg, EditNotebookInput{Action: "purge", NotebookID: "nb", Confirm: "nb"}); !strings.HasPrefix(text, "Error") {
		t.Errorf("purge of a live notebook must be refused, got: %s", text)
	}
	editNotebook(t, reg, EditNotebookInput{Action: "delete", NotebookID: "nb", Confirm: "nb"})
	text = editNotebook(t, reg, EditNotebookInput{Action: "purge", NotebookID: "nb", Confirm: "nb"})
	if !strings.Contains(text, "PURGED: 1 entries") {
		t.Errorf("purge result: %s", text)
	}
}

func TestEditNotebookWritesReportEntryCounts(t *testing.T) {
	reg, _ := setupOutlineHandler(t)

	text := editNotebook(t, reg, EditNotebookInput{Action: "add", NotebookID: "rel-notes", Kind: "task", Prose: "one"})
	if !strings.Contains(text, "Entries: 0 → 1.") {
		t.Errorf("add should report before/after, got: %s", text)
	}
	id := entryIDRe.FindStringSubmatch(text)[1]
	if text = editNotebook(t, reg, EditNotebookInput{Action: "check", EntryID: id}); !strings.Contains(text, "Entries: 1 → 1.") {
		t.Errorf("check should report counts, got: %s", text)
	}
	if text = editNotebook(t, reg, EditNotebookInput{Action: "remove", EntryID: id}); !strings.Contains(text, "Entries: 1 → 0.") {
		t.Errorf("remove should report counts, got: %s", text)
	}
}

func TestEditNotebookTaskAddNudges(t *testing.T) {
	reg, _ := setupOutlineHandler(t)

	text := editNotebook(t, reg, EditNotebookInput{Action: "add", NotebookID: "rel-notes", Kind: "task", Prose: "plain task"})
	id := entryIDRe.FindStringSubmatch(text)[1]
	if !strings.Contains(text, "Close later with action=check, entry_id="+id) {
		t.Errorf("task add should say how to close, got: %s", text)
	}
	if strings.Contains(text, "add does not close") {
		t.Errorf("no completion nudge for a normal task: %s", text)
	}

	for _, prose := range []string{"DONE: shipped", "done - x", "✅ shipped", "[x] shipped", "COMPLETE the thing", "Completed: y"} {
		text = editNotebook(t, reg, EditNotebookInput{Action: "add", NotebookID: "rel-notes", Kind: "task", Prose: prose})
		if !strings.Contains(text, "add does not close anything") {
			t.Errorf("%q should draw the completion nudge, got: %s", prose, text)
		}
	}
	text = editNotebook(t, reg, EditNotebookInput{Action: "add", NotebookID: "rel-notes", Kind: "task", Prose: "donate to charity"})
	if strings.Contains(text, "add does not close") {
		t.Errorf("'donate' is not a completion marker: %s", text)
	}
}

func TestReadNotebookProjectLimitAndEntryID(t *testing.T) {
	reg, _ := setupOutlineHandler(t)
	long := "alpha: " + strings.Repeat("full text sentence. ", 20)
	id := mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: long})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "beta: other"})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "alpha: second"})

	read := func(in ReadNotebookInput) string {
		res, _, _ := reg.handleReadNotebook(context.Background(), nil, in)
		return resultText(res)
	}

	text := read(ReadNotebookInput{NotebookID: "rel-notes", Project: "alpha"})
	if strings.Contains(text, "beta: other") || !strings.Contains(text, "alpha: second") || !strings.Contains(text, "**Showing:** 2") {
		t.Errorf("project filter wrong: %s", text)
	}
	text = read(ReadNotebookInput{NotebookID: "rel-notes", Limit: "1"})
	if strings.Contains(text, "beta: other") || !strings.Contains(text, "alpha: second") {
		t.Errorf("limit should keep the last entry: %s", text)
	}
	// single-entry read: no notebook_id needed, never clipped even at level=1
	text = read(ReadNotebookInput{EntryID: id, Level: "1"})
	if !strings.Contains(text, strings.TrimSpace(long)) || !strings.Contains(text, "**Showing:** 1") {
		t.Errorf("entry_id read should return the full task text: %s", text)
	}
}
