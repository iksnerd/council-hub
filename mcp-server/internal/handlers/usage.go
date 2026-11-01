package handlers

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"time"

	"council-hub/internal/council"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool usage counter.
//
// Privacy: the tool_usage table holds ONLY the tool name, a call count, an
// error count and two timestamps. No arguments, message text, author or room
// id is ever read, kept or written here. Only names of tools that are actually
// registered are counted, so a client cannot grow the table. Deleting the
// rows (DELETE FROM tool_usage) resets the counter.

const (
	usageBuffer        = 1024
	usageFlushInterval = 5 * time.Second
	usageTimeFmt       = "2006-01-02 15:04:05"
)

type usageEvent struct {
	tool string
	err  bool
	at   time.Time
}

type usageAgg struct {
	calls, errors int
	last          time.Time
}

type usageCounter struct {
	srv     *council.Server
	ch      chan usageEvent
	flushCh chan chan struct{}
	stop    chan struct{}
	done    chan struct{}
	dropped atomic.Int64
	once    sync.Once

	names map[string]bool // registered tool names; owned by the drain goroutine
	agg   map[string]*usageAgg
}

// installUsageCounter wires the counting middleware. It never blocks or fails a call.
func (r *Registry) installUsageCounter() {
	u := &usageCounter{
		srv:     r.Server,
		ch:      make(chan usageEvent, usageBuffer),
		flushCh: make(chan chan struct{}),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		agg:     map[string]*usageAgg{},
	}
	r.usage = u
	go u.run()
	r.Server.MCP.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			if method != "tools/call" {
				return res, err
			}
			cp, ok := req.GetParams().(*mcp.CallToolParamsRaw)
			if !ok || cp == nil {
				return res, err
			}
			failed := err != nil
			if call, ok := res.(*mcp.CallToolResult); ok && call != nil && call.IsError {
				failed = true
			}
			u.record(cp.Name, failed)
			return res, err
		}
	})
}

// record enqueues without blocking; a full buffer drops the event.
func (u *usageCounter) record(tool string, failed bool) {
	select {
	case u.ch <- usageEvent{tool: tool, err: failed, at: time.Now().UTC()}:
	default:
		u.dropped.Add(1)
	}
}

func (u *usageCounter) run() {
	defer close(u.done)
	t := time.NewTicker(usageFlushInterval)
	defer t.Stop()
	for {
		select {
		case ev := <-u.ch:
			u.add(ev)
		case <-t.C:
			u.flush()
		case ack := <-u.flushCh:
			u.drain()
			u.flush()
			close(ack)
		case <-u.stop:
			u.drain()
			u.flush()
			return
		}
	}
}

func (u *usageCounter) drain() {
	for {
		select {
		case ev := <-u.ch:
			u.add(ev)
		default:
			return
		}
	}
}

func (u *usageCounter) add(ev usageEvent) {
	if !u.known(ev.tool) {
		return
	}
	a := u.agg[ev.tool]
	if a == nil {
		a = &usageAgg{}
		u.agg[ev.tool] = a
	}
	a.calls++
	if ev.err {
		a.errors++
	}
	a.last = ev.at
}

// known reports whether name is a registered tool, resolving the list lazily
// (tools are registered after the middleware) through a throwaway in-memory session.
func (u *usageCounter) known(name string) bool {
	if u.names == nil {
		u.names = u.resolveNames()
	}
	return u.names[name]
}

func (u *usageCounter) resolveNames() map[string]bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t1, t2 := mcp.NewInMemoryTransports()
	ss, err := u.srv.MCP.Connect(ctx, t1, nil)
	if err != nil {
		return nil
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "usage-probe", Version: "0"}, nil).Connect(ctx, t2, nil)
	if err != nil {
		return nil
	}
	defer func() { _ = cs.Close() }()
	names := map[string]bool{}
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			return nil
		}
		names[tool.Name] = true
	}
	if len(names) == 0 {
		return nil
	}
	u.seed(names)
	return names
}

// seed inserts a zero row per registered tool so never-called tools are visible.
func (u *usageCounter) seed(names map[string]bool) {
	now := time.Now().UTC().Format(usageTimeFmt)
	u.srv.Mu.Lock()
	defer u.srv.Mu.Unlock()
	for n := range names {
		_, _ = u.srv.DB.Exec(`INSERT OR IGNORE INTO tool_usage (tool, calls, errors, first_used) VALUES (?, 0, 0, ?)`, n, now)
	}
}

func (u *usageCounter) flush() {
	if len(u.agg) == 0 {
		return
	}
	u.srv.Mu.Lock()
	defer u.srv.Mu.Unlock()
	tx, err := u.srv.DB.Begin()
	if err != nil {
		return // keep aggregates; retry next tick
	}
	for tool, a := range u.agg {
		ts := a.last.Format(usageTimeFmt)
		if _, err := tx.Exec(`INSERT INTO tool_usage (tool, calls, errors, first_used, last_used) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(tool) DO UPDATE SET calls = calls + excluded.calls, errors = errors + excluded.errors, last_used = excluded.last_used`,
			tool, a.calls, a.errors, ts, ts); err != nil {
			_ = tx.Rollback()
			return
		}
	}
	if tx.Commit() != nil {
		return
	}
	u.agg = map[string]*usageAgg{}
}

// FlushUsage synchronously writes pending counts (tests, shutdown).
func (r *Registry) FlushUsage() {
	if r.usage == nil {
		return
	}
	ack := make(chan struct{})
	select {
	case r.usage.flushCh <- ack:
		<-ack
	case <-r.usage.done:
	}
}

// StopUsage flushes and stops the counter goroutine; safe to call more than once.
func (r *Registry) StopUsage() {
	if r.usage == nil {
		return
	}
	r.usage.once.Do(func() { close(r.usage.stop) })
	<-r.usage.done
}

// UsageSummary builds the compact /health block. Nil when the table is unreadable.
func UsageSummary(db *sql.DB) map[string]any {
	rows, err := db.Query(`SELECT tool, calls, errors, COALESCE(last_used, '') FROM tool_usage ORDER BY calls DESC, tool LIMIT 10`)
	if err != nil {
		return nil
	}
	top := []map[string]any{}
	for rows.Next() {
		var tool, last string
		var calls, errs int
		if rows.Scan(&tool, &calls, &errs, &last) == nil {
			top = append(top, map[string]any{"tool": tool, "calls": calls, "errors": errs, "last_used": last})
		}
	}
	_ = rows.Close()
	var never, total int
	var since sql.NullString
	if db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(calls = 0), 0), MIN(first_used) FROM tool_usage`).Scan(&total, &never, &since) != nil {
		return nil
	}
	return map[string]any{"top": top, "never_called": never, "counting_since": since.String}
}
