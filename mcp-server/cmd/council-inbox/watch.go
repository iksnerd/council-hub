package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"
)

// council-inbox watch: a harness-agnostic presence sidecar. It joins the
// dashboard's "presence:agents" Phoenix channel over a WebSocket and keeps the
// connection alive, so the dashboard can show which agents are live. It never
// forks or daemonizes; the caller decides how to background it.

const (
	defaultPresenceURL = "ws://localhost:4000/socket/websocket?vsn=2.0.0"
	defaultClient      = "council-inbox"
	presenceTopic      = "presence:agents"
	maxClientLen       = 40
	maxRooms           = 50
	maxRoomLen         = 128
	joinRef            = "1"

	exitBadConfig = 2
)

var agentNameRE = regexp.MustCompile(`^[A-Za-z0-9._@:-]{1,64}$`)

// watchConfig carries everything runWatch needs. Durations are fields so tests
// can shrink them.
type watchConfig struct {
	Agent            string
	Client           string
	Rooms            []string
	URL              string
	FollowPID        int
	ExitOnStdinClose bool
	Quiet            bool
	Token            string // sent as X-Council-Token on the dial, never in the URL

	HeartbeatInterval time.Duration // default 30s
	ReplyTimeout      time.Duration // default 10s
	BackoffInitial    time.Duration // default 1s
	BackoffMax        time.Duration // default 60s
	FullDelay         time.Duration // minimum wait after presence_full, default 10s
	PIDPollInterval   time.Duration // default 2s
	LogInterval       time.Duration // default 10m
	LeaveTimeout      time.Duration // default 300ms

	Stdin  io.Reader
	Stderr io.Writer
}

func (c *watchConfig) applyDefaults() {
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&c.HeartbeatInterval, 30*time.Second)
	def(&c.ReplyTimeout, 10*time.Second)
	def(&c.BackoffInitial, time.Second)
	def(&c.BackoffMax, 60*time.Second)
	def(&c.FullDelay, 10*time.Second)
	def(&c.PIDPollInterval, 2*time.Second)
	def(&c.LogInterval, 10*time.Minute)
	def(&c.LeaveTimeout, 300*time.Millisecond)
	if c.Client == "" {
		c.Client = defaultClient
	}
	if c.URL == "" {
		c.URL = defaultPresenceURL
	}
	if c.Stderr == nil {
		c.Stderr = io.Discard
	}
	c.Client = truncateRunes(c.Client, maxClientLen)
	c.Rooms = clampRooms(c.Rooms)
}

type roomList []string

func (r *roomList) String() string { return strings.Join(*r, ",") }
func (r *roomList) Set(v string) error {
	*r = append(*r, v)
	return nil
}

// runWatchCmd parses the watch flags and runs the sidecar until a signal.
func runWatchCmd(args []string, getenv func(string) string, stdin io.Reader, stderr io.Writer) int {
	cfg, code, ok := parseWatchFlags(args, getenv, stderr)
	if !ok {
		return code
	}
	cfg.Stdin = stdin
	cfg.Stderr = stderr
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runWatch(ctx, cfg)
}

