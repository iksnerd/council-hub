package handlers

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"council-hub/internal/council"
	"council-hub/internal/tailnet"
)

func newTailscaleHandler(t *testing.T) (http.HandlerFunc, *tailnet.Manager) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cs, err := council.NewServer(filepath.Join(t.TempDir(), "council.db"), log)
	if err != nil {
		t.Fatal(err)
	}
	reg := &Registry{Server: cs}
	store := tailnet.Store{Path: filepath.Join(t.TempDir(), "tailscale.json")}
	// A nil factory is fine: every request here keeps Enabled false, so no node starts.
	m := tailnet.NewManager(store, nil, http.NewServeMux(), log)
	return reg.UITailscaleHandler(m), m
}

func call(h http.HandlerFunc, method, remote, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/ui/tailscale", bytes.NewBufferString(body))
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestUITailscaleRefusesNonLoopback(t *testing.T) {
	h, _ := newTailscaleHandler(t)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		// A tailnet caller is not loopback.
		if rec := call(h, method, "192.0.2.1:5555", `{}`); rec.Code != http.StatusForbidden {
			t.Fatalf("%s from a tailnet address = %d, want 403", method, rec.Code)
		}
	}
}

func TestUITailscaleGetAndApply(t *testing.T) {
	h, m := newTailscaleHandler(t)

	rec := call(h, http.MethodGet, "127.0.0.1:1234", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"state":"off"`) {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}

	rec = call(h, http.MethodPost, "[::1]:1234", `{"enabled":false,"hostname":"hub-two","access":"members"}`)
	if rec.Code != 200 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	if c := m.Status().Config; c.Hostname != "hub-two" || c.Access != tailnet.AccessMembers {
		t.Fatalf("config not applied: %+v", c)
	}

	rec = call(h, http.MethodPost, "127.0.0.1:1234", `{"hostname":"NO","access":"owner"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid config = %d, want 400", rec.Code)
	}
	rec = call(h, http.MethodPost, "127.0.0.1:1234", `not json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", rec.Code)
	}
	rec = call(h, http.MethodDelete, "127.0.0.1:1234", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE = %d, want 405", rec.Code)
	}
}

func TestUITailscaleNeverEchoesAuthKey(t *testing.T) {
	h, _ := newTailscaleHandler(t)
	rec := call(h, http.MethodPost, "127.0.0.1:1", `{"enabled":false,"hostname":"x","access":"owner","auth_key":"tskey-auth-SECRET"}`)
	if strings.Contains(rec.Body.String(), "SECRET") {
		t.Fatalf("response leaked the auth key: %s", rec.Body)
	}
}

func TestUITailscaleOmittedFieldsKeepSavedValues(t *testing.T) {
	h, m := newTailscaleHandler(t)

	// First run: a bare request uses the defaults.
	if rec := call(h, http.MethodPost, "127.0.0.1:1", `{"enabled":false}`); rec.Code != 200 {
		t.Fatalf("bare POST = %d %s", rec.Code, rec.Body)
	}
	if c := m.Status().Config; c.Hostname != "council-hub" || c.Access != tailnet.AccessOwner {
		t.Fatalf("defaults not used: %+v", c)
	}

	// Save some settings, then a bare request must not reset them.
	call(h, http.MethodPost, "127.0.0.1:1", `{"enabled":false,"hostname":"hub-two","access":"members","tags":["tag:ci"],"control_url":"https://hs.example.net"}`)
	call(h, http.MethodPost, "127.0.0.1:1", `{"enabled":false}`)
	c := m.Status().Config
	if c.Hostname != "hub-two" || c.Access != tailnet.AccessMembers || len(c.Tags) != 1 || c.ControlURL != "https://hs.example.net" {
		t.Fatalf("a bare request reset saved settings: %+v", c)
	}

	// An explicit empty value does clear it.
	call(h, http.MethodPost, "127.0.0.1:1", `{"enabled":false,"control_url":"","tags":[],"access":"owner"}`)
	c = m.Status().Config
	if c.ControlURL != "" || len(c.Tags) != 0 {
		t.Fatalf("explicit empty must clear: %+v", c)
	}
}

func TestUITailscaleLogoutAndUnknownAction(t *testing.T) {
	h, m := newTailscaleHandler(t)
	if rec := call(h, http.MethodPost, "127.0.0.1:1", `{"action":"logout"}`); rec.Code != 200 {
		t.Fatalf("logout = %d %s", rec.Code, rec.Body)
	}
	if st := m.Status().State; st != tailnet.StateOff {
		t.Fatalf("state after logout = %q", st)
	}
	if rec := call(h, http.MethodPost, "127.0.0.1:1", `{"action":"explode"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown action = %d, want 400", rec.Code)
	}
}
