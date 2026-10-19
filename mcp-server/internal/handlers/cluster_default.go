package handlers

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// defaultWideNote is the line a read carries when it went cluster-wide without
// being asked to.
const defaultWideNote = "cluster-wide by default; pass cluster_wide=false for this node only"

type readFn func() (*mcp.CallToolResult, ToolOutput, error)

// clusterReadsLocal reports the operator switch COUNCIL_CLUSTER_READS=local,
// which restores opt-in cluster reads. Anything else means "auto".
func clusterReadsLocal() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("COUNCIL_CLUSTER_READS")), "local")
}

// peersConnected is true when the cached node count says a peer exists. The
// count includes this node.
func (r *Registry) peersConnected() bool {
	return r.clusterNodeCount() > 1
}

// routeClusterRead decides where a discovery-style read runs.
//
//	cluster_wide=true  -> cluster(), as before.
//	cluster_wide=false -> not handled: the caller runs its local path.
//	unset              -> cluster() when eligible, COUNCIL_CLUSTER_READS is not
//	                      "local" and a peer is connected; otherwise not handled.
//
// A default cluster-wide read never errors because of the fan-out: if cluster()
// fails, local() runs and a warning line is appended. local must force the
// local path (cluster_wide=false) so it cannot route back here.
func (r *Registry) routeClusterRead(cw StringBool, eligible bool, cluster, local readFn) (bool, *mcp.CallToolResult, ToolOutput, error) {
	switch cw {
	case "true":
		res, out, err := cluster()
		return true, res, out, err
	case "false":
		return false, nil, ToolOutput{}, nil
	}
	if !eligible || clusterReadsLocal() || !r.peersConnected() {
		return false, nil, ToolOutput{}, nil
	}
	res, out, err := cluster()
	if err != nil || strings.HasPrefix(resultTextOf(res), "Error: cluster") {
		reason := "cluster fan-out failed"
		if err != nil {
			reason += ": " + err.Error()
		}
		res, out, err = local()
		if err == nil {
			res = annotate(res, "**Cluster Warning:** "+reason+"; showing this node only.")
		}
		return true, res, out, err
	}
	return true, annotate(res, defaultWideNote), out, nil
}

func resultTextOf(res *mcp.CallToolResult) string { return toolResultText(res) }

// annotate adds a note to a text result. A JSON-object body (get_digest) gets a
// "note" field so it stays parseable; anything else gets an appended line.
func annotate(res *mcp.CallToolResult, note string) *mcp.CallToolResult {
	if res == nil || len(res.Content) == 0 {
		return res
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok || tc == nil {
		return res
	}
	var obj map[string]any
	if strings.HasPrefix(strings.TrimSpace(tc.Text), "{") && json.Unmarshal([]byte(tc.Text), &obj) == nil {
		if prev, _ := obj["note"].(string); prev != "" {
			note = prev + "; " + note
		}
		obj["note"] = note
		if b, err := json.MarshalIndent(obj, "", "  "); err == nil {
			tc.Text = string(b)
			return res
		}
	}
	tc.Text = strings.TrimRight(tc.Text, "\n") + "\n\n" + note + "\n"
	return res
}
