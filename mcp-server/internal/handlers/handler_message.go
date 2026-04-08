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

// GetMentionsInput represents the parameters for querying messages that mention an agent.
type GetMentionsInput struct {
	Author string `json:"author"`
	Limit  string `json:"limit"`
}

// SearchMessagesInput represents the parameters for searching messages.
type SearchMessagesInput struct {
	Query          string `json:"query"`
	Author         string `json:"author"`
	MessageType    string `json:"message_type"`
	RoomID         string `json:"room_id"`
	RoomIDs        string `json:"room_ids"`
	IncludeRelated string `json:"include_related"`
	Project        string `json:"project"`
	Limit          string `json:"limit"`
	Since          string `json:"since"`
	Until          string `json:"until"`
	SummaryOnly    string `json:"summary_only"`
	FullContent    string `json:"full_content"`
	ClusterWide    string `json:"cluster_wide"`
	Semantic       string `json:"semantic"`
}

// MoveMessagesInput represents the parameters for moving messages between rooms.
type MoveMessagesInput struct {
	MessageIDs   string `json:"message_ids"`
	TargetRoomID string `json:"target_room_id"`
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

// GetMessagesInput represents the parameters for fetching messages by ID or by room.
type GetMessagesInput struct {
	MessageIDs  string `json:"message_ids"`
	RoomID      string `json:"room_id"`
	LastN       string `json:"last_n"`
	AfterID     string `json:"after_id"`
	ClusterWide string `json:"cluster_wide"`
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
		return msg(fmt.Sprintf("Error: Invalid message_type '%s'. Must be one of: message, thought, decision, code, review, action, critique, synthesis.", args.MessageType))
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

func (r *Registry) handleSearchMessages(ctx context.Context, req *mcp.CallToolRequest, args SearchMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	if args.ClusterWide == "true" {
		if args.Semantic == "true" {
			// Semantic search uses sqlite-vec which is local-only; the Phoenix
			// cluster fan-out path uses Elixir LIKE queries and cannot do vector
			// search. Fall back to local semantic search with a warning.
			msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: text}},
				}, ToolOutput{Message: text}, nil
			}
			if args.Query == "" {
				return msg("Error: query is required for semantic search.")
			}
			effectiveRoomIDs := args.RoomIDs
			if args.RoomID != "" && args.RoomIDs == "" {
				effectiveRoomIDs = args.RoomID
			}
			limit := 20
			if args.Limit != "" {
				if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
					limit = 20
				}
			}
			messages, err := r.Server.SearchMessagesSemantic(args.Query, effectiveRoomIDs, args.Project, args.Author, args.MessageType, args.Since, args.Until, limit)
			if err != nil {
				return msg(fmt.Sprintf("Error: semantic search unavailable — %s", err.Error()))
			}
			var b strings.Builder
			b.WriteString("Note: semantic search is local-only (cluster_wide ignored — vector search requires sqlite-vec, not available on remote nodes).\n\n")
			if len(messages) == 0 {
				b.WriteString("No messages found matching the given filters.")
			} else {
				fmt.Fprintf(&b, "Found %d message(s):\n\n", len(messages))
				for _, m := range messages {
					fmt.Fprintf(&b, "**[#%.8s %s] %s (%s):** %s\n\n", m.ID, m.Timestamp.Format("2006-01-02"), m.Author, m.MessageType, m.Content)
				}
			}
			return msg(b.String())
		}
		return r.handleSearchMessagesCluster(args)
	}

	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.Query == "" && args.Author == "" && args.MessageType == "" && args.RoomID == "" && args.RoomIDs == "" && args.Project == "" {
		return msg("Error: at least one search filter is required (query, author, message_type, room_id, room_ids, or project).")
	}

	// Merge room_id into room_ids for unified handling, expanding related rooms if requested.
	effectiveRoomIDs := args.RoomIDs
	if args.RoomID != "" && args.RoomIDs == "" {
		effectiveRoomIDs = args.RoomID
	}

	// include_related: expand scope to 1-level related rooms when room_id is set.
	var relatedNote string
	if args.IncludeRelated == "true" && args.RoomID != "" {
		room, err := r.Server.GetRoom(args.RoomID)
		if err == nil && room.RelatedRooms != "" {
			allIDs := []string{args.RoomID}
			for _, rel := range strings.Split(room.RelatedRooms, ",") {
				rel = strings.TrimSpace(rel)
				if rel != "" {
					allIDs = append(allIDs, rel)
				}
			}
			effectiveRoomIDs = strings.Join(allIDs, ",")
			relatedNote = fmt.Sprintf("(searched %d rooms: %s)\n\n", len(allIDs), effectiveRoomIDs)
		}
	}

	limit := 20
	if args.Limit != "" {
		if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
			limit = 20
		}
	}

	var messages []council.Message
	var err error

	if args.Semantic == "true" {
		if args.Query == "" {
			return msg("Error: query is required for semantic search.")
		}
		messages, err = r.Server.SearchMessagesSemantic(args.Query, effectiveRoomIDs, args.Project, args.Author, args.MessageType, args.Since, args.Until, limit)
	} else {
		messages, err = r.Server.SearchMessages(args.Query, args.Author, args.MessageType, effectiveRoomIDs, args.Project, args.Since, args.Until, limit)
	}
	if err != nil {
		r.Server.Logger.Error("Failed to search messages", "error", err)
		return nil, ToolOutput{}, err
	}

	if len(messages) == 0 {
		noResult := "No messages found matching the given filters."
		if relatedNote != "" {
			noResult = relatedNote + noResult
		}
		return msg(noResult)
	}

	var b strings.Builder
	b.WriteString(relatedNote)
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
			fmt.Fprintf(&b, "- #%.8s | %s | %s | %s | %s | %s\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, excerpt)
		}
	} else {
		for _, m := range messages {
			ts := m.Timestamp.Format("2006-01-02 15:04:05")
			snippet := m.Content
			if args.FullContent != "true" && len(snippet) > 300 {
				snippet = snippet[:300] + "..."
			}
			fmt.Fprintf(&b, "- **#%s** [%s] %s in **%s** (%s):\n  %s\n\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, snippet)
		}
	}

	return msg(b.String())
}

