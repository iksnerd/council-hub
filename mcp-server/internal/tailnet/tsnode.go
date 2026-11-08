package tailnet

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"tailscale.com/client/local"
	"tailscale.com/tsnet"
)

// NewTSNetFactory returns a factory that builds real tsnet nodes whose state
// (node key, certificates) lives in stateDir, so a restart rejoins without a
// new sign-in.
func NewTSNetFactory(stateDir string, log *slog.Logger) NodeFactory {
	return func(cfg Config, authKey string) Node {
		return &tsNode{srv: &tsnet.Server{
			Dir:           stateDir,
			Hostname:      cfg.Hostname,
			AuthKey:       authKey,
			ControlURL:    cfg.ControlURL,
			AdvertiseTags: cfg.Tags,
			// tsnet's own logs are chatty and include the sign-in URL; keep the
			// user-facing ones only, and route them through slog.
			Logf: func(string, ...any) {},
			UserLogf: func(f string, a ...any) {
				msg := fmt.Sprintf(f, a...)
				// The sign-in link is repeated every few seconds while waiting. The UI
				// shows it, and it is a one-time login URL, so keep it out of the log.
				if strings.Contains(msg, "go to:") {
					return
				}
				log.Info("tailscale: " + msg)
			},
		}}
	}
}

type tsNode struct {
	srv *tsnet.Server
	lc  *local.Client
}

func (n *tsNode) Start() error {
	if err := n.srv.Start(); err != nil {
		return err
	}
	lc, err := n.srv.LocalClient()
	if err != nil {
		return err
	}
	n.lc = lc
	return nil
}

func (n *tsNode) Status(ctx context.Context) (NodeStatus, error) {
	st, err := n.lc.Status(ctx)
	if err != nil {
		return NodeStatus{}, err
	}
	out := NodeStatus{AuthURL: st.AuthURL}
	if st.BackendState != "Running" || st.Self == nil {
		return out, nil
	}
	out.Running = true
	out.DNSName = strings.TrimSuffix(st.Self.DNSName, ".")
	// A tagged device has no owner; leave Owner empty so owner mode admits nobody.
	if st.Self.Tags == nil || st.Self.Tags.Len() == 0 {
		if u, ok := st.User[st.Self.UserID]; ok {
			out.Owner = u.LoginName
		}
	}
	return out, nil
}

func (n *tsNode) Identify(ctx context.Context, remoteAddr string) (Identity, error) {
	who, err := n.lc.WhoIs(ctx, remoteAddr)
	if err != nil {
		return Identity{}, err
	}
	if who.Node != nil && who.Node.IsTagged() {
		return Identity{Tagged: true}, nil
	}
	if who.UserProfile == nil {
		return Identity{}, nil
	}
	return Identity{LoginName: who.UserProfile.LoginName}, nil
}

func (n *tsNode) ListenTLS(addr string) (net.Listener, error) {
	st, err := n.lc.Status(context.Background())
	if err == nil && len(st.CertDomains) == 0 {
		return nil, errors.New("HTTPS certificates are off for this tailnet: turn on MagicDNS and HTTPS certificates in the Tailscale admin console")
	}
	return n.srv.ListenTLS("tcp", addr)
}

func (n *tsNode) WarmCert(ctx context.Context, domain string) error {
	_, _, err := n.lc.CertPair(ctx, domain)
	return err
}

func (n *tsNode) Logout(ctx context.Context) error { return n.lc.Logout(ctx) }

func (n *tsNode) Close() error { return n.srv.Close() }
