package handlers

import (
	"encoding/json"
	"testing"
	"time"
)

func seenRow(t *testing.T, reg *Registry, agent string) (string, string, bool) {
	t.Helper()
	reg.seen.wait()
	var at, tool string
	err := reg.Server.DB.QueryRow(`SELECT last_seen_at, last_tool FROM agent_seen WHERE agent = ?`, agent).Scan(&at, &tool)
	return at, tool, err == nil
}

func TestAgentSeenDebounce(t *testing.T) {
	reg := setupHandlerTest(t)
	clock := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	reg.seen.now = func() time.Time { return clock }

	reg.seen.touch("  Claude ", "read_room") // normalised to "claude"
	at, tool, ok := seenRow(t, reg, "claude")
	if !ok || tool != "read_room" || at == "" {
		t.Fatalf("first write: %q %q %v", at, tool, ok)
	}
	first := at

	clock = clock.Add(10 * time.Second)
	reg.seen.touch("claude", "post_to_room") // inside the window
	if at, tool, _ := seenRow(t, reg, "claude"); at != first || tool != "read_room" {
		t.Errorf("debounced call wrote: %q %q", at, tool)
	}

	reg.seen.touch("bob", "list_rooms") // a different agent writes
	if _, _, ok := seenRow(t, reg, "bob"); !ok {
		t.Error("a different agent was debounced")
	}

	clock = clock.Add(31 * time.Second)
	reg.seen.touch("claude", "post_to_room")
	if at, tool, _ := seenRow(t, reg, "claude"); at == first || tool != "post_to_room" {
		t.Errorf("write after window: %q %q", at, tool)
	}
}

func TestAgentSeenFailedWriteIsHarmless(t *testing.T) {
	reg := setupHandlerTest(t)
	if _, err := reg.Server.DB.Exec(`DROP TABLE agent_seen`); err != nil {
		t.Fatal(err)
	}
	reg.seen.touch("claude", "read_room") // must not panic or block
	reg.seen.wait()
}

func TestAgentSeenFailedWriteDoesNotFailCall(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	if _, err := reg.Server.DB.Exec(`DROP TABLE agent_seen`); err != nil {
		t.Fatal(err)
	}
	res := callTool(t, cs, "create_room", map[string]any{"id": "seen-fail", "topic": "t"})
	if res.IsError {
		t.Fatalf("call failed: %s", resultText(res))
	}
	reg.seen.wait()
}

func TestAgentSeenMiddlewareUsesAuthorThenClientName(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	callTool(t, cs, "create_room", map[string]any{"id": "seen-mw", "topic": "t"})
	callTool(t, cs, "post_to_room", map[string]any{"room_id": "seen-mw", "message": "hi", "author": "Alice"})
	reg.seen.wait()
	if _, _, ok := seenRow(t, reg, "test-client"); !ok {
		t.Error("client name not recorded for a call without author")
	}
	// author arrived inside the 30s window of the same agent? it is a different
	// key ("alice" vs "test-client"), so both rows exist.
	if _, tool, ok := seenRow(t, reg, "alice"); !ok || tool != "post_to_room" {
		t.Errorf("author not recorded: %q %v", tool, ok)
	}
}

func TestAgentFromCall(t *testing.T) {
	cases := []struct{ args, client, want string }{
		{`{"author":"alice"}`, "cli", "alice"},
		{`{"agent":"bob"}`, "cli", "bob"},
		{`{"author":"alice","agent":"bob"}`, "cli", "alice"},
		{`{}`, "cli", "cli"},
		{`{"author":""}`, "cli", "cli"},
		{`{"author":"default"}`, "cli", "cli"},
		{`{"author":"default"}`, "", "default"},
		{`{}`, "", ""},
		{`{"author":5}`, "cli", "cli"},
	}
	for _, c := range cases {
		if got := agentFromCall(json.RawMessage(c.args), c.client); got != c.want {
			t.Errorf("agentFromCall(%s, %q) = %q, want %q", c.args, c.client, got, c.want)
		}
	}
}
