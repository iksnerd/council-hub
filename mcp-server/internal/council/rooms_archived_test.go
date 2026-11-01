package council

import (
	"testing"
	"time"
)

func TestArchiveRoomSetsArchivedAtAndKeepsUpdatedAt(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("arch-at", "Archive stamp", "proj", "", "", "", "")
	s.PostMessage("arch-at", "Claude", "hello", "message", "")
	if _, err := s.DB.Exec(`UPDATE rooms SET updated_at = '2020-01-01 00:00:00' WHERE id = 'arch-at'`); err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetRoom("arch-at")
	if before.ArchivedAt != nil {
		t.Fatalf("new room must not be archived, got %v", before.ArchivedAt)
	}

	if _, err := s.ArchiveRoom("arch-at"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	after, _ := s.GetRoom("arch-at")
	if after.ArchivedAt == nil || time.Since(*after.ArchivedAt) > time.Minute {
		t.Fatalf("archived_at not set to now: %v", after.ArchivedAt)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("archiving must not bump updated_at: before %v after %v", before.UpdatedAt, after.UpdatedAt)
	}
	if after.Status != "resolved" {
		t.Errorf("status = %s", after.Status)
	}
}

func TestListRoomsArchivedFilter(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("done-archived", "a", "p", "", "", "", "")
	s.CreateRoom("done-pending", "b", "p", "", "", "", "")
	s.UpdateStatus("done-pending", "resolved")
	if _, err := s.ArchiveRoom("done-archived"); err != nil {
		t.Fatal(err)
	}

	pending, err := s.ListRoomsFiltered(ListRoomsOptions{Status: "resolved", Archived: "false"})
	if err != nil || len(pending) != 1 || pending[0].ID != "done-pending" {
		t.Fatalf("archived=false: %v %v", pending, err)
	}
	done, err := s.ListRoomsFiltered(ListRoomsOptions{Status: "resolved", Archived: "true"})
	if err != nil || len(done) != 1 || done[0].ID != "done-archived" || done[0].ArchivedAt == nil {
		t.Fatalf("archived=true: %v %v", done, err)
	}
	n, _ := s.CountRooms(ListRoomsOptions{Status: "resolved", Archived: "true"})
	if n != 1 {
		t.Errorf("count archived=true = %d", n)
	}
}

func TestGetLastActivityTracksMessagesOnly(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("act", "x", "p", "", "", "", "")
	s.PostMessage("act", "Claude", "hi", "message", "")
	if _, err := s.DB.Exec(`UPDATE messages SET timestamp = '2021-03-04 05:06:07' WHERE room_id = 'act'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ArchiveRoom("act"); err != nil {
		t.Fatal(err)
	}
	got := s.GetLastActivity()["act"]
	want := time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("last activity = %v, want %v", got, want)
	}
}
