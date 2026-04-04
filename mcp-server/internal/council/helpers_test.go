package council

import (
	"log/slog"
	"os"
	"testing"
)

func init() {
	// Clean up any leftover test archives
	os.RemoveAll("archives")
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := NewServer(":memory:", testLogger())
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	t.Cleanup(func() { s.DB.Close() })
	return s
}

// --- Room builder ---

type testRoom struct {
	id, description, project, techStack, tags, systemPrompt, relatedRooms string
}

func withDescription(d string) func(*testRoom)   { return func(r *testRoom) { r.description = d } }
func withProject(p string) func(*testRoom)       { return func(r *testRoom) { r.project = p } }
func withTechStack(ts string) func(*testRoom)    { return func(r *testRoom) { r.techStack = ts } }
func withTags(tags string) func(*testRoom)       { return func(r *testRoom) { r.tags = tags } }
func withSystemPrompt(sp string) func(*testRoom) { return func(r *testRoom) { r.systemPrompt = sp } }
func withRelatedRooms(rr string) func(*testRoom) { return func(r *testRoom) { r.relatedRooms = rr } }

func mustCreateRoom(t *testing.T, s *Server, id string, opts ...func(*testRoom)) {
	t.Helper()
	r := testRoom{id: id, description: "Test room"}
	for _, o := range opts {
		o(&r)
	}
	if err := s.CreateRoom(r.id, r.description, r.project, r.techStack, r.tags, r.systemPrompt, r.relatedRooms); err != nil {
		t.Fatalf("CreateRoom(%s) failed: %v", id, err)
	}
}

// --- Message helpers ---

func mustPost(t *testing.T, s *Server, roomID, author, content string) string {
	t.Helper()
	id, err := s.PostMessage(roomID, author, content, "message", "")
	if err != nil {
		t.Fatalf("PostMessage failed: %v", err)
	}
	return id
}

func mustPostTyped(t *testing.T, s *Server, roomID, author, content, msgType string) string {
	t.Helper()
	id, err := s.PostMessage(roomID, author, content, msgType, "")
	if err != nil {
		t.Fatalf("PostMessage failed: %v", err)
	}
	return id
}
