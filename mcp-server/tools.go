package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolResultText extracts the text content from a CallToolResult.
func toolResultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	if tc, ok := r.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

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

// ReadTranscriptInput represents the parameters for reading a room transcript.
type ReadTranscriptInput struct {
	RoomID         string `json:"room_id"`
	RoomIDs        string `json:"room_ids"`
	LastN          string `json:"last_n"`
	AfterID        string `json:"after_id"`
	Mode           string `json:"mode"`
	IncludeRelated string `json:"include_related"`
}

// DigestInput represents the parameters for the project digest tool.
type DigestInput struct {
	Project string `json:"project"`
	Since   string `json:"since"`
}

// SearchMessagesInput represents the parameters for searching messages.
type SearchMessagesInput struct {
	Query       string `json:"query"`
	Author      string `json:"author"`
	MessageType string `json:"message_type"`
	RoomID      string `json:"room_id"`
	Project     string `json:"project"`
	Limit       string `json:"limit"`
	SummaryOnly string `json:"summary_only"`
}

// RoomStatsInput represents the parameters for getting room statistics.
type RoomStatsInput struct {
	RoomID string `json:"room_id"`
}

// PinMessageInput represents the parameters for pinning/unpinning a message.
type PinMessageInput struct {
	RoomID    string `json:"room_id"`
	MessageID string `json:"message_id"`
}

// UpdateMessageInput represents the parameters for editing a message in-place.
type UpdateMessageInput struct {
	MessageID   string `json:"message_id"`
	Content     string `json:"content"`
	MessageType string `json:"message_type"`
}

// DeleteMessagesInput represents the parameters for deleting messages.
type DeleteMessagesInput struct {
	MessageIDs string `json:"message_ids"`
	DryRun     string `json:"dry_run"`
}

// ArchiveRoomInput represents the parameters for archiving a room.
type ArchiveRoomInput struct {
	RoomID string `json:"room_id"`
	Delete string `json:"delete"`
}

