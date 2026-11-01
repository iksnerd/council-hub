package handlers

import (
	"context"
	"strings"
	"testing"
)

func clusterListText(t *testing.T, reg *Registry, in ListRoomsInput) (string, ListRoomsOutput) {
	t.Helper()
	res, out, err := reg.handleListRooms(context.Background(), nil, in)
	if err != nil {
		t.Fatal(err)
	}
	return resultText(res), out
}

func TestClusterListRoomsCarriesArchiveActivityAndTypedLinks(t *testing.T) {
	newRoom := peerRoom("peer-new", "resolved", "2026-10-01T10:00:00")
	newRoom["archived_at"] = "2026-10-30T12:34:56"
	newRoom["last_activity"] = "2026-10-31T01:02:03"
	newRoom["related_rooms"] = "old,other"
	newRoom["related_types"] = "old=continues"
	// A v0.63.x peer sends none of the three keys.
	oldRoom := peerRoom("peer-old", "resolved", "2026-10-02T10:00:00")
	oldRoom["related_rooms"] = "old"

	reg, _ := parityPeer(t, map[string]any{"list_rooms": []map[string]any{newRoom, oldRoom}})

	text, out := clusterListText(t, reg, ListRoomsInput{Status: "resolved"})
	var newLine, oldLine string
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, "**peer-new**") {
			newLine = l
		}
		if strings.Contains(l, "**peer-old**") {
			oldLine = l
		}
	}
	if !strings.Contains(newLine, "resolved (archived 2026-10-30T12:34Z)") || !strings.Contains(newLine, "last msg 2026-10-31T01:02Z") {
		t.Errorf("compact line lacks archive/activity: %q", newLine)
	}
	if strings.Contains(oldLine, "archived") || strings.Contains(oldLine, "last msg") {
		t.Errorf("old peer's room must not claim archive state or activity: %q", oldLine)
	}
	for _, lr := range out.Rooms {
		switch lr.ID {
		case "peer-new":
			if lr.ArchivedAt != "2026-10-30T12:34:56Z" || lr.LastActivity != "2026-10-31T01:02:03Z" {
				t.Errorf("structured fields: %+v", lr)
			}
		case "peer-old":
			if lr.ArchivedAt != "" || lr.LastActivity != "" {
				t.Errorf("old peer fields must stay empty: %+v", lr)
			}
		}
	}

	verbose, _ := clusterListText(t, reg, ListRoomsInput{Status: "resolved", Verbose: "true"})
	for _, want := range []string{"Related: continues:old, other", "Last message: 2026-10-31T01:02:03Z", "Archived: 2026-10-30T12:34:56Z", "Related: old\n"} {
		if !strings.Contains(verbose, want) {
			t.Errorf("verbose output missing %q:\n%s", want, verbose)
		}
	}
}

func TestClusterListRoomsForwardsArchivedAndFiltersLocal(t *testing.T) {
	reg, bodies := parityPeer(t, map[string]any{"list_rooms": []map[string]any{}})
	mustCreateRoom(t, reg.Server, "local-live")
	mustCreateRoom(t, reg.Server, "local-archived")
	// Set the column directly: ArchiveRoom writes a file into the working directory.
	if _, err := reg.Server.DB.Exec(`UPDATE rooms SET archived_at = '2026-10-30 12:34:56' WHERE id = 'local-archived'`); err != nil {
		t.Fatal(err)
	}

	text, _ := clusterListText(t, reg, ListRoomsInput{Archived: "true", Status: "all"})
	if got := bodies["list_rooms"]["archived"]; got != "true" {
		t.Errorf("archived not forwarded to peers: %v", got)
	}
	if !strings.Contains(text, "local-archived") || strings.Contains(text, "local-live") {
		t.Errorf("local archived filter: %s", text)
	}
	if !strings.Contains(text, "(archived ") {
		t.Errorf("local archived room should show its archive time: %s", text)
	}

	bad, _ := clusterListText(t, reg, ListRoomsInput{Archived: "maybe"})
	if !strings.Contains(bad, "archived must be") {
		t.Errorf("invalid archived: %s", bad)
	}
}