func parseWatchFlags(args []string, getenv func(string) string, stderr io.Writer) (watchConfig, int, bool) {
	fs := flag.NewFlagSet("council-inbox watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cfg watchConfig
	var rooms roomList
	fs.StringVar(&cfg.Agent, "agent", firstNonEmpty(getenv("COUNCIL_AUTHOR"), defaultAgent), "agent name to report (env COUNCIL_AUTHOR)")
	fs.StringVar(&cfg.Client, "client", firstNonEmpty(getenv("COUNCIL_CLIENT"), defaultClient), "client/harness name to report (env COUNCIL_CLIENT)")
	fs.Var(&rooms, "room", "room to report (repeatable)")
	fs.StringVar(&cfg.URL, "presence-url", firstNonEmpty(getenv("COUNCIL_PRESENCE_URL"), defaultPresenceURL), "dashboard WebSocket URL (env COUNCIL_PRESENCE_URL)")
	fs.IntVar(&cfg.FollowPID, "follow-pid", 0, "exit when this process no longer exists")
	fs.BoolVar(&cfg.ExitOnStdinClose, "exit-on-stdin-close", false, "exit when stdin reaches EOF")
	fs.BoolVar(&cfg.Quiet, "quiet", false, "print nothing to stderr")
	tokenFile := fs.String("token-file", "", "file whose first line is the hub token (env COUNCIL_AUTH_TOKEN); never pass the token as a flag")
	if err := fs.Parse(args); err != nil {
		return cfg, exitBadConfig, false
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "council-inbox watch: unexpected argument %q\n", fs.Arg(0))
		return cfg, exitBadConfig, false
	}
	if cfg.FollowPID < 0 {
		_, _ = fmt.Fprintln(stderr, "council-inbox watch: --follow-pid must be a positive PID")
		return cfg, exitBadConfig, false
	}
	cfg.Rooms = rooms
	tok, err := loadToken(getenv, *tokenFile, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "council-inbox watch: %v\n", err)
		return cfg, exitBadConfig, false
	}
	cfg.Token = tok
	return cfg, 0, true
}

// runWatch runs until ctx ends (exit 0), a stop condition fires (exit 0), or
// the config is rejected (exit 2).
func runWatch(ctx context.Context, cfg watchConfig) int {
	cfg.applyDefaults()
	if !agentNameRE.MatchString(cfg.Agent) {
		_, _ = fmt.Fprintf(cfg.Stderr, "council-inbox watch: invalid agent name %q (allowed: letters, digits and . _ @ : -, 1 to 64 chars)\n", cfg.Agent)
		return exitBadConfig
	}
	if u, err := url.Parse(cfg.URL); err != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
		_, _ = fmt.Fprintf(cfg.Stderr, "council-inbox watch: invalid presence URL: need a ws:// or wss:// URL\n")
		return exitBadConfig
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if cfg.FollowPID > 0 {
		go followPID(ctx, cancel, cfg.FollowPID, cfg.PIDPollInterval)
	}
	if cfg.ExitOnStdinClose && cfg.Stdin != nil {
		go func() {
			_, _ = io.Copy(io.Discard, cfg.Stdin)
			cancel()
		}()
	}

	lg := &throttledLog{w: cfg.Stderr, every: cfg.LogInterval, quiet: cfg.Quiet}
	attempt := 0
	for ctx.Err() == nil {
		res := runSession(ctx, cfg)
		if res.authRejected {
			_, _ = fmt.Fprintf(cfg.Stderr, "council-inbox watch: %s\n", authFailedMsg)
			return exitBadConfig
		}
		if res.fatal != "" {
			_, _ = fmt.Fprintf(cfg.Stderr, "council-inbox watch: dashboard rejected this config: %s (check --agent and --room)\n", res.fatal)
			return exitBadConfig
		}
		if ctx.Err() != nil {
			break
		}
		if res.joined {
			attempt = 0
		}
		lg.printf("council-inbox watch: presence connection to %s lost or unavailable (%v); retrying in the background", redactURL(cfg.URL), res.err)
		delay := backoffDelay(cfg, attempt)
		if res.full && delay < cfg.FullDelay {
			delay = cfg.FullDelay
		}
		attempt++
		select {
		case <-ctx.Done():
		case <-time.After(delay):
		}
	}
	return 0
}

func backoffDelay(cfg watchConfig, attempt int) time.Duration {
	d := cfg.BackoffInitial
	for i := 0; i < attempt && d < cfg.BackoffMax; i++ {
		d *= 2
	}
	if d > cfg.BackoffMax {
		d = cfg.BackoffMax
	}
	// jitter: 50% to 100% of the nominal delay
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}

type sessionResult struct {
	err   error
	fatal string // invalid_agent or invalid_payload: retrying cannot help
	// authRejected: the hub refused the token (HTTP 401/403 on the upgrade, or an
	// unauthorized join reply). Retrying with the same token cannot help.
	authRejected bool
	full         bool // presence_full
	joined       bool
}

