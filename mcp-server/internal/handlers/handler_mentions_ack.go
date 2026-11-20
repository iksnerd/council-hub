package handlers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mentionsPeerWindow is how many mentions each peer is asked for when ack needs
// a cluster merge. Peers cannot see this node's cursor, so they return their
// newest window and the cursor filter runs here.
const mentionsPeerWindow = 100

// handleGetMentionsAck serves get_mentions(ack=true): only mentions newer than
// the agent's mention cursor, oldest first up to limit, then the cursor moves
// to the newest id returned. Taking the oldest unseen (not the newest) means a
// limit never makes the cursor skip a mention it did not show; the rest come
// on the next call. Output is newest-first, like the plain call.
//
// Cluster: the cursor is node-local, ids sort the same everywhere, so when the
// call is cluster-wide the local and peer mentions are merged first and the
// cursor filter and advance apply to that merged list.
func (r *Registry) handleGetMentionsAck(args GetMentionsInput, limit int) (*mcp.CallToolResult, ToolOutput, error) {
	cursor, err := r.Server.GetMentionCursor(args.Author)
	if err != nil {
		return nil, ToolOutput{}, err
	}
	local, remaining, err := r.Server.GetMentionsAfter(args.Author, args.Project, cursor, limit)
	if err != nil {
		return nil, ToolOutput{}, err
	}
	hidden, err := r.Server.CountMentionsUpTo(args.Author, args.Project, cursor)
	if err != nil {
		return nil, ToolOutput{}, err
	}

	var items []ClusterMentionResult
	for _, m := range local {
		items = append(items, ClusterMentionResult{
			ID: m.ID, RoomID: m.RoomID, Author: m.Author, Content: council.DisplayContent(m),
			Timestamp: m.Timestamp.Format("2006-01-02 15:04:05"),
		})
	}

	scope := ""
	var warnings []string
	wide := args.ClusterWide == "true" ||
		(args.ClusterWide != "false" && !clusterReadsLocal() && r.peersConnected())
	if wide {
		raw, w, err := r.clusterCall("get_mentions", map[string]any{
			"author": args.Author, "project": args.Project, "limit": mentionsPeerWindow,
		})
		warnings = w
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("peer fan-out unavailable: %s; showing this node only", err.Error()))
		} else {
			var remote []ClusterMentionResult
			if err := json.Unmarshal(raw, &remote); err != nil {
				return nil, ToolOutput{}, fmt.Errorf("decode cluster mentions: %w", err)
			}
			seen := map[string]bool{}
			for _, m := range items {
				seen[m.ID] = true
			}
			for _, m := range remote {
				if seen[m.ID] {
					continue
				}
				seen[m.ID] = true
				if m.ID <= cursor {
					hidden++
					continue
				}
				items = append(items, m)
			}
			scope = " across the cluster"
		}
	} else if note := r.localOnlyNote(); note != "" {
		scope = " (" + note + ")"
	}

	// Oldest-first, cap to limit, so the cursor lands on the newest id shown.
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if len(items) > limit {
		remaining += len(items) - limit
		items = items[:limit]
	}

	var b strings.Builder
	if len(items) == 0 {
		fmt.Fprintf(&b, "No new mentions of @%s%s.", args.Author, scope)
	} else {
		fmt.Fprintf(&b, "Found %d new message(s) mentioning @%s%s:\n\n", len(items), args.Author, scope)
		for i := len(items) - 1; i >= 0; i-- {
			m := items[i]
			excerpt := strings.ReplaceAll(council.TruncateRunes(m.Content, 200, " ", 150), "\n", " ")
			node := ""
			if m.SourceNode != "" {
				node = "[" + m.SourceNode + "] "
			}
			fmt.Fprintf(&b, "- **#%s** %s[%s] %s in **%s**: %s\n", m.ID, node, m.Timestamp, m.Author, m.RoomID, excerpt)
		}
		newest := items[len(items)-1].ID
		ids := make([]string, len(items))
		for i, m := range items {
			ids[i] = m.ID
		}
		if err := r.Server.AckMentions(args.Author, ids); err != nil {
			return nil, ToolOutput{}, err
		}
		fmt.Fprintf(&b, "\nMention cursor for @%s advanced to #%s.", args.Author, newest)
	}
	if remaining > 0 {
		fmt.Fprintf(&b, "\n%d more new mention(s) not shown (limit %d); call again with ack=true to get them.", remaining, limit)
	}
	if hidden > 0 {
		fmt.Fprintf(&b, "\n%d older mention(s) hidden by the cursor; call get_mentions without ack to see them.", hidden)
	}
	b.WriteString("\n")
	formatClusterWarnings(&b, warnings)
	return textResult(b.String())
}
