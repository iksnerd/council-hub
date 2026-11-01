package council

import (
	"strings"
	"testing"
)

func TestFindSimilarRoomsByTag(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "existing-auth", withProject("myapp"), withTags("go,auth,api"))
	mustCreateRoom(t, s, "existing-db", withProject("myapp"), withTags("go,postgres,api"))

	similar, err := s.FindSimilarRooms("new-room", "Auth service implementation", "myapp", "go,auth", 5)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	if len(similar) == 0 {
		t.Fatal("expected at least one similar room")
	}
	if similar[0].ID != "existing-auth" {
		t.Errorf("expected existing-auth as top match, got %s", similar[0].ID)
	}
}

func TestFindSimilarRoomsByDescription(t *testing.T) {
	s := setupTestServer(t)
	// Need score >= 3: use 3 overlapping keywords to reach threshold
	mustCreateRoom(t, s, "auth-service", withDescription("Authentication middleware design patterns"))

	similar, err := s.FindSimilarRooms("new-room", "Authentication middleware design overview", "", "", 5)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	if len(similar) == 0 {
		t.Fatal("expected a match on description keywords (authentication, middleware, design)")
	}
	if similar[0].ID != "auth-service" {
		t.Errorf("expected auth-service as match, got %s", similar[0].ID)
	}
}

func TestFindSimilarRoomsExcludesSelf(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "self-room", withTags("go,auth"))

	similar, err := s.FindSimilarRooms("self-room", "Auth", "", "go,auth", 5)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	for _, r := range similar {
		if r.ID == "self-room" {
			t.Error("FindSimilarRooms should not return the excluded room")
		}
	}
}

func TestFindSimilarRoomsNoSignal(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "some-room", withTags("go"))

	similar, err := s.FindSimilarRooms("new-room", "the a to", "", "", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(similar) != 0 {
		t.Errorf("expected no results with no signal, got %d", len(similar))
	}
}

func TestFindSimilarRoomsLimit(t *testing.T) {
	s := setupTestServer(t)
	for i := 0; i < 5; i++ {
		mustCreateRoom(t, s, strings.Repeat("r", i+1)+"-room", withTags("go,auth,api"))
	}

	similar, err := s.FindSimilarRooms("new", "Auth", "", "go,auth,api", 2)
	if err != nil {
		t.Fatalf("FindSimilarRooms error: %v", err)
	}
	if len(similar) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(similar))
	}
}
