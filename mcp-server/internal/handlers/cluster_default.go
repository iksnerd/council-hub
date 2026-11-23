package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
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

// peersConnected is true when the cached node count says a peer exists. It is
// false for an unknown count; callers that can say so use clusterNodeCount.
func (r *Registry) peersConnected() bool {
	return r.clusterNodeCount().Peers()
}

// clusterReadError is the typed failure of a cluster read (the Phoenix call
// failed and there was nothing local to show). routeClusterRead falls back to
// the local path on any error, so no caller needs to look at message text.
type clusterReadError struct {
	op  string
	err error
}

func (e *clusterReadError) Error() string { return fmt.Sprintf("cluster %s failed: %s", e.op, e.err) }
func (e *clusterReadError) Unwrap() error { return e.err }

// clusterErr builds the error a cluster handler returns when the fan-out failed.
func clusterErr(op string, err error) error { return &clusterReadError{op: op, err: err} }

// routeClusterRead decides where a discovery-style read runs.
//
//	cluster_wide=true  -> cluster(), as before.
//	cluster_wide=false -> not handled: the caller runs its local path.
//	unset              -> cluster() when eligible, COUNCIL_CLUSTER_READS is not
//	                      "local" and a peer is connected; otherwise not handled.
//
// A default cluster-wide read never errors because of the fan-out: if cluster()
// returns an error (a *clusterReadError from the handlers; never judged from
// result text), local() runs and a warning line is appended. If the node count
// is unknown (the lookup failed), local() runs with a "cluster state unknown"
// line. local must force the local path (cluster_wide=false) so it cannot route
// back here.
func (r *Registry) routeClusterRead(cw StringBool, eligible bool, cluster, local readFn) (bool, *mcp.CallToolResult, ToolOutput, error) {
	switch cw {
	case "true":
		res, out, err := cluster()
		var ce *clusterReadError
		if errors.As(err, &ce) {
			// An explicit cluster read keeps its readable error result.
			res, out, err = textResult("Error: " + ce.Error())
		}
		return true, res, out, err
	case "false":
		return false, nil, ToolOutput{}, nil
	}
	if !eligible || clusterReadsLocal() {
		return false, nil, ToolOutput{}, nil
	}
	count := r.clusterNodeCount()
	if count.Unknown() {
		// Not "no peers": run locally and say the cluster could not be checked.
		res, out, err := local()
		if err == nil {
			res = annotateOnce(res, "**Cluster Warning:** "+clusterUnknownNote+".", clusterUnknownNote)
		}
		return true, res, out, err
	}
	if !count.Peers() {
		if len(count.missing) == 0 {
			return false, nil, ToolOutput{}, nil
		}
		// No peer is connected, but some are known: a clean local answer would
		// read as complete. Say which peers are missing.
		note := missingPeersNote(count.missing)
		res, out, err := local()
		if err == nil {
			res = annotateOnce(res, "**Cluster Warning:** "+note+".", note)
		}
		return true, res, out, err
	}
	res, out, err := cluster()
	if err != nil {
		reason := "cluster fan-out failed: " + err.Error()
		res, out, err = local()
		if err == nil {
			res = annotate(res, "**Cluster Warning:** "+reason+"; showing this node only.")
		}
		return true, res, out, err
	}
	return true, annotate(res, defaultWideNote), out, nil
}

// annotateOnce is annotate, skipped when the text already carries marker (the
// local path may have appended the same note itself).
func annotateOnce(res *mcp.CallToolResult, note, marker string) *mcp.CallToolResult {
	if strings.Contains(toolResultText(res), marker) {
		return res
	}
	return annotate(res, note)
}

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
