package handlers

import (
	"context"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetConceptMapInput represents the parameters for getting a room's conceptual graph.
type GetConceptMapInput struct {
	RoomID    string `json:"room_id"`
	MaxDepth  string `json:"max_depth"`
	InferFrom string `json:"infer_from"`
}

func (r *Registry) handleGetConceptMap(ctx context.Context, req *mcp.CallToolRequest, args GetConceptMapInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	maxDepth := 3
	if args.MaxDepth != "" {
		if _, err := fmt.Sscanf(args.MaxDepth, "%d", &maxDepth); err != nil {
			maxDepth = 3
		}
	}

	nodes, err := r.Server.GetConceptMap(args.RoomID, maxDepth, args.InferFrom)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	var b strings.Builder
	header := fmt.Sprintf("## Concept Map: %s (depth %d)", args.RoomID, maxDepth)
	if args.InferFrom != "" {
		header += fmt.Sprintf(" [infer_from: %s]", args.InferFrom)
	}
	fmt.Fprintln(&b, header)

	// Group nodes by depth for flat Markdown output
	nodesByDepth := make(map[int][]council.ConceptMapNode)
	maxReachedDepth := 0
	for _, n := range nodes {
		nodesByDepth[n.Depth] = append(nodesByDepth[n.Depth], n)
		if n.Depth > maxReachedDepth {
			maxReachedDepth = n.Depth
		}
	}

	for d := 0; d <= maxReachedDepth; d++ {
		layer := nodesByDepth[d]
		if len(layer) == 0 {
			continue
		}

		if d == 0 {
			fmt.Fprintf(&b, "\n### Depth %d (root)\n", d)
		} else {
			fmt.Fprintf(&b, "\n### Depth %d\n", d)
		}

		for _, n := range layer {
			annotation := ""
			if n.Inferred != "" {
				annotation = fmt.Sprintf(" (inferred: %s)", n.Inferred)
			} else if n.Via != "" {
				annotation = fmt.Sprintf(" (via: %s)", n.Via)
			}
			tags := ""
			if n.Room.Tags != "" {
				tags = fmt.Sprintf(" | tags: %s", n.Room.Tags)
			}
			fmt.Fprintf(&b, "- **%s** [%s]%s%s — %s\n", n.Room.ID, n.Room.Status, tags, annotation, n.Room.Description)
		}
	}

	if len(nodes) == 1 && args.InferFrom == "" {
		b.WriteString("\n⚠️ No related rooms configured for this room. Add links via update_room(related_rooms=...) or use infer_from=project to discover related rooms automatically.")
	} else if len(nodes) == 1 {
		b.WriteString("\n⚠️ No related or inferred rooms found.")
	}

	return msg(b.String())
}
