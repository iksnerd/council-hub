package tailnet

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeNode stands in for tsnet: it signs in on the second Status call, listens
// on a plain loopback port, and names every caller as id.
type fakeNode struct {
	mu        sync.Mutex
	signIn    chan struct{} // closed when the operator "finishes" signing in
	id        Identity
	owner     string
	addr      chan string
	started   bool
	loggedOut bool
	closed    bool
	warm      chan struct{} // when set, WarmCert fails until it is closed
	warmed    bool
}

func newFakeNode(owner string, id Identity) *fakeNode {
	f := &fakeNode{owner: owner, id: id, addr: make(chan string, 1), signIn: make(chan struct{})}
	close(f.signIn) // already signed in; a test that wants the sign-in step swaps this out
	return f
}

func (f *fakeNode) Start() error { f.mu.Lock(); f.started = true; f.mu.Unlock(); return nil }
func (f *fakeNode) Status(context.Context) (NodeStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.signIn:
	default:
		return NodeStatus{AuthURL: "https://login.example/abc"}, nil
	}
	return NodeStatus{Running: true, DNSName: "council-hub.example.ts.net", Owner: f.owner}, nil
}
func (f *fakeNode) Identify(context.Context, string) (Identity, error) { return f.id, nil }
func (f *fakeNode) ListenTLS(string) (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err == nil {
		f.addr <- ln.Addr().String()
	}
	return ln, err
}
func (f *fakeNode) WarmCert(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.warm != nil {
		select {
		case <-f.warm:
		default:
			return errors.New("cert not ready")
		}
	}
	f.warmed = true
	return nil
}
func (f *fakeNode) Logout(context.Context) error {
	f.mu.Lock()
	f.loggedOut = true
	f.mu.Unlock()
	return nil
}
func (f *fakeNode) Close() error { f.mu.Lock(); f.closed = true; f.mu.Unlock(); return nil }

func testManager(t *testing.T, node *fakeNode) (*Manager, Store) {
	t.Helper()
	store := Store{Path: filepath.Join(t.TempDir(), "tailscale.json")}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok") })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewManager(store, func(Config, string) Node { return node }, mux, log)
	m.poll = 5 * time.Millisecond
	t.Cleanup(m.Stop)
	return m, store
}

