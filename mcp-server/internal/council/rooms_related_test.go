package council

import (
	"reflect"
	"testing"
)

func relatedIDs(refs []RelatedRef) []string {
	var out []string
	for _, r := range refs {
		out = append(out, r.ID)
	}
	return out
}

func TestParseRelated_toleratesMalformedInput(t *testing.T) {
	want := []string{"alpha", "beta"}
	for name, in := range map[string]string{
		"csv":          "alpha,beta",
		"spaced csv":   "alpha, beta",
		"json array":   `["alpha","beta"]`,
		"escaped json": `[\"alpha\", \"beta\"]`,
		"single quote": `['alpha', 'beta']`,
		"newlines":     "alpha\nbeta",
		"duplicates":   "alpha, beta, alpha",
	} {
		if got := relatedIDs(ParseRelated(in)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestParseRelated_types(t *testing.T) {
	refs := ParseRelated(`["continues:a", "blocks: b", "c", "unknown:d"]`)
	want := []RelatedRef{{"a", "continues"}, {"b", "blocks"}, {"c", "relates"}, {"unknown:d", "relates"}}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("got %v, want %v", refs, want)
	}
}

func TestCreateRoom_storesCleanIDsForJSONArrayString(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "rag-spec")
	mustCreateRoom(t, s, "connector", withRelatedRooms(`["rag-spec","other"]`))

	room, _ := s.GetRoom("connector")
	if room.RelatedRooms != "rag-spec,other" {
		t.Fatalf("stored %q", room.RelatedRooms)
	}
	back, _ := s.GetRoom("rag-spec")
	if back.RelatedRooms != "connector" {
		t.Fatalf("reverse link: %q", back.RelatedRooms)
	}
}

func TestTypedLinks_reverseUsesInverseType(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "old")
	mustCreateRoom(t, s, "new", withRelatedRooms("continues:old"))

	newRoom, _ := s.GetRoom("new")
	if got := newRoom.RelatedDisplay(); got != "continues:old" {
		t.Fatalf("new: %q", got)
	}
	if newRoom.RelatedRooms != "old" {
		t.Fatalf("plain ids must stay readable by old consumers, got %q", newRoom.RelatedRooms)
	}
	oldRoom, _ := s.GetRoom("old")
	if got := oldRoom.RelatedDisplay(); got != "continued-by:new" {
		t.Fatalf("old: %q", got)
	}
}

func TestUpdateRoom_replaceDropsOldTypes(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "a")
	mustCreateRoom(t, s, "b")
	mustCreateRoom(t, s, "x", withRelatedRooms("blocks:a"))
	if err := s.UpdateRoom("x", "", "", "", "", "", "", "", "b"); err != nil {
		t.Fatal(err)
	}
	room, _ := s.GetRoom("x")
	if room.RelatedDisplay() != "b" || room.RelatedTypes != "" {
		t.Fatalf("got %q / %q", room.RelatedDisplay(), room.RelatedTypes)
	}
}

func TestMergeRelated_keepsTypesAndOverridesPlain(t *testing.T) {
	cur := Room{RelatedRooms: "a,b", RelatedTypes: "a=continues"}
	if got := MergeRelated(cur, "supersedes:b, c"); got != "continues:a, supersedes:b, c" {
		t.Fatalf("got %q", got)
	}
}

func TestMissingRooms(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "here")
	got := s.MissingRooms(`["here","ghost"]`)
	if !reflect.DeepEqual(got, []string{"ghost"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRepairRelatedRooms_normalisesStoredValues(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "broken")
	mustCreateRoom(t, s, "fine", withRelatedRooms("broken"))
	if _, err := s.DB.Exec(`UPDATE rooms SET related_rooms = ? WHERE id = 'broken'`, `["[\"x\"", "\"y\"]"]`); err != nil {
		t.Fatal(err)
	}
	n, err := repairRelatedRooms(s.DB, testLogger())
	if err != nil || n == 0 {
		t.Fatalf("repair: n=%d err=%v", n, err)
	}
	room, _ := s.GetRoom("broken")
	if room.RelatedRooms != "x,y" {
		t.Fatalf("got %q", room.RelatedRooms)
	}
	// Second pass is a no-op.
	if n, _ := repairRelatedRooms(s.DB, testLogger()); n != 0 {
		t.Fatalf("not idempotent: %d", n)
	}
}
