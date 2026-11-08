package tailnet

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Node is the part of a tsnet server the Manager needs. The real one is in
// tsnode.go; tests use a fake so no network or Tailscale account is involved.
type Node interface {
	// Start begins joining the tailnet and returns without waiting for sign-in.
	Start() error
	// Status reports the backend state, the sign-in URL (when one is pending),
	// this device's DNS name, and its owner's login name.
	Status(ctx context.Context) (NodeStatus, error)
	// Logout signs the device out of Tailscale: its saved login is removed and
	// its key expires. The device stays in the tailnet's device list (offline)
	// until someone deletes it in the Tailscale admin console.
	Logout(ctx context.Context) error
	// Identify names the Tailscale user or device behind a connection.
	Identify(ctx context.Context, remoteAddr string) (Identity, error)
	// ListenTLS listens on the tailnet with a Tailscale-issued certificate.
	ListenTLS(addr string) (net.Listener, error)
	// WarmCert fetches the HTTPS certificate for domain now. The first request
	// would otherwise wait for issuance and fail while the card already said
	// "running".
	WarmCert(ctx context.Context, domain string) error
	Close() error
}

// NodeStatus is a snapshot of the node.
type NodeStatus struct {
	Running bool   // joined the tailnet and ready to serve
	AuthURL string // sign-in link, set while the node waits for the operator
	DNSName string // e.g. council-hub.tailnet-name.ts.net (no trailing dot)
	Owner   string // login name of the user who owns this device; empty if tagged
}

// NodeFactory builds a node for a config. authKey is used once for a headless
// sign-in and is never saved.
type NodeFactory func(cfg Config, authKey string) Node

// State is what the UI shows.
type State string

const (
	StateOff        State = "off"
	StateStarting   State = "starting"
	StateNeedsLogin State = "needs-login"
	StateRunning    State = "running"
	StateError      State = "error"
)

