package handlers

import (
	"log/slog"
	"os"
	"testing"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func setupTestServer(t *testing.T) *council.Server {
	t.Helper()
	s, err := council.NewServer(":memory:", testLogger())
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	t.Cleanup(func() { s.DB.Close() })
	return s
}

// setupHandlerTestWithTempDB creates a registry using a real temp DB file so that
// archive operations get an isolated directory per test.
func setupHandlerTestWithTempDB(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	s, err := council.NewServer(dbPath, testLogger())
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	t.Cleanup(func() { s.DB.Close() })
	r := &Registry{Server: s}
	r.RegisterTools()
	return r
}

// setupHandlerTest creates a test registry with tools registered.
func setupHandlerTest(t *testing.T) *Registry {
	t.Helper()
	s := setupTestServer(t)
	r := &Registry{Server: s}
	r.RegisterTools()
	return r
}

// resultText extracts the text from a tool call result.
func resultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	if tc, ok := r.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

// setupHandlerServer creates a handler test registry with a pre-populated room and message.
func setupHandlerServer(t *testing.T) *Registry {
	t.Helper()
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "hdb-room", withDescription("Handler DB test"), withProject("proj"), withTechStack("Go"), withTags("tag"))
	mustPost(t, reg.Server, "hdb-room", "Claude", "Hello")
	return reg
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

func mustCreateRoom(t *testing.T, s *council.Server, id string, opts ...func(*testRoom)) {
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

func mustPost(t *testing.T, s *council.Server, roomID, author, content string) string {
	t.Helper()
	id, err := s.PostMessage(roomID, author, content, "message", "")
	if err != nil {
		t.Fatalf("PostMessage failed: %v", err)
	}
	return id
}

func mustPostTyped(t *testing.T, s *council.Server, roomID, author, content, msgType string) string {
	t.Helper()
	id, err := s.PostMessage(roomID, author, content, msgType, "")
	if err != nil {
		t.Fatalf("PostMessage failed: %v", err)
	}
	return id
}

// ========== toolResultText edge cases ==========

func TestToolResultTextNilResult(t *testing.T) {
	if toolResultText(nil) != "" {
		t.Error("expected empty string for nil result")
	}
}

func TestToolResultTextEmptyContent(t *testing.T) {
	r := &mcp.CallToolResult{Content: []mcp.Content{}}
	if toolResultText(r) != "" {
		t.Error("expected empty string for empty content")
	}
}

func TestToolResultTextNonTextContent(t *testing.T) {
	// Content that is not *mcp.TextContent returns ""
	r := &mcp.CallToolResult{Content: []mcp.Content{(*mcp.TextContent)(nil)}}
	_ = toolResultText(r) // just exercise the path, no panic
}
