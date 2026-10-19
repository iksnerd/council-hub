package council

import "testing"

// A database created before soft delete has no deleted_at column; opening it
// must add the column without losing notebooks.
func TestNewServerMigratesNotebooksDeletedAt(t *testing.T) {
	path := t.TempDir() + "/old.db"
	s, err := NewServer(path, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateNotebook("old-nb", "", "Old"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`ALTER TABLE notebooks DROP COLUMN deleted_at`); err != nil {
		t.Fatalf("simulating a pre-migration database failed: %v", err)
	}
	_ = s.DB.Close()

	s2, err := NewServer(path, testLogger())
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	t.Cleanup(func() { _ = s2.DB.Close() })
	if _, err := s2.GetNotebook("old-nb"); err != nil {
		t.Fatalf("pre-migration notebook must stay readable: %v", err)
	}
	if _, err := s2.DeleteNotebook("old-nb"); err != nil {
		t.Fatalf("soft delete after migration failed: %v", err)
	}
}
