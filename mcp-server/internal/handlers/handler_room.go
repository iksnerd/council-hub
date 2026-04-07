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
	Template     string `json:"template"`
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
	Project     string `json:"project"`
	Tag         string `json:"tag"`
	Status      string `json:"status"`
	Search      string `json:"search"`
	Compact     string `json:"compact"` // deprecated: compact is now default; kept for backwards compat
	Verbose     string `json:"verbose"`
	ClusterWide string `json:"cluster_wide"`
}

// UpdateRoomInput represents the parameters for updating a room's metadata.
type UpdateRoomInput struct {
	RoomID       string `json:"room_id"`
	RoomIDs      string `json:"room_ids"`
	Topic        string `json:"topic"`
	Project      string `json:"project"`
	TechStack    string `json:"tech_stack"`
	Tags         string `json:"tags"`
	AddTags      string `json:"add_tags"`
	RemoveTags   string `json:"remove_tags"`
	SystemPrompt string `json:"system_prompt"`
	RelatedRooms string `json:"related_rooms"`
}

// ReadRoomInput represents the parameters for reading a room's metadata.
type ReadRoomInput struct {
	RoomID                  string `json:"room_id"`
	ClusterWide             string `json:"cluster_wide"`
	IncludeRelatedSummaries string `json:"include_related_summaries"`
}

// DeleteRoomInput represents the parameters for deleting a room.
type DeleteRoomInput struct {
	RoomID string `json:"room_id"`
}

