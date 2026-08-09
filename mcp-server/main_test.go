package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"council-hub/internal/council"
)

func testServer(t *testing.T) *council.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	s, err := council.NewServer(":memory:", logger)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}
	t.Cleanup(func() { s.DB.Close() })
	return s
}

func TestClusterNodesDecodesNodeIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"nodes": [{"node": "council_hub@10.0.0.5", "version": "0.52.0"}],
			"count": 1,
			"version_mismatch": false,
			"node_identity": {"registered": "10.0.0.4", "current": "10.0.0.5", "drifted?": true}
		}`))
	}))
	defer srv.Close()

	result := clusterNodes(srv.URL, srv.Client())
	if result == nil {
		t.Fatal("expected a non-nil result")
	}
	if result.NodeIdentity == nil {
		t.Fatal("expected NodeIdentity to be decoded")
	}
	if !result.NodeIdentity.Drifted || result.NodeIdentity.Registered != "10.0.0.4" || result.NodeIdentity.Current != "10.0.0.5" {
		t.Fatalf("unexpected NodeIdentity: %+v", result.NodeIdentity)
	}
}

func TestHealthHandlerSurfacesNodeIdentityDrift(t *testing.T) {
	phoenix := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"nodes": [{"node": "council_hub@10.0.0.5", "version": "0.52.0"}],
			"count": 1,
			"version_mismatch": false,
			"node_identity": {"registered": "10.0.0.4", "current": "10.0.0.5", "drifted?": true}
		}`))
	}))
	defer phoenix.Close()

	cs := testServer(t)
	handler := healthHandler(cs, phoenix.URL, phoenix.Client())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	warning, ok := body["node_identity_warning"].(string)
	if !ok || warning == "" {
		t.Fatalf("expected node_identity_warning to be set, got body: %+v", body)
	}
	if _, ok := body["node_identity"]; !ok {
		t.Fatal("expected node_identity to be present in the response body")
	}
}

func TestHealthHandlerOmitsNodeIdentityWhenNotDrifted(t *testing.T) {
	phoenix := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"nodes": [{"node": "nonode@nohost", "version": "0.52.0"}],
			"count": 1,
			"version_mismatch": false,
			"node_identity": {"registered": null, "current": null, "drifted?": false}
		}`))
	}))
	defer phoenix.Close()

	cs := testServer(t)
	handler := healthHandler(cs, phoenix.URL, phoenix.Client())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := body["node_identity_warning"]; ok {
		t.Fatalf("did not expect node_identity_warning in body: %+v", body)
	}
	if _, ok := body["node_identity"]; ok {
		t.Fatalf("did not expect node_identity in body: %+v", body)
	}
}

func TestHealthHandlerWithoutPhoenix(t *testing.T) {
	cs := testServer(t)
	handler := healthHandler(cs, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %+v", body)
	}
	if _, ok := body["node_identity"]; ok {
		t.Fatal("did not expect node_identity when Phoenix is unreachable")
	}
}