func waitState(t *testing.T, m *Manager, want State) View {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if v := m.Status(); v.State == want {
			return v
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("state never became %q, last %+v", want, m.Status())
	return View{}
}

func get(t *testing.T, addr, path string) int {
	t.Helper()
	resp, err := http.Get("http://" + addr + path)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func enabled() Config { c := DefaultConfig(); c.Enabled = true; return c }

func TestManagerSignInThenServeAsOwner(t *testing.T) {
	node := newFakeNode("me@example.com", Identity{LoginName: "me@example.com"})
	node.signIn = make(chan struct{}) // hold at the sign-in step
	m, store := testManager(t, node)

	if st := m.Status().State; st != StateOff {
		t.Fatalf("initial state = %q, want off", st)
	}
	if err := m.Apply(enabled(), ""); err != nil {
		t.Fatal(err)
	}
	v := waitState(t, m, StateNeedsLogin)
	if v.AuthURL != "https://login.example/abc" {
		t.Fatalf("auth url = %q", v.AuthURL)
	}
	close(node.signIn) // the operator finishes signing in
	v = waitState(t, m, StateRunning)
	if v.URL != "https://council-hub.example.ts.net" || v.Owner != "me@example.com" {
		t.Fatalf("running view = %+v", v)
	}

	addr := <-node.addr
	if code := get(t, addr, "/health"); code != 200 {
		t.Fatalf("owner on /health = %d, want 200", code)
	}
	// A path the tailnet mux does not carry must not exist: this is how the
	// localhost-only /api/ui/* routes stay off the tailnet.
	if code := get(t, addr, "/api/ui/post"); code != 404 {
		t.Fatalf("/api/ui/post over the tailnet = %d, want 404", code)
	}

	saved, err := store.Load()
	if err != nil || !saved.Enabled {
		t.Fatalf("Apply must persist the config: %+v, %v", saved, err)
	}
}

func TestManagerRefusesNonOwnerAndTagged(t *testing.T) {
	for name, id := range map[string]Identity{
		"another user": {LoginName: "you@example.com"},
		"tagged":       {Tagged: true},
		"unidentified": {},
	} {
		t.Run(name, func(t *testing.T) {
			node := newFakeNode("me@example.com", id)
			m, _ := testManager(t, node)
			if err := m.Apply(enabled(), ""); err != nil {
				t.Fatal(err)
			}
			waitState(t, m, StateRunning)
			if code := get(t, <-node.addr, "/health"); code != 403 {
				t.Fatalf("%s in owner mode = %d, want 403", name, code)
			}
		})
	}
}

func TestManagerMembersModeAdmitsAnotherUser(t *testing.T) {
	node := newFakeNode("me@example.com", Identity{LoginName: "you@example.com"})
	m, _ := testManager(t, node)
	cfg := enabled()
	cfg.Access = AccessMembers
	if err := m.Apply(cfg, ""); err != nil {
		t.Fatal(err)
	}
	waitState(t, m, StateRunning)
	if code := get(t, <-node.addr, "/health"); code != 200 {
		t.Fatalf("members mode, another user = %d, want 200", code)
	}
}

func TestManagerDisableStopsNode(t *testing.T) {
	node := newFakeNode("me@example.com", Identity{LoginName: "me@example.com"})
	m, _ := testManager(t, node)
	if err := m.Apply(enabled(), ""); err != nil {
		t.Fatal(err)
	}
	waitState(t, m, StateRunning)
	addr := <-node.addr

	off := DefaultConfig()
	if err := m.Apply(off, ""); err != nil {
		t.Fatal(err)
	}
	if st := m.Status().State; st != StateOff {
		t.Fatalf("state after disable = %q, want off", st)
	}
	node.mu.Lock()
	closed := node.closed
	node.mu.Unlock()
	if !closed {
		t.Fatal("disabling must close the node")
	}
	if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		c.Close()
		t.Fatal("the tailnet listener must be closed after disable")
	}
}

func TestManagerApplyRejectsInvalidAndKeepsRunning(t *testing.T) {
	node := newFakeNode("me@example.com", Identity{LoginName: "me@example.com"})
	m, _ := testManager(t, node)
	if err := m.Apply(enabled(), ""); err != nil {
		t.Fatal(err)
	}
	waitState(t, m, StateRunning)

	bad := enabled()
	bad.Hostname = "NO"
	if err := m.Apply(bad, ""); err == nil {
		t.Fatal("invalid config must be rejected")
	}
	if st := m.Status().State; st != StateRunning {
		t.Fatalf("a rejected apply must not stop the node, state = %q", st)
	}
}

func TestManagerUnreadableConfigStaysOff(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "tailscale.json")}
	if err := writeFile(store.Path, `{"enabled":true,"hostname":"x","access":"all"}`); err != nil {
		t.Fatal(err)
	}
	called := false
	m := NewManager(store, func(Config, string) Node { called = true; return newFakeNode("", Identity{}) },
		http.NewServeMux(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.Start()
	if called {
		t.Fatal("a config that fails validation must not start a node")
	}
	if v := m.Status(); v.State != StateError || v.Error == "" {
		t.Fatalf("view = %+v, want an error state naming the problem", v)
	}
}

func TestManagerLogoutSignsOutStopsAndSavesDisabled(t *testing.T) {
	node := newFakeNode("me@example.com", Identity{LoginName: "me@example.com"})
	m, store := testManager(t, node)
	if err := m.Apply(enabled(), ""); err != nil {
		t.Fatal(err)
	}
	waitState(t, m, StateRunning)
	addr := <-node.addr

	if err := m.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	node.mu.Lock()
	out, closed := node.loggedOut, node.closed
	node.mu.Unlock()
	if !out || !closed {
		t.Fatalf("logout must sign the device out and close it: loggedOut=%v closed=%v", out, closed)
	}
	if st := m.Status().State; st != StateOff {
		t.Fatalf("state after logout = %q, want off", st)
	}
	saved, err := store.Load()
	if err != nil || saved.Enabled {
		t.Fatalf("logout must save the config as disabled: %+v, %v", saved, err)
	}
	if saved.Hostname != "council-hub" {
		t.Fatalf("logout must keep the other settings, hostname = %q", saved.Hostname)
	}
	if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		c.Close()
		t.Fatal("the tailnet listener must be closed after logout")
	}
}

func TestManagerLogoutWhenOffIsHarmless(t *testing.T) {
	m, _ := testManager(t, newFakeNode("", Identity{}))
	if err := m.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := m.Status().State; st != StateOff {
		t.Fatalf("state = %q, want off", st)
	}
}

func TestManagerReportsRunningOnlyOnceHTTPSIsReady(t *testing.T) {
	node := newFakeNode("me@example.com", Identity{LoginName: "me@example.com"})
	node.warm = make(chan struct{})
	m, _ := testManager(t, node)
	if err := m.Apply(enabled(), ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // many polls while the certificate is pending
	if st := m.Status().State; st != StateStarting {
		t.Fatalf("state while the certificate is pending = %q, want starting", st)
	}
	close(node.warm)
	waitState(t, m, StateRunning)
	node.mu.Lock()
	warmed := node.warmed
	node.mu.Unlock()
	if !warmed {
		t.Fatal("running must follow a successful certificate fetch")
	}
	if code := get(t, <-node.addr, "/health"); code != 200 {
		t.Fatalf("/health after warm = %d, want 200", code)
	}
}
