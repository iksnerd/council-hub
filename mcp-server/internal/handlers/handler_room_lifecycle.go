package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SignalStatusInput represents the parameters for signaling room status.
type SignalStatusInput struct {
	RoomID string `json:"room_id"`
	Status string `json:"status"`
}

// BulkStatusInput represents the parameters for updating multiple rooms' status at once.
type BulkStatusInput struct {
	RoomIDs string `json:"room_ids"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Author  string `json:"author"`
}

func (r *Registry) handleSignalStatus(ctx context.Context, req *mcp.CallToolRequest, args SignalStatusInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	validStatuses := map[string]bool{"active": true, "paused": true, "resolved": true}
	if !validStatuses[args.Status] {
		return msg(fmt.Sprintf("Error: Invalid status '%s'. Must be one of: active, paused, resolved.", args.Status))
	}

	if err := r.Server.UpdateStatus(args.RoomID, args.Status); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	r.Server.Logger.Info("Status updated", "room_id", args.RoomID, "status", args.Status)

	var b strings.Builder
	fmt.Fprintf(&b, "Room '%s' status \u2192 **%s**.", args.RoomID, args.Status)
	if room, err := r.Server.GetRoom(args.RoomID); err == nil {
		if room.Description != "" {
			fmt.Fprintf(&b, "\n**Topic:** %s", room.Description)
		}
		if room.Project != "" {
			fmt.Fprintf(&b, "\n**Project:** %s", room.Project)
		}
	}
	return msg(b.String())
}

func (r *Registry) handleBulkStatusUpdate(ctx context.Context, req *mcp.CallToolRequest, args BulkStatusInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	validStatuses := map[string]bool{"active": true, "paused": true, "resolved": true}
	if !validStatuses[args.Status] {
		return msg(fmt.Sprintf("Error: Invalid status '%s'. Must be one of: active, paused, resolved.", args.Status))
	}

	if args.RoomIDs == "" {
		return msg("Error: room_ids is required (comma-separated list of room IDs).")
	}

	if args.Message != "" && args.Author == "" {
		return msg("Error: author is required when message is provided.")
	}

	parts := strings.Split(args.RoomIDs, ",")
	var updated, notFound []string
	for _, p := range parts {
		roomID := strings.TrimSpace(p)
		if roomID == "" {
			continue
		}
		// Post closing message before status change (if provided)
		if args.Message != "" {
			_, _ = r.Server.PostMessage(roomID, args.Author, args.Message, "decision", "")
		}
		if err := r.Server.UpdateStatus(roomID, args.Status); err != nil {
			notFound = append(notFound, roomID)
		} else {
			updated = append(updated, roomID)
		}
	}

	var b strings.Builder
	if len(updated) > 0 {
		latestIDs := r.Server.GetLatestMessageIDs()
		fmt.Fprintf(&b, "Updated %d room(s) to '%s':\n", len(updated), args.Status)
		for _, id := range updated {
			if lid, ok := latestIDs[id]; ok {
				fmt.Fprintf(&b, "- %s (latest_message_id: %.8s)\n", id, lid)
			} else {
				fmt.Fprintf(&b, "- %s\n", id)
			}
		}
	}
	if len(notFound) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Not found: %s", strings.Join(notFound, ", "))
	}
	if b.Len() == 0 {
		return msg("No valid room IDs provided.")
	}

	r.Server.Logger.Info("Bulk status update", "status", args.Status, "updated", len(updated), "not_found", len(notFound))
	return msg(b.String())
}
