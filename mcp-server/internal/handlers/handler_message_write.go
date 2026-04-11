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
	Mentions    string `json:"mentions"`
}

// UpdateMessageInput represents the parameters for editing a message in-place.
type UpdateMessageInput struct {
	MessageID       string `json:"message_id"`
	Content         string `json:"content"`
	MessageType     string `json:"message_type"`
	ExpectedContent string `json:"expected_content"`
}

// DeleteMessagesInput represents the parameters for deleting messages.
type DeleteMessagesInput struct {
	MessageIDs string `json:"message_ids"`
	DryRun     string `json:"dry_run"`
}

// MoveMessagesInput represents the parameters for moving messages between rooms.
type MoveMessagesInput struct {
	MessageIDs   string `json:"message_ids"`
	TargetRoomID string `json:"target_room_id"`
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
	if err := validateSize("room_id", args.RoomID, maxIDLen); err != nil {
		return msg("Error: " + err.Error())
	}
	if err := validateSize("author", args.Author, maxAuthorLen); err != nil {
		return msg("Error: " + err.Error())
	}
	if err := validateSize("message", args.Message, maxContentLen); err != nil {
		return msg("Error: " + err.Error())
	}

	if args.MessageType == "" {
		args.MessageType = "message"
	}
	if !validMessageTypes[args.MessageType] {
		return msg(fmt.Sprintf("Error: Invalid message_type '%s'. Must be one of: message, thought, draft, decision, code, review, action, critique, synthesis.", args.MessageType))
	}

	// Verify room exists
	if _, err := r.Server.GetRoom(args.RoomID); err != nil {
		return msg(fmt.Sprintf("Error: Room '%s' not found. Create it first with create_room.", args.RoomID))
	}

	msgID, err := r.Server.PostMessageWithMentions(args.RoomID, args.Author, args.Message, args.MessageType, args.ReplyTo, args.Mentions)
	if err != nil {
		r.Server.Logger.Error("Failed to post message", "room_id", args.RoomID, "error", err)
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Message posted", "room_id", args.RoomID, "author", args.Author, "type", args.MessageType, "msg_id", msgID)
	return msg(fmt.Sprintf("Message #%.8s posted to room '%s' by %s.\n\n```json\n{\"message_id\": \"%s\", \"room_id\": \"%s\", \"latest_message_id\": \"%s\"}\n```", msgID, args.RoomID, args.Author, msgID, args.RoomID, msgID))
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
	if err := validateSize("content", args.Content, maxContentLen); err != nil {
		return msg("Error: " + err.Error())
	}

	if args.MessageType != "" && !validMessageTypes[args.MessageType] {
		return msg(fmt.Sprintf("Error: invalid message_type '%s'. Valid types: message, thought, draft, decision, code, review, action, critique, synthesis.", args.MessageType))
	}

	m, err := r.Server.UpdateMessageWithExpected(args.MessageID, args.Content, args.MessageType, args.ExpectedContent)
	if err != nil {
		if err == sql.ErrNoRows {
			return msg(fmt.Sprintf("Error: message #%.8s not found.", args.MessageID))
		}
		if changed, ok := err.(*council.ErrContentChanged); ok {
			return msg(fmt.Sprintf("Error: content changed since last read — re-read before updating.\n\nCurrent content:\n%s", changed.CurrentContent))
		}
		r.Server.Logger.Error("Failed to update message", "id", args.MessageID, "error", err)
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Message updated", "id", args.MessageID, "room", m.RoomID)
	return msg(fmt.Sprintf("Message #%.8s updated. Author: %s, Room: %s, Type: %s.", m.ID, m.Author, m.RoomID, m.MessageType))
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
	var ids []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ids = append(ids, p)
		}
	}

	if args.DryRun == "true" {
		msgs, err := r.Server.GetMessagesByIDs(ids)
		if err != nil {
			r.Server.Logger.Error("Failed to fetch messages for dry run", "error", err)
			return nil, ToolOutput{}, err
		}

		var b strings.Builder
		fmt.Fprintf(&b, "DRY RUN — %d message(s) would be deleted:\n\n", len(msgs))
		foundIDs := make(map[string]bool)
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
			fmt.Fprintf(&b, "  #%.8s | %s | %s | %s | %s\n",
				m.ID, m.Author, m.Timestamp.Format("2006-01-02 15:04:05"), m.RoomID, excerpt)
		}
		for _, id := range ids {
			if !foundIDs[id] {
				fmt.Fprintf(&b, "  #%.8s — not found\n", id)
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

func (r *Registry) handleMoveMessages(ctx context.Context, req *mcp.CallToolRequest, args MoveMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.MessageIDs == "" {
		return msg("Error: message_ids is required (comma-separated list of message IDs).")
	}
	if args.TargetRoomID == "" {
		return msg("Error: target_room_id is required.")
	}
	if err := validateSize("target_room_id", args.TargetRoomID, maxIDLen); err != nil {
		return msg("Error: " + err.Error())
	}

	parts := strings.Split(args.MessageIDs, ",")
	var ids []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ids = append(ids, p)
		}
	}
	if len(ids) == 0 {
		return msg("Error: no valid message IDs provided.")
	}

	moved, err := r.Server.MoveMessages(ids, args.TargetRoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	notMoved := len(ids) - moved
	var b strings.Builder
	fmt.Fprintf(&b, "Moved %d message(s) to room '%s'.", moved, args.TargetRoomID)
	if notMoved > 0 {
		fmt.Fprintf(&b, " %d ID(s) not found and were skipped.", notMoved)
	}
	r.Server.Logger.Info("Messages moved", "count", moved, "target", args.TargetRoomID)
	return msg(b.String())
}
