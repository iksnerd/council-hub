package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"council-hub/internal/council"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// agentSeenWindow is the minimum gap between two agent_seen writes for one agent.
const agentSeenWindow = 30 * time.Second

// agentSeen records "last seen" per agent from tool calls. Writers are
// serialised (SetMaxOpenConns(1)), so it debounces in memory and writes off the
// call path: a failed or slow write never touches the tool call.
type agentSeen struct {
	srv    *council.Server
	now    func() time.Time
	window time.Duration
	mu     sync.Mutex
	last   map[string]time.Time // normalised agent -> last write attempt
	wg     sync.WaitGroup
}

// installAgentSeen adds the receiving middleware. It runs after the call and
// never changes its result.
func (r *Registry) installAgentSeen() {
	a := &agentSeen{srv: r.Server, now: time.Now, window: agentSeenWindow, last: map[string]time.Time{}}
	r.seen = a
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
			client := ""
			if ss, ok := req.GetSession().(*mcp.ServerSession); ok && ss != nil {
				if p := ss.InitializeParams(); p != nil && p.ClientInfo != nil {
					client = p.ClientInfo.Name
				}
			}
			a.touch(agentFromCall(cp.Arguments, client), cp.Name)
			return res, err
		}
	})
}

// agentFromCall picks the agent name: the call's author, else its agent
// argument, else the client name. Empty names are skipped, and the placeholder
// "default" is used only when nothing else is available. "" means no name.
func agentFromCall(args json.RawMessage, client string) string {
	var in struct {
		Author any `json:"author"`
		Agent  any `json:"agent"`
	}
	_ = json.Unmarshal(args, &in)
	fallback := ""
	for _, v := range []any{in.Author, in.Agent, client} {
		c, ok := v.(string)
		c = strings.TrimSpace(c)
		switch {
		case !ok || c == "":
		case strings.EqualFold(c, "default"):
			fallback = c
		default:
			return c
		}
	}
	return fallback
}

// touch records the agent unless it was written within the window. The window
// starts at the attempt, so a failing database is not retried on every call.
func (a *agentSeen) touch(agent, tool string) {
	key := strings.ToLower(strings.TrimSpace(agent))
	if key == "" {
		return
	}
	now := a.now()
	a.mu.Lock()
	if prev, ok := a.last[key]; ok && now.Sub(prev) < a.window {
		a.mu.Unlock()
		return
	}
	a.last[key] = now
	a.mu.Unlock()
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.srv.RecordAgentSeen(key, tool, now); err != nil && a.srv.Logger != nil {
			a.srv.Logger.Debug("agent_seen write failed", "error", err)
		}
	}()
}

// wait blocks until pending writes finish (tests).
func (a *agentSeen) wait() { a.wg.Wait() }
