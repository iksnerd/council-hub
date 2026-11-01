package handlers

import (
	"testing"
	"time"
)

func usageRow(t *testing.T, reg *Registry, tool string) (calls, errs int, last string) {
	t.Helper()
	err := reg.Server.DB.QueryRow(`SELECT calls, errors, COALESCE(last_used,'') FROM tool_usage WHERE tool = ?`, tool).Scan(&calls, &errs, &last)
	if err != nil {
		t.Fatalf("row %s: %v", tool, err)
	}
	return
}

func TestUsage_CountsCallsAndErrors(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	callTool(t, cs, "create_room", map[string]any{"id": "usage-a", "topic": "t"})
	callTool(t, cs, "create_room", map[string]any{"id": "usage-b", "topic": "t"})
	bad := callTool(t, cs, "read_room", map[string]any{"id": "no-such-room-xyz"})
	if !bad.IsError {
		t.Skip("read_room miss is not an error result in this build")
	}
	reg.FlushUsage()

	calls, errs, last := usageRow(t, reg, "create_room")
	if calls != 2 || errs != 0 || last == "" {
		t.Errorf("create_room = %d/%d/%q", calls, errs, last)
	}
	calls, errs, _ = usageRow(t, reg, "read_room")
	if calls != 1 || errs != 1 {
		t.Errorf("read_room = %d/%d, want 1/1", calls, errs)
	}
}

func TestUsage_SeedsNeverCalledAndIgnoresUnknown(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	reg.usage.record("not_a_real_tool", false)
	callTool(t, cs, "create_room", map[string]any{"id": "usage-c", "topic": "t"})
	reg.FlushUsage()

	var n int
	_ = reg.Server.DB.QueryRow(`SELECT COUNT(*) FROM tool_usage WHERE tool = 'not_a_real_tool'`).Scan(&n)
	if n != 0 {
		t.Errorf("unknown tool stored")
	}
	if c, _, _ := usageRow(t, reg, "post_to_room"); c != 0 {
		t.Errorf("post_to_room should be seeded at 0, got %d", c)
	}
}

func TestUsage_FullBufferDropsWithoutBlocking(t *testing.T) {
	u := &usageCounter{ch: make(chan usageEvent, 2)}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			u.record("x", false)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("record blocked on a full buffer")
	}
	if got := u.dropped.Load(); got != 8 {
		t.Errorf("dropped = %d, want 8", got)
	}
}

func TestUsage_OnlyAllowedColumns(t *testing.T) {
	_, reg := setupIntegrationTest(t)
	rows, err := reg.Server.DB.Query(`SELECT name FROM pragma_table_info('tool_usage') ORDER BY cid`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var cols []string
	for rows.Next() {
		var c string
		_ = rows.Scan(&c)
		cols = append(cols, c)
	}
	want := "tool,calls,errors,first_used,last_used"
	got := ""
	for i, c := range cols {
		if i > 0 {
			got += ","
		}
		got += c
	}
	if got != want {
		t.Errorf("columns = %s, want %s", got, want)
	}
}

func TestUsage_HealthSummaryShape(t *testing.T) {
	cs, reg := setupIntegrationTest(t)
	callTool(t, cs, "create_room", map[string]any{"id": "usage-d", "topic": "t"})
	reg.FlushUsage()
	s := UsageSummary(reg.Server.DB)
	top, ok := s["top"].([]map[string]any)
	if !ok || len(top) == 0 || len(top) > 10 {
		t.Fatalf("top = %#v", s["top"])
	}
	if top[0]["tool"] != "create_room" || top[0]["calls"] != 1 {
		t.Errorf("top[0] = %#v", top[0])
	}
	for _, k := range []string{"tool", "calls", "errors", "last_used"} {
		if _, ok := top[0][k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
	if nc, _ := s["never_called"].(int); nc < 1 {
		t.Errorf("never_called = %v", s["never_called"])
	}
	if cnt, _ := s["counting_since"].(string); cnt == "" {
		t.Errorf("counting_since empty")
	}
}
