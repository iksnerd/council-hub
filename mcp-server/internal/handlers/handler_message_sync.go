package handlers

import (
	"context"
	"fmt"
	"strings"

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
	agent := args.Agent
	if agent == "" {
		agent = "default"
	}
	if args.RoomID == "*" {
		return r.markReadAll(agent, args.Cursor)
	}
	if args.Cursor == "" {
		return msg("Error: cursor (message ID) is required.")
	}

	// Checked before the write so "new name" is still observable.
	isNew, neighbours, _ := r.Server.CursorNeighbours(agent)

	if err := r.Server.MarkRead(agent, args.RoomID, args.Cursor); err != nil {
		r.Server.Logger.Error("Failed to mark read", "agent", agent, "room_id", args.RoomID, "error", err)
		if note := r.remoteRoomNote(args.RoomID, "read cursors are node-local, so mark it read there"); note != "" {
			return msg(fmt.Sprintf("Error: %s%s", err.Error(), note))
		}
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Cursor saved", "agent", agent, "room_id", args.RoomID, "cursor", args.Cursor)
	out := fmt.Sprintf("Cursor saved for agent '%s' in room '%s': #%.8s. Use get_digest(unread_only=true, agent=%s) to see only new messages.", agent, args.RoomID, args.Cursor, agent)
	if isNew && len(neighbours) > 0 {
		out += fmt.Sprintf("\nWarning: '%s' is a substring neighbour of existing cursor name(s): %s. get_mentions matches by substring, so these names share mentions but keep separate read cursors. Use one stable, distinctive agent name.", agent, strings.Join(neighbours, ", "))
	}
	return msg(out)
}

// markReadAll serves mark_read(room_id="*"): every room on this node jumps to
// its latest message, so a new agent name starts from now.
func (r *Registry) markReadAll(agent, cursor string) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult
	if cursor != "" {
		return msg("Error: cursor is not allowed with room_id=\"*\"; every room is set to its latest message. Omit cursor, or pass a real room_id to set one cursor.")
	}
	isNew, neighbours, _ := r.Server.CursorNeighbours(agent)
	updated, skipped, err := r.Server.MarkAllRead(agent)
	if err != nil {
		r.Server.Logger.Error("Failed to mark all read", "agent", agent, "error", err)
		return nil, ToolOutput{}, err
	}
	r.Server.Logger.Info("Cursors saved for all rooms", "agent", agent, "updated", updated, "skipped", skipped)
	out := fmt.Sprintf("Cursors saved for agent '%s' in %d rooms, each at its latest message", agent, updated)
	if skipped > 0 {
		out += fmt.Sprintf(" (%d had no messages and were skipped)", skipped)
	}
	out += fmt.Sprintf(". Use get_digest(unread_only=true, agent=%s) to see only what changes from now.", agent)
	if isNew && len(neighbours) > 0 {
		out += fmt.Sprintf("\nWarning: '%s' is a substring neighbour of existing cursor name(s): %s. get_mentions matches by substring, so these names share mentions but keep separate read cursors. Use one stable, distinctive agent name.", agent, strings.Join(neighbours, ", "))
	}
	if note := r.localOnlyNote(); note != "" {
		out += "\n(" + note + "; read cursors are node-local, so run this on each node you read from.)"
	}
	return msg(out)
}