// GetMessagesInput represents the parameters for fetching messages by ID or by room.
type GetMessagesInput struct {
	MessageIDs string `json:"message_ids"`
	RoomID     string `json:"room_id"`
	LastN      string `json:"last_n"`
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
		Description: "Create a new council room (virtual workspace) for a topic or task. Does nothing if the room already exists. Related rooms are automatically linked in both directions.",
		InputSchema: schema([]string{"id"}, map[string]map[string]any{
			"id":            prop("string", "Unique room identifier (e.g. auth-migration-v2)"),
			"topic":         prop("string", "What this room is about"),
			"project":       prop("string", "Project grouping for filtering"),
			"tech_stack":    prop("string", "Technologies involved"),
			"tags":          prop("string", "Comma-separated labels"),
			"system_prompt": prop("string", "Instructions injected into transcripts for LLM context"),
			"related_rooms": prop("string", "Comma-separated IDs of related rooms — bidirectional: linked rooms automatically link back"),
		}),
	}, cs.handleCreateRoom)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "get_or_create_room",
		Description: "Get an existing room (with recent messages) or create it if it doesn't exist. Saves 2-3 round trips vs list_rooms → check → read_recent or create_room.",
		InputSchema: schema([]string{"id"}, map[string]map[string]any{
			"id":            prop("string", "Room identifier — returns existing room if found, creates if not"),
			"topic":         prop("string", "Topic (used only when creating)"),
			"project":       prop("string", "Project grouping (used only when creating)"),
			"tech_stack":    prop("string", "Technologies (used only when creating)"),
			"tags":          prop("string", "Comma-separated labels (used only when creating)"),
			"system_prompt": prop("string", "Instructions (used only when creating)"),
			"related_rooms": prop("string", "Comma-separated related room IDs (used only when creating)"),
			"last_n":        prop("string", "Number of recent messages to return for existing rooms (default 5, max 50)"),
		}),
	}, cs.handleGetOrCreateRoom)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "post_to_room",
		Description: "Post a message, thought, critique, or code snippet to a council room's ledger. Returns JSON with message_id and latest_message_id for delta-read cursor tracking via read_transcript(after_id).",
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
		Name:        "bulk_status_update",
		Description: "Update the status of multiple rooms in one call. Optionally post a closing message to each room. Useful for closing out a sprint or batch-resolving rooms.",
		InputSchema: schema([]string{"room_ids", "status"}, map[string]map[string]any{
			"room_ids": prop("string", "Comma-separated room IDs (e.g. bug-123,bug-456,feature-x)"),
			"status":   prop("string", "One of: active, paused, resolved"),
			"message":  prop("string", "Optional closing message to post to each room before updating status"),
			"author":   prop("string", "Author name for the closing message (required if message is provided)"),
		}),
	}, cs.handleBulkStatusUpdate)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "list_rooms",
		Description: "List council rooms, optionally filtered by project, tag, status, or keyword search. Returns compact one-line-per-room format by default (saves ~60-80% tokens vs verbose). Set verbose=true for full metadata.",
		InputSchema: schema(nil, map[string]map[string]any{
			"project": prop("string", "Filter by project name"),
			"tag":     prop("string", "Filter by tag"),
			"status":  prop("string", "Filter by status (active, paused, resolved)"),
			"search":  prop("string", "Keyword search across room ID, topic/description, and tags"),
			"verbose": prop("string", "Set to 'true' for full metadata per room (system_prompt, tech_stack, tags, related_rooms)"),
		}),
	}, cs.handleListRooms)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "update_room",
		Description: "Update a room's metadata. Only provided fields are changed; omitted fields are left unchanged. Returns the full updated room state. Related rooms are bidirectionally linked.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id":       prop("string", "Target room ID"),
			"topic":         prop("string", "New topic/description"),
			"project":       prop("string", "New project grouping"),
			"tech_stack":    prop("string", "New tech stack"),
			"tags":          prop("string", "New comma-separated tags"),
			"system_prompt": prop("string", "New system prompt"),
			"related_rooms": prop("string", "Comma-separated IDs of related rooms — bidirectional: linked rooms automatically link back"),
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
		Description: "Search messages across rooms. All filter params are optional — omit or leave empty to skip. Returns snippets with message IDs; use get_messages to fetch full content. Use summary_only=true for compact results (id, author, timestamp, 120-char excerpt).",
		InputSchema: schema(nil, map[string]map[string]any{
			"query":        prop("string", "Text to search for in message content"),
			"author":       prop("string", "Filter by author name"),
			"message_type": prop("string", "Filter by type: message, thought, decision, code, review, action, critique"),
			"room_id":      prop("string", "Scope search to a specific room"),
			"project":      prop("string", "Scope search to rooms in this project"),
			"limit":        prop("string", "Max results to return (default 20, max 100)"),
			"summary_only": prop("string", "Set to 'true' for compact output: id, author, timestamp, room, type, and 120-char excerpt"),
		}),
	}, cs.handleSearchMessages)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "get_messages",
		Description: "Fetch specific messages by ID, or browse a room's recent messages. Best for: retrieving search results by ID, or getting raw message content without room headers. For formatted transcripts with room context, use read_transcript instead.",
		InputSchema: schema(nil, map[string]map[string]any{
			"message_ids": prop("string", "Comma-separated message IDs (e.g. 48,52,55)"),
			"room_id":     prop("string", "Browse messages from this room (alternative to message_ids)"),
			"last_n":      prop("string", "Number of recent messages to fetch when using room_id (default 10, max 50)"),
		}),
	}, cs.handleGetMessages)

mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "room_stats",
		Description: "Get lightweight statistics for a room: message count, latest_message_id (for after_id cursor), participants with per-author counts, type breakdown, and first/last activity timestamps. Use before read_transcript to decide whether to read.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
		}),
	}, cs.handleRoomStats)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "update_message",
		Description: "Edit a message's content in-place. Useful for maintaining living status tables or correcting mistakes. Preserves author, timestamp, room, and other fields.",
		InputSchema: schema([]string{"message_id", "content"}, map[string]map[string]any{
			"message_id":   prop("string", "ID of the message to update"),
			"content":      prop("string", "New message content (replaces existing)"),
			"message_type": prop("string", "Optionally change message type (message, thought, decision, code, review, action, critique)"),
		}),
	}, cs.handleUpdateMessage)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "pin_message",
		Description: "Pin a message as the living TL;DR for a room. Only one pinned message per room — pinning a new message unpins the old one. Pinning an already-pinned message unpins it (toggle). Pinned messages appear first in transcripts.",
		InputSchema: schema([]string{"room_id", "message_id"}, map[string]map[string]any{
			"room_id":    prop("string", "Target room ID"),
			"message_id": prop("string", "ID of the message to pin/unpin"),
		}),
	}, cs.handlePinMessage)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "delete_messages",
		Description: "Delete specific messages by their IDs. Provide a comma-separated list of message IDs. Use dry_run=true to preview what would be deleted without actually deleting.",
		InputSchema: schema([]string{"message_ids"}, map[string]map[string]any{
			"message_ids": prop("string", "Comma-separated message IDs to delete"),
			"dry_run":     prop("string", "Set to 'true' to preview deletions without executing. Returns message details (id, author, timestamp, room, excerpt)."),
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
		Description: "Read a room's transcript with full context (room header, system_prompt, pinned message, messages). The primary tool for reading rooms. Supports: last_n for recent messages, after_id for delta reads (includes pinned message for context), mode=summary for orientation (pinned + latest per type), mode=changelog for decisions+actions only. Use room_ids for batch multi-room reads. Use include_related=true to auto-append related room summaries.",
		InputSchema: schema(nil, map[string]map[string]any{
			"room_id":         prop("string", "Target room ID (use this OR room_ids, not both)"),
			"room_ids":        prop("string", "Comma-separated room IDs for batch reads (e.g. room-a,room-b,room-c). Each room rendered with the same mode/last_n settings."),
			"last_n":          prop("string", "Return only the last N messages (default: all). Keeps room header and system prompt."),
			"after_id":        prop("string", "Return only messages with ID greater than this value. For delta reads after context compaction."),
			"mode":            prop("string", "Set to 'summary' for system_prompt + latest per type, or 'changelog' for only decision + action messages chronologically (ideal for PR descriptions/release notes)."),
			"include_related": prop("string", "Set to 'true' to append a summary of each related room after the main transcript. Resolves related_rooms automatically."),
		}),
	}, cs.handleReadTranscript)

	mcp.AddTool(cs.mcp, &mcp.Tool{
		Name:        "get_digest",
		Description: "Get a project activity digest showing rooms with new messages since a given timestamp. Returns room ID, new message count, and latest message excerpt per room. Perfect for start-of-session 'what changed?' orientation.",
		InputSchema: schema([]string{"since"}, map[string]map[string]any{
			"project": prop("string", "Filter to rooms in this project (optional — omit for all projects)"),
			"since":   prop("string", "ISO timestamp (e.g. 2026-03-31T12:00:00). Returns only rooms with messages after this time."),
		}),
	}, cs.handleGetDigest)
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

