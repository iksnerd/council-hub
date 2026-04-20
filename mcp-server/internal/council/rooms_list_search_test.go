package council

import (
	"testing"
)

func TestListRooms(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-a", "Auth work", "project-alpha", "Go", "auth,api", "", "")
	s.CreateRoom("room-b", "Frontend", "project-beta", "React, TypeScript", "frontend", "", "")
	s.CreateRoom("room-c", "More auth", "project-alpha", "Go", "auth", "", "")

	// Filter by project
	rooms, err := s.ListRooms("project-alpha", "", "", "", 100, 0)
	if err != nil {
		t.Fatalf("listRooms failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms for project-alpha, got %d", len(rooms))
	}

	// Filter by tag
	rooms, _ = s.ListRooms("", "auth", "", "", 100, 0)
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms with tag 'auth', got %d", len(rooms))
	}

	// Filter by tag that only one room has
	rooms, _ = s.ListRooms("", "frontend", "", "", 100, 0)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room with tag 'frontend', got %d", len(rooms))
	}

	// No filter — all rooms
	rooms, _ = s.ListRooms("", "", "", "", 100, 0)
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms total, got %d", len(rooms))
	}

	// Filter by project + tag
	rooms, _ = s.ListRooms("project-alpha", "api", "", "", 100, 0)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room for project-alpha+api, got %d", len(rooms))
	}
}

func TestListRoomsByStatus(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("active-room", "Active", "", "", "", "", "")
	s.CreateRoom("paused-room", "Paused", "", "", "", "", "")
	s.UpdateStatus("paused-room", "paused")

	rooms, _ := s.ListRooms("", "", "paused", "", 100, 0)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 paused room, got %d", len(rooms))
	}
	if rooms[0].ID != "paused-room" {
		t.Errorf("expected 'paused-room', got '%s'", rooms[0].ID)
	}
}

func TestListRoomsByStatusFilter(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "ls-active")
	mustCreateRoom(t, s, "ls-resolved")
	s.UpdateStatus("ls-resolved", "resolved")

	rooms, _ := s.ListRooms("", "", "active", "", 100, 0)
	if len(rooms) != 1 || rooms[0].ID != "ls-active" {
		t.Errorf("expected only active room, got %d rooms", len(rooms))
	}

	rooms, _ = s.ListRooms("", "", "resolved", "", 100, 0)
	if len(rooms) != 1 || rooms[0].ID != "ls-resolved" {
		t.Errorf("expected only resolved room, got %d rooms", len(rooms))
	}
}

func TestListRoomsMultiWordSearch(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "council-hub-multi", withDescription("Multi agent collaboration platform"))

	// Both words match room ID ("council" and "hub" in "council-hub-multi")
	rooms, err := s.ListRooms("", "", "", "council hub", 100, 0)
	if err != nil {
		t.Fatalf("ListRooms multi-word search failed: %v", err)
	}
	if len(rooms) != 1 {
		t.Errorf("expected 1 room matching 'council hub', got %d", len(rooms))
	}

	// Both words match description ("agent" and "platform")
	rooms, _ = s.ListRooms("", "", "", "agent platform", 100, 0)
	if len(rooms) != 1 {
		t.Errorf("expected 1 room matching 'agent platform', got %d", len(rooms))
	}

	// AND logic: both words must match somewhere, "nonexistent" matches nothing
	rooms, _ = s.ListRooms("", "", "", "nonexistent xyz", 100, 0)
	if len(rooms) != 0 {
		t.Errorf("expected 0 rooms matching 'nonexistent xyz', got %d", len(rooms))
	}
}

// Replicates the field report that triggered the OR fallback: an agent
// searched for "council hub feedback suggestions" looking for a room that
// contained three of the four words. Strict AND returned nothing because
// "feedback" appears nowhere. The fallback should still surface the room.
func TestListRoomsSearchORFallback(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "council-hub-tool-suggestions",
		withDescription("Improving Council Hub Tooling for Agents"),
		withTags("mcp,tools,suggestions,dx"))
	mustCreateRoom(t, s, "other-room", withDescription("Unrelated topic"))

	// Strict AND would reject this (no room contains "feedback"), but the
	// fallback pass should match on "council"/"hub"/"suggestions".
	rooms, err := s.ListRooms("", "", "", "council hub feedback suggestions", 100, 0)
	if err != nil {
		t.Fatalf("ListRooms OR fallback failed: %v", err)
	}
	if len(rooms) != 1 || rooms[0].ID != "council-hub-tool-suggestions" {
		t.Errorf("expected OR fallback to return council-hub-tool-suggestions, got %+v", rooms)
	}

	// Single-word searches keep their original semantics — no fallback kicks
	// in, and a non-matching word returns nothing.
	rooms, _ = s.ListRooms("", "", "", "nothinghere", 100, 0)
	if len(rooms) != 0 {
		t.Errorf("expected 0 rooms for single non-matching word, got %d", len(rooms))
	}

	// AND still wins when it can — exact multi-word match must not get
	// diluted by the fallback running unnecessarily.
	rooms, _ = s.ListRooms("", "", "", "council hub", 100, 0)
	if len(rooms) != 1 {
		t.Errorf("expected AND to handle 'council hub' directly, got %d", len(rooms))
	}
}

