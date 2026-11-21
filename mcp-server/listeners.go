package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"council-hub/internal/handlers"
)

// validateListenAddrs refuses a trusted address that equals the main one. An
// empty trusted address means no trusted listener.
func validateListenAddrs(mainAddr, trustedAddr string) error {
	if trustedAddr != "" && strings.TrimSpace(trustedAddr) == strings.TrimSpace(mainAddr) {
		return fmt.Errorf("COUNCIL_TRUSTED_ADDR (%s) must differ from COUNCIL_HTTP_ADDR: the trusted listener serves /mcp without a token, the main one enforces it", trustedAddr)
	}
	return nil
}

// buildMux wires every HTTP route. mcpEntry is the only part that differs
// between listeners: the main listener passes the bearer-guarded handler, the
// trusted one the bare MCP handler. Everything else (including the /api/ui/*
// loopback check and the /api/internal/* cookie check, which live inside the
// handlers) is identical on both, and shares one Registry.
func buildMux(mcpEntry http.Handler, health http.HandlerFunc, reg *handlers.Registry) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpEntry)
	mux.Handle("/mcp/", mcpEntry)
	mux.HandleFunc("/health", health)
	// Cross-node write receiver (authenticated by the shared cluster secret).
	mux.HandleFunc("/api/internal/post_to_room", reg.InternalPostHandler())
	mux.HandleFunc("/api/internal/signal_status", reg.InternalStatusHandler())
	mux.HandleFunc("/api/internal/pin_message", reg.InternalPinHandler())
	// UI write endpoints (localhost-only, no auth — for the Phoenix dashboard
	// compose box and the notebook "pin into outline" button).
	mux.HandleFunc("/api/ui/post", reg.UIPostHandler())
	mux.HandleFunc("/api/ui/notebook_entry", reg.UINotebookEntryHandler())
	mux.HandleFunc("/api/ui/backfill_embeddings", reg.UIBackfillEmbeddingsHandler())
	return mux
}

func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// bindAll opens every server's listener before any of them serves, so a bind
// failure on one (the trusted address in particular) stops startup instead of
// leaving the process half-configured. Already-opened listeners are closed.
func bindAll(servers []*http.Server) ([]net.Listener, error) {
	var lns []net.Listener
	for _, s := range servers {
		ln, err := net.Listen("tcp", s.Addr)
		if err != nil {
			for _, l := range lns {
				_ = l.Close()
			}
			return nil, fmt.Errorf("listen on %s: %w", s.Addr, err)
		}
		lns = append(lns, ln)
	}
	return lns, nil
}

// serveAll serves each server on its listener until ctx is done, then shuts
// all of them down gracefully. It returns the first serve error (not a clean
// close); one failing server stops the others.
func serveAll(ctx context.Context, logger *slog.Logger, servers []*http.Server, lns []net.Listener) error {
	errc := make(chan error, len(servers))
	for i, s := range servers {
		go func(s *http.Server, ln net.Listener) {
			if err := s.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- err
				return
			}
			errc <- nil
		}(s, lns[i])
	}
	var first error
	stopping := false
	shutdown := func() {
		if stopping {
			return
		}
		stopping = true
		logger.Info("Shutting down HTTP server")
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, s := range servers {
			if err := s.Shutdown(sctx); err != nil {
				logger.Error("HTTP server shutdown error", "error", err)
			}
		}
	}
	done := ctx.Done()
	for n := 0; n < len(servers); {
		select {
		case <-done:
			shutdown()
			done = nil // keep draining errc
		case err := <-errc:
			n++
			if err != nil && first == nil {
				first = err
				shutdown()
			}
		}
	}
	return first
}
