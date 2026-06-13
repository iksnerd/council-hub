package council

import "testing"

func TestCreateAndGetLinks(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "link-room")
	a := mustPostTyped(t, s, "link-room", "Claude", "design A", "decision")
	b := mustPostTyped(t, s, "link-room", "Gemini", "design B refines A", "decision")

	id, err := s.CreateLink(b, a, "refines", "Gemini")
	if err != nil {
		t.Fatalf("CreateLink error: %v", err)
	}
	if id == "" {
		t.Fatal("expected a link id")
	}

	// From B: outgoing refines A.
	out, _, err := s.GetLinks(b)
	if err != nil {
		t.Fatalf("GetLinks(b) error: %v", err)
	}
	if len(out) != 1 || out[0].ToID != a || out[0].Relation != "refines" {
		t.Errorf("expected B --refines--> A, got %+v", out)
	}

	// From A: incoming backlink from B.
	_, in, err := s.GetLinks(a)
	if err != nil {
		t.Fatalf("GetLinks(a) error: %v", err)
	}
	if len(in) != 1 || in[0].FromID != b || in[0].Relation != "refines" {
		t.Errorf("expected backlink B refines A, got %+v", in)
	}
}

func TestCreateLinkValidation(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "lv-room")
	a := mustPost(t, s, "lv-room", "Claude", "a")
	b := mustPost(t, s, "lv-room", "Claude", "b")

	if _, err := s.CreateLink(a, b, "bogus", ""); err == nil {
		t.Error("expected error for invalid relation")
	}
	if _, err := s.CreateLink(a, a, "relates", ""); err == nil {
		t.Error("expected error linking a message to itself")
	}
	if _, err := s.CreateLink(a, "nonexistent-id", "relates", ""); err == nil {
		t.Error("expected error for missing target message")
	}
}

func TestCreateLinkIdempotent(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "li-room")
	a := mustPost(t, s, "li-room", "Claude", "a")
	b := mustPost(t, s, "li-room", "Claude", "b")

	id1, err := s.CreateLink(a, b, "relates", "")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := s.CreateLink(a, b, "relates", "")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("re-asserting the same link should return the existing id: %s vs %s", id1, id2)
	}

	out, _, _ := s.GetLinks(a)
	if len(out) != 1 {
		t.Errorf("expected exactly 1 link, got %d", len(out))
	}
}

func TestGetLinksIncludesImplicitEdges(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "imp-room")
	parent := mustPostTyped(t, s, "imp-room", "Claude", "v1", "synthesis")
	// A reply to parent, and a synthesis that supersedes parent.
	reply, err := s.PostMessage("imp-room", "Gemini", "re", "message", parent)
	if err != nil {
		t.Fatal(err)
	}
	superseder, err := s.PostMessageWithRefs("imp-room", "Claude", "v2", "synthesis", "", "", parent)
	if err != nil {
		t.Fatal(err)
	}

	_, in, err := s.GetLinks(parent)
	if err != nil {
		t.Fatal(err)
	}
	var sawReply, sawSupersedes bool
	for _, l := range in {
		if !l.Implicit {
			t.Errorf("reply/supersedes edges should be marked implicit: %+v", l)
		}
		if l.Relation == "reply" && l.FromID == reply {
			sawReply = true
		}
		if l.Relation == "supersedes" && l.FromID == superseder {
			sawSupersedes = true
		}
	}
	if !sawReply {
		t.Errorf("expected implicit reply backlink from %s, got %+v", reply, in)
	}
	if !sawSupersedes {
		t.Errorf("expected implicit supersedes backlink from %s, got %+v", superseder, in)
	}
}

func TestGetLinkNeighborhood(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "nbhd-room")
	// Chain: a <-refines- b <-refines- c, plus a reply d -> a.
	a := mustPostTyped(t, s, "nbhd-room", "Claude", "root decision", "decision")
	b := mustPostTyped(t, s, "nbhd-room", "Gemini", "refinement 1", "decision")
	c := mustPostTyped(t, s, "nbhd-room", "Claude", "refinement 2", "decision")
	d, _ := s.PostMessage("nbhd-room", "Amp", "a question", "message", a)
	s.CreateLink(b, a, "refines", "")
	s.CreateLink(c, b, "refines", "")

	// Depth 1 from a: reaches b (refines a) and d (reply to a).
	nodes, _, err := s.GetLinkNeighborhood(a, 1)
	if err != nil {
		t.Fatal(err)
	}
	dist := map[string]int{}
	for _, n := range nodes {
		dist[n.ID] = n.Distance
	}
	if dist[a] != 0 {
		t.Errorf("focus should be distance 0, got %d", dist[a])
	}
	if dist[b] != 1 || dist[d] != 1 {
		t.Errorf("b and d should be distance 1, got b=%d d=%d", dist[b], dist[d])
	}
	if _, ok := dist[c]; ok {
		t.Error("c is 2 hops away — should not appear at depth 1")
	}

	// Depth 2 reaches c through b.
	nodes2, _, _ := s.GetLinkNeighborhood(a, 2)
	dist2 := map[string]int{}
	for _, n := range nodes2 {
		dist2[n.ID] = n.Distance
	}
	if dist2[c] != 2 {
		t.Errorf("c should be distance 2 at depth 2, got %d", dist2[c])
	}
}

func TestDeleteLink(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "dl-room")
	a := mustPost(t, s, "dl-room", "Claude", "a")
	b := mustPost(t, s, "dl-room", "Claude", "b")
	id, _ := s.CreateLink(a, b, "relates", "")

	if err := s.DeleteLink(id); err != nil {
		t.Fatalf("DeleteLink error: %v", err)
	}
	out, _, _ := s.GetLinks(a)
	if len(out) != 0 {
		t.Errorf("expected no links after delete, got %d", len(out))
	}
	if err := s.DeleteLink("nonexistent"); err == nil {
		t.Error("expected error deleting a missing link")
	}
}

func TestDeleteMessagesCascadesLinks(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "casc-room")
	a := mustPost(t, s, "casc-room", "Claude", "a")
	b := mustPost(t, s, "casc-room", "Claude", "b")
	s.CreateLink(a, b, "relates", "")

	if _, err := s.DeleteMessages([]string{b}); err != nil {
		t.Fatalf("DeleteMessages error: %v", err)
	}

	// The link referencing the deleted message must be gone (no dangling edges).
	out, _, _ := s.GetLinks(a)
	if len(out) != 0 {
		t.Errorf("expected links to %s cleaned up after delete, got %+v", b, out)
	}
}
