package council

import (
	"reflect"
	"testing"
)

func TestGetDigestPostedFromDistinctAcrossAuthors(t *testing.T) {
	s := setupTestServer(t)
	if err := s.CreateRoom("dpf-room", "t", "dpf-proj", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	post := func(author, from string) {
		t.Helper()
		if _, _, err := s.PostMessageOpts("dpf-room", author, "hello from "+author, "note", "", "", "", PostOptions{PostedFrom: from}); err != nil {
			t.Fatal(err)
		}
	}
	post("alice", "/work/alpha/")
	post("alice", "/work/alpha")
	post("bob", "/other/beta")
	post("carol", "") // no posted_from: contributes nothing

	entries, err := s.GetDigest("dpf-proj", "2000-01-01 00:00:00")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	got := entries[0]
	if got.NewMessages != 4 {
		t.Errorf("aggregate must be undisturbed: new_messages=%d, want 4", got.NewMessages)
	}
	if want := []string{"alpha", "beta"}; !reflect.DeepEqual(got.PostedFrom, want) {
		t.Errorf("PostedFrom = %v, want %v", got.PostedFrom, want)
	}
}
