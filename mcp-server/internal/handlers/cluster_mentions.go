package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ClusterMentionResult is one mentioning message returned by a peer's
// /api/internal/cluster/get_mentions fan-out.
type ClusterMentionResult struct {
	ID         string `json:"id"`
	RoomID     string `json:"room_id"`
	Author     string `json:"author"`
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp"`
	SourceNode string `json:"source_node"`
}

// nodeCountTTL bounds how often a read may cost a Phoenix round trip to learn
// whether peers are connected.
const nodeCountTTL = 10 * time.Second

// nodeCountFailTTL is how long a failed lookup (timeout, error, bad status) is
// remembered, as "unknown" and never as "no peers": every read in the window
// says so. It is longer than nodeCountTTL so a hung Phoenix (its ClusterManager
// stuck dialing a dead peer) costs one short stall per window rather than one
// per 10 s.
var nodeCountFailTTL = 30 * time.Second

// nodeProbeTimeout caps the peer-count lookup on the read path. It is separate
// from HTTPClient's timeout, which sizes the multicall fan-out (5 s).
var nodeProbeTimeout = 1 * time.Second

// nodeCount is the answer to "how many nodes are in the cluster". Unknown is a
// state of its own: a failed lookup is not evidence of a single node.
type nodeCount struct {
	n       int
	unknown bool
}

// Unknown reports that the lookup failed (timeout, error, bad status).
func (c nodeCount) Unknown() bool { return c.unknown }

// Peers reports a known count with at least one peer. It is false when unknown.
func (c nodeCount) Peers() bool { return !c.unknown && c.n > 1 }

// clusterNodeCount asks Phoenix how many nodes the cluster has. "Not
// configured" and COUNCIL_UI=off are answers (a known single node); a timeout or
// error is Unknown, which callers must say aloud rather than treat as local.
func (r *Registry) clusterNodeCount() nodeCount {
	r.nodeMu.Lock()
	defer r.nodeMu.Unlock()
	ttl := nodeCountTTL
	if r.nodeFailed {
		ttl = nodeCountFailTTL
	}
	if !r.nodeAt.IsZero() && time.Since(r.nodeAt) < ttl {
		return nodeCount{n: r.nodeN, unknown: r.nodeFailed}
	}
	r.nodeN, r.nodeFailed = r.fetchClusterNodeCount()
	r.nodeAt = time.Now()
	return nodeCount{n: r.nodeN, unknown: r.nodeFailed}
}

// fetchClusterNodeCount returns the node count and whether the lookup failed.
// "Not configured" and "UI off" are answers, not failures.
func (r *Registry) fetchClusterNodeCount() (int, bool) {
	if strings.EqualFold(os.Getenv("COUNCIL_UI"), "off") {
		return 1, false
	}
	if r.HTTPClient == nil || r.PhoenixURL == "" {
		return 1, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), nodeProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.PhoenixURL+"/api/internal/cluster/nodes", nil)
	if err != nil {
		return 0, true
	}
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return 0, true
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, true
	}
	var payload struct {
		Nodes []json.RawMessage `json:"nodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, true
	}
	return len(payload.Nodes), false
}

// clusterUnknownNote is the line a local-only read carries when the node count
// could not be learned.
const clusterUnknownNote = "cluster state unknown; showing this node only"

// localOnlyNote is the one-line footer for a read that ran on this node only:
// "local node only; N nodes in the cluster", or the unknown-state line when the
// node count could not be learned. Empty on a known single-node cluster. Reads
// are local by default, and a bare empty result gives no hint that a peer may
// hold the answer.
func (r *Registry) localOnlyNote() string {
	c := r.clusterNodeCount()
	switch {
	case c.Unknown():
		return clusterUnknownNote
	case c.Peers():
		return fmt.Sprintf("local node only; %d nodes in the cluster", c.n)
	}
	return ""
}

// handleGetMentionsCluster serves get_mentions(cluster_wide=true): the local
// result merged with every peer's, newest first. Peer failure degrades to the
// local result plus a warning.
func (r *Registry) handleGetMentionsCluster(args GetMentionsInput, limit int) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	var results []ClusterMentionResult
	if r.Server != nil {
		local, err := r.Server.GetMentions(args.Author, args.Project, limit)
		if err != nil {
			return nil, ToolOutput{}, err
		}
		for _, m := range local {
			results = append(results, ClusterMentionResult{
				ID: m.ID, RoomID: m.RoomID, Author: m.Author,
				Content:   council.DisplayContent(m),
				Timestamp: m.Timestamp.Format("2006-01-02 15:04:05"),
			})
		}
	}

	raw, warnings, err := r.clusterCall("get_mentions", map[string]any{
		"author": args.Author, "project": args.Project, "limit": limit,
	})
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s", err.Error()))
	} else {
		var remote []ClusterMentionResult
		if err := json.Unmarshal(raw, &remote); err != nil {
			return nil, ToolOutput{}, fmt.Errorf("decode cluster mentions: %w", err)
		}
		results = append(results, remote...)
	}

	// UUIDv7 ids sort chronologically across nodes.
	seen := map[string]bool{}
	merged := results[:0]
	for _, m := range results {
		if !seen[m.ID] {
			seen[m.ID] = true
			merged = append(merged, m)
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ID > merged[j].ID })
	if len(merged) > limit {
		merged = merged[:limit]
	}

	var b strings.Builder
	if len(merged) == 0 {
		fmt.Fprintf(&b, "No messages mention @%s on any reachable node.\n", args.Author)
	} else {
		fmt.Fprintf(&b, "Found %d message(s) mentioning @%s across the cluster:\n\n", len(merged), args.Author)
		for _, m := range merged {
			excerpt := strings.ReplaceAll(council.TruncateRunes(m.Content, 200, " ", 150), "\n", " ")
			node := ""
			if m.SourceNode != "" {
				node = "[" + m.SourceNode + "] "
			}
			fmt.Fprintf(&b, "- **#%s** %s[%s] %s in **%s**: %s\n", m.ID, node, m.Timestamp, m.Author, m.RoomID, excerpt)
		}
	}
	formatClusterWarnings(&b, warnings)
	return msg(b.String())
}
