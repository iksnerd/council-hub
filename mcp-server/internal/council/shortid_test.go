package council

import (
	"strings"
	"testing"
)

func TestNewShortIDsMinimumAndExtension(t *testing.T) {
	a := "01a14ecc-0000-7000-8000-000000000001"
	b := "01a14ecc-0000-7000-8000-000000000002"
	c := "01a15000-0000-7000-8000-000000000003"
	ids := NewShortIDs([]string{a, b, c})

	if got := ids.Of(c); got != "01a15000" {
		t.Errorf("unique id should keep 8 chars, got %q", got)
	}
	pa, pb := ids.Of(a), ids.Of(b)
	if pa == pb || len(pa) <= 8 || len(pb) <= 8 {
		t.Errorf("colliding ids must extend past 8 chars and differ: %q %q", pa, pb)
	}
	if !strings.HasPrefix(a, pa) || !strings.HasPrefix(b, pb) {
		t.Errorf("shortened ids must be prefixes: %q %q", pa, pb)
	}
	var nilIDs *ShortIDs
	if got := nilIDs.Of(a); got != "01a14ecc" {
		t.Errorf("nil ShortIDs should fall back to 8 chars, got %q", got)
	}
}

// Every printed prefix must resolve back to the message it was printed for.
func TestShortIDsRoundTripThroughResolver(t *testing.T) {
	s := setupTestServer(t)
	if err := s.CreateRoom("room-a", "t", "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	all := []string{
		"01a14ecc-0000-7000-8000-000000000001",
		"01a14ecc-0000-7000-8000-000000000002",
		"01a14ecc-0000-7000-8100-000000000003",
		"01a14ecd-0000-7000-8000-000000000004",
	}
	for _, id := range all {
		insertRawMessage(t, s, id, "room-a", "x")
	}
	ids, err := s.RoomShortIDs("room-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range all {
		p := ids.Of(id)
		got, err := s.ResolveMessageIDInRoom(p, "room-a")
		if err != nil || got != id {
			t.Errorf("prefix %q resolved to %q, %v; want %q", p, got, err, id)
		}
	}
}

func TestTranscriptUsesUniqueShortIDs(t *testing.T) {
	room := Room{ID: "r", Status: "active"}
	a := Message{ID: "01a14ecc-0000-7000-8000-000000000001", RoomID: "r", Author: "al", Content: "one", MessageType: "note"}
	b := Message{ID: "01a14ecc-0000-7000-8000-000000000002", RoomID: "r", Author: "bo", Content: "two", MessageType: "synthesis", ReplyTo: a.ID}
	out := FormatTranscript(room, []Message{a, b})
	ids := NewShortIDs([]string{a.ID, b.ID})
	for _, id := range []string{a.ID, b.ID} {
		if !strings.Contains(out, "#"+ids.Of(id)) {
			t.Errorf("transcript missing #%s:\n%s", ids.Of(id), out)
		}
	}
	if strings.Contains(out, "[#01a14ecc ") {
		t.Errorf("transcript still prints the colliding 8-char prefix:\n%s", out)
	}
	if !strings.Contains(out, "re: #"+ids.Of(a.ID)) {
		t.Errorf("reply tag should use the unique prefix:\n%s", out)
	}
}
