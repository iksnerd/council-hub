package main

import (
	"fmt"
	"log/slog"
	"os"
	"testing"

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

func setupTestServer(t *testing.T) *CouncilServer {
	t.Helper()
	cs, err := NewCouncilServer(":memory:", testLogger())
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	t.Cleanup(func() { cs.db.Close() })
	return cs
}

// setupHandlerTest creates a test server with tools registered.
func setupHandlerTest(t *testing.T) *CouncilServer {
	t.Helper()
	cs := setupTestServer(t)
	registerTools(cs)
	return cs
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

// setupHandlerServer creates a handler test server with a pre-populated room and message.
func setupHandlerServer(t *testing.T) *CouncilServer {
	t.Helper()
	cs := setupHandlerTest(t)
	mustCreateRoom(t, cs, "hdb-room", withDescription("Handler DB test"), withProject("proj"), withTechStack("Go"), withTags("tag"))
	mustPost(t, cs, "hdb-room", "Claude", "Hello")
	return cs
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

func mustCreateRoom(t *testing.T, cs *CouncilServer, id string, opts ...func(*testRoom)) {
	t.Helper()
	r := testRoom{id: id, description: "Test room"}
	for _, o := range opts {
		o(&r)
	}
	if err := cs.createRoom(r.id, r.description, r.project, r.techStack, r.tags, r.systemPrompt, r.relatedRooms); err != nil {
		t.Fatalf("createRoom(%s) failed: %v", id, err)
	}
}

// --- Message helpers ---

func mustPost(t *testing.T, cs *CouncilServer, roomID, author, content string) int64 {
	t.Helper()
	id, err := cs.postMessage(roomID, author, content, "message", 0)
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}
	return id
}

func mustPostTyped(t *testing.T, cs *CouncilServer, roomID, author, content, msgType string) int64 {
	t.Helper()
	id, err := cs.postMessage(roomID, author, content, msgType, 0)
	if err != nil {
		t.Fatalf("postMessage failed: %v", err)
	}
	return id
}

func mustPostReply(t *testing.T, cs *CouncilServer, roomID, author, content string, replyTo int64) int64 {
	t.Helper()
	id, err := cs.postMessage(roomID, author, content, "message", replyTo)
	if err != nil {
		t.Fatalf("postMessage (reply) failed: %v", err)
	}
	return id
}

// setupRoomWithMessages creates a room and posts n messages from alternating authors.
func setupRoomWithMessages(t *testing.T, cs *CouncilServer, roomID string, n int) {
	t.Helper()
	mustCreateRoom(t, cs, roomID)
	authors := []string{"Claude", "Gemini"}
	for i := 0; i < n; i++ {
		mustPost(t, cs, roomID, authors[i%2], fmt.Sprintf("Message %d", i+1))
	}
}
