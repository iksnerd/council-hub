package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// missingServer answers /api/internal/cluster/nodes with `nodes` connected
// nodes and the given known-but-not-connected peers.
func missingServer(t *testing.T, nodes int, missing []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ns := make([]string, nodes)
		for i := range ns {
			ns[i] = "n"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"nodes": ns, "missing_peers": missing})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func missingRegistry(srv *httptest.Server) *Registry {
	return &Registry{HTTPClient: &http.Client{Timeout: 2 * time.Second}, PhoenixURL: srv.URL}
}

const gonePeer = "council_hub@10.0.0.9"

// A peer the cluster manager keeps alive but that is not connected right now is
// a reason the answer may be incomplete: a clean local result must say so.
func TestLocalOnlyNoteNamesKnownPeersThatAreNotConnected(t *testing.T) {
	reg := missingRegistry(missingServer(t, 1, []string{gonePeer}))
	got := reg.localOnlyNote()
	if !strings.Contains(got, gonePeer) || !strings.Contains(got, "not connected") {
		t.Errorf("localOnlyNote = %q, want it to name %s as not connected", got, gonePeer)
	}
}

func TestRouteClusterReadWarnsWhenAKnownPeerIsNotConnected(t *testing.T) {
	reg := missingRegistry(missingServer(t, 1, []string{gonePeer}))
	clusterCalled := false
	ok, res, _, err := reg.routeClusterRead("", true,
		func() (*mcp.CallToolResult, ToolOutput, error) { clusterCalled = true; return textResult("wide") },
		func() (*mcp.CallToolResult, ToolOutput, error) { return textResult("local rows") })
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, want a handled local answer", ok, err)
	}
	if clusterCalled {
		t.Error("the cluster path must not run with no peer connected")
	}
	text := resultText(res)
	if !strings.Contains(text, "local rows") || !strings.Contains(text, gonePeer) || !strings.Contains(text, "not connected") {
		t.Errorf("got %q", text)
	}
}

// A genuine single node (no known peers) stays silent and local, as before.
func TestRouteClusterReadSilentForASingleNodeWithNoKnownPeers(t *testing.T) {
	reg := missingRegistry(missingServer(t, 1, nil))
	ok, _, _, _ := reg.routeClusterRead("", true,
		func() (*mcp.CallToolResult, ToolOutput, error) { return textResult("wide") },
		func() (*mcp.CallToolResult, ToolOutput, error) { return textResult("local") })
	if ok {
		t.Error("a single node with no known peers must not be handled by the cluster router")
	}
	if got := reg.localOnlyNote(); got != "" {
		t.Errorf("note = %q, want empty", got)
	}
}

// A payload from a Phoenix that predates missing_peers reads as none missing.
func TestMissingPeersAbsentFromOlderPhoenixReadsAsNone(t *testing.T) {
	reg := missingRegistry(nodesServer(t, http.StatusOK, 1))
	if got := reg.localOnlyNote(); got != "" {
		t.Errorf("note = %q, want empty", got)
	}
}
