package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SignalStatusInput represents the parameters for signaling room status.
type SignalStatusInput struct {
	RoomID string `json:"room_id"`
	Status string `json:"status"`
}

// BulkStatusInput represents the parameters for updating multiple rooms' status at once.
type BulkStatusInput struct {
	RoomIDs         string `json:"room_ids"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	Author          string `json:"author"`
	AutoArchiveDays string `json:"auto_archive_days"`
}

// RenameProjectInput represents the parameters for rewriting a project name across rooms.
type RenameProjectInput struct {
	From string `json:"from"`
	To   string `json:"to"`
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

	autoArchiveDays := 0
	if args.AutoArchiveDays != "" {
		if _, err := fmt.Sscanf(args.AutoArchiveDays, "%d", &autoArchiveDays); err != nil || autoArchiveDays < 0 {
			return msg("Error: auto_archive_days must be a non-negative integer.")
		}
	}

	parts := strings.Split(args.RoomIDs, ",")
	var updated, notFound, archived []string
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
			continue
		}
		updated = append(updated, roomID)

		// Auto-archive: only on resolved transitions, only if last activity is old enough
		if autoArchiveDays > 0 && args.Status == "resolved" {
			stats, err := r.Server.GetRoomStats(roomID)
			if err != nil {
				continue
			}
			cutoff := time.Now().Add(-time.Duration(autoArchiveDays) * 24 * time.Hour)
			if stats.MessageCount == 0 || stats.LastMessage.Before(cutoff) {
				if _, err := r.Server.ArchiveRoom(roomID); err == nil {
					if delErr := r.Server.DeleteRoom(roomID); delErr == nil {
						archived = append(archived, roomID)
					}
				}
			}
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
	if len(archived) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Auto-archived %d room(s) inactive for \u2265%d day(s): %s\n", len(archived), autoArchiveDays, strings.Join(archived, ", "))
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

	r.Server.Logger.Info("Bulk status update", "status", args.Status, "updated", len(updated), "archived", len(archived), "not_found", len(notFound))
	return msg(b.String())
}

func (r *Registry) handleRenameProject(ctx context.Context, req *mcp.CallToolRequest, args RenameProjectInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.From == "" || args.To == "" {
		return msg("Error: both 'from' and 'to' are required.")
	}

	count, err := r.Server.RenameProject(args.From, args.To)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	r.Server.Logger.Info("Project renamed", "from", args.From, "to", args.To, "rooms_updated", count)
	return msg(fmt.Sprintf("Renamed project '%s' \u2192 '%s' across %d room(s).", args.From, args.To, count))
}
