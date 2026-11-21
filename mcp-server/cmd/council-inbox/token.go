package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

const authFailedMsg = "authentication failed: set COUNCIL_AUTH_TOKEN or --token-file"

// loadToken returns the hub token. --token-file wins over the environment.
// There is deliberately no --token VALUE flag: it would show in `ps`. A token
// file readable by group or others only earns a warning on stderr. An empty
// result means the hub is assumed to run without auth.
func loadToken(getenv func(string) string, tokenFile string, stderr io.Writer) (string, error) {
	if tokenFile == "" {
		return strings.TrimSpace(getenv("COUNCIL_AUTH_TOKEN")), nil
	}
	if fi, err := os.Stat(tokenFile); err == nil && fi.Mode().Perm()&0o077 != 0 {
		_, _ = fmt.Fprintf(stderr, "council-inbox: warning: %s is readable by group or others (mode %04o); chmod 600 it\n", tokenFile, fi.Mode().Perm())
	}
	b, err := os.ReadFile(tokenFile)
	if err != nil {
		// Report the path and the OS reason, never file content.
		return "", fmt.Errorf("cannot read --token-file: %w", err)
	}
	first, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(first), nil
}

// authTransport adds the bearer token to every request and notes a 401 so the
// caller can tell an auth rejection from any other failure, whatever shape the
// SDK wraps the error in.
type authTransport struct {
	base     http.RoundTripper
	token    string
	rejected atomic.Bool
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err == nil && resp.StatusCode == http.StatusUnauthorized {
		t.rejected.Store(true)
	}
	return resp, err
}
