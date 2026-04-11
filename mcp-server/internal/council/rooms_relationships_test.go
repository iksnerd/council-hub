package council

import (
	"strings"
	"testing"
)

func TestBidirectionalRelatedRoomsOnCreate(t *testing.T) {
	s := setupTestServer(t)
	// Create target room first
	s.CreateRoom("target-room", "Target", "", "", "", "", "")
	// Create source room linking to target
	s.CreateRoom("source-room", "Source", "", "", "", "", "target-room")

	// Verify source has target
	src, _ := s.GetRoom("source-room")
	if src.RelatedRooms != "target-room" {
		t.Errorf("expected source related_rooms 'target-room', got '%s'", src.RelatedRooms)
	}

	// Verify target now has reverse link to source
	tgt, _ := s.GetRoom("target-room")
	if tgt.RelatedRooms != "source-room" {
		t.Errorf("expected target related_rooms 'source-room', got '%s'", tgt.RelatedRooms)
	}
}

func TestBidirectionalRelatedRoomsOnUpdate(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("upd-src", "Source", "", "", "", "", "")
	s.CreateRoom("upd-tgt", "Target", "", "", "", "", "")

	// Update source to link to target
	s.UpdateRoom("upd-src", "", "", "", "", "", "", "", "upd-tgt")

	// Verify reverse link
	tgt, _ := s.GetRoom("upd-tgt")
	if tgt.RelatedRooms != "upd-src" {
		t.Errorf("expected reverse link 'upd-src', got '%s'", tgt.RelatedRooms)
	}
}

func TestBidirectionalNoDuplicateLinks(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("dup-a", "Room A", "", "", "", "", "")
	s.CreateRoom("dup-b", "Room B", "", "", "", "", "dup-a")

	// Update again with same link — should not duplicate
	s.UpdateRoom("dup-b", "", "", "", "", "", "", "", "dup-a")

	a, _ := s.GetRoom("dup-a")
	if a.RelatedRooms != "dup-b" {
		t.Errorf("expected 'dup-b', got '%s'", a.RelatedRooms)
	}
}

func TestUpdateRoomRelatedRooms(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("link-room", "Link test", "", "", "", "", "")

	if err := s.UpdateRoom("link-room", "", "", "", "", "", "", "", "room-a,room-b"); err != nil {
		t.Fatalf("updateRoom failed: %v", err)
	}

	room, _ := s.GetRoom("link-room")
	if room.RelatedRooms != "room-a,room-b" {
		t.Errorf("expected related_rooms 'room-a,room-b', got '%s'", room.RelatedRooms)
	}
}

func TestDeleteRoomCleansRelatedRooms(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room-a", "A", "", "", "", "", "")
	s.CreateRoom("room-b", "B", "", "", "", "", "room-a")
	s.CreateRoom("room-c", "C", "", "", "", "", "room-a")

	if err := s.DeleteRoom("room-a"); err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}

	roomB, _ := s.GetRoom("room-b")
	if strings.Contains(roomB.RelatedRooms, "room-a") {
		t.Errorf("room-b still references deleted room-a: %q", roomB.RelatedRooms)
	}

	roomC, _ := s.GetRoom("room-c")
	if strings.Contains(roomC.RelatedRooms, "room-a") {
		t.Errorf("room-c still references deleted room-a: %q", roomC.RelatedRooms)
	}
}

func TestDeleteRoomCleansReverseLinks(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("target", "Target", "", "", "", "", "")
	// Creating source with related_rooms=target triggers syncReverseLinks,
	// so target gets a reverse link back to source.
	s.CreateRoom("source", "Source", "", "", "", "", "target")

	// Verify bidirectional link was established
	target, _ := s.GetRoom("target")
	if !strings.Contains(target.RelatedRooms, "source") {
		t.Fatalf("reverse link not established; target.RelatedRooms=%q", target.RelatedRooms)
	}

	// Delete source — should clean the reverse link from target
	if err := s.DeleteRoom("source"); err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}

	target, _ = s.GetRoom("target")
	if strings.Contains(target.RelatedRooms, "source") {
		t.Errorf("target still references deleted source: %q", target.RelatedRooms)
	}
}

func TestDeleteRoomNoFalsePositiveCleanup(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("room", "Short name", "", "", "", "", "")
	s.CreateRoom("room-extra", "Longer name", "", "", "", "", "")
	s.CreateRoom("holder", "Holds both", "", "", "", "", "room, room-extra")

	if err := s.DeleteRoom("room"); err != nil {
		t.Fatalf("DeleteRoom failed: %v", err)
	}

	holder, _ := s.GetRoom("holder")
	if strings.Contains(holder.RelatedRooms, "room-extra") == false {
		t.Errorf("room-extra was incorrectly removed from holder: %q", holder.RelatedRooms)
	}
	if strings.Contains(holder.RelatedRooms, "room") && !strings.Contains(holder.RelatedRooms, "room-extra") {
		// This would mean "room" is still there but "room-extra" is gone — wrong
		t.Errorf("cleanup removed too much: %q", holder.RelatedRooms)
	}
	// Exact check: only "room-extra" should remain
	got := strings.TrimSpace(holder.RelatedRooms)
	if got != "room-extra" {
		t.Errorf("expected holder.RelatedRooms = 'room-extra', got %q", got)
	}
}
