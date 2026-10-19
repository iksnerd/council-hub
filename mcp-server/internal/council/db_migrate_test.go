package council

import (
	"path/filepath"
	"testing"
)

// The 'code' message type was retired in v0.45.0; rows that still carry it are
// rewritten to 'note' on startup, leaving content untouched, and the migration
// is a no-op the second time.
func TestStartupRetiresCodeMessageType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retire.db")

	s, err := NewServer(path, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	mustCreateRoom(t, s, "r1")
	id := mustPost(t, s, "r1", "alice", "func main() {}")
	keep := mustPostTyped(t, s, "r1", "alice", "a decision", "decision")
	if _, err := s.DB.Exec(`UPDATE messages SET message_type = 'code' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	_ = s.DB.Close()

	for pass := 1; pass <= 2; pass++ {
		s, err = NewServer(path, testLogger())
		if err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		var typ, content string
		if err := s.DB.QueryRow(`SELECT message_type, content FROM messages WHERE id = ?`, id).Scan(&typ, &content); err != nil {
			t.Fatal(err)
		}
		if typ != "note" {
			t.Errorf("pass %d: message_type = %q, want note", pass, typ)
		}
		if content != "func main() {}" {
			t.Errorf("pass %d: content changed to %q", pass, content)
		}
		var other string
		_ = s.DB.QueryRow(`SELECT message_type FROM messages WHERE id = ?`, keep).Scan(&other)
		if other != "decision" {
			t.Errorf("pass %d: unrelated row changed to %q", pass, other)
		}
		_ = s.DB.Close()
	}
}
