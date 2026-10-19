package council

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupToProducesOpenableCopy(t *testing.T) {
	dir := t.TempDir()
	s, err := NewServer(filepath.Join(dir, "live.db"), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.DB.Close() }()
	mustCreateRoom(t, s, "bk")
	mustPost(t, s, "bk", "alice", "keep me")

	dest := filepath.Join(dir, "copy.db")
	if err := s.BackupTo(dest); err != nil {
		t.Fatalf("BackupTo: %v", err)
	}

	c, err := NewServer(dest, testLogger())
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer func() { _ = c.DB.Close() }()
	msgs, err := c.GetRecentMessages("bk", 5)
	if err != nil || len(msgs) != 1 || msgs[0].Content != "keep me" {
		t.Fatalf("backup content wrong: %v %v", msgs, err)
	}
}

func TestBackupToRefusesExistingFile(t *testing.T) {
	s := setupTestServer(t)
	dest := filepath.Join(t.TempDir(), "exists.db")
	if err := os.WriteFile(dest, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.BackupTo(dest); err == nil {
		t.Fatal("expected refusal to overwrite an existing file")
	}
}
