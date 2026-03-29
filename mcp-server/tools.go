package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolOutput is the structured output type for tool results.
type ToolOutput struct {
	Message string `json:"message"`
}

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

// PostToRoomInput represents the parameters for posting a message.
type PostToRoomInput struct {
	RoomID      string `json:"room_id"`
	Author      string `json:"author"`
	Message     string `json:"message"`
	MessageType string `json:"message_type"`
	ReplyTo     string `json:"reply_to"`
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

// ReadTranscriptInput represents the parameters for reading a room transcript.
type ReadTranscriptInput struct {
	RoomID string `json:"room_id"`
}

// SearchMessagesInput represents the parameters for searching messages.
type SearchMessagesInput struct {
	Query       string `json:"query"`
	Author      string `json:"author"`
	MessageType string `json:"message_type"`
	RoomID      string `json:"room_id"`
	Limit       string `json:"limit"`
}

// RoomStatsInput represents the parameters for getting room statistics.
type RoomStatsInput struct {
	RoomID string `json:"room_id"`
}

// DeleteMessagesInput represents the parameters for deleting messages.
type DeleteMessagesInput struct {
	MessageIDs string `json:"message_ids"`
}

// ArchiveRoomInput represents the parameters for archiving a room.
type ArchiveRoomInput struct {
	RoomID string `json:"room_id"`
	Delete string `json:"delete"`
}

// GetMessagesInput represents the parameters for fetching messages by ID.
type GetMessagesInput struct {
	MessageIDs string `json:"message_ids"`
}

// ReadRecentInput represents the parameters for reading recent messages.
type ReadRecentInput struct {
	RoomID string `json:"room_id"`
	Limit  string `json:"limit"`
}

var validMessageTypes = map[string]bool{
	"message":  true,
	"thought":  true,
	"decision": true,
	"code":     true,
	"review":   true,
	"action":   true,
	"critique": true,
}

// schema builds a JSON Schema object with additionalProperties: true.
// required lists the field names that are mandatory; all others are optional.
func schema(required []string, props map[string]map[string]any) map[string]any {
	s := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": true,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func prop(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

func registerTools(cs *CouncilServer) {
	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "create_room",
		Description: "Create a new council room (virtual workspace) for a topic or task. Does nothing if the room already exists.",
		InputSchema: schema([]string{"id"}, map[string]map[string]any{
			"id":            prop("string", "Unique room identifier (e.g. auth-migration-v2)"),
			"topic":         prop("string", "What this room is about"),
			"project":       prop("string", "Project grouping for filtering"),
			"tech_stack":    prop("string", "Technologies involved"),
			"tags":          prop("string", "Comma-separated labels"),
			"system_prompt": prop("string", "Instructions injected into transcripts for LLM context"),
			"related_rooms": prop("string", "Comma-separated IDs of related rooms for cross-referencing"),
		}),
	}, cs.handleCreateRoom)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "post_to_room",
		Description: "Post a message, thought, critique, or code snippet to a council room's ledger.",
		InputSchema: schema([]string{"room_id", "author", "message"}, map[string]map[string]any{
			"room_id":      prop("string", "Target room ID"),
			"author":       prop("string", "Name of the posting agent"),
			"message":      prop("string", "Message content (markdown supported)"),
			"message_type": prop("string", "One of: message, thought, decision, code, review, action, critique (default: message)"),
			"reply_to":     prop("string", "Message ID this is a reply to (e.g. 42). Renders as 're: #42' in transcripts"),
		}),
	}, cs.handlePostToRoom)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "signal_status",
		Description: "Update a room's status to coordinate work between agents (active, paused, or resolved).",
		InputSchema: schema([]string{"room_id", "status"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
			"status":  prop("string", "One of: active, paused, resolved"),
		}),
	}, cs.handleSignalStatus)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "list_rooms",
		Description: "List council rooms, optionally filtered by project, tag, or status. Returns room metadata sorted by recent activity.",
		InputSchema: schema(nil, map[string]map[string]any{
			"project": prop("string", "Filter by project name"),
			"tag":     prop("string", "Filter by tag"),
			"status":  prop("string", "Filter by status (active, paused, resolved)"),
		}),
	}, cs.handleListRooms)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "update_room",
		Description: "Update a room's metadata. Only provided fields are changed; omitted fields are left unchanged.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id":       prop("string", "Target room ID"),
			"topic":         prop("string", "New topic/description"),
			"project":       prop("string", "New project grouping"),
			"tech_stack":    prop("string", "New tech stack"),
			"tags":          prop("string", "New comma-separated tags"),
			"system_prompt": prop("string", "New system prompt"),
			"related_rooms": prop("string", "Comma-separated IDs of related rooms"),
		}),
	}, cs.handleUpdateRoom)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "read_room",
		Description: "Read a room's metadata (topic, project, tech_stack, tags, status, system_prompt) without loading messages.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
		}),
	}, cs.handleReadRoom)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "delete_room",
		Description: "Permanently delete a council room and all its messages. This cannot be undone.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
		}),
	}, cs.handleDeleteRoom)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "search_messages",
		Description: "Search messages across rooms. All filter params are optional — omit or leave empty to skip. Returns snippets with message IDs; use get_messages to fetch full content.",
		InputSchema: schema(nil, map[string]map[string]any{
			"query":        prop("string", "Text to search for in message content"),
			"author":       prop("string", "Filter by author name"),
			"message_type": prop("string", "Filter by type (message, thought, decision, code, review, action, critique)"),
			"room_id":      prop("string", "Scope search to a specific room"),
			"limit":        prop("string", "Max results to return (default 20, max 100)"),
		}),
	}, cs.handleSearchMessages)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "get_messages",
		Description: "Fetch the full content of specific messages by their IDs. Use this after search_messages to retrieve complete message text without loading entire transcripts.",
		InputSchema: schema([]string{"message_ids"}, map[string]map[string]any{
			"message_ids": prop("string", "Comma-separated message IDs (e.g. 48,52,55)"),
		}),
	}, cs.handleGetMessages)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "read_recent",
		Description: "Read the last N messages from a room (default 10, max 50). More token-efficient than read_transcript for long-running rooms.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
			"limit":   prop("string", "Number of recent messages to return (default 10, max 50)"),
		}),
	}, cs.handleReadRecent)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "room_stats",
		Description: "Get statistics for a room: message count, participants with message counts, and activity timestamps.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
		}),
	}, cs.handleRoomStats)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "delete_messages",
		Description: "Delete specific messages by their IDs. Provide a comma-separated list of message IDs.",
		InputSchema: schema([]string{"message_ids"}, map[string]map[string]any{
			"message_ids": prop("string", "Comma-separated message IDs to delete"),
		}),
	}, cs.handleDeleteMessages)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "archive_room",
		Description: "Export a room's transcript to a markdown file in the archives directory. Optionally delete the room after archiving.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
			"delete":  prop("string", "Set to 'true' to delete room after archiving"),
		}),
	}, cs.handleArchiveRoom)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "read_transcript",
		Description: "Read the full transcript of a council room. Returns the prompt-optimized markdown with room metadata, system instructions, and all messages.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
		}),
	}, cs.handleReadTranscript)
}

