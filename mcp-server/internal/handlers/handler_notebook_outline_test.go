package handlers

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

// entryIDRe matches the full entry UUID in edit_notebook responses. Full IDs
// are deliberate: UUIDv7 short prefixes collide within one millisecond.
var entryIDRe = regexp.MustCompile(`Entry ([0-9a-f-]{36})`)

// editNotebook is a test shorthand for one edit_notebook call.
func editNotebook(t *testing.T, reg *Registry, args EditNotebookInput) string {
	t.Helper()
	res, _, err := reg.handleEditNotebook(context.Background(), nil, args)
	if err != nil {
		t.Fatalf("handleEditNotebook(%+v) failed: %v", args, err)
	}
	return resultText(res)
}

// mustAddEntry adds an entry and returns its full ID.
func mustAddEntry(t *testing.T, reg *Registry, args EditNotebookInput) string {
	t.Helper()
	args.Action = "add"
	text := editNotebook(t, reg, args)
	m := entryIDRe.FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("no entry ID in response: %s", text)
	}
	return m[1]
}

func setupOutlineHandler(t *testing.T) (*Registry, string) {
	t.Helper()
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "ol-room", withProject("ol-proj"))
	if err := reg.Server.SetRepo("ol-room", "alice/widgets"); err != nil {
		t.Fatalf("SetRepo failed: %v", err)
	}
	msgID := mustPostTyped(t, reg.Server, "ol-room", "claude", "key decision {sha:abc1234}", "decision")
	editNotebook(t, reg, EditNotebookInput{Action: "create", NotebookID: "rel-notes", Project: "ol-proj", Title: "Release Notes"})
	return reg, msgID
}

func TestHandleEditNotebookCreateAndDuplicate(t *testing.T) {
	reg, _ := setupOutlineHandler(t)

	text := editNotebook(t, reg, EditNotebookInput{Action: "create", NotebookID: "rel-notes", Project: "ol-proj"})
	if !strings.Contains(text, "already exists") {
		t.Errorf("expected duplicate error, got: %s", text)
	}
}

func TestHandleEditNotebookAddAndReadOutline(t *testing.T) {
	reg, msgID := setupOutlineHandler(t)

	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Prose: "## Shipped this week"})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", RefID: msgID})

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{NotebookID: "rel-notes"})
	text := resultText(res)

	if !strings.Contains(text, "# 📓 Release Notes") {
		t.Errorf("missing outline header, got: %s", text)
	}
	if !strings.Contains(text, "## Shipped this week") {
		t.Error("missing prose entry")
	}
	// Transcluded ref with room/author/type and resolved commit link
	if !strings.Contains(text, "[ol-room] claude (decision)") {
		t.Errorf("missing transcluded ref header, got: %s", text)
	}
	if !strings.Contains(text, "https://github.com/alice/widgets/commit/abc1234") {
		t.Error("commit ref not resolved in transcluded content")
	}
	// Full entry IDs shown for addressability (short prefixes collide
	// within one UUIDv7 millisecond)
	if !regexp.MustCompile(`\*\(entry [0-9a-f-]{36}\)\*`).MatchString(text) {
		t.Error("missing full entry ID markers")
	}
}

func TestHandleEditNotebookKindInference(t *testing.T) {
	reg, msgID := setupOutlineHandler(t)

	text := editNotebook(t, reg, EditNotebookInput{Action: "add", NotebookID: "rel-notes", RefID: msgID})
	if !strings.Contains(text, "kind: ref") {
		t.Errorf("ref_id should infer kind=ref, got: %s", text)
	}
	text = editNotebook(t, reg, EditNotebookInput{Action: "add", NotebookID: "rel-notes", Prose: "some prose"})
	if !strings.Contains(text, "kind: prose") {
		t.Errorf("prose should infer kind=prose, got: %s", text)
	}
}

