package council

import (
	"strings"
	"testing"
)

func TestNormalizeProject(t *testing.T) {
	cases := []struct{ in, want string }{
		{"council-hub", "council-hub"},
		{"Council-Hub", "council-hub"},
		{"COUNCIL_HUB", "council-hub"},
		{"My Project", "my-project"},
		{"  spaces  ", "spaces"},
		{"a--b", "a-b"},
		{"special!@#chars", "specialchars"},
		{"", ""},
		{"---", ""},
		{"foo_bar_baz", "foo-bar-baz"},
	}
	for _, tc := range cases {
		got := normalizeProject(tc.in)
		if got != tc.want {
			t.Errorf("normalizeProject(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCreateRoomNormalizesProject(t *testing.T) {
	s := setupTestServer(t)
	if err := s.CreateRoom("norm-room", "desc", "Council-Hub", "", "", "", ""); err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}
	room, _ := s.GetRoom("norm-room")
	if room.Project != "council-hub" {
		t.Errorf("expected project 'council-hub', got '%s'", room.Project)
	}
}

func TestUpdateRoomNormalizesProject(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("upd-norm-room", "desc", "old-project", "", "", "", "")
	if err := s.UpdateRoom("upd-norm-room", "", "NEW_PROJECT", "", "", "", "", "", ""); err != nil {
		t.Fatalf("UpdateRoom failed: %v", err)
	}
	room, _ := s.GetRoom("upd-norm-room")
	if room.Project != "new-project" {
		t.Errorf("expected project 'new-project', got '%s'", room.Project)
	}
}

func TestListRoomsNormalizesProjectFilter(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("filter-room", "desc", "My Project", "", "", "", "")

	// Verify stored value is normalized
	room, _ := s.GetRoom("filter-room")
	if room.Project != "my-project" {
		t.Fatalf("expected stored project 'my-project', got '%s'", room.Project)
	}

	// Filter with different casing/format — should still find it
	rooms, err := s.ListRooms("MY_PROJECT", "", "", "", 100, 0)
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}
	if len(rooms) != 1 {
		t.Errorf("expected 1 room, got %d", len(rooms))
	}
}

func TestNormalizeTags(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`["mtls","gateway"]`, "mtls,gateway"},
		{`["single"]`, "single"},
		{`mtls,gateway`, "mtls,gateway"},
		{` mtls , gateway `, "mtls,gateway"},
		{`[]`, ""},
		{``, ""},
		{`["a", "b", "c"]`, "a,b,c"},
	}
	for _, c := range cases {
		got := normalizeTags(c.input)
		if got != c.want {
			t.Errorf("normalizeTags(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestCreateRoom_NormalizesTags(t *testing.T) {
	s := setupTestServer(t)
	if err := s.CreateRoom("norm-room", "test", "", "", `["auth","mtls"]`, "", ""); err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}
	room, _ := s.GetRoom("norm-room")
	if room.Tags != "auth,mtls" {
		t.Errorf("expected tags 'auth,mtls', got %q", room.Tags)
	}
}

func TestUpdateRoom(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("update-room", "Original topic", "old-project", "Go", "old-tag", "Old prompt", "")

	// Update only project and tags
	if err := s.UpdateRoom("update-room", "", "new-project", "", "new-tag", "", "", "", ""); err != nil {
		t.Fatalf("updateRoom failed: %v", err)
	}

	room, _ := s.GetRoom("update-room")
	if room.Project != "new-project" {
		t.Errorf("expected project 'new-project', got '%s'", room.Project)
	}
	if room.Tags != "new-tag" {
		t.Errorf("expected tags 'new-tag', got '%s'", room.Tags)
	}
	// Unchanged fields should remain
	if room.Description != "Original topic" {
		t.Errorf("expected description 'Original topic', got '%s'", room.Description)
	}
	if room.TechStack != "Go" {
		t.Errorf("expected tech_stack 'Go', got '%s'", room.TechStack)
	}
	if room.SystemPrompt != "Old prompt" {
		t.Errorf("expected system_prompt 'Old prompt', got '%s'", room.SystemPrompt)
	}
}

func TestUpdateRoomNotFound(t *testing.T) {
	s := setupTestServer(t)

	err := s.UpdateRoom("nonexistent", "topic", "", "", "", "", "", "", "")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestUpdateRoomAddRemoveTags(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("tags-room", "", "", "", "foo,bar", "", "")

	// Add a tag
	err := s.UpdateRoom("tags-room", "", "", "", "", "baz", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	room, _ := s.GetRoom("tags-room")
	if !strings.Contains(room.Tags, "foo") || !strings.Contains(room.Tags, "bar") || !strings.Contains(room.Tags, "baz") {
		t.Errorf("Expected tags to contain foo, bar, baz, got '%s'", room.Tags)
	}

	// Remove a tag
	err = s.UpdateRoom("tags-room", "", "", "", "", "", "bar", "", "")
	if err != nil {
		t.Fatal(err)
	}
	room, _ = s.GetRoom("tags-room")
	if strings.Contains(room.Tags, "bar") {
		t.Errorf("Expected tags to NOT contain bar, got '%s'", room.Tags)
	}
	if !strings.Contains(room.Tags, "foo") || !strings.Contains(room.Tags, "baz") {
		t.Errorf("Expected tags to contain foo and baz, got '%s'", room.Tags)
	}

	// Overwrite tags
	err = s.UpdateRoom("tags-room", "", "", "", "new-only", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	room, _ = s.GetRoom("tags-room")
	if room.Tags != "new-only" {
		t.Errorf("Expected tags to be 'new-only', got '%s'", room.Tags)
	}
}

func TestUpdateStatus_ClearsHealthTagsOnResolve(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("health-room", "Test", "", "", "needs-synthesis,important", "", "")

	if err := s.UpdateStatus("health-room", "resolved"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	room, _ := s.GetRoom("health-room")
	if room.Status != "resolved" {
		t.Errorf("expected status 'resolved', got '%s'", room.Status)
	}
	if hasTag(room.Tags, "needs-synthesis") {
		t.Errorf("expected 'needs-synthesis' to be stripped on resolve, got tags: %s", room.Tags)
	}
	if !hasTag(room.Tags, "important") {
		t.Errorf("expected 'important' tag to be preserved, got tags: %s", room.Tags)
	}
}

func TestUpdateStatus_ClearsStaleTagOnResolve(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("stale-room", "Test", "", "", "stale,backlog", "", "")

	if err := s.UpdateStatus("stale-room", "resolved"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	room, _ := s.GetRoom("stale-room")
	if hasTag(room.Tags, "stale") {
		t.Errorf("expected 'stale' to be stripped on resolve, got tags: %s", room.Tags)
	}
	if !hasTag(room.Tags, "backlog") {
		t.Errorf("expected 'backlog' tag to be preserved, got tags: %s", room.Tags)
	}
}

func TestUpdateStatus_NoTagStripOnActiveOrPaused(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("active-room", "Test", "", "", "needs-synthesis,stale", "", "")

	// Pausing should not strip health tags
	if err := s.UpdateStatus("active-room", "paused"); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	room, _ := s.GetRoom("active-room")
	if !hasTag(room.Tags, "needs-synthesis") || !hasTag(room.Tags, "stale") {
		t.Errorf("expected health tags preserved on pause, got tags: %s", room.Tags)
	}
}
