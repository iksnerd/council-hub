package handlers

import (
	"strings"
	"testing"
	"time"
)

func noteCount(t *testing.T, reg *Registry, room string) int {
	t.Helper()
	msgs, err := reg.Server.GetTranscript(room)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, m := range msgs {
		if m.MessageType == "note" {
			n++
		}
	}
	return n
}

func leaseHandlerFixture(t *testing.T, room string) (*Registry, string, *time.Time) {
	t.Helper()
	reg, _ := setupOutlineHandler(t)
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	reg.Server.Clock = func() time.Time { return now }
	args := EditNotebookInput{NotebookID: "rel-notes", Kind: "task", Prose: "leased work"}
	if room != "" {
		args.Room = sp(room)
	}
	return reg, mustAddEntry(t, reg, args), &now
}

func TestClaimWritesOneNoteToTaskRoom(t *testing.T) {
	reg, id, _ := leaseHandlerFixture(t, "ol-room")
	before := noteCount(t, reg, "ol-room")
	out := editNotebook(t, reg, EditNotebookInput{Action: "claim", EntryID: id, Owner: sp("alice"), LeaseSeconds: "600"})
	if !strings.Contains(out, "claimed") || !strings.Contains(out, "alice") {
		t.Fatalf("claim reply: %s", out)
	}
	if got := noteCount(t, reg, "ol-room"); got != before+1 {
		t.Fatalf("claim must write exactly one note, notes %d -> %d", before, got)
	}
	msgs, _ := reg.Server.GetTranscript("ol-room")
	last := msgs[len(msgs)-1]
	if last.Author != "alice" || last.MessageType != "note" {
		t.Errorf("note author/type wrong: %+v", last)
	}

	// A refused claim writes nothing.
	out = editNotebook(t, reg, EditNotebookInput{Action: "claim", EntryID: id, Owner: sp("bob")})
	if !strings.Contains(out, "Error") || !strings.Contains(out, "alice") {
		t.Fatalf("second claim must be refused naming alice: %s", out)
	}
	if got := noteCount(t, reg, "ol-room"); got != before+1 {
		t.Errorf("refused claim wrote a note")
	}

	editNotebook(t, reg, EditNotebookInput{Action: "release", EntryID: id, Owner: sp("alice")})
	if got := noteCount(t, reg, "ol-room"); got != before+2 {
		t.Errorf("release must write a note, got %d", got)
	}
	editNotebook(t, reg, EditNotebookInput{Action: "claim", EntryID: id, Owner: sp("alice")})
	editNotebook(t, reg, EditNotebookInput{Action: "check", EntryID: id, Owner: sp("alice")})
	if got := noteCount(t, reg, "ol-room"); got != before+4 {
		t.Errorf("claim + done must each write a note, got %d", got)
	}
}

func TestClaimWithoutRoomWritesNoNote(t *testing.T) {
	reg, id, _ := leaseHandlerFixture(t, "")
	out := editNotebook(t, reg, EditNotebookInput{Action: "claim", EntryID: id, Owner: sp("alice")})
	if strings.Contains(out, "Error") {
		t.Fatalf("claim without a room must work: %s", out)
	}
	if got := noteCount(t, reg, "ol-room"); got != 0 {
		t.Errorf("no task_room, yet %d notes exist", got)
	}
}

func TestLeaseHandlerRefusals(t *testing.T) {
	reg, id, now := leaseHandlerFixture(t, "ol-room")
	editNotebook(t, reg, EditNotebookInput{Action: "claim", EntryID: id, Owner: sp("alice"), LeaseSeconds: "60"})

	for _, a := range []string{"renew", "release"} {
		out := editNotebook(t, reg, EditNotebookInput{Action: a, EntryID: id, Owner: sp("bob")})
		if !strings.Contains(out, "Error") || !strings.Contains(out, "alice") {
			t.Errorf("%s by non-owner must be refused naming alice: %s", a, out)
		}
	}
	// No client identity in a test call, so start/check are by an unknown caller.
	for _, a := range []string{"start", "check"} {
		out := editNotebook(t, reg, EditNotebookInput{Action: a, EntryID: id})
		if !strings.Contains(out, "Error") || !strings.Contains(out, "alice") {
			t.Errorf("%s by non-owner on a live lease must be refused naming alice: %s", a, out)
		}
	}
	if out := editNotebook(t, reg, EditNotebookInput{Action: "claim", EntryID: id, Owner: sp("alice"), LeaseSeconds: "999999"}); !strings.Contains(out, "Error") {
		t.Errorf("an oversize lease must be refused: %s", out)
	}

	*now = now.Add(2 * time.Minute)
	if out := editNotebook(t, reg, EditNotebookInput{Action: "claim", EntryID: id, Owner: sp("bob")}); strings.Contains(out, "Error") {
		t.Errorf("claim after expiry must work: %s", out)
	}
}

func TestUnleasedStartCheckUnchanged(t *testing.T) {
	reg, id, _ := leaseHandlerFixture(t, "ol-room")
	for _, a := range []string{"start", "check", "uncheck"} {
		if out := editNotebook(t, reg, EditNotebookInput{Action: a, EntryID: id}); strings.Contains(out, "Error") {
			t.Errorf("unleased %s must work: %s", a, out)
		}
	}
	if got := noteCount(t, reg, "ol-room"); got != 0 {
		t.Errorf("unleased task changes wrote %d notes", got)
	}
}

func TestStaleLeaseRendering(t *testing.T) {
	reg, id, now := leaseHandlerFixture(t, "")
	editNotebook(t, reg, EditNotebookInput{Action: "claim", EntryID: id, Owner: sp("alice"), LeaseSeconds: "60"})

	out := readOutline(t, reg, ReadNotebookInput{NotebookID: "rel-notes"})
	if !strings.Contains(out, "owner alice") || !strings.Contains(out, "lease until 2026-11-23T12:01:00Z") || strings.Contains(out, "STALE") {
		t.Fatalf("live lease render wrong:\n%s", out)
	}

	*now = now.Add(time.Hour)
	out = readOutline(t, reg, ReadNotebookInput{NotebookID: "rel-notes"})
	if !strings.Contains(out, "STALE") || !strings.Contains(out, "alice") {
		t.Fatalf("stale lease must render as stale naming the former owner:\n%s", out)
	}
	// Reading mutated nothing, and the owner filter still finds it.
	e, err := reg.Server.TaskLeaseInfo(id)
	if err != nil || e.Status != "doing" || e.Owner != "alice" {
		t.Errorf("read must not mutate: %+v %v", e, err)
	}
	ow := readOutline(t, reg, ReadNotebookInput{NotebookID: "rel-notes", Owner: "alice"})
	if !strings.Contains(ow, "leased work") {
		t.Errorf("owner filter lost the stale task:\n%s", ow)
	}
}
