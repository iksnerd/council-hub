package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestUINotebookEntryHandler(t *testing.T) {
	reg, msgID := setupOutlineHandler(t)
	handler := reg.UINotebookEntryHandler()

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/ui/notebook_entry", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:54321"
		w := httptest.NewRecorder()
		handler(w, req)
		return w
	}

	// Pin a message (ref kind inferred from ref_id)
	w := post(`{"notebook_id":"rel-notes","ref_id":"` + msgID + `"}`)
	var resp struct {
		EntryID string `json:"entry_id"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "" || resp.EntryID == "" {
		t.Fatalf("expected entry_id, got %+v", resp)
	}
	_, entries, _ := reg.Server.GetOutline("rel-notes")
	if len(entries) != 1 || entries[0].Kind != "ref" || entries[0].RefID != msgID {
		t.Errorf("pinned entry wrong: %+v", entries)
	}

	// Validation errors come back as JSON error, not HTTP error
	w = post(`{"ref_id":"x"}`)
	if !strings.Contains(w.Body.String(), "notebook_id is required") {
		t.Errorf("expected notebook_id error, got: %s", w.Body.String())
	}
	w = post(`{"notebook_id":"rel-notes","ref_id":"ghost"}`)
	if !strings.Contains(w.Body.String(), "not found") {
		t.Errorf("expected ref-not-found error, got: %s", w.Body.String())
	}

	// Non-loopback rejected
	req := httptest.NewRequest(http.MethodPost, "/api/ui/notebook_entry", strings.NewReader(`{}`))
	req.RemoteAddr = "10.0.0.9:54321"
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-loopback, got %d", rec.Code)
	}

	// GET rejected
	req = httptest.NewRequest(http.MethodGet, "/api/ui/notebook_entry", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rec.Code)
	}
}

func TestHandleReadNotebookOutlineStatusGrouping(t *testing.T) {
	reg, _ := setupOutlineHandler(t)

	// Two work-list rooms in one global notebook: one live, one resolved.
	mustCreateRoom(t, reg.Server, "wip-room", withProject("ol-proj"))
	mustCreateRoom(t, reg.Server, "shipped-room", withProject("ol-proj"))
	if err := reg.Server.UpdateStatus("shipped-room", "resolved"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	editNotebook(t, reg, EditNotebookInput{Action: "create", NotebookID: "current-work", Title: "Current Work"})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "current-work", Kind: "room_ref", RefID: "wip-room"})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "current-work", Kind: "room_ref", RefID: "shipped-room"})

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{NotebookID: "current-work"})
	text := resultText(res)

	inFlight := strings.Index(text, "🔄 In flight")
	doneAt := strings.Index(text, "✅ Done")
	if inFlight < 0 || doneAt < 0 {
		t.Fatalf("expected In flight + Done group headers, got: %s", text)
	}
	if inFlight > doneAt {
		t.Errorf("In flight group should render before Done, got: %s", text)
	}
	// The live room sorts under In flight, the resolved one under Done.
	if strings.Index(text, "wip-room") > doneAt {
		t.Errorf("active room should land in the In flight group, got: %s", text)
	}
	if shipped := strings.Index(text, "shipped-room"); shipped < doneAt {
		t.Errorf("resolved room should land in the Done group, got: %s", text)
	}
}

func TestHandleReadNotebookTaskGrouping(t *testing.T) {
	reg, _ := setupOutlineHandler(t)
	editNotebook(t, reg, EditNotebookInput{Action: "create", NotebookID: "current-work", Title: "Current Work"})

	doing := mustAddEntry(t, reg, EditNotebookInput{NotebookID: "current-work", Kind: "task", Prose: "actively shipping this"})
	mustAddEntry(t, reg, EditNotebookInput{NotebookID: "current-work", Kind: "task", Prose: "still in the backlog"})
	finished := mustAddEntry(t, reg, EditNotebookInput{NotebookID: "current-work", Kind: "task", Prose: "already finished"})

	editNotebook(t, reg, EditNotebookInput{Action: "start", EntryID: doing})
	editNotebook(t, reg, EditNotebookInput{Action: "check", EntryID: finished})

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{NotebookID: "current-work"})
	text := resultText(res)

	inProgress := strings.Index(text, "🔄 In progress")
	openAt := strings.Index(text, "☐ Open")
	doneAt := strings.Index(text, "☑ Done")
	if inProgress < 0 || openAt < 0 || doneAt < 0 {
		t.Fatalf("expected In progress + Open + Done task headers, got: %s", text)
	}
	if !(inProgress < openAt && openAt < doneAt) {
		t.Errorf("task groups should render In progress → Open → Done, got: %s", text)
	}
	// The started task renders with a [~] box, the checked one with [x].
	if !strings.Contains(text, "[~] actively shipping this") {
		t.Errorf("started task should render with [~], got: %s", text)
	}
	if !strings.Contains(text, "[x] already finished") {
		t.Errorf("checked task should render with [x], got: %s", text)
	}
	if !strings.Contains(text, "[ ] still in the backlog") {
		t.Errorf("open task should render with [ ], got: %s", text)
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
