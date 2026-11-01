package handlers

import (
	"context"
	"strings"
	"testing"
)

func sp(s string) *string { return &s }

func readOutline(t *testing.T, reg *Registry, args ReadNotebookInput) string {
	t.Helper()
	res, _, err := reg.handleReadNotebook(context.Background(), nil, args)
	if err != nil {
		t.Fatal(err)
	}
	return resultText(res)
}

func TestTaskFieldsRenderAndOverdue(t *testing.T) {
	reg, _ := setupOutlineHandler(t)
	late := mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "monthly sweep",
		Due: sp("2020-01-01"), Owner: sp("OWNER"), Repo: sp("a/b"), Room: sp("ol-room")})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "far future", Due: sp("2999-01-01"), Owner: sp("bot")})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "no fields"})

	out := readOutline(t, reg, ReadNotebookInput{NotebookID: "rel-notes"})
	if !strings.Contains(out, "monthly sweep [due 2020-01-01 OVERDUE · owner OWNER · repo a/b · room ol-room]") {
		t.Fatalf("overdue render missing:\n%s", out)
	}
	if !strings.Contains(out, "far future [due 2999-01-01 · owner bot]") {
		t.Fatalf("future render wrong:\n%s", out)
	}

	od := readOutline(t, reg, ReadNotebookInput{NotebookID: "rel-notes", Overdue: "true"})
	if !strings.Contains(od, "monthly sweep") || strings.Contains(od, "far future") || strings.Contains(od, "no fields") {
		t.Fatalf("overdue filter wrong:\n%s", od)
	}
	ow := readOutline(t, reg, ReadNotebookInput{NotebookID: "rel-notes", Owner: "BOT"})
	if !strings.Contains(ow, "far future") || strings.Contains(ow, "monthly sweep") {
		t.Fatalf("owner filter wrong:\n%s", ow)
	}

	// Done tasks are never overdue; a fields-only update keeps the label.
	editNotebook(t, reg, EditNotebookInput{Action: "check", EntryID: late})
	if out := readOutline(t, reg, ReadNotebookInput{NotebookID: "rel-notes", Overdue: "true"}); strings.Contains(out, "monthly sweep") {
		t.Fatalf("done task listed as overdue:\n%s", out)
	}
	editNotebook(t, reg, EditNotebookInput{Action: "update", EntryID: late, Due: sp("2999-02-02"), Owner: sp("")})
	out = readOutline(t, reg, ReadNotebookInput{NotebookID: "rel-notes"})
	if !strings.Contains(out, "monthly sweep [due 2999-02-02 · repo a/b · room ol-room]") {
		t.Fatalf("update fields wrong:\n%s", out)
	}
}

func TestTaskFieldsValidation(t *testing.T) {
	reg, _ := setupOutlineHandler(t)
	if got := editNotebook(t, reg, EditNotebookInput{Action: "add", NotebookID: "rel-notes", Kind: "task", Prose: "x", Due: sp("11/01/2026")}); !strings.Contains(got, "YYYY-MM-DD") {
		t.Fatalf("bad due accepted: %s", got)
	}
	if got := editNotebook(t, reg, EditNotebookInput{Action: "add", NotebookID: "rel-notes", Kind: "prose", Prose: "x", Owner: sp("me")}); !strings.Contains(got, "Error") {
		t.Fatalf("owner on prose accepted: %s", got)
	}
}

func TestSignalStatusListsOpenTasksForRoom(t *testing.T) {
	reg, _ := setupOutlineHandler(t)
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "by field", Room: sp("ol-room")})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "by text, see ol-room for context"})
	done := mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "finished ol-room thing"})
	editNotebook(t, reg, EditNotebookInput{Action: "check", EntryID: done})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "unrelated"})

	res, _, _ := reg.handleSignalStatus(context.Background(), nil, SignalStatusInput{RoomID: "ol-room", Status: "resolved"})
	out := resultText(res)
	if !strings.Contains(out, "2 open notebook task(s)") || !strings.Contains(out, "by field") || !strings.Contains(out, "by text") ||
		strings.Contains(out, "finished") || strings.Contains(out, "unrelated") || !strings.Contains(out, "Nothing was closed") {
		t.Fatalf("note wrong:\n%s", out)
	}
}
