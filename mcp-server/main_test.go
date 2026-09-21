package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"council-hub/internal/council"
)

// stubEmbedder returns a fixed vector for any input — enough to make
// cs.Embedder non-nil for /health's embedding_coverage passthrough.
type stubEmbedder struct{}

func (stubEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, council.EmbedDim), nil
}

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

func TestHealthHandlerOmitsEmbeddingCoverageWithoutEmbedder(t *testing.T) {
	cs := testServer(t)
	handler := healthHandler(cs, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := body["embedding_coverage"]; ok {
		t.Fatal("did not expect embedding_coverage when no embedder is configured")
	}
}

func TestHealthHandlerIncludesEmbeddingCoverage(t *testing.T) {
	cs := testServer(t)
	cs.Embedder = &stubEmbedder{}
	handler := healthHandler(cs, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	coverage, ok := body["embedding_coverage"].(map[string]any)
	if !ok {
		t.Fatalf("expected embedding_coverage to be present, got body: %+v", body)
	}
	if coverage["messages"] != "0/0" || coverage["rooms"] != "0/0" {
		t.Fatalf("expected 0/0 coverage on an empty DB, got: %+v", coverage)
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

func TestHealthHandlerSurfacesSeedWarning(t *testing.T) {
	phoenix := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"nodes": [{"node": "council_hub@10.0.0.6", "version": "0.58.4"}],
			"count": 1,
			"version_mismatch": false,
			"seed_status": {
				"warning": "no cluster peers connected — seed 10.0.0.4 is up but reports node peer@10.0.0.5, an address it does not hold",
				"findings": [{"seed": "10.0.0.4", "host": "10.0.0.4", "reported_node": "peer@10.0.0.5", "status": "stale_name", "message": "seed 10.0.0.4 is up but reports node peer@10.0.0.5"}]
			}
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

	warning, ok := body["seed_warning"].(string)
	if !ok || !strings.Contains(warning, "peer@10.0.0.5") {
		t.Fatalf("expected seed_warning naming the stale peer, got body: %+v", body)
	}
	if _, ok := body["seed_status"]; !ok {
		t.Fatal("expected seed_status to be present in the response body")
	}
}

func TestHealthHandlerOmitsSeedWarningWhenClusterHealthy(t *testing.T) {
	phoenix := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"nodes": [{"node": "council_hub@10.0.0.6", "version": "0.58.4"}],
			"count": 1,
			"version_mismatch": false,
			"seed_status": {"warning": null, "findings": []}
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

	if _, ok := body["seed_warning"]; ok {
		t.Fatalf("did not expect seed_warning in body: %+v", body)
	}
	if _, ok := body["seed_status"]; ok {
		t.Fatalf("did not expect seed_status in body: %+v", body)
	}
}

func TestHealthHandlerSurfacesAdvertisedWarning(t *testing.T) {
	phoenix := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"nodes": [{"node": "council_hub@192.168.0.10", "version": "0.58.4"}],
			"count": 1,
			"version_mismatch": false,
			"advertised": {
				"host": "192.168.0.10",
				"port": 4369,
				"checkable?": true,
				"reachable?": false,
				"warning": "nothing answers on 192.168.0.10:4369, the address this node advertises to peers"
			}
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

	warning, ok := body["advertised_warning"].(string)
	if !ok || !strings.Contains(warning, "192.168.0.10:4369") {
		t.Fatalf("expected advertised_warning naming the dead address, got body: %+v", body)
	}
	if _, ok := body["advertised"]; !ok {
		t.Fatal("expected advertised to be present in the response body")
	}
}

func TestHealthHandlerOmitsAdvertisedWhenReachable(t *testing.T) {
	phoenix := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"nodes": [{"node": "council_hub@192.168.0.6", "version": "0.58.4"}],
			"count": 1,
			"version_mismatch": false,
			"advertised": {"host": "192.168.0.6", "port": 4369, "checkable?": true, "reachable?": true, "warning": ""}
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

	if _, ok := body["advertised_warning"]; ok {
		t.Fatalf("did not expect advertised_warning in body: %+v", body)
	}
}
