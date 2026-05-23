package council

import (
	"strings"
	"testing"
)

func TestFindSimilarRoomsByTag(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "existing-auth", withProject("myapp"), withTags("go,auth,api"))
	mustCreateRoom(t, s, "existing-db", withProject("myapp"), withTags("go,postgres,api"))

	similar, err := s.FindSimilarRooms("new-room", "Auth service implementation", "myapp", "go,auth", 5)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	if len(similar) == 0 {
		t.Fatal("expected at least one similar room")
	}
	if similar[0].ID != "existing-auth" {
		t.Errorf("expected existing-auth as top match, got %s", similar[0].ID)
	}
}

func TestFindSimilarRoomsByDescription(t *testing.T) {
	s := setupTestServer(t)
	// Need score >= 3: use 3 overlapping keywords to reach threshold
	mustCreateRoom(t, s, "auth-service", withDescription("Authentication middleware design patterns"))

	similar, err := s.FindSimilarRooms("new-room", "Authentication middleware design overview", "", "", 5)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	if len(similar) == 0 {
		t.Fatal("expected a match on description keywords (authentication, middleware, design)")
	}
	if similar[0].ID != "auth-service" {
		t.Errorf("expected auth-service as match, got %s", similar[0].ID)
	}
}

func TestFindSimilarRoomsExcludesSelf(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "self-room", withTags("go,auth"))

	similar, err := s.FindSimilarRooms("self-room", "Auth", "", "go,auth", 5)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	for _, r := range similar {
		if r.ID == "self-room" {
			t.Error("FindSimilarRooms should not return the excluded room")
		}
	}
}

func TestFindSimilarRoomsNoSignal(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "some-room", withTags("go"))

	similar, err := s.FindSimilarRooms("new-room", "the a to", "", "", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(similar) != 0 {
		t.Errorf("expected no results with no signal, got %d", len(similar))
	}
}

func TestFindSimilarRoomsLimit(t *testing.T) {
	s := setupTestServer(t)
	for i := 0; i < 5; i++ {
		mustCreateRoom(t, s, strings.Repeat("r", i+1)+"-room", withTags("go,auth,api"))
	}

	similar, err := s.FindSimilarRooms("new", "Auth", "", "go,auth,api", 2)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	if len(similar) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(similar))
	}
}

func TestGetConceptMap(t *testing.T) {
	s := setupTestServer(t)

	// Create a graph:
	// root -> a, b
	// a -> c
	// c -> root (cycle)
	// b -> d
	// e (unconnected)
	// NOTE: CreateRoom triggers syncReverseLinks, so root <-> a, root <-> b, a <-> c, etc.
	s.CreateRoom("root", "Root topic", "proj", "", "tag1", "", "a, b")
	s.CreateRoom("a", "Topic A", "proj", "", "tag2", "", "c")
	s.CreateRoom("b", "Topic B", "proj", "", "tag3", "", "d")
	s.CreateRoom("c", "Topic C", "proj", "", "tag4", "", "root")
	s.CreateRoom("d", "Topic D", "proj", "", "tag5", "", "")
	s.CreateRoom("e", "Topic E", "proj", "", "tag6", "", "")

	// Test 1: Depth 1
	nodes, err := s.GetConceptMap("root", 1, "")
	if err != nil {
		t.Fatalf("GetConceptMap depth 1 failed: %v", err)
	}
	// Expected: root (0), a (1), b (1), c (1)
	// 'c' is depth 1 because s.CreateRoom("c", ..., "root") created a link root -> c.
	if len(nodes) != 4 {
		t.Errorf("expected 4 nodes at depth 1, got %d", len(nodes))
	}

	// Test 2: Full traversal (Depth 3)
	nodes, err = s.GetConceptMap("root", 3, "")
	if err != nil {
		t.Fatalf("GetConceptMap depth 3 failed: %v", err)
	}
	// Expected: root (0), a (1), b (1), c (1), d (2)
	// 'e' should be missing.
	if len(nodes) != 5 {
		t.Errorf("expected 5 nodes at depth 3, got %d", len(nodes))
	}

	depthMap := make(map[string]int)
	viaMap := make(map[string]string)
	for _, n := range nodes {
		depthMap[n.Room.ID] = n.Depth
		viaMap[n.Room.ID] = n.Via
	}

	if depthMap["root"] != 0 {
		t.Errorf("expected root depth 0, got %d", depthMap["root"])
	}
	if depthMap["a"] != 1 || viaMap["a"] != "root" {
		t.Errorf("expected a depth 1 via root, got depth %d via %s", depthMap["a"], viaMap["a"])
	}
	if depthMap["c"] != 1 || viaMap["c"] != "root" {
		t.Errorf("expected c depth 1 via root, got depth %d via %s", depthMap["c"], viaMap["c"])
	}
	if depthMap["d"] != 2 || viaMap["d"] != "b" {
		t.Errorf("expected d depth 2 via b, got depth %d via %s", depthMap["d"], viaMap["d"])
	}

	// Test 3: Unconnected
	nodes, _ = s.GetConceptMap("e", 3, "")
	if len(nodes) != 1 || nodes[0].Room.ID != "e" {
		t.Errorf("expected only 'e' for unconnected start, got %d nodes", len(nodes))
	}

	// Test 4: Max depth enforcement
	nodes, _ = s.GetConceptMap("root", 10, "") // should be capped to 5
	// Our graph only goes to depth 2, so this should still return 5 nodes
	if len(nodes) != 5 {
		t.Errorf("expected 5 nodes for deep search on shallow graph, got %d", len(nodes))
	}
}

