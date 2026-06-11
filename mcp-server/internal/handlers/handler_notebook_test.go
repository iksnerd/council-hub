package handlers

import (
	"context"
	"strings"
	"testing"
)

// seedNotebookRooms creates two rooms in project "nb-proj" (one with a repo)
// and posts typed messages across them. Returns the typed message IDs in order.
func seedNotebookRooms(t *testing.T, reg *Registry) []string {
	t.Helper()
	mustCreateRoom(t, reg.Server, "nb-a", withProject("nb-proj"))
	mustCreateRoom(t, reg.Server, "nb-b", withProject("nb-proj"))
	if err := reg.Server.SetRepo("nb-a", "alice/widgets"); err != nil {
		t.Fatalf("SetRepo failed: %v", err)
	}

	var ids []string
	ids = append(ids, mustPostTyped(t, reg.Server, "nb-a", "claude", "use SQLite {sha:abc1234}", "decision"))
	mustPost(t, reg.Server, "nb-a", "claude", "plain chatter")
	ids = append(ids, mustPostTyped(t, reg.Server, "nb-b", "gemini", "parser shipped", "action"))
	return ids
}

func TestHandleReadNotebook(t *testing.T) {
	reg := setupHandlerTest(t)
	ids := seedNotebookRooms(t, reg)

	res, _, err := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{Project: "nb-proj"})
	if err != nil {
		t.Fatalf("handleReadNotebook failed: %v", err)
	}
	text := resultText(res)

	if !strings.Contains(text, "# Notebook — nb-proj") {
		t.Error("missing notebook header")
	}
	// Both rooms woven into one timeline
	if !strings.Contains(text, "[nb-a]") || !strings.Contains(text, "[nb-b]") {
		t.Errorf("expected entries from both rooms, got:\n%s", text)
	}
	// Default types exclude plain messages
	if strings.Contains(text, "plain chatter") {
		t.Error("untyped message leaked into default notebook view")
	}
	// {sha:...} resolved against the owning room's repo
	if !strings.Contains(text, "https://github.com/alice/widgets/commit/abc1234") {
		t.Errorf("commit ref not resolved against room repo, got:\n%s", text)
	}
	// Cursor footer carries the latest typed message ID
	if !strings.Contains(text, `"latest_message_id":"`+ids[1]+`"`) {
		t.Error("missing or wrong latest_message_id in JSON footer")
	}
}

func TestHandleReadNotebookMissingProject(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{})
	if !strings.Contains(resultText(res), "project or notebook_id is required") {
		t.Error("expected project-required error")
	}
}

func TestHandleReadNotebookUnknownProject(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{Project: "no-such"})
	if !strings.Contains(resultText(res), "no rooms found for project 'no-such'") {
		t.Errorf("expected no-rooms error, got: %s", resultText(res))
	}
}

func TestHandleReadNotebookEmptyButRoomsExist(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "nb-empty", withProject("nb-proj"))
	mustPost(t, reg.Server, "nb-empty", "claude", "only plain messages here")

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{Project: "nb-proj"})
	text := resultText(res)
	if !strings.Contains(text, "No notebook entries") {
		t.Errorf("expected empty-notebook message, got: %s", text)
	}
}

func TestHandleReadNotebookTypesFilter(t *testing.T) {
	reg := setupHandlerTest(t)
	seedNotebookRooms(t, reg)

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{Project: "nb-proj", Types: "action"})
	text := resultText(res)
	if strings.Contains(text, "use SQLite") {
		t.Error("decision entry present despite types=action")
	}
	if !strings.Contains(text, "parser shipped") {
		t.Error("action entry missing with types=action")
	}
}

func TestHandleReadNotebookInvalidType(t *testing.T) {
	reg := setupHandlerTest(t)
	seedNotebookRooms(t, reg)

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{Project: "nb-proj", Types: "decision,bogus"})
	if !strings.Contains(resultText(res), "invalid message type 'bogus'") {
		t.Errorf("expected invalid-type error, got: %s", resultText(res))
	}
}

func TestHandleReadNotebookAfterID(t *testing.T) {
	reg := setupHandlerTest(t)
	ids := seedNotebookRooms(t, reg)

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{Project: "nb-proj", AfterID: ids[0]})
	text := resultText(res)
	if strings.Contains(text, "use SQLite") {
		t.Error("after_id delta read included the cursor message")
	}
	if !strings.Contains(text, "parser shipped") {
		t.Error("after_id delta read missing newer entry")
	}
}

func TestHandleReadNotebookPinnedMarker(t *testing.T) {
	reg := setupHandlerTest(t)
	ids := seedNotebookRooms(t, reg)

	if _, err := reg.Server.PinMessage("nb-a", ids[0]); err != nil {
		t.Fatalf("PinMessage failed: %v", err)
	}

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{Project: "nb-proj"})
	if !strings.Contains(resultText(res), "📌") {
		t.Error("pinned entry missing 📌 marker")
	}
}

func TestHandleReadNotebookClusterNotConfigured(t *testing.T) {
	reg := setupHandlerTest(t)
	seedNotebookRooms(t, reg)

	res, _, _ := reg.handleReadNotebook(context.Background(), nil, ReadNotebookInput{Project: "nb-proj", ClusterWide: "true"})
	if !strings.Contains(resultText(res), "cluster") {
		t.Errorf("expected cluster error when Phoenix is not configured, got: %s", resultText(res))
	}
}
