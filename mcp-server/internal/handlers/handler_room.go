package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateRoomInput represents the parameters for creating a room.
type CreateRoomInput struct {
	ID           string `json:"id"`
	Topic        string `json:"topic"`
	Project      string `json:"project"`
	TechStack    string `json:"tech_stack"`
	Tags         string `json:"tags"`
	SystemPrompt string `json:"system_prompt"`
	RelatedRooms string `json:"related_rooms"`
}

// SignalStatusInput represents the parameters for signaling room status.
type SignalStatusInput struct {
	RoomID string `json:"room_id"`
	Status string `json:"status"`
}

// ListRoomsInput represents the parameters for listing rooms.
type ListRoomsInput struct {
	Project string `json:"project"`
	Tag     string `json:"tag"`
	Status  string `json:"status"`
	Search  string `json:"search"`
	Compact string `json:"compact"` // deprecated: compact is now default; kept for backwards compat
	Verbose string `json:"verbose"`
}

// UpdateRoomInput represents the parameters for updating a room's metadata.
type UpdateRoomInput struct {
	RoomID       string `json:"room_id"`
	Topic        string `json:"topic"`
	Project      string `json:"project"`
	TechStack    string `json:"tech_stack"`
	Tags         string `json:"tags"`
	SystemPrompt string `json:"system_prompt"`
	RelatedRooms string `json:"related_rooms"`
}

// ReadRoomInput represents the parameters for reading a room's metadata.
type ReadRoomInput struct {
	RoomID string `json:"room_id"`
}

// DeleteRoomInput represents the parameters for deleting a room.
type DeleteRoomInput struct {
	RoomID string `json:"room_id"`
}

// RoomStatsInput represents the parameters for getting room statistics.
type RoomStatsInput struct {
	RoomID string `json:"room_id"`
}