func (cs *CouncilServer) handleGetOrCreateRoom(ctx context.Context, req *mcp.CallToolRequest, args GetOrCreateRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.ID == "" {
		return msg("Error: id is required.")
	}

	// Try to get existing room
	room, err := cs.getRoom(args.ID)
	created := false
	if err != nil {
		// Room doesn't exist — create it
		if err := cs.createRoom(args.ID, args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt, args.RelatedRooms); err != nil {
			cs.logger.Error("Failed to create room", "id", args.ID, "error", err)
			return nil, ToolOutput{}, err
		}
		room, _ = cs.getRoom(args.ID)
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
		messages, _ := cs.getRecentMessages(args.ID, limit)
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

	cs.logger.Info("get_or_create_room", "id", args.ID, "created", created)
	return msg(b.String())
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
	return msg(fmt.Sprintf("Message #%d posted to room '%s' by %s.\n\n```json\n{\"message_id\": %d, \"room_id\": \"%s\", \"latest_message_id\": %d}\n```", msgID, args.RoomID, args.Author, msgID, args.RoomID, msgID))
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

	var b strings.Builder
	fmt.Fprintf(&b, "Room '%s' status → **%s**.", args.RoomID, args.Status)
	if room, err := cs.getRoom(args.RoomID); err == nil {
		if room.Description != "" {
			fmt.Fprintf(&b, "\n**Topic:** %s", room.Description)
		}
		if room.Project != "" {
			fmt.Fprintf(&b, "\n**Project:** %s", room.Project)
		}
	}
	return msg(b.String())
}

func (cs *CouncilServer) handleBulkStatusUpdate(ctx context.Context, req *mcp.CallToolRequest, args BulkStatusInput) (*mcp.CallToolResult, ToolOutput, error) {
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
			cs.postMessage(roomID, args.Author, args.Message, "decision", 0)
		}
		if err := cs.updateStatus(roomID, args.Status); err != nil {
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

	cs.logger.Info("Bulk status update", "status", args.Status, "updated", len(updated), "not_found", len(notFound))
	return msg(b.String())
}

func (cs *CouncilServer) handleListRooms(ctx context.Context, req *mcp.CallToolRequest, args ListRoomsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	rooms, err := cs.listRooms(args.Project, args.Tag, args.Status, args.Search)
	if err != nil {
		cs.logger.Error("Failed to list rooms", "error", err)
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
		msgCounts := cs.getMessageCounts()

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

	var b strings.Builder
	fmt.Fprintf(&b, "Room '%s' updated: %s.", args.RoomID, strings.Join(updated, ", "))
	if room, err := cs.getRoom(args.RoomID); err == nil {
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

	if args.Query == "" && args.Author == "" && args.MessageType == "" && args.RoomID == "" && args.Project == "" {
		return msg("Error: at least one search filter is required (query, author, message_type, room_id, or project).")
	}

	limit := 20
	if args.Limit != "" {
		if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
			limit = 20
		}
	}

	messages, err := cs.searchMessages(args.Query, args.Author, args.MessageType, args.RoomID, args.Project, limit)
	if err != nil {
		cs.logger.Error("Failed to search messages", "error", err)
		return nil, ToolOutput{}, err
	}

	if len(messages) == 0 {
		return msg("No messages found matching the given filters.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s):\n\n", len(messages))

	if args.SummaryOnly == "true" {
		for _, m := range messages {
			ts := m.Timestamp.Format("2006-01-02 15:04")
			excerpt := m.Content
			if len(excerpt) > 120 {
				excerpt = excerpt[:120]
				if i := strings.LastIndex(excerpt, " "); i > 80 {
					excerpt = excerpt[:i]
				}
				excerpt += "..."
			}
			// Replace newlines in excerpt for single-line display
			excerpt = strings.ReplaceAll(excerpt, "\n", " ")
			fmt.Fprintf(&b, "- #%d | %s | %s | %s | %s | %s\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, excerpt)
		}
	} else {
		for _, m := range messages {
			ts := m.Timestamp.Format("2006-01-02 15:04:05")
			snippet := m.Content
			if len(snippet) > 300 {
				snippet = snippet[:300] + "..."
			}
			fmt.Fprintf(&b, "- **#%d** [%s] %s in **%s** (%s):\n  %s\n\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, snippet)
		}
	}

	return msg(b.String())
}

func (cs *CouncilServer) handleGetMessages(ctx context.Context, req *mcp.CallToolRequest, args GetMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	var messages []Message

	if args.MessageIDs != "" {
		// Mode 1: fetch by explicit IDs
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

		var err error
		messages, err = cs.getMessagesByIDs(ids)
		if err != nil {
			cs.logger.Error("Failed to get messages", "error", err)
			return nil, ToolOutput{}, err
		}
	} else if args.RoomID != "" {
		// Mode 2: browse room messages by last_n
		limit := 10
		if args.LastN != "" {
			if _, err := fmt.Sscanf(args.LastN, "%d", &limit); err != nil {
				limit = 10
			}
		}
		if limit <= 0 {
			limit = 10
		}
		if limit > 50 {
			limit = 50
		}

		var err error
		messages, err = cs.getRecentMessages(args.RoomID, limit)
		if err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
	} else {
		return msg("Error: provide either message_ids or room_id.")
	}

	if len(messages) == 0 {
		return msg("No messages found.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s):\n\n", len(messages))
	for _, m := range messages {
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		fmt.Fprintf(&b, "---\n**#%d** [%s] %s in **%s** (%s):\n\n%s\n\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, m.Content)
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

func (cs *CouncilServer) handlePinMessage(ctx context.Context, req *mcp.CallToolRequest, args PinMessageInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}
	if args.MessageID == "" {
		return msg("Error: message_id is required.")
	}

	var id int64
	if _, err := fmt.Sscanf(args.MessageID, "%d", &id); err != nil {
		return msg(fmt.Sprintf("Error: '%s' is not a valid message ID.", args.MessageID))
	}

	pinned, err := cs.pinMessage(args.RoomID, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return msg(fmt.Sprintf("Error: message #%d not found.", id))
		}
		cs.logger.Error("Failed to pin message", "id", id, "error", err)
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	if pinned {
		cs.logger.Info("Message pinned", "id", id, "room", args.RoomID)
		return msg(fmt.Sprintf("Message #%d pinned in room '%s'. It will appear first in transcripts.", id, args.RoomID))
	}
	cs.logger.Info("Message unpinned", "id", id, "room", args.RoomID)
	return msg(fmt.Sprintf("Message #%d unpinned in room '%s'.", id, args.RoomID))
}

func (cs *CouncilServer) handleUpdateMessage(ctx context.Context, req *mcp.CallToolRequest, args UpdateMessageInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.MessageID == "" {
		return msg("Error: message_id is required.")
	}
	if args.Content == "" {
		return msg("Error: content is required.")
	}

	var id int64
	if _, err := fmt.Sscanf(args.MessageID, "%d", &id); err != nil {
		return msg(fmt.Sprintf("Error: '%s' is not a valid message ID.", args.MessageID))
	}

	if args.MessageType != "" && !validMessageTypes[args.MessageType] {
		return msg(fmt.Sprintf("Error: invalid message_type '%s'. Valid types: message, thought, decision, code, review, action, critique.", args.MessageType))
	}

	m, err := cs.updateMessage(id, args.Content, args.MessageType)
	if err != nil {
		if err == sql.ErrNoRows {
			return msg(fmt.Sprintf("Error: message #%d not found.", id))
		}
		cs.logger.Error("Failed to update message", "id", id, "error", err)
		return nil, ToolOutput{}, err
	}

	cs.logger.Info("Message updated", "id", id, "room", m.RoomID)
	return msg(fmt.Sprintf("Message #%d updated. Author: %s, Room: %s, Type: %s.", m.ID, m.Author, m.RoomID, m.MessageType))
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

	if args.DryRun == "true" {
		msgs, err := cs.getMessagesByIDs(ids)
		if err != nil {
			cs.logger.Error("Failed to fetch messages for dry run", "error", err)
			return nil, ToolOutput{}, err
		}

		var b strings.Builder
		fmt.Fprintf(&b, "DRY RUN — %d message(s) would be deleted:\n\n", len(msgs))
		foundIDs := make(map[int64]bool)
		for _, m := range msgs {
			foundIDs[m.ID] = true
			excerpt := m.Content
			if len(excerpt) > 120 {
				excerpt = excerpt[:120] + "..."
			}
			excerpt = strings.ReplaceAll(excerpt, "\n", " ")
			fmt.Fprintf(&b, "  #%d | %s | %s | %s | %s\n",
				m.ID, m.Author, m.Timestamp.Format("2006-01-02 15:04:05"), m.RoomID, excerpt)
		}
		for _, id := range ids {
			if !foundIDs[id] {
				fmt.Fprintf(&b, "  #%d — not found\n", id)
			}
		}
		return msg(b.String())
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

	// Batch mode: room_ids takes precedence
	if args.RoomIDs != "" {
		ids := strings.Split(args.RoomIDs, ",")
		var combined strings.Builder
		for i, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if i > 0 {
				combined.WriteString("\n---\n\n")
			}
			singleArgs := ReadTranscriptInput{
				RoomID:  id,
				LastN:   args.LastN,
				AfterID: args.AfterID,
				Mode:    args.Mode,
			}
			result, _, err := cs.readSingleTranscript(singleArgs)
			if err != nil {
				fmt.Fprintf(&combined, "# %s — Error: %s\n", id, err.Error())
			} else {
				combined.WriteString(toolResultText(result))
			}
		}
		return msg(combined.String())
	}

	if args.RoomID == "" {
		return msg("Error: room_id or room_ids is required.")
	}

	result, output, err := cs.readSingleTranscript(args)
	if err != nil {
		return nil, output, err
	}

	// Append related room summaries if requested
	if args.IncludeRelated == "true" {
		room, roomErr := cs.getRoom(args.RoomID)
		if roomErr == nil && room.RelatedRooms != "" {
			var related strings.Builder
			related.WriteString(toolResultText(result))
			relatedIDs := strings.Split(room.RelatedRooms, ",")
			for _, rid := range relatedIDs {
				rid = strings.TrimSpace(rid)
				if rid == "" {
					continue
				}
				related.WriteString("\n---\n\n")
				summaryArgs := ReadTranscriptInput{
					RoomID: rid,
					Mode:   "summary",
				}
				sResult, _, sErr := cs.readSingleTranscript(summaryArgs)
				if sErr != nil {
					fmt.Fprintf(&related, "# %s (related) — Error: %s\n", rid, sErr.Error())
				} else {
					related.WriteString(toolResultText(sResult))
				}
			}
			return msg(related.String())
		}
	}

	return result, output, err
}

// readSingleTranscript handles a single room transcript read (all modes).
func (cs *CouncilServer) readSingleTranscript(args ReadTranscriptInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	room, err := cs.getRoom(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: Room '%s' not found.", args.RoomID))
	}

	// Mode: summary — return system_prompt + latest message per type
	if args.Mode == "summary" {
		latestMsgs, err := cs.getLatestPerType(args.RoomID)
		if err != nil {
			cs.logger.Error("Failed to get summary", "room_id", args.RoomID, "error", err)
			return nil, ToolOutput{}, err
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# %s [%s] — summary\n", room.ID, room.Status)
		fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
		if room.SystemPrompt != "" {
			fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
		}
		b.WriteString("---\n")

		// Show pinned message prominently in summary
		pinned, _ := cs.getPinnedMessage(args.RoomID)
		if pinned != nil {
			ts := pinned.Timestamp.Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "**PINNED [#%d %s] %s:**\n%s\n---\n", pinned.ID, ts, pinned.Author, pinned.Content)
		}

		if len(latestMsgs) == 0 {
			b.WriteString("No messages yet.\n")
		} else {
			seenType := map[string]int{}
			for _, m := range latestMsgs {
				if pinned != nil && m.ID == pinned.ID {
					continue // already shown above
				}
				ts := m.Timestamp.Format("2006-01-02 15:04:05")
				snippet := m.Content
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				seenType[m.MessageType]++
				label := "Latest"
				if seenType[m.MessageType] > 1 {
					label = "Previous"
				}
				fmt.Fprintf(&b, "**%s %s** [#%d %s] %s:\n  %s\n\n", label, m.MessageType, m.ID, ts, m.Author, snippet)
			}
		}
		return msg(b.String())
	}

	// Mode: changelog — only decision + action messages, chronological
	if args.Mode == "changelog" {
		messages, err := cs.getTranscript(args.RoomID)
		if err != nil {
			cs.logger.Error("Failed to get transcript", "room_id", args.RoomID, "error", err)
			return nil, ToolOutput{}, err
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# %s — changelog\n", room.ID)
		if room.Description != "" {
			fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
		}
		b.WriteString("---\n")

		count := 0
		for _, m := range messages {
			if m.MessageType != "decision" && m.MessageType != "action" {
				continue
			}
			ts := m.Timestamp.Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "\n**[#%d %s] %s (%s):**\n%s\n", m.ID, ts, m.Author, m.MessageType, m.Content)
			count++
		}
		if count == 0 {
			b.WriteString("\nNo decisions or actions recorded yet.\n")
		}
		return msg(b.String())
	}

	// Mode: after_id — delta read
	if args.AfterID != "" {
		var afterID int64
		if _, err := fmt.Sscanf(args.AfterID, "%d", &afterID); err != nil {
			return msg(fmt.Sprintf("Error: after_id '%s' is not a valid message ID.", args.AfterID))
		}

		messages, err := cs.getMessagesAfterID(args.RoomID, afterID)
		if err != nil {
			cs.logger.Error("Failed to get messages after ID", "room_id", args.RoomID, "after_id", afterID, "error", err)
			return nil, ToolOutput{}, err
		}

		// Find the latest message ID in the room for cursor tracking
		var latestID int64
		for _, m := range messages {
			if m.ID > latestID {
				latestID = m.ID
			}
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# %s — %d message(s) after #%d", room.ID, len(messages), afterID)
		if latestID > 0 {
			fmt.Fprintf(&b, " (latest: #%d)", latestID)
		}
		b.WriteString("\n")
		if room.SystemPrompt != "" {
			fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
		}
		b.WriteString("---\n")

		// Include pinned message at top of delta reads for context
		pinned, _ := cs.getPinnedMessage(args.RoomID)
		if pinned != nil {
			ts := pinned.Timestamp.Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "\n**PINNED [#%d %s] %s:**\n%s\n---\n", pinned.ID, ts, pinned.Author, pinned.Content)
		}

		for _, m := range messages {
			ts := m.Timestamp.Format("2006-01-02 15:04:05")
			replyTag := ""
			if m.ReplyTo > 0 {
				replyTag = fmt.Sprintf(", re: #%d", m.ReplyTo)
			}
			if m.IsSummary {
				fmt.Fprintf(&b, "\n**[%s] SUMMARY:**\n%s\n", ts, m.Content)
			} else if m.MessageType != "" && m.MessageType != "message" {
				fmt.Fprintf(&b, "\n**[#%d %s] %s (%s%s):**\n%s\n", m.ID, ts, m.Author, m.MessageType, replyTag, m.Content)
			} else {
				fmt.Fprintf(&b, "\n**[#%d %s] %s:**\n%s\n", m.ID, ts, m.Author, m.Content)
			}
		}
		return msg(b.String())
	}

	// Default: full transcript (with optional last_n)
	messages, err := cs.getTranscript(args.RoomID)
	if err != nil {
		cs.logger.Error("Failed to get transcript", "room_id", args.RoomID, "error", err)
		return nil, ToolOutput{}, err
	}

	// Apply last_n: keep only the last N non-summary messages (summaries always included)
	if args.LastN != "" {
		var lastN int
		if _, err := fmt.Sscanf(args.LastN, "%d", &lastN); err == nil && lastN > 0 {
			var summaries, regular []Message
			for _, m := range messages {
				if m.IsSummary {
					summaries = append(summaries, m)
				} else {
					regular = append(regular, m)
				}
			}
			if lastN < len(regular) {
				regular = regular[len(regular)-lastN:]
			}
			messages = append(summaries, regular...)
		}
	}

	transcript := formatTranscript(room, messages)
	return msg(transcript)
}

// handleGetDigest returns a project activity digest.
func (cs *CouncilServer) handleGetDigest(ctx context.Context, req *mcp.CallToolRequest, args DigestInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.Since == "" {
		return msg("Error: since is required (ISO timestamp, e.g. 2026-03-31T12:00:00).")
	}

	digest, err := cs.getDigest(args.Project, args.Since)
	if err != nil {
		cs.logger.Error("Failed to get digest", "project", args.Project, "since", args.Since, "error", err)
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	if len(digest) == 0 {
		projectNote := ""
		if args.Project != "" {
			projectNote = fmt.Sprintf(" in project '%s'", args.Project)
		}
		return msg(fmt.Sprintf("No new activity%s since %s.", projectNote, args.Since))
	}

	var b strings.Builder
	projectNote := ""
	if args.Project != "" {
		projectNote = fmt.Sprintf(" [%s]", args.Project)
	}
	fmt.Fprintf(&b, "# Activity Digest%s — since %s\n\n", projectNote, args.Since)
	fmt.Fprintf(&b, "%d room(s) with new activity:\n\n", len(digest))

	for _, d := range digest {
		excerpt := digestExcerpt(d.LatestExcerpt)
		fmt.Fprintf(&b, "- **%s** | %d new msg(s) | %s: %s\n", d.RoomID, d.NewMessages, d.LatestAuthor, excerpt)
	}

	return msg(b.String())
}

// digestExcerpt extracts a clean one-line summary from message content.
// Prefers the first markdown heading, then the first non-empty sentence,
// then falls back to a word-boundary truncation at 120 chars.
func digestExcerpt(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	// Try first markdown heading (## Heading or # Heading)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			heading := strings.TrimLeft(line, "# ")
			if heading != "" {
				if len(heading) > 120 {
					heading = heading[:120] + "..."
				}
				return heading
			}
		}
		// Stop looking after first non-empty non-heading line
		if line != "" {
			break
		}
	}

	// Try first sentence (ends with . ! ?)
	flat := strings.ReplaceAll(content, "\n", " ")
	for i, ch := range flat {
		if (ch == '.' || ch == '!' || ch == '?') && i > 10 {
			sentence := strings.TrimSpace(flat[:i+1])
			if len(sentence) <= 150 {
				return sentence
			}
			break
		}
	}

	// Fallback: word-boundary truncation
	if len(flat) > 120 {
		truncated := flat[:120]
		if i := strings.LastIndex(truncated, " "); i > 80 {
			truncated = truncated[:i]
		}
		return truncated + "..."
	}
	return flat
}