func TestListRoomsFiltered_ProjectNotIn(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "alpha-1", withProject("project-alpha"))
	mustCreateRoom(t, s, "beta-1", withProject("project-beta"))
	mustCreateRoom(t, s, "legacy-1", withProject("legacy-x"))
	mustCreateRoom(t, s, "legacy-2", withProject("legacy-y"))

	rooms, err := s.ListRoomsFiltered(ListRoomsOptions{
		ProjectNotIn: []string{"project-alpha", "project-beta"},
		Limit:        100,
	})
	if err != nil {
		t.Fatalf("ListRoomsFiltered ProjectNotIn failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms outside the active list, got %d", len(rooms))
	}
	got := map[string]bool{}
	for _, r := range rooms {
		got[r.ID] = true
	}
	if !got["legacy-1"] || !got["legacy-2"] {
		t.Errorf("expected legacy-1 and legacy-2, got %+v", got)
	}
}

func TestListRoomsFiltered_RelatedTo(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "hub-room")
	mustCreateRoom(t, s, "neighbor-1", withRelatedRooms("hub-room"))
	mustCreateRoom(t, s, "neighbor-2", withRelatedRooms("hub-room,unrelated"))
	mustCreateRoom(t, s, "outsider")

	rooms, err := s.ListRoomsFiltered(ListRoomsOptions{
		RelatedTo: "hub-room",
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("ListRoomsFiltered RelatedTo failed: %v", err)
	}
	// Bidirectional links also wire hub-room.related_rooms back to neighbours,
	// so hub-room itself appears too.
	got := map[string]bool{}
	for _, r := range rooms {
		got[r.ID] = true
	}
	if !got["neighbor-1"] || !got["neighbor-2"] {
		t.Errorf("expected neighbor-1 and neighbor-2 to appear, got %+v", got)
	}
	if got["outsider"] {
		t.Errorf("outsider should not appear, got %+v", got)
	}
}

func TestRenameProject(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "r1", withProject("pi-go"))
	mustCreateRoom(t, s, "r2", withProject("pi-go"))
	mustCreateRoom(t, s, "r3", withProject("other"))

	count, err := s.RenameProject("pi-go", "go-pilot")
	if err != nil {
		t.Fatalf("RenameProject failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rooms renamed, got %d", count)
	}

	rooms, _ := s.ListRooms("go-pilot", "", "", "", 100, 0)
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms in renamed project, got %d", len(rooms))
	}
	rooms, _ = s.ListRooms("pi-go", "", "", "", 100, 0)
	if len(rooms) != 0 {
		t.Errorf("expected 0 rooms in old project, got %d", len(rooms))
	}

	// from == to is a no-op, not an error
	if c, err := s.RenameProject("go-pilot", "go-pilot"); err != nil || c != 0 {
		t.Errorf("expected (0,nil) for no-op rename, got (%d,%v)", c, err)
	}

	// missing from/to is an error
	if _, err := s.RenameProject("", "x"); err == nil {
		t.Error("expected error for empty from")
	}
	if _, err := s.RenameProject("x", ""); err == nil {
		t.Error("expected error for empty to")
	}
}

func TestListRoomsSearch(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "jwt-auth", withDescription("JWT authentication refactor"), withTags("security"))
	mustCreateRoom(t, s, "db-migration", withDescription("Database migration"))
	mustCreateRoom(t, s, "jwt-tokens", withDescription("Token validation"), withTags("jwt"))

	// Match by description keyword
	rooms, err := s.ListRooms("", "", "", "JWT", 100, 0)
	if err != nil {
		t.Fatalf("ListRooms search failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms matching 'JWT', got %d", len(rooms))
	}

	// Match by room ID
	rooms, _ = s.ListRooms("", "", "", "db-migration", 100, 0)
	if len(rooms) != 1 {
		t.Errorf("expected 1 room matching ID 'db-migration', got %d", len(rooms))
	}

	// Match by tag content
	rooms, _ = s.ListRooms("", "", "", "security", 100, 0)
	if len(rooms) != 1 {
		t.Errorf("expected 1 room matching tag 'security', got %d", len(rooms))
	}
}