// View is the Manager's status for the UI.
type View struct {
	Config  Config `json:"config"`
	State   State  `json:"state"`
	AuthURL string `json:"auth_url,omitempty"`
	URL     string `json:"url,omitempty"`
	Owner   string `json:"owner,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Manager owns the config and at most one running node.
type Manager struct {
	store   Store
	factory NodeFactory
	handler http.Handler // served on the tailnet; must not include localhost-only routes
	log     *slog.Logger
	poll    time.Duration

	mu      sync.Mutex // guards cfg, cancel, done, loadErr; held across a stop
	cfg     Config
	cancel  context.CancelFunc
	done    chan struct{}
	node    Node
	loadErr error

	viewMu sync.Mutex // guards view only, so the node goroutine never waits on a stop
	view   View
}

// NewManager loads the saved config. A config that cannot be read is kept as an
// error shown in the UI and the node stays off; it is never silently replaced.
func NewManager(store Store, factory NodeFactory, handler http.Handler, log *slog.Logger) *Manager {
	m := &Manager{store: store, factory: factory, handler: handler, log: log, poll: time.Second}
	cfg, err := store.Load()
	m.cfg, m.loadErr = cfg, err
	if err != nil {
		m.cfg = DefaultConfig()
		m.set(View{Config: m.cfg, State: StateError, Error: err.Error()})
		return m
	}
	m.set(View{Config: cfg, State: StateOff})
	return m
}

// Start brings the node up when the saved config says enabled.
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadErr == nil && m.cfg.Enabled {
		m.startLocked("")
	}
}

// Apply validates, saves and applies a new config, restarting the node when
// it is enabled. authKey may be empty (browser sign-in).
func (m *Manager) Apply(cfg Config, authKey string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.Save(cfg); err != nil {
		return err
	}
	m.loadErr = nil
	m.stopLocked()
	m.cfg = cfg
	m.set(View{Config: cfg, State: StateOff})
	if cfg.Enabled {
		m.startLocked(authKey)
	}
	return nil
}

// Logout signs the device out of Tailscale (the saved login is gone and the next
// start needs a fresh sign-in; the device stays listed, offline, in the tailnet's
// admin console until deleted there), stops it, and saves the config as disabled.
// A failed sign-out is logged and the node is stopped anyway: the operator asked
// for it to be off.
func (m *Manager) Logout(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.node != nil {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := m.node.Logout(c); err != nil {
			m.log.Warn("tailscale: sign-out did not complete", "error", err)
		}
		cancel()
	}
	m.stopLocked()
	cfg := m.cfg
	cfg.Enabled = false
	if err := m.store.Save(cfg); err != nil {
		return err
	}
	m.cfg = cfg
	m.set(View{Config: cfg, State: StateOff})
	return nil
}

// Status returns the current view.
func (m *Manager) Status() View {
	m.viewMu.Lock()
	defer m.viewMu.Unlock()
	return m.view
}

// Stop shuts the node down (server exit).
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	if m.cancel == nil {
		return
	}
	m.cancel()
	<-m.done
	m.cancel, m.done, m.node = nil, nil, nil
}

func (m *Manager) startLocked(authKey string) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	node := m.factory(m.cfg, authKey)
	m.cancel, m.done, m.node = cancel, done, node
	m.set(View{Config: m.cfg, State: StateStarting})
	go func() {
		defer close(done)
		m.run(ctx, node, m.cfg)
	}()
}

func (m *Manager) set(v View) {
	m.viewMu.Lock()
	m.view = v
	m.viewMu.Unlock()
}

func (m *Manager) fail(cfg Config, err error) {
	m.log.Error("tailscale", "error", err)
	m.set(View{Config: cfg, State: StateError, Error: err.Error()})
}

func (m *Manager) run(ctx context.Context, node Node, cfg Config) {
	defer node.Close()
	if err := node.Start(); err != nil {
		m.fail(cfg, err)
		return
	}

	var st NodeStatus
	for {
		var err error
		st, err = node.Status(ctx)
		if ctx.Err() != nil {
			return
		}
		switch {
		case err != nil:
			m.fail(cfg, err)
			return
		case st.Running:
		case st.AuthURL != "":
			m.set(View{Config: cfg, State: StateNeedsLogin, AuthURL: st.AuthURL})
		default:
			m.set(View{Config: cfg, State: StateStarting})
		}
		if st.Running {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.poll):
		}
	}

	ln, err := node.ListenTLS(":443")
	if err != nil {
		m.fail(cfg, err)
		return
	}
	// Report running only once HTTPS answers: first-time certificate issuance
	// takes a few seconds, and a connection command copied before then fails.
	m.set(View{Config: cfg, State: StateStarting})
	if err := m.warmCert(ctx, node, st.DNSName); err != nil {
		if ctx.Err() == nil {
			m.fail(cfg, err)
		}
		ln.Close()
		return
	}
	srv := &http.Server{
		Handler:           m.gate(node, cfg.Access, st.Owner),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	m.set(View{Config: cfg, State: StateRunning, URL: "https://" + st.DNSName, Owner: st.Owner})
	m.log.Info("tailscale: serving", "url", "https://"+st.DNSName, "access", cfg.Access)

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) && ctx.Err() == nil {
		m.fail(cfg, err)
	}
}

// gate admits a request only when Tailscale names the caller and the access
// mode lets them in. It is the only thing between the tailnet and /mcp.
func (m *Manager) gate(node Node, mode Access, owner string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := node.Identify(r.Context(), r.RemoteAddr)
		if err != nil || !Allowed(mode, owner, id) {
			if err != nil {
				m.log.Warn("tailscale: could not identify caller", "remote", r.RemoteAddr, "error", err)
			}
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		m.handler.ServeHTTP(w, r)
	})
}

// warmCert retries until the certificate is issued, for up to two minutes.
func (m *Manager) warmCert(ctx context.Context, node Node, domain string) error {
	deadline := time.Now().Add(2 * time.Minute)
	for {
		err := node.WarmCert(ctx, domain)
		if err == nil || ctx.Err() != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("HTTPS certificate for %s was not issued: %w", domain, err)
		}
		m.log.Info("tailscale: waiting for HTTPS certificate", "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.poll):
		}
	}
}