func TestHandleEditNotebookUpdateMoveRemove(t *testing.T) {
	reg, _ := setupOutlineHandler(t)

	e1 := mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Prose: "first"})
	e2 := mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", Prose: "second"})

	// update
	editNotebook(t, reg, EditNotebookInput{Action: "update", EntryID: e1, Prose: "first (edited)"})
	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{NotebookID: "rel-notes"})
	if !strings.Contains(resultText(res), "first (edited)") {
		t.Error("prose update not visible in outline")
	}

	// move e2 to top
	editNotebook(t, reg, EditNotebookInput{Action: "move", EntryID: e2})
	_, entries, _ := reg.Server.GetOutline("rel-notes")
	if entries[0].ID != e2 {
		t.Error("move to top failed")
	}

	// remove e1
	editNotebook(t, reg, EditNotebookInput{Action: "remove", EntryID: e1})
	_, entries, _ = reg.Server.GetOutline("rel-notes")
	if len(entries) != 1 || entries[0].ID != e2 {
		t.Errorf("remove failed: %+v", entries)
	}
}

func TestHandleEditNotebookValidation(t *testing.T) {
	reg, _ := setupOutlineHandler(t)

	for _, tc := range []struct {
		args EditNotebookInput
		want string
	}{
		{EditNotebookInput{Action: "bogus"}, "unknown action"},
		{EditNotebookInput{Action: "create"}, "notebook_id is required"},
		{EditNotebookInput{Action: "add"}, "notebook_id is required"},
		{EditNotebookInput{Action: "add", NotebookID: "rel-notes", RefID: "ghost"}, "not found"},
		{EditNotebookInput{Action: "update"}, "entry_id is required"},
		{EditNotebookInput{Action: "remove", EntryID: "ghost"}, "not found"},
		{EditNotebookInput{Action: "delete", NotebookID: "ghost"}, "not found"},
	} {
		text := editNotebook(t, reg, tc.args)
		if !strings.Contains(text, tc.want) {
			t.Errorf("args %+v: expected %q in response, got: %s", tc.args, tc.want, text)
		}
	}
}

func TestHandleReadNotebookOutlineEmptyAndMissing(t *testing.T) {
	reg, _ := setupOutlineHandler(t)

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{NotebookID: "rel-notes"})
	if !strings.Contains(resultText(res), "Empty notebook") {
		t.Errorf("expected empty-notebook hint, got: %s", resultText(res))
	}

	res, _, _ = reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{NotebookID: "ghost"})
	if !strings.Contains(resultText(res), "not found") {
		t.Errorf("expected not-found error, got: %s", resultText(res))
	}
}

func TestHandleReadNotebookOutlineClusterRejected(t *testing.T) {
	reg, _ := setupOutlineHandler(t)

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{NotebookID: "rel-notes", ClusterWide: "true"})
	if !strings.Contains(resultText(res), "node-local") {
		t.Errorf("expected node-local error, got: %s", resultText(res))
	}
}

func TestHandleReadNotebookOutlineDanglingRef(t *testing.T) {
	reg, msgID := setupOutlineHandler(t)

	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", RefID: msgID})
	if _, err := reg.Server.DeleteMessages([]string{msgID}); err != nil {
		t.Fatalf("DeleteMessages failed: %v", err)
	}

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{NotebookID: "rel-notes"})
	if !strings.Contains(resultText(res), "not found") {
		t.Errorf("expected dangling-ref warning, got: %s", resultText(res))
	}
}

func TestHandleReadNotebookTimelineListsCuratedNotebooks(t *testing.T) {
	reg, msgID := setupOutlineHandler(t)
	_ = msgID

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{Project: "ol-proj"})
	text := resultText(res)
	if !strings.Contains(text, "Curated notebooks") || !strings.Contains(text, "rel-notes") {
		t.Errorf("timeline should list curated notebooks, got: %s", text)
	}
}

func TestHandleEditNotebookDelete(t *testing.T) {
	reg, msgID := setupOutlineHandler(t)

	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "rel-notes", RefID: msgID})
	text := editNotebook(t, reg, EditNotebookInput{Action: "delete", NotebookID: "rel-notes"})
	if !strings.Contains(text, "deleted") {
		t.Errorf("expected deletion confirmation, got: %s", text)
	}
	// Referenced message survives
	if _, err := reg.Server.GetMessageByID(msgID); err != nil {
		t.Errorf("referenced message should survive: %v", err)
	}
}
