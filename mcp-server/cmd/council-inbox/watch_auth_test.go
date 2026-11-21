package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWatchSendsTokenHeaderNotInURL(t *testing.T) {
	const tok = "test-token-0123456789abcdef"
	f := newFake()
	f.requireToken = tok
	cfg := fastCfg(f.serve(t))
	cfg.Token = tok
	cancel, done := startWatch(t, cfg)
	defer cancel()
	waitFor(t, "join", func() bool { return f.joins.Load() >= 1 })
	f.mu.Lock()
	if len(f.tokens) == 0 || f.tokens[0] != tok {
		t.Fatalf("X-Council-Token not sent: %q", f.tokens)
	}
	for _, u := range f.urls {
		if strings.Contains(u, tok) {
			t.Fatalf("token leaked into the URL: %s", u)
		}
	}
	f.mu.Unlock()
	cancel()
	waitExit(t, done, time.Second)
}

func TestWatchWithoutTokenSendsNoHeader(t *testing.T) {
	f := newFake()
	cancel, done := startWatch(t, fastCfg(f.serve(t)))
	defer cancel()
	waitFor(t, "join", func() bool { return f.joins.Load() >= 1 })
	f.mu.Lock()
	if f.tokens[0] != "" {
		t.Fatalf("header sent without a token: %q", f.tokens)
	}
	f.mu.Unlock()
	cancel()
	waitExit(t, done, time.Second)
}

func TestWatchAuthRejectionExitsTwoWithoutRetry(t *testing.T) {
	f := newFake()
	f.requireToken = "test-token-0123456789abcdef"
	cfg := fastCfg(f.serve(t))
	cfg.Token = "wrong-token-0123456789abc"
	var errBuf bytes.Buffer
	cfg.Stderr = &errBuf
	_, done := startWatch(t, cfg)
	if code := waitExit(t, done, 2*time.Second); code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "authentication failed: set COUNCIL_AUTH_TOKEN or --token-file") {
		t.Fatalf("stderr: %q", errBuf.String())
	}
	if strings.Contains(errBuf.String(), "wrong-token") {
		t.Fatalf("stderr leaks the token: %q", errBuf.String())
	}
	f.mu.Lock()
	n := len(f.tokens)
	f.mu.Unlock()
	if n != 1 {
		t.Fatalf("dialed %d times; retrying cannot help", n)
	}
}

func TestWatchFlagsReadTokenFromEnvAndFile(t *testing.T) {
	var e bytes.Buffer
	cfg, _, ok := parseWatchFlags(nil, func(k string) string {
		if k == "COUNCIL_AUTH_TOKEN" {
			return "test-token-0123456789abcdef"
		}
		return ""
	}, &e)
	if !ok || cfg.Token != "test-token-0123456789abcdef" {
		t.Fatalf("env token: ok=%v", ok)
	}
	p := writeTokenFile(t, "file-token-0123456789abcd\n", 0o600)
	cfg, _, ok = parseWatchFlags([]string{"--token-file", p}, func(string) string { return "" }, &e)
	if !ok || cfg.Token != "file-token-0123456789abcd" {
		t.Fatalf("file token: ok=%v", ok)
	}
}
