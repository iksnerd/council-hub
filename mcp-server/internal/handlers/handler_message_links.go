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

	id, err := r.Server.CreateLink(args.FromID, args.ToID, args.Relation, args.Author)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s. Valid relations: %s.", err.Error(), validLinkRelationList()))
	}

	r.Server.Logger.Info("Link created", "from", args.FromID, "to", args.ToID, "relation", args.Relation)
	return msg(fmt.Sprintf("Linked #%.8s --%s--> #%.8s (link #%.8s).", args.FromID, strings.ToLower(args.Relation), args.ToID, id))
}

func (r *Registry) handleUnlinkMessages(ctx context.Context, req *mcp.CallToolRequest, args UnlinkMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.LinkID == "" {
		return msg("Error: link_id is required.")
	}
	if err := r.Server.DeleteLink(args.LinkID); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}
	return msg(fmt.Sprintf("Link #%.8s removed.", args.LinkID))
}

func (r *Registry) handleGetLinks(ctx context.Context, req *mcp.CallToolRequest, args GetLinksInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.MessageID == "" {
		return msg("Error: message_id is required.")
	}

	outgoing, incoming, err := r.Server.GetLinks(args.MessageID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	if len(outgoing) == 0 && len(incoming) == 0 {
		return msg(fmt.Sprintf("#%.8s has no links (no replies, supersessions, or typed links).", args.MessageID))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Link graph for #%.8s:\n", args.MessageID)
	if len(outgoing) > 0 {
		b.WriteString("\n**Outgoing** (this message points at):\n")
		for _, l := range outgoing {
			fmt.Fprintf(&b, "- %s #%.8s%s\n", l.Relation, l.ToID, implicitTag(l.Implicit, l.ID))
		}
	}
	if len(incoming) > 0 {
		b.WriteString("\n**Incoming** (backlinks — point here):\n")
		for _, l := range incoming {
			fmt.Fprintf(&b, "- #%.8s %s this%s\n", l.FromID, l.Relation, implicitTag(l.Implicit, l.ID))
		}
	}
	return msg(b.String())
}

func implicitTag(implicit bool, linkID string) string {
	if implicit {
		return " _(implicit)_"
	}
	return fmt.Sprintf(" _(link #%.8s)_", linkID)
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