func TestGetConceptMapInferFrom(t *testing.T) {
	s := setupTestServer(t)

	// Two rooms in the same project, not explicitly linked.
	s.CreateRoom("alpha", "Alpha topic", "myproject", "", "go,api", "", "")
	s.CreateRoom("beta", "Beta topic", "myproject", "", "go,grpc", "", "")
	// Third room in a different project with a shared tag.
	s.CreateRoom("gamma", "Gamma topic", "otherproject", "", "grpc,proto", "", "")
	// Room with no overlap.
	s.CreateRoom("delta", "Delta topic", "isolated", "", "rust", "", "")

	// infer_from=project: alpha should discover beta via shared project.
	nodes, err := s.GetConceptMap("alpha", 1, "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := make(map[string]string)
	for _, n := range nodes {
		ids[n.Room.ID] = n.Inferred
	}
	if _, ok := ids["beta"]; !ok {
		t.Errorf("expected beta to be discovered via project inference, got nodes: %v", ids)
	}
	if ids["beta"] != "project" {
		t.Errorf("expected Inferred='project' for beta, got %q", ids["beta"])
	}
	if _, ok := ids["gamma"]; ok {
		t.Errorf("gamma should not appear (different project), but it did")
	}

	// infer_from=tags depth=1: alpha (go,api) shares "go" with beta (go,grpc) → beta discovered.
	// gamma (grpc,proto) shares nothing with alpha directly — it's only reachable at depth=2.
	nodes, err = s.GetConceptMap("alpha", 1, "tags")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids = make(map[string]string)
	for _, n := range nodes {
		ids[n.Room.ID] = n.Inferred
	}
	if _, ok := ids["beta"]; !ok {
		t.Errorf("expected beta to be discovered via tag inference (shared: go)")
	}
	if _, ok := ids["delta"]; ok {
		t.Errorf("delta should not appear (no shared tags)")
	}

	// infer_from=tags depth=2: alpha → beta (go) → gamma (grpc).
	nodes, err = s.GetConceptMap("alpha", 2, "tags")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids = make(map[string]string)
	for _, n := range nodes {
		ids[n.Room.ID] = n.Inferred
	}
	if _, ok := ids["gamma"]; !ok {
		t.Errorf("expected gamma at depth=2 via beta (shared tag: grpc)")
	}

	// infer_from=project,tags depth=1: beta discovered via both project and tags.
	nodes, err = s.GetConceptMap("alpha", 1, "project,tags")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids = make(map[string]string)
	for _, n := range nodes {
		ids[n.Room.ID] = n.Inferred
	}
	if _, ok := ids["beta"]; !ok {
		t.Errorf("expected beta with project,tags inference")
	}
}
