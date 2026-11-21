package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"council-hub/internal/handlers"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// startListeners runs the real wiring (buildMux, bindAll, serveAll) on random
// local ports; trusted=false starts the main listener only. It stops
// everything on cleanup.
type listenerFixture struct {
	mainURL, trustedURL string
	reg                 *handlers.Registry
}

func startListeners(t *testing.T, token string, trusted bool) *listenerFixture {
	t.Helper()
	cs := testServer(t)
	reg := &handlers.Registry{Server: cs, ClusterSecret: "test-cluster-secret"}
	reg.RegisterTools()
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return cs.MCP }, nil)

	var mcpEntry http.Handler = mcpHandler
	if token != "" {
		mcpEntry = requireBearer(token, newFailureLog(discardLogger()), mcpHandler)
	}
	health := healthHandlerOpts(cs, "", nil, token != "", trusted)
	servers := []*http.Server{newHTTPServer("127.0.0.1:0", buildMux(mcpEntry, health, reg))}
	if trusted {
		servers = append(servers, newHTTPServer("127.0.0.1:0", buildMux(mcpHandler, health, reg)))
	}
	lns, err := bindAll(servers)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveAll(ctx, discardLogger(), servers, lns) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Error("serveAll did not stop")
		}
	})
	f := &listenerFixture{reg: reg, mainURL: "http://" + lns[0].Addr().String()}
	if trusted {
		f.trustedURL = "http://" + lns[1].Addr().String()
	}
	return f
}

func get(t *testing.T, url, authz string) (int, string) {
	t.Helper()
	return send(t, http.MethodGet, url, authz)
}

func send(t *testing.T, method, url, authz string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader("{}"))
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestValidateListenAddrs(t *testing.T) {
	if err := validateListenAddrs(":3001", ""); err != nil {
		t.Fatalf("no trusted addr must pass: %v", err)
	}
	if err := validateListenAddrs(":3001", ":3002"); err != nil {
		t.Fatalf("different addrs must pass: %v", err)
	}
	err := validateListenAddrs(":3001", ":3001")
	if err == nil || !strings.Contains(err.Error(), "COUNCIL_TRUSTED_ADDR") {
		t.Fatalf("equal addrs must be refused with a clear message: %v", err)
	}
}

func TestBindFailureOnTrustedAddrIsRefused(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = taken.Close() }()
	servers := []*http.Server{
		newHTTPServer("127.0.0.1:0", http.NewServeMux()),
		newHTTPServer(taken.Addr().String(), http.NewServeMux()),
	}
	lns, err := bindAll(servers)
	if err == nil || lns != nil {
		t.Fatalf("bind failure must be an error with no listeners left open: %v", err)
	}
}

func TestAuthOffBothListenersOpen(t *testing.T) {
	f := startListeners(t, "", true)
	for _, base := range []string{f.mainURL, f.trustedURL} {
		if code, _ := send(t, http.MethodPost, base+"/mcp", ""); code == http.StatusUnauthorized {
			t.Errorf("%s: auth off must not 401", base)
		}
	}
}

func TestAuthOnMainEnforcesTrustedOpen(t *testing.T) {
	f := startListeners(t, testAuthToken, true)
	if code, _ := send(t, http.MethodPost, f.mainURL+"/mcp", ""); code != http.StatusUnauthorized {
		t.Fatalf("main without token: %d", code)
	}
	if code, _ := send(t, http.MethodPost, f.mainURL+"/mcp", "Bearer "+testAuthToken); code == http.StatusUnauthorized {
		t.Fatalf("main with token must pass auth: %d", code)
	}
	for _, p := range []string{"/mcp", "/mcp/"} {
		if code, _ := send(t, http.MethodPost, f.trustedURL+p, ""); code == http.StatusUnauthorized {
			t.Fatalf("trusted %s without token must not 401", p)
		}
	}
}

func TestHealthTrustedListenerFlag(t *testing.T) {
	flag := func(url string) any {
		code, body := get(t, url+"/health", "")
		if code != 200 {
			t.Fatalf("health %d", code)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(body), &m); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(body, url[len("http://"):]) || strings.Contains(body, testAuthToken) {
			t.Fatalf("health leaks an address or secret: %s", body)
		}
		return m["trusted_listener"]
	}
	f := startListeners(t, testAuthToken, true)
	if flag(f.mainURL) != true || flag(f.trustedURL) != true {
		t.Fatal("trusted_listener must be true on both listeners when configured")
	}
	g := startListeners(t, testAuthToken, false)
	if flag(g.mainURL) != false {
		t.Fatal("trusted_listener must be false when not configured")
	}
}

func TestInternalAndUIBehaveIdenticallyOnBothListeners(t *testing.T) {
	f := startListeners(t, testAuthToken, true)
	for _, path := range []string{"/api/internal/post_to_room", "/api/ui/post"} {
		mc, mb := send(t, http.MethodPost, f.mainURL+path, "")
		tc, tb := send(t, http.MethodPost, f.trustedURL+path, "")
		if mc != tc || mb != tb {
			t.Errorf("%s differs: main %d %q, trusted %d %q", path, mc, mb, tc, tb)
		}
		// /api/ui/* is loopback-only and the test client is loopback, so only
		// the cookie-guarded /api/internal/* must refuse here.
		if strings.HasPrefix(path, "/api/internal/") && mc != http.StatusForbidden {
			t.Errorf("%s: expected 403 without the cluster secret, got %d", path, mc)
		}
	}
}

type bearerRT struct{ authz string }

func (b bearerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if b.authz != "" {
		r.Header.Set("Authorization", b.authz)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func mcpSession(t *testing.T, url, authz string) *mcp.ClientSession {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "listener-test", Version: "0"}, nil)
	sess, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   url + "/mcp",
		HTTPClient: &http.Client{Transport: bearerRT{authz}},
	}, nil)
	if err != nil {
		t.Fatalf("connect %s: %v", url, err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func callTool(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var out strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			out.WriteString(tc.Text)
		}
	}
	return out.String()
}

func TestTrustedListenerSharesState(t *testing.T) {
	f := startListeners(t, testAuthToken, true)
	viaMain := mcpSession(t, f.mainURL, "Bearer "+testAuthToken)
	viaTrusted := mcpSession(t, f.trustedURL, "")

	callTool(t, viaMain, "create_room", map[string]any{"id": "shared-room", "topic": "t"})
	callTool(t, viaMain, "post_to_room", map[string]any{"room_id": "shared-room", "author": "alice", "message": "posted-via-main"})
	callTool(t, viaTrusted, "post_to_room", map[string]any{"room_id": "shared-room", "author": "bob", "message": "posted-via-trusted"})

	for name, s := range map[string]*mcp.ClientSession{"main": viaMain, "trusted": viaTrusted} {
		out := callTool(t, s, "get_messages", map[string]any{"room_id": "shared-room"})
		if !strings.Contains(out, "posted-via-main") || !strings.Contains(out, "posted-via-trusted") {
			t.Errorf("%s listener does not see both messages:\n%s", name, out)
		}
	}
}
