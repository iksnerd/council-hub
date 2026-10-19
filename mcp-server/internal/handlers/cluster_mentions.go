package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

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

// clusterNodeCount asks Phoenix how many nodes the cluster has. It returns 0
// when that cannot be determined (no Phoenix, UI off, error), so callers stay
// silent rather than guess.
func (r *Registry) clusterNodeCount() int {
	if r.HTTPClient == nil || r.PhoenixURL == "" {
		return 0
	}
	resp, err := r.HTTPClient.Get(r.PhoenixURL + "/api/internal/cluster/nodes")
	if err != nil {
		return 0
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	var payload struct {
		Nodes []json.RawMessage `json:"nodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0
	}
	return len(payload.Nodes)
}

// localOnlyNote is the one-line footer for a read that ran on this node only:
// "local node only; N nodes in the cluster". Empty on a single-node cluster or
// when the node count is unknown. Reads are local by default, and a bare empty
// result gives no hint that a peer may hold the answer.
func (r *Registry) localOnlyNote() string {
	if n := r.clusterNodeCount(); n > 1 {
		return fmt.Sprintf("local node only; %d nodes in the cluster", n)
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
