package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PostToRoomInput represents the parameters for posting a message.
type PostToRoomInput struct {
	RoomID      string `json:"room_id"`
	Author      string `json:"author"`
	Message     string `json:"message"`
	MessageType string `json:"message_type"`
	ReplyTo     string `json:"reply_to"`
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

// GetMessagesInput represents the parameters for fetching messages by ID or by room.
type GetMessagesInput struct {
	MessageIDs string `json:"message_ids"`
	RoomID     string `json:"room_id"`
	LastN      string `json:"last_n"`
}

// PinMessageInput represents the parameters for pinning/unpinning a message.
type PinMessageInput struct {
	RoomID    string `json:"room_id"`
	MessageID string `json:"message_id"`
}

func (r *Registry) handlePostToRoom(ctx context.Context, req *mcp.CallToolRequest, args PostToRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
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
	if _, err := r.Server.GetRoom(args.RoomID); err != nil {
		return msg(fmt.Sprintf("Error: Room '%s' not found. Create it first with create_room.", args.RoomID))
	}

	var replyTo int64
	if args.ReplyTo != "" {
		if _, err := fmt.Sscanf(args.ReplyTo, "%d", &replyTo); err != nil {
			return msg(fmt.Sprintf("Error: reply_to '%s' is not a valid message ID.", args.ReplyTo))
		}
	}

	msgID, err := r.Server.PostMessage(args.RoomID, args.Author, args.Message, args.MessageType, replyTo)
	if err != nil {
		r.Server.Logger.Error("Failed to post message", "room_id", args.RoomID, "error", err)
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Message posted", "room_id", args.RoomID, "author", args.Author, "type", args.MessageType, "msg_id", msgID)
	return msg(fmt.Sprintf("Message #%d posted to room '%s' by %s.\n\n```json\n{\"message_id\": %d, \"room_id\": \"%s\", \"latest_message_id\": %d}\n```", msgID, args.RoomID, args.Author, msgID, args.RoomID, msgID))
}

func (r *Registry) handleSearchMessages(ctx context.Context, req *mcp.CallToolRequest, args SearchMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
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

	messages, err := r.Server.SearchMessages(args.Query, args.Author, args.MessageType, args.RoomID, args.Project, limit)
	if err != nil {
		r.Server.Logger.Error("Failed to search messages", "error", err)
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

func (r *Registry) handleGetMessages(ctx context.Context, req *mcp.CallToolRequest, args GetMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	var messages []council.Message

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
		messages, err = r.Server.GetMessagesByIDs(ids)
		if err != nil {
			r.Server.Logger.Error("Failed to get messages", "error", err)
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
		messages, err = r.Server.GetRecentMessages(args.RoomID, limit)
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

func (r *Registry) handlePinMessage(ctx context.Context, req *mcp.CallToolRequest, args PinMessageInput) (*mcp.CallToolResult, ToolOutput, error) {
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

	pinned, err := r.Server.PinMessage(args.RoomID, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return msg(fmt.Sprintf("Error: message #%d not found.", id))
		}
		r.Server.Logger.Error("Failed to pin message", "id", id, "error", err)
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	if pinned {
		r.Server.Logger.Info("Message pinned", "id", id, "room", args.RoomID)
		return msg(fmt.Sprintf("Message #%d pinned in room '%s'. It will appear first in transcripts.", id, args.RoomID))
	}
	r.Server.Logger.Info("Message unpinned", "id", id, "room", args.RoomID)
	return msg(fmt.Sprintf("Message #%d unpinned in room '%s'.", id, args.RoomID))
}

func (r *Registry) handleUpdateMessage(ctx context.Context, req *mcp.CallToolRequest, args UpdateMessageInput) (*mcp.CallToolResult, ToolOutput, error) {
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

	m, err := r.Server.UpdateMessage(id, args.Content, args.MessageType)
	if err != nil {
		if err == sql.ErrNoRows {
			return msg(fmt.Sprintf("Error: message #%d not found.", id))
		}
		r.Server.Logger.Error("Failed to update message", "id", id, "error", err)
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Message updated", "id", id, "room", m.RoomID)
	return msg(fmt.Sprintf("Message #%d updated. Author: %s, Room: %s, Type: %s.", m.ID, m.Author, m.RoomID, m.MessageType))
}

func (r *Registry) handleDeleteMessages(ctx context.Context, req *mcp.CallToolRequest, args DeleteMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
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
		msgs, err := r.Server.GetMessagesByIDs(ids)
		if err != nil {
			r.Server.Logger.Error("Failed to fetch messages for dry run", "error", err)
			return nil, ToolOutput{}, err
		}

		var b strings.Builder
		fmt.Fprintf(&b, "DRY RUN — %d message(s) would be deleted:\n\n", len(msgs))
		foundIDs := make(map[int64]bool)
		for _, m := range msgs {
			foundIDs[m.ID] = true
			excerpt := m.Content
			if len(excerpt) > 120 {
				excerpt = excerpt[:120]
				if i := strings.LastIndex(excerpt, " "); i > 80 {
					excerpt = excerpt[:i]
				}
				excerpt += "..."
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

	count, err := r.Server.DeleteMessages(ids)
	if err != nil {
		r.Server.Logger.Error("Failed to delete messages", "error", err)
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Messages deleted", "count", count, "ids", args.MessageIDs)
	return msg(fmt.Sprintf("Deleted %d message(s).", count))
}
