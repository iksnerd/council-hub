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

// UpdateRoomInput represents the parameters for updating a room's metadata.
type UpdateRoomInput struct {
	RoomID       string `json:"room_id"`
	RoomIDs      string `json:"room_ids"`
	WhereProject string `json:"where_project"`
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
	IncludeLastN            string `json:"include_last_n"`
}

// DeleteRoomInput represents the parameters for deleting a room.
type DeleteRoomInput struct {
	RoomID string `json:"room_id"`
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
		seen[args.RoomID] = true
		ids = append(ids, args.RoomID)
	}

	// where_project expands the target set to every room currently in that project
	if args.WhereProject != "" {
		matched, listErr := r.Server.ListRooms(args.WhereProject, "", "", "", 100, 0)
		if listErr != nil {
			return msg(fmt.Sprintf("Error: failed to expand where_project: %s", listErr.Error()))
		}
		for _, rm := range matched {
			if !seen[rm.ID] {
				seen[rm.ID] = true
				ids = append(ids, rm.ID)
			}
		}
	}

	if len(ids) == 0 {
		return msg("Error: room_id, room_ids, or where_project is required.")
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

	// Append recent messages if requested
	if args.IncludeLastN != "" {
		lastN := 0
		_, _ = fmt.Sscanf(args.IncludeLastN, "%d", &lastN)
		if lastN > 50 {
			lastN = 50
		}
		if lastN > 0 {
			messages, _ := r.Server.GetRecentMessages(args.RoomID, lastN)
			if len(messages) > 0 {
				fmt.Fprintf(&b, "\n---\n**Recent messages (%d):**\n", len(messages))
				for _, m := range messages {
					ts := m.Timestamp.Format("2006-01-02 15:04:05")
					if m.MessageType != "" && m.MessageType != "message" {
						fmt.Fprintf(&b, "\n**[#%.8s %s] %s (%s):**\n%s\n", m.ID, ts, m.Author, m.MessageType, m.Content)
					} else {
						fmt.Fprintf(&b, "\n**[#%.8s %s] %s:**\n%s\n", m.ID, ts, m.Author, m.Content)
					}
				}
			}
		}
	}

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