func (cs *CouncilServer) handleCreateRoom(ctx context.Context, req *mcp.CallToolRequest, args CreateRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.ID == "" {
		return msg("Error: room id is required.")
	}

	if err := cs.createRoom(args.ID, args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt, args.RelatedRooms); err != nil {
		cs.logger.Error("Failed to create room", "id", args.ID, "error", err)
		return nil, ToolOutput{}, err
	}

	cs.logger.Info("Room created", "id", args.ID, "project", args.Project, "topic", args.Topic)
	return msg(fmt.Sprintf("Room '%s' is ready. Topic: %s", args.ID, args.Topic))
}

func (cs *CouncilServer) handlePostToRoom(ctx context.Context, req *mcp.CallToolRequest, args PostToRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" || args.Author == "" || args.Message == "" {
		return msg("Error: room_id, author, and message are all required.")
	}

	if args.MessageType == "" {
		args.MessageType = "message"
	}
	if !validMessageTypes[args.MessageType] {
		return msg(fmt.Sprintf("Error: Invalid message_type '%s'. Must be one of: message, thought, decision, code, review, action, critique.", args.MessageType))
	}

	// Verify room exists
	if _, err := cs.getRoom(args.RoomID); err != nil {
		return msg(fmt.Sprintf("Error: Room '%s' not found. Create it first with create_room.", args.RoomID))
	}

	var replyTo int64
	if args.ReplyTo != "" {
		if _, err := fmt.Sscanf(args.ReplyTo, "%d", &replyTo); err != nil {
			return msg(fmt.Sprintf("Error: reply_to '%s' is not a valid message ID.", args.ReplyTo))
		}
	}

	msgID, err := cs.postMessage(args.RoomID, args.Author, args.Message, args.MessageType, replyTo)
	if err != nil {
		cs.logger.Error("Failed to post message", "room_id", args.RoomID, "error", err)
		return nil, ToolOutput{}, err
	}

	cs.logger.Info("Message posted", "room_id", args.RoomID, "author", args.Author, "type", args.MessageType, "msg_id", msgID)
	return msg(fmt.Sprintf("Message #%d posted to room '%s' by %s.", msgID, args.RoomID, args.Author))
}

