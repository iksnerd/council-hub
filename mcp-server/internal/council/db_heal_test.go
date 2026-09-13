package council

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

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

	healed, err := healIndexes(s.DB, testLogger())
	if err != nil {
		t.Fatalf("healIndexes on healthy DB failed: %v", err)
	}
	if healed {
		t.Errorf("expected healed=false on healthy DB, got true")
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

// A message written straight to SQLite with no id (messages.id is a TEXT PRIMARY
// KEY, which SQLite lets be NULL) can't be fetched, linked or embedded. Startup
// gives it a UUIDv7 carrying its own timestamp, so it keeps its chronological
// place in id-ordered reads, and a second start leaves that id alone.
func TestNewServerHealsNullMessageIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "council.db")
	s, err := NewServer(path, testLogger())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	mustCreateRoom(t, s, "null-id-room")
	written := time.Date(2026, 4, 1, 17, 15, 10, 0, time.UTC)
	if _, err := s.DB.Exec(
		`INSERT INTO messages (id, room_id, author, content, timestamp) VALUES (NULL, 'null-id-room', 'Gemini CLI', 'written without an id', ?)`,
		written.Format("2006-01-02 15:04:05"),
	); err != nil {
		t.Fatalf("seed NULL-id row: %v", err)
	}
	_ = s.DB.Close()

	idAfterStart := func() string {
		t.Helper()
		s, err := NewServer(path, testLogger())
		if err != nil {
			t.Fatalf("NewServer: %v", err)
		}
		defer func() { _ = s.DB.Close() }()
		msgs, err := s.GetTranscript("null-id-room")
		if err != nil || len(msgs) != 1 {
			t.Fatalf("GetTranscript: %d messages, err=%v", len(msgs), err)
		}
		return msgs[0].ID
	}

	id := idAfterStart()
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.Version() != 7 {
		t.Fatalf("expected a UUIDv7 id, got %q (err=%v)", id, err)
	}
	sec, nsec := parsed.Time().UnixTime()
	if got := time.Unix(sec, nsec).UTC(); !got.Equal(written) {
		t.Errorf("id should carry the message timestamp %v, got %v", written, got)
	}
	if again := idAfterStart(); again != id {
		t.Errorf("a second start must not change the healed id: %q then %q", id, again)
	}
}
