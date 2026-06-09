package handlers

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MarkReadInput represents the parameters for persisting an agent's read cursor.
type MarkReadInput struct {
	RoomID string `json:"room_id"`
	Cursor string `json:"cursor"`
	Agent  string `json:"agent"`
}

func (r *Registry) handleMarkRead(ctx context.Context, req *mcp.CallToolRequest, args MarkReadInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}
	if args.Cursor == "" {
		return msg("Error: cursor (message ID) is required.")
	}
	agent := args.Agent
	if agent == "" {
		agent = "default"
	}

	if err := r.Server.MarkRead(agent, args.RoomID, args.Cursor); err != nil {
		r.Server.Logger.Error("Failed to mark read", "agent", agent, "room_id", args.RoomID, "error", err)
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Cursor saved", "agent", agent, "room_id", args.RoomID, "cursor", args.Cursor)
	return msg(fmt.Sprintf("Cursor saved for agent '%s' in room '%s': #%.8s. Use get_digest(unread_only=true, agent=%s) to see only new messages.", agent, args.RoomID, args.Cursor, agent))
}
