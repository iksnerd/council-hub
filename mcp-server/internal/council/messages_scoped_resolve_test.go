package council

import (
	"errors"
	"testing"
)

func insertRawMessage(t *testing.T, s *Server, id, room, author string) {
	t.Helper()
	if _, err := s.DB.Exec(`INSERT INTO messages (id, room_id, author, content, message_type) VALUES (?, ?, ?, 'x', 'note')`, id, room, author); err != nil {
		t.Fatalf("insert %s: %v", id, err)
	}
}

func TestResolveMessageIDInRoomIgnoresOtherRooms(t *testing.T) {
	s := setupTestServer(t)
	for _, r := range []string{"room-a", "room-b"} {
		if err := s.CreateRoom(r, "t", "", "", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	insertRawMessage(t, s, "01a0aaaa-0000-7000-8000-000000000001", "room-a", "x")
	insertRawMessage(t, s, "01a0aaaa-0000-7000-8000-000000000002", "room-b", "y")

	// Globally ambiguous...
	var amb *ErrAmbiguousMessageID
	if _, err := s.ResolveMessageID("01a0aaaa"); !errors.As(err, &amb) {
		t.Fatalf("expected global ambiguity, got %v", err)
	}
	// ...but unique inside room-a.
	got, err := s.ResolveMessageIDInRoom("01a0aaaa", "room-a")
	if err != nil || got != "01a0aaaa-0000-7000-8000-000000000001" {
		t.Fatalf("scoped resolve = %q, %v", got, err)
	}
}

func TestResolveMessageIDInRoomAmbiguousWithinRoom(t *testing.T) {
	s := setupTestServer(t)
	if err := s.CreateRoom("room-a", "t", "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	insertRawMessage(t, s, "01a0bbbb-0000-7000-8000-000000000001", "room-a", "x")
	insertRawMessage(t, s, "01a0bbbb-0000-7000-8000-000000000002", "room-a", "y")
	var amb *ErrAmbiguousMessageID
	if _, err := s.ResolveMessageIDInRoom("01a0bbbb", "room-a"); !errors.As(err, &amb) {
		t.Fatalf("expected ambiguity within the room, got %v", err)
	}
}

func TestResolveMessageIDInRoomFallsBackToGlobal(t *testing.T) {
	s := setupTestServer(t)
	for _, r := range []string{"room-a", "room-b"} {
		if err := s.CreateRoom(r, "t", "", "", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	insertRawMessage(t, s, "01a0cccc-0000-7000-8000-000000000001", "room-b", "y")
	got, err := s.ResolveMessageIDInRoom("01a0cccc", "room-a")
	if err != nil || got != "01a0cccc-0000-7000-8000-000000000001" {
		t.Fatalf("fallback resolve = %q, %v", got, err)
	}
}

func TestPostMessageOptsIdempotencyAndMeta(t *testing.T) {
	s := setupTestServer(t)
	if err := s.CreateRoom("room-a", "t", "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	opts := PostOptions{Tags: "meta-feedback,x", PostedFrom: "/work/tree", IdempotencyKey: "k1"}
	id1, dup, err := s.PostMessageOpts("room-a", "me", "hello", "note", "", "", "", opts)
	if err != nil || dup {
		t.Fatalf("first post: %v dup=%v", err, dup)
	}
	id2, dup, err := s.PostMessageOpts("room-a", "me", "hello again", "note", "", "", "", opts)
	if err != nil || !dup || id2 != id1 {
		t.Fatalf("retry should return existing: id2=%s dup=%v err=%v", id2, dup, err)
	}
	// Same key, different author is a different claim.
	id3, dup, _ := s.PostMessageOpts("room-a", "other", "hi", "note", "", "", "", opts)
	if dup || id3 == id1 {
		t.Fatalf("key must be scoped to author")
	}
	m, err := s.GetMessageByID(id1)
	if err != nil {
		t.Fatal(err)
	}
	if m.Tags != "meta-feedback,x" || m.PostedFrom != "/work/tree" {
		t.Errorf("meta not stored: %+v", m)
	}
}