type frame struct {
	joinRef *string
	ref     *string
	topic   string
	event   string
	payload json.RawMessage
}

// Phoenix V2 JSON serializer: [join_ref, ref, topic, event, payload].
func encodeFrame(jref, ref *string, topic, event string, payload any) ([]byte, error) {
	return json.Marshal([]any{jref, ref, topic, event, payload})
}

func decodeFrame(b []byte) (frame, error) {
	var parts []json.RawMessage
	if err := json.Unmarshal(b, &parts); err != nil || len(parts) != 5 {
		return frame{}, errors.New("not a phoenix v2 frame")
	}
	var f frame
	if err := json.Unmarshal(parts[0], &f.joinRef); err != nil {
		return frame{}, err
	}
	if err := json.Unmarshal(parts[1], &f.ref); err != nil {
		return frame{}, err
	}
	if err := json.Unmarshal(parts[2], &f.topic); err != nil {
		return frame{}, err
	}
	if err := json.Unmarshal(parts[3], &f.event); err != nil {
		return frame{}, err
	}
	f.payload = parts[4]
	return f, nil
}

type readResult struct {
	data []byte
	err  error
}

// runSession dials, joins and keeps one connection alive until it fails or
// ctx ends. On ctx end it sends phx_leave first.
func runSession(ctx context.Context, cfg watchConfig) sessionResult {
	dialCtx, dialCancel := context.WithTimeout(ctx, cfg.ReplyTimeout)
	var opts *websocket.DialOptions
	if cfg.Token != "" {
		opts = &websocket.DialOptions{HTTPHeader: http.Header{"X-Council-Token": []string{cfg.Token}}}
	}
	c, resp, err := websocket.Dial(dialCtx, cfg.URL, opts)
	dialCancel()
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return sessionResult{authRejected: true, err: errors.New("token rejected")}
		}
		return sessionResult{err: err}
	}
	defer func() { _ = c.CloseNow() }()
	c.SetReadLimit(1 << 20)

	sctx, scancel := context.WithCancel(context.Background())
	defer scancel()
	reads := make(chan readResult, 8)
	go func() {
		for {
			_, data, err := c.Read(sctx)
			select {
			case reads <- readResult{data, err}:
			case <-sctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	send := func(sendCtx context.Context, jref, ref *string, topic, event string, payload any) error {
		b, err := encodeFrame(jref, ref, topic, event, payload)
		if err != nil {
			return err
		}
		return c.Write(sendCtx, websocket.MessageText, b)
	}
	// sendT writes with its own timeout, never the session context: cancelling a
	// context mid-write makes the library drop the connection, and then the leave
	// frame below could not be sent (a heartbeat racing with shutdown did exactly that).
	sendT := func(jref, ref *string, topic, event string, payload any) error {
		tctx, tcancel := context.WithTimeout(context.Background(), cfg.ReplyTimeout)
		defer tcancel()
		return send(tctx, jref, ref, topic, event, payload)
	}
	leave := func() {
		lctx, lcancel := context.WithTimeout(context.Background(), cfg.LeaveTimeout)
		defer lcancel()
		jr, r := joinRef, "leave"
		if err := send(lctx, &jr, &r, presenceTopic, "phx_leave", map[string]any{}); err != nil {
			return
		}
		// Close the socket properly, bounded by the same timeout. Tearing it down
		// right after the write can lose the leave frame (the server sees the
		// connection drop before it reads it); the close frame queues behind the
		// leave, so the server reads the leave first.
		closed := make(chan struct{})
		go func() {
			_ = c.Close(websocket.StatusNormalClosure, "leaving")
			close(closed)
		}()
		select {
		case <-closed:
		case <-lctx.Done():
		}
	}

	jr, r0 := joinRef, joinRef
	payload := map[string]any{"agent": cfg.Agent, "client": cfg.Client, "rooms": cfg.Rooms}
	if err := sendT(&jr, &r0, presenceTopic, "phx_join", payload); err != nil {
		return sessionResult{err: err}
	}

	var (
		res        sessionResult
		hbTicker   *time.Ticker
		hbTick     <-chan time.Time
		hbRef      string
		hbSeq      int
		hbDeadline <-chan time.Time
	)
	defer func() {
		if hbTicker != nil {
			hbTicker.Stop()
		}
	}()
	joinDeadline := time.After(cfg.ReplyTimeout)

	for {
		select {
		case <-ctx.Done():
			leave()
			res.err = ctx.Err()
			return res

		case <-joinDeadline:
			if !res.joined {
				res.err = errors.New("no join reply from dashboard")
				return res
			}

		case <-hbDeadline:
			res.err = errors.New("no heartbeat reply from dashboard")
			return res

		case <-hbTick:
			if hbRef != "" {
				continue // previous heartbeat still pending; its deadline decides
			}
			hbSeq++
			hbRef = strconv.Itoa(hbSeq + 1)
			ref := hbRef
			if err := sendT(nil, &ref, "phoenix", "heartbeat", map[string]any{}); err != nil {
				res.err = err
				return res
			}
			hbDeadline = time.After(cfg.ReplyTimeout)

		case rr := <-reads:
			if rr.err != nil {
				res.err = rr.err
				return res
			}
			f, err := decodeFrame(rr.data)
			if err != nil {
				continue
			}
			switch {
			case f.topic == "phoenix" && f.event == "phx_reply" && hbRef != "" && f.ref != nil && *f.ref == hbRef:
				hbRef, hbDeadline = "", nil
			case f.topic == presenceTopic && f.event == "phx_reply" && f.ref != nil && *f.ref == joinRef && !res.joined:
				status, reason := parseReply(f.payload)
				switch {
				case status == "ok":
					res.joined = true
					hbTicker = time.NewTicker(cfg.HeartbeatInterval)
					hbTick = hbTicker.C
				case reason == "unauthorized" || reason == "invalid_token":
					res.authRejected = true
					return res
				case reason == "invalid_agent" || reason == "invalid_payload":
					res.fatal = reason
					return res
				case reason == "presence_full":
					res.full = true
					res.err = errors.New("presence_full")
					return res
				default:
					res.err = fmt.Errorf("join refused: %s", firstNonEmpty(reason, status))
					return res
				}
			case f.topic == presenceTopic && (f.event == "phx_close" || f.event == "phx_error"):
				res.err = fmt.Errorf("channel %s", f.event)
				return res
			}
		}
	}
}

func parseReply(p json.RawMessage) (status, reason string) {
	var v struct {
		Status   string `json:"status"`
		Response struct {
			Reason string `json:"reason"`
		} `json:"response"`
	}
	_ = json.Unmarshal(p, &v)
	return v.Status, v.Response.Reason
}

// followPID cancels ctx once the process is gone.
func followPID(ctx context.Context, cancel context.CancelFunc, pid int, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if !pidAlive(pid) {
			cancel()
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// throttledLog prints at most one line per interval, and nothing when quiet.
type throttledLog struct {
	w     io.Writer
	every time.Duration
	quiet bool
	last  time.Time
}

func (l *throttledLog) printf(format string, args ...any) {
	if l.quiet || (!l.last.IsZero() && time.Since(l.last) < l.every) {
		return
	}
	l.last = time.Now()
	_, _ = fmt.Fprintf(l.w, format+"\n", args...)
}

// redactURL drops userinfo and query so a log line never carries credentials.
func redactURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return "<presence url>"
	}
	u.User = nil
	u.RawQuery = ""
	return u.String()
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func clampRooms(in []string) []string {
	out := make([]string, 0, len(in))
	for _, r := range in {
		if len(out) == maxRooms {
			break
		}
		out = append(out, truncateRunes(r, maxRoomLen))
	}
	return out
}