// BulkStatusInput represents the parameters for updating multiple rooms' status at once.
type BulkStatusInput struct {
	RoomIDs string `json:"room_ids"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Author  string `json:"author"`
}

// GetOrCreateRoomInput represents the parameters for upserting a room.
type GetOrCreateRoomInput struct {
	ID           string `json:"id"`
	Topic        string `json:"topic"`
	Project      string `json:"project"`
	TechStack    string `json:"tech_stack"`
	Tags         string `json:"tags"`
	SystemPrompt string `json:"system_prompt"`
	RelatedRooms string `json:"related_rooms"`
	LastN        string `json:"last_n"`
}

func (r *Registry) handleCreateRoom(ctx context.Context, req *mcp.CallToolRequest, args CreateRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.ID == "" {
		return msg("Error: room id is required.")
	}

	if err := r.Server.CreateRoom(args.ID, args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt, args.RelatedRooms); err != nil {
		r.Server.Logger.Error("Failed to create room", "id", args.ID, "error", err)
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Room created", "id", args.ID, "project", args.Project, "topic", args.Topic)

	var b strings.Builder
	fmt.Fprintf(&b, "Room '%s' created.\n", args.ID)
	if args.Topic != "" {
		fmt.Fprintf(&b, "**Topic:** %s\n", args.Topic)
	}
	if args.Project != "" {
		fmt.Fprintf(&b, "**Project:** %s\n", args.Project)
	}
	if args.Tags != "" {
		fmt.Fprintf(&b, "**Tags:** %s\n", args.Tags)
	}
	if args.RelatedRooms != "" {
		fmt.Fprintf(&b, "**Related rooms:** %s (bidirectional links created)\n", args.RelatedRooms)
	}
	return msg(b.String())
}

func (r *Registry) handleGetOrCreateRoom(ctx context.Context, req *mcp.CallToolRequest, args GetOrCreateRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.ID == "" {
		return msg("Error: id is required.")
	}

	// Try to get existing room
	room, err := r.Server.GetRoom(args.ID)
	created := false
	if err != nil {
		// Room doesn't exist — create it
		if err := r.Server.CreateRoom(args.ID, args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt, args.RelatedRooms); err != nil {
			r.Server.Logger.Error("Failed to create room", "id", args.ID, "error", err)
			return nil, ToolOutput{}, err
		}
		room, _ = r.Server.GetRoom(args.ID)
		created = true
	}

	limit := 5
	if args.LastN != "" {
		if _, err := fmt.Sscanf(args.LastN, "%d", &limit); err != nil {
			limit = 5
		}
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	var b strings.Builder
	if created {
		fmt.Fprintf(&b, "**Created** room '%s'.\n", room.ID)
	} else {
		fmt.Fprintf(&b, "**Found** room '%s'.\n", room.ID)
	}
	fmt.Fprintf(&b, "**%s** [%s]\n", room.ID, room.Status)
	fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
	if room.SystemPrompt != "" {
		fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
	}

	if !created {
		messages, _ := r.Server.GetRecentMessages(args.ID, limit)
		if len(messages) > 0 {
			fmt.Fprintf(&b, "---\n**Recent messages (%d):**\n", len(messages))
			for _, m := range messages {
				ts := m.Timestamp.Format("2006-01-02 15:04:05")
				if m.MessageType != "" && m.MessageType != "message" {
					fmt.Fprintf(&b, "\n**[#%d %s] %s (%s):**\n%s\n", m.ID, ts, m.Author, m.MessageType, m.Content)
				} else {
					fmt.Fprintf(&b, "\n**[#%d %s] %s:**\n%s\n", m.ID, ts, m.Author, m.Content)
				}
			}
		} else {
			b.WriteString("No messages yet.\n")
		}
	}

	r.Server.Logger.Info("get_or_create_room", "id", args.ID, "created", created)
	return msg(b.String())
}

func (r *Registry) handleUpdateRoom(ctx context.Context, req *mcp.CallToolRequest, args UpdateRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	if args.Topic == "" && args.Project == "" && args.TechStack == "" && args.Tags == "" && args.SystemPrompt == "" && args.RelatedRooms == "" {
		return msg("Error: at least one field to update must be provided (topic, project, tech_stack, tags, system_prompt, related_rooms).")
	}

	if err := r.Server.UpdateRoom(args.RoomID, args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt, args.RelatedRooms); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	var updated []string
	if args.Topic != "" {
		updated = append(updated, "topic")
	}
	if args.Project != "" {
		updated = append(updated, "project")
	}
	if args.TechStack != "" {
		updated = append(updated, "tech_stack")
	}
	if args.Tags != "" {
		updated = append(updated, "tags")
	}
	if args.SystemPrompt != "" {
		updated = append(updated, "system_prompt")
	}
	if args.RelatedRooms != "" {
		updated = append(updated, "related_rooms")
	}

	r.Server.Logger.Info("Room updated", "room_id", args.RoomID, "fields", strings.Join(updated, ", "))

	var b strings.Builder
	fmt.Fprintf(&b, "Room '%s' updated: %s.", args.RoomID, strings.Join(updated, ", "))
	if room, err := r.Server.GetRoom(args.RoomID); err == nil {
		fmt.Fprintf(&b, "\n\n**Current state:**")
		if room.Description != "" {
			fmt.Fprintf(&b, "\n- Topic: %s", room.Description)
		}
		if room.Project != "" {
			fmt.Fprintf(&b, "\n- Project: %s", room.Project)
		}
		if room.Tags != "" {
			fmt.Fprintf(&b, "\n- Tags: %s", room.Tags)
		}
		if room.RelatedRooms != "" {
			fmt.Fprintf(&b, "\n- Related rooms: %s", room.RelatedRooms)
		}
		fmt.Fprintf(&b, "\n- Status: %s", room.Status)
	}
	return msg(b.String())
}

func (r *Registry) handleReadRoom(ctx context.Context, req *mcp.CallToolRequest, args ReadRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	room, err := r.Server.GetRoom(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: Room '%s' not found.", args.RoomID))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%s** [%s]\n", room.ID, room.Status)
	fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
	if room.Project != "" {
		fmt.Fprintf(&b, "**Project:** %s\n", room.Project)
	}
	if room.TechStack != "" {
		fmt.Fprintf(&b, "**Tech Stack:** %s\n", room.TechStack)
	}
	if room.Tags != "" {
		fmt.Fprintf(&b, "**Tags:** %s\n", room.Tags)
	}
	if room.SystemPrompt != "" {
		fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
	}
	if room.RelatedRooms != "" {
		fmt.Fprintf(&b, "**Related Rooms:** %s\n", room.RelatedRooms)
	}
	fmt.Fprintf(&b, "**Created:** %s\n", room.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "**Updated:** %s\n", room.UpdatedAt.Format("2006-01-02 15:04:05"))

	return msg(b.String())
}

func (r *Registry) handleDeleteRoom(ctx context.Context, req *mcp.CallToolRequest, args DeleteRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	if err := r.Server.DeleteRoom(args.RoomID); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	r.Server.Logger.Info("Room deleted", "room_id", args.RoomID)
	return msg(fmt.Sprintf("Room '%s' and all its messages have been permanently deleted.", args.RoomID))
}

func (r *Registry) handleListRooms(ctx context.Context, req *mcp.CallToolRequest, args ListRoomsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	rooms, err := r.Server.ListRooms(args.Project, args.Tag, args.Status, args.Search)
	if err != nil {
		r.Server.Logger.Error("Failed to list rooms", "error", err)
		return nil, ToolOutput{}, err
	}

	if len(rooms) == 0 {
		return msg("No rooms found matching the given filters.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d room(s):\n\n", len(rooms))

	// Compact is the default. Verbose mode is opt-in via verbose=true.
	// Legacy compact=false maps to verbose for backwards compat.
	useVerbose := args.Verbose == "true" || args.Compact == "false"
	if !useVerbose {
		// Fetch message counts for compact display
		msgCounts := r.Server.GetMessageCounts()

		for _, r := range rooms {
			topic := r.Description
			if len(topic) > 60 {
				topic = topic[:60] + "..."
			}
			project := r.Project
			if project == "" {
				project = "-"
			}
			count := msgCounts[r.ID]
			fmt.Fprintf(&b, "- **%s** | %s | %s | %d msgs | %s | %s\n", r.ID, project, r.Status, count, topic, r.UpdatedAt.Format("2006-01-02 15:04"))
		}
	} else {
		for _, r := range rooms {
			fmt.Fprintf(&b, "- **%s** [%s]", r.ID, r.Status)
			if r.Project != "" {
				fmt.Fprintf(&b, " | project: %s", r.Project)
			}
			if r.Tags != "" {
				fmt.Fprintf(&b, " | tags: %s", r.Tags)
			}
			fmt.Fprintf(&b, "\n  %s\n", r.Description)
			if r.TechStack != "" {
				fmt.Fprintf(&b, "  Tech: %s\n", r.TechStack)
			}
			if r.RelatedRooms != "" {
				fmt.Fprintf(&b, "  Related: %s\n", r.RelatedRooms)
			}
			fmt.Fprintf(&b, "  Last activity: %s\n", r.UpdatedAt.Format("2006-01-02 15:04:05"))
		}
	}

	text := b.String()
	return msg(text)
}

func (r *Registry) handleRoomStats(ctx context.Context, req *mcp.CallToolRequest, args RoomStatsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	stats, err := r.Server.GetRoomStats(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%s** [%s]\n", stats.RoomID, stats.Status)
	fmt.Fprintf(&b, "**Messages:** %d\n", stats.MessageCount)
	if stats.LatestMessageID > 0 {
		fmt.Fprintf(&b, "**Latest message ID:** %d\n", stats.LatestMessageID)
	}

	if len(stats.Participants) > 0 {
		var parts []string
		for author, count := range stats.Participants {
			parts = append(parts, fmt.Sprintf("%s (%d)", author, count))
		}
		fmt.Fprintf(&b, "**Participants:** %s\n", strings.Join(parts, ", "))
		fmt.Fprintf(&b, "**First message:** %s\n", stats.FirstMessage.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&b, "**Last message:** %s\n", stats.LastMessage.Format("2006-01-02 15:04:05"))
	}

	if len(stats.TypeCounts) > 0 {
		var types []string
		for msgType, count := range stats.TypeCounts {
			types = append(types, fmt.Sprintf("%s: %d", msgType, count))
		}
		fmt.Fprintf(&b, "**Types:** %s\n", strings.Join(types, ", "))
	}

	return msg(b.String())
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
			r.Server.PostMessage(roomID, args.Author, args.Message, "decision", 0)
		}
		if err := r.Server.UpdateStatus(roomID, args.Status); err != nil {
			notFound = append(notFound, roomID)
		} else {
			updated = append(updated, roomID)
		}
	}

	var b strings.Builder
	if len(updated) > 0 {
		fmt.Fprintf(&b, "Updated %d room(s) to '%s': %s", len(updated), args.Status, strings.Join(updated, ", "))
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
