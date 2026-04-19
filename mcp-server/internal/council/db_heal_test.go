package council

import "testing"

func TestIsIndexOnlyCorruption(t *testing.T) {
	cases := []struct {
		name   string
		issues []string
		want   bool
	}{
		{"empty slice is not corruption", nil, false},
		{"named index wrong count", []string{"wrong # of entries in index idx_messages_room_id"}, true},
		{"autoindex wrong count", []string{"wrong # of entries in index sqlite_autoindex_messages_1"}, true},
		{"row missing from index", []string{"row 42 missing from index idx_messages_timestamp"}, true},
		{"non-unique entry in index", []string{"non-unique entry in index idx_rooms_project"}, true},
		{"multiple index issues", []string{
			"wrong # of entries in index idx_messages_room_id",
			"wrong # of entries in index sqlite_autoindex_rooms_1",
		}, true},
		{"null in not null column is data corruption", []string{"NULL value in messages.content"}, false},
		{"rowid out of order is data corruption", []string{"rowid not in ascending order"}, false},
		{"freelist corruption is data corruption", []string{"Main freelist: free-page count mismatch"}, false},
		{"mixed index and data errors are not index-only", []string{
			"wrong # of entries in index idx_foo",
			"rowid not in ascending order",
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isIndexOnlyCorruption(tc.issues); got != tc.want {
				t.Errorf("isIndexOnlyCorruption(%v) = %v, want %v", tc.issues, got, tc.want)
			}
		})
	}
}

func TestHealIndexesHealthyDB(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "room-1")
	mustPost(t, s, "room-1", "claude", "hello")

	if err := healIndexes(s.DB, testLogger()); err != nil {
		t.Fatalf("healIndexes on healthy DB failed: %v", err)
	}

	issues, err := integrityCheck(s.DB)
	if err != nil {
		t.Fatalf("integrityCheck: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues after heal on healthy DB, got %v", issues)
	}
}

func TestIntegrityCheckHealthyDB(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "room-1")
	mustPost(t, s, "room-1", "claude", "hello")

	issues, err := integrityCheck(s.DB)
	if err != nil {
		t.Fatalf("integrityCheck failed: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues on fresh DB, got %v", issues)
	}
}