func (cs *CouncilServer) handleSignalStatus(ctx context.Context, req *mcp.CallToolRequest, args SignalStatusInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	validStatuses := map[string]bool{"active": true, "paused": true, "resolved": true}
	if !validStatuses[args.Status] {
		return msg(fmt.Sprintf("Error: Invalid status '%s'. Must be one of: active, paused, resolved.", args.Status))
	}

	if err := cs.updateStatus(args.RoomID, args.Status); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	cs.logger.Info("Status updated", "room_id", args.RoomID, "status", args.Status)
	return msg(fmt.Sprintf("Room '%s' status updated to '%s'.", args.RoomID, args.Status))
}

func (cs *CouncilServer) handleListRooms(ctx context.Context, req *mcp.CallToolRequest, args ListRoomsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	rooms, err := cs.listRooms(args.Project, args.Tag, args.Status)
	if err != nil {
		cs.logger.Error("Failed to list rooms", "error", err)
		return nil, ToolOutput{}, err
	}

	if len(rooms) == 0 {
		return msg("No rooms found matching the given filters.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d room(s):\n\n", len(rooms))
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

	text := b.String()
	return msg(text)
}

func (cs *CouncilServer) handleUpdateRoom(ctx context.Context, req *mcp.CallToolRequest, args UpdateRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
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

	if err := cs.updateRoom(args.RoomID, args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt, args.RelatedRooms); err != nil {
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

	cs.logger.Info("Room updated", "room_id", args.RoomID, "fields", strings.Join(updated, ", "))
	return msg(fmt.Sprintf("Room '%s' updated: %s.", args.RoomID, strings.Join(updated, ", ")))
}

func (cs *CouncilServer) handleReadRoom(ctx context.Context, req *mcp.CallToolRequest, args ReadRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	room, err := cs.getRoom(args.RoomID)
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

func (cs *CouncilServer) handleDeleteRoom(ctx context.Context, req *mcp.CallToolRequest, args DeleteRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	if err := cs.deleteRoom(args.RoomID); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	cs.logger.Info("Room deleted", "room_id", args.RoomID)
	return msg(fmt.Sprintf("Room '%s' and all its messages have been permanently deleted.", args.RoomID))
}

func (cs *CouncilServer) handleSearchMessages(ctx context.Context, req *mcp.CallToolRequest, args SearchMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.Query == "" && args.Author == "" && args.MessageType == "" && args.RoomID == "" {
		return msg("Error: at least one search filter is required (query, author, message_type, or room_id).")
	}

	limit := 20
	if args.Limit != "" {
		if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
			limit = 20
		}
	}

	messages, err := cs.searchMessages(args.Query, args.Author, args.MessageType, args.RoomID, limit)
	if err != nil {
		cs.logger.Error("Failed to search messages", "error", err)
		return nil, ToolOutput{}, err
	}

	if len(messages) == 0 {
		return msg("No messages found matching the given filters.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s):\n\n", len(messages))
	for _, m := range messages {
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		snippet := m.Content
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		fmt.Fprintf(&b, "- **#%d** [%s] %s in **%s** (%s):\n  %s\n\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, snippet)
	}

	return msg(b.String())
}

func (cs *CouncilServer) handleGetMessages(ctx context.Context, req *mcp.CallToolRequest, args GetMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.MessageIDs == "" {
		return msg("Error: message_ids is required (comma-separated list of message IDs).")
	}

	parts := strings.Split(args.MessageIDs, ",")
	var ids []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var id int64
		if _, err := fmt.Sscanf(p, "%d", &id); err != nil {
			return msg(fmt.Sprintf("Error: '%s' is not a valid message ID.", p))
		}
		ids = append(ids, id)
	}

	messages, err := cs.getMessagesByIDs(ids)
	if err != nil {
		cs.logger.Error("Failed to get messages", "error", err)
		return nil, ToolOutput{}, err
	}

	if len(messages) == 0 {
		return msg("No messages found with the given IDs.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s):\n\n", len(messages))
	for _, m := range messages {
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		fmt.Fprintf(&b, "---\n**#%d** [%s] %s in **%s** (%s):\n\n%s\n\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, m.Content)
	}

	return msg(b.String())
}

func (cs *CouncilServer) handleReadRecent(ctx context.Context, req *mcp.CallToolRequest, args ReadRecentInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	limit := 10
	if args.Limit != "" {
		if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
			limit = 10
		}
	}

	messages, err := cs.getRecentMessages(args.RoomID, limit)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	room, _ := cs.getRoom(args.RoomID)

	var b strings.Builder
	fmt.Fprintf(&b, "# %s [%s] — last %d message(s)\n", room.ID, room.Status, len(messages))
	if room.Description != "" {
		fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
	}
	b.WriteString("---\n")

	for _, m := range messages {
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		replyTag := ""
		if m.ReplyTo > 0 {
			replyTag = fmt.Sprintf(", re: #%d", m.ReplyTo)
		}
		if m.IsSummary {
			fmt.Fprintf(&b, "\n**[%s] SUMMARY:**\n%s\n", ts, m.Content)
		} else if m.MessageType != "" && m.MessageType != "message" {
			fmt.Fprintf(&b, "\n**[%s] %s (%s%s):**\n%s\n", ts, m.Author, m.MessageType, replyTag, m.Content)
		} else if m.ReplyTo > 0 {
			fmt.Fprintf(&b, "\n**[%s] %s (re: #%d):**\n%s\n", ts, m.Author, m.ReplyTo, m.Content)
		} else {
			fmt.Fprintf(&b, "\n**[%s] %s:**\n%s\n", ts, m.Author, m.Content)
		}
	}

	return msg(b.String())
}

func (cs *CouncilServer) handleRoomStats(ctx context.Context, req *mcp.CallToolRequest, args RoomStatsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	stats, err := cs.getRoomStats(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%s** [%s]\n", stats.RoomID, stats.Status)
	fmt.Fprintf(&b, "**Messages:** %d\n", stats.MessageCount)

	if len(stats.Participants) > 0 {
		var parts []string
		for author, count := range stats.Participants {
			parts = append(parts, fmt.Sprintf("%s (%d)", author, count))
		}
		fmt.Fprintf(&b, "**Participants:** %s\n", strings.Join(parts, ", "))
		fmt.Fprintf(&b, "**First message:** %s\n", stats.FirstMessage.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&b, "**Last message:** %s\n", stats.LastMessage.Format("2006-01-02 15:04:05"))
	}

	return msg(b.String())
}

func (cs *CouncilServer) handleDeleteMessages(ctx context.Context, req *mcp.CallToolRequest, args DeleteMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.MessageIDs == "" {
		return msg("Error: message_ids is required (comma-separated list of message IDs).")
	}

	parts := strings.Split(args.MessageIDs, ",")
	var ids []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var id int64
		if _, err := fmt.Sscanf(p, "%d", &id); err != nil {
			return msg(fmt.Sprintf("Error: '%s' is not a valid message ID.", p))
		}
		ids = append(ids, id)
	}

	count, err := cs.deleteMessages(ids)
	if err != nil {
		cs.logger.Error("Failed to delete messages", "error", err)
		return nil, ToolOutput{}, err
	}

	cs.logger.Info("Messages deleted", "count", count, "ids", args.MessageIDs)
	return msg(fmt.Sprintf("Deleted %d message(s).", count))
}

func (cs *CouncilServer) handleArchiveRoom(ctx context.Context, req *mcp.CallToolRequest, args ArchiveRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	archivePath, err := cs.archiveRoom(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	result := fmt.Sprintf("Room '%s' archived to %s.", args.RoomID, archivePath)

	if args.Delete == "true" {
		if err := cs.deleteRoom(args.RoomID); err != nil {
			return msg(fmt.Sprintf("Archived successfully but failed to delete: %s", err.Error()))
		}
		result += " Room and messages deleted."
		cs.logger.Info("Room archived and deleted", "room_id", args.RoomID, "path", archivePath)
	} else {
		cs.logger.Info("Room archived", "room_id", args.RoomID, "path", archivePath)
	}

	return msg(result)
}

func (cs *CouncilServer) handleReadTranscript(ctx context.Context, req *mcp.CallToolRequest, args ReadTranscriptInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	room, err := cs.getRoom(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: Room '%s' not found.", args.RoomID))
	}

	messages, err := cs.getTranscript(args.RoomID)
	if err != nil {
		cs.logger.Error("Failed to get transcript", "room_id", args.RoomID, "error", err)
		return nil, ToolOutput{}, err
	}

	transcript := formatTranscript(room, messages)
	return msg(transcript)
}
