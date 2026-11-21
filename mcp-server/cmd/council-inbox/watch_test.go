package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakePhoenix is a minimal Phoenix-v2-style channel server. It records every
// frame it receives and answers joins and heartbeats per its settings.
type fakePhoenix struct {
	joinReply   func(n int) (status, reason string) // nil = always ok
	dropAfter   func(n int) bool                    // close the socket after join n
	noHBReply   bool
	joins       atomic.Int32
	heartbeats  atomic.Int32
	leaves      atomic.Int32
	mu          sync.Mutex
	joinPayload []map[string]any
	frames      chan []json.RawMessage
}

func newFake() *fakePhoenix { return &fakePhoenix{frames: make(chan []json.RawMessage, 64)} }

func (f *fakePhoenix) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			var p []json.RawMessage
			if json.Unmarshal(data, &p) != nil || len(p) != 5 {
				continue
			}
			select {
			case f.frames <- p:
			default:
			}
			var jref, ref *string
			var topic, event string
			_ = json.Unmarshal(p[0], &jref)
			_ = json.Unmarshal(p[1], &ref)
			_ = json.Unmarshal(p[2], &topic)
			_ = json.Unmarshal(p[3], &event)
			reply := func(top, status string, resp any) {
				b, _ := json.Marshal([]any{jref, ref, top, "phx_reply", map[string]any{"status": status, "response": resp}})
				_ = c.Write(r.Context(), websocket.MessageText, b)
			}
			switch {
			case event == "phx_join":
				n := int(f.joins.Add(1))
				var pl map[string]any
				_ = json.Unmarshal(p[4], &pl)
				f.mu.Lock()
				f.joinPayload = append(f.joinPayload, pl)
				f.mu.Unlock()
				status, reason := "ok", ""
				if f.joinReply != nil {
					status, reason = f.joinReply(n)
				}
				if status == "ok" {
					reply(topic, "ok", map[string]any{})
				} else {
					reply(topic, "error", map[string]any{"reason": reason})
				}
				if f.dropAfter != nil && f.dropAfter(n) {
					c.Close(websocket.StatusGoingAway, "bye")
					return
				}
			case topic == "phoenix" && event == "heartbeat":
				f.heartbeats.Add(1)
				if !f.noHBReply {
					reply("phoenix", "ok", map[string]any{})
				}
			case event == "phx_leave":
				f.leaves.Add(1)
			}
		}
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/socket/websocket?vsn=2.0.0"
}

func fastCfg(url string) watchConfig {
	return watchConfig{
		Agent:             "codex",
		Client:            "codex-cli",
		Rooms:             []string{"room-a", "room-b"},
		URL:               url,
		HeartbeatInterval: 30 * time.Millisecond,
		ReplyTimeout:      300 * time.Millisecond,
		BackoffInitial:    10 * time.Millisecond,
		BackoffMax:        40 * time.Millisecond,
		FullDelay:         150 * time.Millisecond,
		PIDPollInterval:   20 * time.Millisecond,
		LeaveTimeout:      200 * time.Millisecond,
		Stderr:            &bytes.Buffer{},
	}
}

func startWatch(t *testing.T, cfg watchConfig) (cancel context.CancelFunc, done <-chan int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan int, 1)
	go func() { ch <- runWatch(ctx, cfg) }()
	t.Cleanup(cancel)
	return cancel, ch
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func waitExit(t *testing.T, done <-chan int, within time.Duration) int {
	t.Helper()
	select {
	case code := <-done:
		return code
	case <-time.After(within):
		t.Fatal("watch did not exit in time")
		return -1
	}
}

