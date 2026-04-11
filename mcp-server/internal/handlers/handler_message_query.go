package handlers

import (
	"context"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

// GetMessagesInput represents the parameters for fetching messages by ID or by room.
type GetMessagesInput struct {
	MessageIDs  string `json:"message_ids"`
	RoomID      string `json:"room_id"`
	LastN       string `json:"last_n"`
	AfterID     string `json:"after_id"`
	ClusterWide string `json:"cluster_wide"`
}

// GetMentionsInput represents the parameters for querying messages that mention an agent.
type GetMentionsInput struct {
	Author string `json:"author"`
	Limit  string `json:"limit"`
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
