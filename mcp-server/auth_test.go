package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testAuthToken = "test-token-0123456789abcdef"

func TestAuthTokenFromEnv(t *testing.T) {
	if tok, err := authTokenFromEnv(""); tok != "" || err != nil {
		t.Fatalf("empty must mean off: %q %v", tok, err)
	}
	short := "short-secret-15c"[:15]
	_, err := authTokenFromEnv(short)
	if err == nil {
		t.Fatal("15 chars must be refused")
	}
	if !strings.Contains(err.Error(), "COUNCIL_AUTH_TOKEN") || !strings.Contains(err.Error(), "16") {
		t.Fatalf("error must name the variable and the minimum: %v", err)
	}
	if strings.Contains(err.Error(), short) {
		t.Fatalf("error leaks the value: %v", err)
	}
	if tok, err := authTokenFromEnv(strings.Repeat("a", 16)); err != nil || len(tok) != 16 {
		t.Fatalf("16 chars must be accepted: %v", err)
	}
}

// guardedMux mirrors the wiring in main: only /mcp and /mcp/ are guarded.
func guardedMux(token string, logger *slog.Logger) (*http.ServeMux, *int) {
	reached := 0
	mcp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
		_, _ = w.Write([]byte("mcp-ok"))
	})
	open := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("open")) })
	var entry http.Handler = mcp
	if token != "" {
		entry = requireBearer(token, newFailureLog(logger), mcp)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", entry)
	mux.Handle("/mcp/", entry)
	mux.Handle("/health", open)
	mux.Handle("/api/internal/post_to_room", open)
	mux.Handle("/api/ui/post", open)
	return mux, &reached
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)) }

func do(mux http.Handler, path, authz string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAuthOffIsOpen(t *testing.T) {
	mux, reached := guardedMux("", discardLogger())
	if rec := do(mux, "/mcp", ""); rec.Code != 200 || *reached != 1 {
		t.Fatalf("code=%d reached=%d", rec.Code, *reached)
	}
}

func TestAuthRejections(t *testing.T) {
	cases := map[string]string{
		"no header":    "",
		"wrong scheme": "Basic " + testAuthToken,
		"bare token":   testAuthToken,
		"wrong token":  "Bearer test-token-0123456789abcdeX",
		"empty bearer": "Bearer ",
		"longer token": "Bearer " + testAuthToken + "x",
	}
	for name, authz := range cases {
		for _, path := range []string{"/mcp", "/mcp/"} {
			mux, reached := guardedMux(testAuthToken, discardLogger())
			rec := do(mux, path, authz)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s: code=%d", name, path, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != `Bearer realm="council-hub"` {
				t.Errorf("%s %s: WWW-Authenticate=%q", name, path, got)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("%s %s: body must be empty, got %q", name, path, rec.Body.String())
			}
			if *reached != 0 {
				t.Errorf("%s %s: handler reached", name, path)
			}
		}
	}
}

func TestAuthRightTokenReachesHandler(t *testing.T) {
	mux, reached := guardedMux(testAuthToken, discardLogger())
	for _, path := range []string{"/mcp", "/mcp/"} {
		if rec := do(mux, path, "Bearer "+testAuthToken); rec.Code != 200 || rec.Body.String() != "mcp-ok" {
			t.Fatalf("%s: code=%d body=%q", path, rec.Code, rec.Body.String())
		}
	}
	if *reached != 2 {
		t.Fatalf("reached=%d", *reached)
	}
}

func TestAuthExemptPathsStayOpen(t *testing.T) {
	mux, _ := guardedMux(testAuthToken, discardLogger())
	for _, path := range []string{"/health", "/api/internal/post_to_room", "/api/ui/post"} {
		if rec := do(mux, path, ""); rec.Code != 200 {
			t.Errorf("%s: code=%d", path, rec.Code)
		}
	}
}

func TestAuthFailureLogRateLimitedAndRedacted(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fl := newFailureLog(logger)
	fl.now = func() time.Time { return now }
	mcp := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	h := requireBearer(testAuthToken, fl, mcp)
	presented := "Bearer presented-secret-value-xyz"
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", presented)
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	if n := strings.Count(buf.String(), "\n"); n != 1 {
		t.Fatalf("want 1 log line in the first minute, got %d:\n%s", n, buf.String())
	}
	now = now.Add(61 * time.Second)
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", presented)
	h.ServeHTTP(httptest.NewRecorder(), req)
	out := buf.String()
	if strings.Count(out, "\n") != 2 || !strings.Contains(out, "failed_attempts=50") {
		t.Fatalf("second line must carry the count of the suppressed attempts:\n%s", out)
	}
	if strings.Contains(out, "presented-secret-value-xyz") || strings.Contains(out, testAuthToken) {
		t.Fatalf("log leaks a token:\n%s", out)
	}
}

func healthBody(t *testing.T, authEnabled bool, cookieDefault bool, nodes int) map[string]any {
	t.Helper()
	phoenix := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var list []string
		for i := 0; i < nodes; i++ {
			list = append(list, fmt.Sprintf(`{"node": "council_hub@10.0.0.%d", "version": "0.66.1"}`, i+5))
		}
		_, _ = fmt.Fprintf(w, `{"nodes": [%s], "version_mismatch": false, "cookie_is_default": %t}`, strings.Join(list, ","), cookieDefault)
	}))
	defer phoenix.Close()
	cs := testServer(t)
	rec := httptest.NewRecorder()
	healthHandlerAuth(cs, phoenix.URL, phoenix.Client(), authEnabled)(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestHealthAuthEnabledFlag(t *testing.T) {
	if b := healthBody(t, true, false, 1); b["auth_enabled"] != true {
		t.Fatalf("auth_enabled: %+v", b)
	}
	if b := healthBody(t, false, false, 1); b["auth_enabled"] != false {
		t.Fatalf("auth_enabled: %+v", b)
	}
}

func TestHealthAuthWarningOnlyUnderStatedConditions(t *testing.T) {
	cases := []struct {
		name         string
		auth, cookie bool
		nodes        int
		wantWarning  bool
	}{
		{"all three", true, true, 2, true},
		{"auth off", false, true, 2, false},
		{"custom cookie", true, false, 2, false},
		{"single node", true, true, 1, false},
	}
	for _, c := range cases {
		b := healthBody(t, c.auth, c.cookie, c.nodes)
		_, has := b["auth_warning"]
		if has != c.wantWarning {
			t.Errorf("%s: auth_warning present=%v", c.name, has)
		}
		if has && b["auth_warning"] != authWarningDefaultCookie {
			t.Errorf("%s: wrong warning text %v", c.name, b["auth_warning"])
		}
	}
}