func TestWatchJoinFrameShape(t *testing.T) {
	f := newFake()
	cancel, done := startWatch(t, fastCfg(f.serve(t)))
	defer cancel()

	select {
	case p := <-f.frames:
		var jref, ref, topic, event string
		_ = json.Unmarshal(p[0], &jref)
		_ = json.Unmarshal(p[1], &ref)
		_ = json.Unmarshal(p[2], &topic)
		_ = json.Unmarshal(p[3], &event)
		if jref != "1" || ref != "1" || topic != "presence:agents" || event != "phx_join" {
			t.Fatalf("join frame = %s", p)
		}
		var pl struct {
			Agent  string   `json:"agent"`
			Client string   `json:"client"`
			Rooms  []string `json:"rooms"`
		}
		if err := json.Unmarshal(p[4], &pl); err != nil {
			t.Fatal(err)
		}
		if pl.Agent != "codex" || pl.Client != "codex-cli" || len(pl.Rooms) != 2 || pl.Rooms[0] != "room-a" {
			t.Fatalf("payload = %+v", pl)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no join frame")
	}
	cancel()
	waitExit(t, done, time.Second)
}

func TestWatchTruncatesPayload(t *testing.T) {
	f := newFake()
	cfg := fastCfg(f.serve(t))
	cfg.Client = strings.Repeat("c", 100)
	cfg.Rooms = nil
	for i := 0; i < 60; i++ {
		cfg.Rooms = append(cfg.Rooms, strings.Repeat("r", 300))
	}
	cancel, done := startWatch(t, cfg)
	waitFor(t, "join", func() bool { return f.joins.Load() >= 1 })
	f.mu.Lock()
	pl := f.joinPayload[0]
	f.mu.Unlock()
	if got := len(pl["client"].(string)); got != 40 {
		t.Fatalf("client len %d", got)
	}
	rooms := pl["rooms"].([]any)
	if len(rooms) != 50 || len(rooms[0].(string)) != 128 {
		t.Fatalf("rooms %d x %d", len(rooms), len(rooms[0].(string)))
	}
	cancel()
	waitExit(t, done, time.Second)
}

func TestWatchHeartbeatOnInterval(t *testing.T) {
	f := newFake()
	cancel, done := startWatch(t, fastCfg(f.serve(t)))
	waitFor(t, "3 heartbeats", func() bool { return f.heartbeats.Load() >= 3 })
	// heartbeat frame shape: [null, ref, "phoenix", "heartbeat", {}]
	for {
		select {
		case p := <-f.frames:
			if string(p[3]) == `"heartbeat"` {
				if string(p[0]) != "null" || string(p[2]) != `"phoenix"` || string(p[4]) != "{}" {
					t.Fatalf("heartbeat frame = %s", p)
				}
				cancel()
				waitExit(t, done, time.Second)
				return
			}
		default:
			t.Fatal("no heartbeat frame recorded")
		}
	}
}

func TestWatchReconnectsAndRejoins(t *testing.T) {
	f := newFake()
	f.dropAfter = func(n int) bool { return n == 1 }
	cancel, done := startWatch(t, fastCfg(f.serve(t)))
	waitFor(t, "rejoin", func() bool { return f.joins.Load() >= 2 })
	f.mu.Lock()
	second := f.joinPayload[1]
	f.mu.Unlock()
	if second["agent"] != "codex" || len(second["rooms"].([]any)) != 2 {
		t.Fatalf("rejoin payload = %v", second)
	}
	cancel()
	waitExit(t, done, time.Second)
}

func TestWatchMissingHeartbeatReplyReconnects(t *testing.T) {
	f := newFake()
	f.noHBReply = true
	cfg := fastCfg(f.serve(t))
	cfg.ReplyTimeout = 60 * time.Millisecond
	cancel, done := startWatch(t, cfg)
	waitFor(t, "rejoin after missed reply", func() bool { return f.joins.Load() >= 2 })
	cancel()
	waitExit(t, done, time.Second)
}

func TestWatchInvalidAgentNeverDials(t *testing.T) {
	f := newFake()
	cfg := fastCfg(f.serve(t))
	cfg.Agent = "bad name!"
	var errBuf bytes.Buffer
	cfg.Stderr = &errBuf
	code := runWatch(context.Background(), cfg)
	if code != 2 {
		t.Fatalf("exit = %d", code)
	}
	if f.joins.Load() != 0 || len(f.frames) != 0 {
		t.Fatal("dialed despite invalid agent")
	}
	if !strings.Contains(errBuf.String(), "invalid agent name") {
		t.Fatalf("stderr = %q", errBuf.String())
	}
	for _, bad := range []string{"", strings.Repeat("a", 65), "a b", "a/b"} {
		cfg.Agent = bad
		if runWatch(context.Background(), cfg) != 2 {
			t.Fatalf("agent %q accepted", bad)
		}
	}
}

func TestWatchServerRejectionsExit2(t *testing.T) {
	for _, reason := range []string{"invalid_agent", "invalid_payload"} {
		f := newFake()
		f.joinReply = func(int) (string, string) { return "error", reason }
		cfg := fastCfg(f.serve(t))
		var errBuf bytes.Buffer
		cfg.Stderr = &errBuf
		_, done := startWatch(t, cfg)
		if code := waitExit(t, done, 2*time.Second); code != 2 {
			t.Fatalf("%s: exit = %d", reason, code)
		}
		if !strings.Contains(errBuf.String(), reason) {
			t.Fatalf("%s: stderr = %q", reason, errBuf.String())
		}
		if f.joins.Load() != 1 {
			t.Fatalf("%s: retried (%d joins)", reason, f.joins.Load())
		}
	}
}

func TestWatchPresenceFullRetriesSlowly(t *testing.T) {
	f := newFake()
	f.joinReply = func(n int) (string, string) {
		if n < 3 {
			return "error", "presence_full"
		}
		return "ok", ""
	}
	cfg := fastCfg(f.serve(t))
	cancel, done := startWatch(t, cfg)
	time.Sleep(100 * time.Millisecond)
	// BackoffInitial is 10ms but FullDelay is 150ms: only the first join so far.
	if n := f.joins.Load(); n != 1 {
		t.Fatalf("joins after 100ms = %d, want 1 (slow retry)", n)
	}
	waitFor(t, "eventual join", func() bool { return f.joins.Load() >= 3 })
	cancel()
	waitExit(t, done, time.Second)
}

func TestWatchFollowPIDExitsWhenProcessDies(t *testing.T) {
	f := newFake()
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skip("cannot start sleep:", err)
	}
	pid := cmd.Process.Pid
	reaped := false
	defer func() {
		if !reaped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	cfg := fastCfg(f.serve(t))
	cfg.FollowPID = pid
	_, done := startWatch(t, cfg)
	waitFor(t, "join", func() bool { return f.joins.Load() >= 1 })
	select {
	case <-done:
		t.Fatal("exited while followed process was alive")
	case <-time.After(100 * time.Millisecond):
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait() // reap so the PID no longer exists
	reaped = true
	if code := waitExit(t, done, 2*time.Second); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	waitFor(t, "leave", func() bool { return f.leaves.Load() >= 1 })
}

func TestWatchExitOnStdinClose(t *testing.T) {
	f := newFake()
	cfg := fastCfg(f.serve(t))
	cfg.ExitOnStdinClose = true
	pr, pw := io.Pipe()
	cfg.Stdin = pr
	_, done := startWatch(t, cfg)
	waitFor(t, "join", func() bool { return f.joins.Load() >= 1 })
	pw.Close()
	if code := waitExit(t, done, time.Second); code != 0 {
		t.Fatalf("exit = %d", code)
	}
}

func TestWatchGracefulLeaveOnCancel(t *testing.T) {
	f := newFake()
	cancel, done := startWatch(t, fastCfg(f.serve(t)))
	waitFor(t, "join", func() bool { return f.joins.Load() >= 1 })
	start := time.Now()
	cancel()
	if code := waitExit(t, done, time.Second); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("took %v to exit", time.Since(start))
	}
	waitFor(t, "phx_leave frame", func() bool { return f.leaves.Load() == 1 })
}

func TestWatchNoDashboardIsQuietAndSurvives(t *testing.T) {
	// A listener that was closed gives a port nothing listens on.
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	var errBuf bytes.Buffer
	cfg := fastCfg("ws://" + addr + "/socket/websocket?vsn=2.0.0")
	cfg.Stderr = &errBuf
	cfg.LogInterval = time.Hour
	cancel, done := startWatch(t, cfg)
	time.Sleep(250 * time.Millisecond)
	cancel()
	if code := waitExit(t, done, time.Second); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if n := strings.Count(errBuf.String(), "\n"); n != 1 {
		t.Fatalf("stderr lines = %d: %q", n, errBuf.String())
	}

	// --quiet prints nothing at all.
	errBuf.Reset()
	cfg.Quiet = true
	cancel, done = startWatch(t, cfg)
	time.Sleep(100 * time.Millisecond)
	cancel()
	waitExit(t, done, time.Second)
	if errBuf.Len() != 0 {
		t.Fatalf("quiet printed %q", errBuf.String())
	}
}

func TestParseWatchFlags(t *testing.T) {
	env := map[string]string{"COUNCIL_AUTHOR": "env-agent", "COUNCIL_CLIENT": "env-client", "COUNCIL_PRESENCE_URL": "ws://example.invalid:4000/x"}
	getenv := func(k string) string { return env[k] }
	cfg, _, ok := parseWatchFlags([]string{"--room", "a", "--room", "b", "--follow-pid", "42", "--quiet"}, getenv, &bytes.Buffer{})
	if !ok || cfg.Agent != "env-agent" || cfg.Client != "env-client" || cfg.URL != "ws://example.invalid:4000/x" ||
		len(cfg.Rooms) != 2 || cfg.FollowPID != 42 || !cfg.Quiet {
		t.Fatalf("cfg = %+v ok=%v", cfg, ok)
	}
	cfg, _, ok = parseWatchFlags([]string{"--agent", "x", "--client", "y", "--presence-url", "ws://h:1/p"}, getenv, &bytes.Buffer{})
	if !ok || cfg.Agent != "x" || cfg.Client != "y" || cfg.URL != "ws://h:1/p" {
		t.Fatalf("flags should beat env: %+v", cfg)
	}
	cfg, _, _ = parseWatchFlags(nil, func(string) string { return "" }, &bytes.Buffer{})
	if cfg.Agent != "claude-code" || cfg.Client != "council-inbox" || cfg.URL != defaultPresenceURL {
		t.Fatalf("defaults = %+v", cfg)
	}
	if _, code, ok := parseWatchFlags([]string{"--bogus"}, getenv, &bytes.Buffer{}); ok || code != 2 {
		t.Fatal("unknown flag accepted")
	}
}