// RoomStatsInput represents the parameters for getting room statistics.
type RoomStatsInput struct {
	RoomID      string `json:"room_id"`
	ClusterWide string `json:"cluster_wide"`
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
	if err := validateSize("id", args.ID, maxIDLen); err != nil {
		return msg("Error: " + err.Error())
	}
	if err := validateRoomMetadata(args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt); err != nil {
		return msg("Error: " + err.Error())
	}

	// Apply template defaults (explicit user args take precedence)
	if args.Template != "" {
		tpl, ok := roomTemplates[args.Template]
		if !ok {
			return msg(fmt.Sprintf("Error: unknown template '%s'. Available: %s",
				args.Template, strings.Join(templateNames(), ", ")))
		}
		if args.Topic == "" {
			args.Topic = tpl.Topic
		}
		if args.Tags == "" {
			args.Tags = tpl.Tags
		}
		if args.TechStack == "" {
			args.TechStack = tpl.TechStack
		}
		if args.SystemPrompt == "" {
			args.SystemPrompt = tpl.SystemPrompt
		}
	}

	// Pre-check existence so we know whether to post the initial message
	_, roomAlreadyExists := r.Server.GetRoom(args.ID)

	if err := r.Server.CreateRoom(args.ID, args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt, args.RelatedRooms); err != nil {
		r.Server.Logger.Error("Failed to create room", "id", args.ID, "error", err)
		return nil, ToolOutput{}, err
	}

	// Post initial message for new rooms created from a template
	if args.Template != "" && roomAlreadyExists != nil {
		tpl := roomTemplates[args.Template]
		if tpl.InitialMsg != "" {
			r.Server.PostMessage(args.ID, "system", tpl.InitialMsg, "thought", "") //nolint:errcheck
		}
	}

	r.Server.Logger.Info("Room created", "id", args.ID, "project", args.Project, "topic", args.Topic)

	var b strings.Builder
	fmt.Fprintf(&b, "Room '%s' created.\n", args.ID)
	if args.Template != "" {
		fmt.Fprintf(&b, "**Template:** %s\n", args.Template)
	}
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

	if args.RelatedRooms == "" {
		fmt.Fprintf(&b, "\n**Tip:** No related_rooms set — link parent/sibling rooms for cross-room navigation.\n")
	}

	// Advisory duplicate check — never blocks creation.
	if similar, err := r.Server.FindSimilarRooms(args.ID, args.Topic, args.Project, args.Tags, 3); err == nil && len(similar) > 0 {
		fmt.Fprintf(&b, "\n**Note:** Similar room(s) already exist:\n")
		for _, sr := range similar {
			fmt.Fprintf(&b, "- **%s** — %s (%s)\n", sr.ID, sr.Description, sr.MatchReason)
		}
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
	if err := validateSize("id", args.ID, maxIDLen); err != nil {
		return msg("Error: " + err.Error())
	}
	if err := validateRoomMetadata(args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt); err != nil {
		return msg("Error: " + err.Error())
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
					fmt.Fprintf(&b, "\n**[#%.8s %s] %s (%s):**\n%s\n", m.ID, ts, m.Author, m.MessageType, m.Content)
				} else {
					fmt.Fprintf(&b, "\n**[#%.8s %s] %s:**\n%s\n", m.ID, ts, m.Author, m.Content)
				}
			}
		} else {
			b.WriteString("No messages yet.\n")
		}
	}

	// Hint to link related rooms on creation
	if created && args.RelatedRooms == "" {
		fmt.Fprintf(&b, "\n**Tip:** No related_rooms set — link parent/sibling rooms for cross-room navigation.\n")
	}

	// Advisory duplicate check on newly created rooms only.
	if created {
		if similar, err := r.Server.FindSimilarRooms(args.ID, args.Topic, args.Project, args.Tags, 3); err == nil && len(similar) > 0 {
			fmt.Fprintf(&b, "\n**Note:** Similar room(s) already exist:\n")
			for _, sr := range similar {
				fmt.Fprintf(&b, "- **%s** — %s (%s)\n", sr.ID, sr.Description, sr.MatchReason)
			}
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

	// Collect all target room IDs (room_ids takes comma-separated list; room_id is single/legacy)
	seen := map[string]bool{}
	var ids []string
	for _, id := range strings.Split(args.RoomIDs, ",") {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if args.RoomID != "" && !seen[args.RoomID] {
		ids = append(ids, args.RoomID)
	}

	if len(ids) == 0 {
		return msg("Error: room_id or room_ids is required.")
	}

	if args.Topic == "" && args.Project == "" && args.TechStack == "" && args.Tags == "" && args.AddTags == "" && args.RemoveTags == "" && args.SystemPrompt == "" && args.RelatedRooms == "" {
		return msg("Error: at least one field to update must be provided (topic, project, tech_stack, tags, add_tags, remove_tags, system_prompt, related_rooms).")
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
	if args.AddTags != "" {
		updated = append(updated, "add_tags")
	}
	if args.RemoveTags != "" {
		updated = append(updated, "remove_tags")
	}
	if args.SystemPrompt != "" {
		updated = append(updated, "system_prompt")
	}
	if args.RelatedRooms != "" {
		updated = append(updated, "related_rooms")
	}
	fieldsLabel := strings.Join(updated, ", ")

	var b strings.Builder
	for _, id := range ids {
		if err := r.Server.UpdateRoom(id, args.Topic, args.Project, args.TechStack, args.Tags, args.AddTags, args.RemoveTags, args.SystemPrompt, args.RelatedRooms); err != nil {
			fmt.Fprintf(&b, "Error updating '%s': %s\n", id, err.Error())
			continue
		}
		r.Server.Logger.Info("Room updated", "room_id", id, "fields", fieldsLabel)
		fmt.Fprintf(&b, "Room '%s' updated: %s.\n", id, fieldsLabel)
	}

	// For single-room updates, append current state (backward-compatible behaviour)
	if len(ids) == 1 {
		if room, err := r.Server.GetRoom(ids[0]); err == nil {
			fmt.Fprintf(&b, "\n**Current state:**")
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
	}

	return msg(strings.TrimRight(b.String(), "\n"))
}

func (r *Registry) handleReadRoom(ctx context.Context, req *mcp.CallToolRequest, args ReadRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	if args.ClusterWide == "true" {
		return r.handleReadRoomCluster(args)
	}

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

	// Append related room summaries if requested
	if args.IncludeRelatedSummaries == "true" && room.RelatedRooms != "" {
		relatedIDs := strings.Split(room.RelatedRooms, ",")
		for _, rid := range relatedIDs {
			rid = strings.TrimSpace(rid)
			if rid == "" {
				continue
			}
			relRoom, err := r.Server.GetRoom(rid)
			if err != nil {
				fmt.Fprintf(&b, "\n---\n**%s** — (not found)\n", rid)
				continue
			}
			fmt.Fprintf(&b, "\n---\n**%s** [%s]\n", relRoom.ID, relRoom.Status)
			fmt.Fprintf(&b, "**Topic:** %s\n", relRoom.Description)
			if relRoom.SystemPrompt != "" {
				fmt.Fprintf(&b, "**System Prompt:** %s\n", relRoom.SystemPrompt)
			}
			pinned, _ := r.Server.GetPinnedMessage(rid)
			if pinned != nil {
				excerpt := pinned.Content
				if len(excerpt) > 200 {
					excerpt = excerpt[:200]
					if i := strings.LastIndex(excerpt, " "); i > 120 {
						excerpt = excerpt[:i]
					}
					excerpt += "..."
				}
				fmt.Fprintf(&b, "📌 %s\n", excerpt)
			}
		}
	}

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
	if args.ClusterWide == "true" {
		return r.handleListRoomsCluster(args)
	}

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
	// Collect room IDs for batch queries
	roomIDs := make([]string, len(rooms))
	for i, rm := range rooms {
		roomIDs[i] = rm.ID
	}
	msgCounts := r.Server.GetMessageCounts()
	pinnedExcerpts := r.Server.GetPinnedExcerpts(roomIDs)
	latestIDs := r.Server.GetLatestMessageIDs()

	useVerbose := args.Verbose == "true" || args.Compact == "false"
	if !useVerbose {
		for _, rm := range rooms {
			topic := rm.Description
			if len(topic) > 60 {
				topic = topic[:60] + "..."
			}
			project := rm.Project
			if project == "" {
				project = "-"
			}
			count := msgCounts[rm.ID]
			cursor := ""
			if lid, ok := latestIDs[rm.ID]; ok {
				cursor = fmt.Sprintf(" | cursor:%.8s", lid)
			}
			if excerpt, ok := pinnedExcerpts[rm.ID]; ok {
				fmt.Fprintf(&b, "- **%s** | %s | %s | %d msgs%s | 📌 %s | %s | %s\n", rm.ID, project, rm.Status, count, cursor, excerpt, topic, rm.UpdatedAt.Format("2006-01-02 15:04"))
			} else {
				fmt.Fprintf(&b, "- **%s** | %s | %s | %d msgs%s | %s | %s\n", rm.ID, project, rm.Status, count, cursor, topic, rm.UpdatedAt.Format("2006-01-02 15:04"))
			}
		}
	} else {
		for _, rm := range rooms {
			fmt.Fprintf(&b, "- **%s** [%s]", rm.ID, rm.Status)
			if rm.Project != "" {
				fmt.Fprintf(&b, " | project: %s", rm.Project)
			}
			if rm.Tags != "" {
				fmt.Fprintf(&b, " | tags: %s", rm.Tags)
			}
			fmt.Fprintf(&b, "\n  %s\n", rm.Description)
			if excerpt, ok := pinnedExcerpts[rm.ID]; ok {
				fmt.Fprintf(&b, "  📌 %s\n", excerpt)
			}
			if rm.TechStack != "" {
				fmt.Fprintf(&b, "  Tech: %s\n", rm.TechStack)
			}
			if rm.RelatedRooms != "" {
				fmt.Fprintf(&b, "  Related: %s\n", rm.RelatedRooms)
			}
			if lid, ok := latestIDs[rm.ID]; ok {
				fmt.Fprintf(&b, "  Latest msg: %.8s\n", lid)
			}
			fmt.Fprintf(&b, "  Last activity: %s\n", rm.UpdatedAt.Format("2006-01-02 15:04:05"))
		}
	}

	text := b.String()
	return msg(text)
}

func (r *Registry) handleRoomStats(ctx context.Context, req *mcp.CallToolRequest, args RoomStatsInput) (*mcp.CallToolResult, ToolOutput, error) {
	if args.ClusterWide == "true" {
		return r.handleRoomStatsCluster(args)
	}

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
	if stats.LatestMessageID != "" {
		fmt.Fprintf(&b, "**Latest message ID:** %.8s\n", stats.LatestMessageID)
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