func (r *Registry) handleGetMessages(ctx context.Context, req *mcp.CallToolRequest, args GetMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	if args.ClusterWide == "true" {
		return r.handleGetMessagesCluster(args)
	}

	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	var messages []council.Message

	if args.MessageIDs != "" {
		// Mode 1: fetch by explicit IDs
		parts := strings.Split(args.MessageIDs, ",")
		var ids []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				ids = append(ids, p)
			}
		}

		var err error
		messages, err = r.Server.GetMessagesByIDs(ids)
		if err != nil {
			r.Server.Logger.Error("Failed to get messages", "error", err)
			return nil, ToolOutput{}, err
		}
	} else if args.RoomID != "" && args.AfterID != "" {
		// Mode 2: delta read — messages after a known ID in a room
		var err error
		messages, err = r.Server.GetMessagesAfterID(args.RoomID, args.AfterID)
		if err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
	} else if args.RoomID != "" {
		// Mode 3: browse room messages by last_n
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
		return msg("Error: provide either message_ids, or room_id (with optional after_id or last_n).")
	}

	if len(messages) == 0 {
		return msg("No messages found.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s):\n\n", len(messages))
	for _, m := range messages {
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		fmt.Fprintf(&b, "---\n**#%s** [%s] %s in **%s** (%s):\n\n%s\n\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, m.Content)
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

	pinned, err := r.Server.PinMessage(args.RoomID, args.MessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return msg(fmt.Sprintf("Error: message #%.8s not found.", args.MessageID))
		}
		r.Server.Logger.Error("Failed to pin message", "id", args.MessageID, "error", err)
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	if pinned {
		r.Server.Logger.Info("Message pinned", "id", args.MessageID, "room", args.RoomID)
		return msg(fmt.Sprintf("Message #%.8s pinned in room '%s'. It will appear first in transcripts.", args.MessageID, args.RoomID))
	}
	r.Server.Logger.Info("Message unpinned", "id", args.MessageID, "room", args.RoomID)
	return msg(fmt.Sprintf("Message #%.8s unpinned in room '%s'.", args.MessageID, args.RoomID))
}

// ReactInput represents the parameters for adding/removing a reaction.
type ReactInput struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
	Author    string `json:"author"`
}

func (r *Registry) handleReactToMessage(ctx context.Context, req *mcp.CallToolRequest, args ReactInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.MessageID == "" {
		return msg("Error: message_id is required.")
	}
	if args.Emoji == "" {
		return msg("Error: emoji is required.")
	}
	if args.Author == "" {
		return msg("Error: author is required.")
	}

	reactions, added, err := r.Server.ReactToMessage(args.MessageID, args.Emoji, args.Author)
	if err != nil {
		r.Server.Logger.Error("Failed to react", "id", args.MessageID, "error", err)
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	// Format reaction summary
	var b strings.Builder
	if added {
		fmt.Fprintf(&b, "%s reaction added to #%.8s by %s.\n", args.Emoji, args.MessageID, args.Author)
	} else {
		fmt.Fprintf(&b, "%s reaction removed from #%.8s by %s.\n", args.Emoji, args.MessageID, args.Author)
	}
	if len(reactions) > 0 {
		b.WriteString("Reactions: ")
		first := true
		for emoji, authors := range reactions {
			if !first {
				b.WriteString("  ")
			}
			fmt.Fprintf(&b, "%s %d", emoji, len(authors))
			first = false
		}
		b.WriteString("\n")
	}
	return msg(b.String())
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
		return msg(fmt.Sprintf("Error: invalid message_type '%s'. Valid types: message, thought, decision, code, review, action, critique, synthesis.", args.MessageType))
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

func (r *Registry) handleGetMentions(ctx context.Context, req *mcp.CallToolRequest, args GetMentionsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.Author == "" {
		return msg("Error: author is required.")
	}

	limit := 20
	if args.Limit != "" {
		if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
			limit = 20
		}
	}

	messages, err := r.Server.GetMentions(args.Author, limit)
	if err != nil {
		r.Server.Logger.Error("Failed to get mentions", "author", args.Author, "error", err)
		return nil, ToolOutput{}, err
	}

	if len(messages) == 0 {
		return msg(fmt.Sprintf("No messages mention @%s.", args.Author))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s) mentioning @%s:\n\n", len(messages), args.Author)
	for _, m := range messages {
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		excerpt := m.Content
		if len(excerpt) > 200 {
			excerpt = excerpt[:200]
			if i := strings.LastIndex(excerpt, " "); i > 150 {
				excerpt = excerpt[:i]
			}
			excerpt += "..."
		}
		excerpt = strings.ReplaceAll(excerpt, "\n", " ")
		fmt.Fprintf(&b, "- **#%s** [%s] %s in **%s** (%s): %s\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, excerpt)
	}

	return msg(b.String())
}
