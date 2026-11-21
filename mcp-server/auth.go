package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// minAuthTokenLen is the shortest COUNCIL_AUTH_TOKEN the server accepts.
const minAuthTokenLen = 16

// authWarningDefaultCookie is reported on /health when the MCP token is set but
// the cluster endpoints it cannot cover are still guarded only by the public
// default cookie.
const authWarningDefaultCookie = "COUNCIL_AUTH_TOKEN does not protect /api/internal/* while the cluster cookie is the public default; set RELEASE_COOKIE to the same non-default value on every node."

// authTokenFromEnv validates the raw COUNCIL_AUTH_TOKEN value. Empty means
// auth is off. A set but short value is an error: the server must fail closed
// rather than run open. The error never contains the value.
func authTokenFromEnv(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if len(raw) < minAuthTokenLen {
		return "", fmt.Errorf("COUNCIL_AUTH_TOKEN is set but shorter than the minimum of %d characters; set a longer token or unset the variable to turn auth off", minAuthTokenLen)
	}
	return raw, nil
}

// failureLog logs failed auth attempts at most once per interval, with the
// number of failures since the last line. It never sees the presented value.
type failureLog struct {
	mu     sync.Mutex
	logger *slog.Logger
	every  time.Duration
	now    func() time.Time
	last   time.Time
	count  int
}

func (f *failureLog) record() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	t := f.now()
	if !f.last.IsZero() && t.Sub(f.last) < f.every {
		return
	}
	f.last = t
	n := f.count
	f.count = 0
	f.logger.Warn("rejected unauthenticated /mcp request", "failed_attempts", n)
}

// unauthorizedBody is the fixed 401 body for /mcp.
const unauthorizedBody = `{"error":"unauthorized","hint":"send the header Authorization: Bearer <token> (ask the operator for COUNCIL_AUTH_TOKEN); there is nothing to sign in to"}`

// requireBearer guards next with an `Authorization: Bearer <token>` check.
// Both sides are hashed with SHA-256 before a constant-time compare, so the
// token length does not leak. A failure is a 401 with a fixed JSON hint that
// does not say which part was wrong and never echoes the presented value.
func requireBearer(token string, fl *failureLog, next http.Handler) http.Handler {
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const scheme = "Bearer "
		h := r.Header.Get("Authorization")
		ok := false
		if len(h) >= len(scheme) && strings.EqualFold(h[:len(scheme)], scheme) {
			got := sha256.Sum256([]byte(h[len(scheme):]))
			ok = subtle.ConstantTimeCompare(got[:], want[:]) == 1
		}
		if !ok {
			fl.record()
			w.Header().Set("WWW-Authenticate", `Bearer realm="council-hub"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(unauthorizedBody))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newFailureLog(logger *slog.Logger) *failureLog {
	return &failureLog{logger: logger, every: time.Minute, now: time.Now}
}
