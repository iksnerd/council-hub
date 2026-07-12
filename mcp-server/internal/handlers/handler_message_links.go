package handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LinkMessagesInput represents the parameters for asserting a typed link.
type LinkMessagesInput struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Relation string `json:"relation"`
	Author   string `json:"author"`
}

// GetLinksInput represents the parameters for querying a message's link graph.
type GetLinksInput struct {
	MessageID string `json:"message_id"`
	Depth     string `json:"depth"`
}

// UnlinkMessagesInput represents the parameters for removing a link.
type UnlinkMessagesInput struct {
	LinkID string `json:"link_id"`
}

func (r *Registry) handleLinkMessages(ctx context.Context, req *mcp.CallToolRequest, args LinkMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.FromID == "" || args.ToID == "" || args.Relation == "" {
		return msg("Error: from_id, to_id, and relation are all required.")
	}
	resolved, idErr := r.resolveIDList([]string{args.FromID, args.ToID})
	if idErr != nil {
		return msg(fmt.Sprintf("Error: %s", idErr.Error()))
	}
	args.FromID, args.ToID = resolved[0], resolved[1]

	id, err := r.Server.CreateLink(args.FromID, args.ToID, args.Relation, args.Author)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s. Valid relations: %s.", err.Error(), validLinkRelationList()))
	}

	// Full link ID (not #%.8s) — like get_links, this output must be pastable
	// straight into another tool's id parameter (unlink_messages).
	r.Server.Logger.Info("Link created", "from", args.FromID, "to", args.ToID, "relation", args.Relation)
	return msg(fmt.Sprintf("Linked #%s --%s--> #%s (link #%s).", args.FromID, strings.ToLower(args.Relation), args.ToID, id))
}

func (r *Registry) handleUnlinkMessages(ctx context.Context, req *mcp.CallToolRequest, args UnlinkMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.LinkID == "" {
		return msg("Error: link_id is required.")
	}
	if err := r.Server.DeleteLink(args.LinkID); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}
	return msg(fmt.Sprintf("Link #%s removed.", args.LinkID))
}

func (r *Registry) handleGetLinks(ctx context.Context, req *mcp.CallToolRequest, args GetLinksInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.MessageID == "" {
		return msg("Error: message_id is required.")
	}
	resolved, idErr := r.resolveSingleID(args.MessageID)
	if idErr != nil {
		return msg(fmt.Sprintf("Error: %s", idErr.Error()))
	}
	args.MessageID = resolved

	// depth > 1 switches to a link-distance neighborhood walk (NLS level-clip).
	if depth := parseDepth(args.Depth); depth > 1 {
		return r.renderNeighborhood(args.MessageID, depth)
	}

	outgoing, incoming, err := r.Server.GetLinks(args.MessageID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	if len(outgoing) == 0 && len(incoming) == 0 {
		return msg(fmt.Sprintf("#%s has no links (no replies, supersessions, or typed links).", args.MessageID))
	}

	// Full IDs here (not the #%.8s prefix used elsewhere): get_links is the
	// graph-addressing tool, so what it prints must be pastable straight into
	// another tool's id parameter — a truncated prefix can collide with other
	// messages posted in the same ~65s window (UUIDv7's shared timestamp bits).
	var b strings.Builder
	fmt.Fprintf(&b, "Link graph for #%s:\n", args.MessageID)
	if len(outgoing) > 0 {
		b.WriteString("\n**Outgoing** (this message points at):\n")
		for _, l := range outgoing {
			fmt.Fprintf(&b, "- %s #%s%s\n", l.Relation, l.ToID, implicitTag(l.Implicit, l.ID))
		}
	}
	if len(incoming) > 0 {
		b.WriteString("\n**Incoming** (backlinks — point here):\n")
		for _, l := range incoming {
			fmt.Fprintf(&b, "- #%s %s this%s\n", l.FromID, l.Relation, implicitTag(l.Implicit, l.ID))
		}
	}
	return msg(b.String())
}

func parseDepth(s string) int {
	if s == "" {
		return 1
	}
	var d int
	if _, err := fmt.Sscanf(s, "%d", &d); err != nil {
		return 1
	}
	return d
}

func (r *Registry) renderNeighborhood(messageID string, depth int) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	nodes, edges, err := r.Server.GetLinkNeighborhood(messageID, depth)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Link neighborhood of #%s (depth %d): %d node(s), %d edge(s)\n", messageID, depth, len(nodes), len(edges))

	// Nodes grouped by hop distance from the focus. Full IDs (see handleGetLinks
	// comment) — this is the graph-addressing tool, so its output must round-trip
	// into another tool's id parameter.
	maxDist := 0
	for _, n := range nodes {
		if n.Distance > maxDist {
			maxDist = n.Distance
		}
	}
	for d := 0; d <= maxDist; d++ {
		var line []string
		for _, n := range nodes {
			if n.Distance == d {
				typeTag := ""
				if n.Type != "" && n.Type != "message" {
					typeTag = " " + n.Type
				}
				line = append(line, fmt.Sprintf("  - #%s [%s%s] %s", n.ID, n.Author, typeTag, n.Excerpt))
			}
		}
		if len(line) > 0 {
			fmt.Fprintf(&b, "\n**Distance %d:**\n%s\n", d, strings.Join(line, "\n"))
		}
	}

	if len(edges) > 0 {
		b.WriteString("\n**Edges:**\n")
		for _, e := range edges {
			fmt.Fprintf(&b, "- #%s --%s--> #%s%s\n", e.FromID, e.Relation, e.ToID, implicitTag(e.Implicit, e.ID))
		}
	}
	return msg(b.String())
}

func implicitTag(implicit bool, linkID string) string {
	if implicit {
		return " _(implicit)_"
	}
	return fmt.Sprintf(" _(link #%s)_", linkID)
}

// validLinkRelationList returns the allowed explicit relations, sorted, for error messages.
func validLinkRelationList() string {
	var rels []string
	for rel := range council.ValidLinkRelations {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	return strings.Join(rels, ", ")
}
