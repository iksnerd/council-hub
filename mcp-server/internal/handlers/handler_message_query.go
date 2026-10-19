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
	Query          string     `json:"query"`
	Author         string     `json:"author"`
	MessageType    string     `json:"message_type"`
	RoomID         string     `json:"room_id"`
	RoomIDs        string     `json:"room_ids"`
	IncludeRelated string     `json:"include_related"`
	Project        string     `json:"project"`
	Limit          string     `json:"limit"`
	Since          string     `json:"since"`
	Until          string     `json:"until"`
	SummaryOnly    string     `json:"summary_only"`
	FullContent    string     `json:"full_content"`
	ClusterWide    StringBool `json:"cluster_wide"`
	Semantic       string     `json:"semantic"`
}

// GetMessagesInput represents the parameters for fetching messages by ID or by room.
type GetMessagesInput struct {
	MessageIDs  string     `json:"message_ids"`
	RoomID      string     `json:"room_id"`
	LastN       string     `json:"last_n"`
	AfterID     string     `json:"after_id"`
	BeforeID    string     `json:"before_id"`
	Limit       string     `json:"limit"`
	ClusterWide StringBool `json:"cluster_wide"`
	History     string     `json:"history"`
}

// GetMentionsInput represents the parameters for querying messages that mention an agent.
type GetMentionsInput struct {
	Author      string     `json:"author"`
	Project     string     `json:"project"`
	Limit       string     `json:"limit"`
	ClusterWide StringBool `json:"cluster_wide"`
}

func (r *Registry) handleSearchMessages(ctx context.Context, req *mcp.CallToolRequest, args SearchMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	// Default cluster-wide (peers connected, cluster_wide unset). Semantic search
	// is node-local, and a filterless search is an error either way, so both
	// stay on the local path.
	hasFilter := args.Query != "" || args.Author != "" || args.MessageType != "" || args.RoomID != "" || args.RoomIDs != "" || args.Project != ""
	if args.ClusterWide != "true" {
		if ok, res, out, err := r.routeClusterRead(args.ClusterWide, args.Semantic != "true" && hasFilter && args.IncludeRelated != "true", // include_related expands rooms locally only

			func() (*mcp.CallToolResult, ToolOutput, error) { return r.handleSearchMessagesCluster(args) },
			func() (*mcp.CallToolResult, ToolOutput, error) {
				a := args
				a.ClusterWide = "false"
				return r.handleSearchMessages(ctx, req, a)
			}); ok {
			return res, out, err
		}
	}
	if args.ClusterWide == "true" {
		if args.Semantic == "true" {
			// Semantic search uses sqlite-vec which is local-only; the Phoenix
			// cluster fan-out path uses Elixir LIKE queries and cannot do vector
			// search. Fall back to local semantic search with a warning.
			msg := textResult
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
				semIDs := newRoomIDCache(r.Server)
				for _, m := range messages {
					fmt.Fprintf(&b, "**[#%s %s] %s (%s):** %s\n\n", semIDs.Of(m.RoomID, m.ID), m.Timestamp.Format("2006-01-02"), m.Author, m.MessageType, council.DisplayContent(m))
				}
			}
			return msg(b.String())
		}
		return r.handleSearchMessagesCluster(args)
	}

	msg := textResult

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
			excerpt := council.DisplayContent(m)
			excerpt = council.TruncateRunes(excerpt, 120, " ", 80)
			// Replace newlines in excerpt for single-line display
			excerpt = strings.ReplaceAll(excerpt, "\n", " ")
			// Full ID: this is the designated lookup surface for get_messages, so
			// the id printed here must be pastable directly into it — an 8-char
			// prefix can collide with other messages from the same ~65s window.
			fmt.Fprintf(&b, "- #%s | %s | %s | %s | %s | %s\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, excerpt)
		}
	} else {
		for _, m := range messages {
			ts := m.Timestamp.Format("2006-01-02 15:04:05")
			snippet := council.DisplayContent(m)
			if args.FullContent != "true" {
				snippet = council.TruncateRunes(snippet, 300, "", 0)
			}
			fmt.Fprintf(&b, "- **#%s** [%s] %s in **%s** (%s):\n  %s\n\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, snippet)
		}
	}

	return msg(b.String())
}

func (r *Registry) handleGetMessages(ctx context.Context, req *mcp.CallToolRequest, args GetMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	// Default cluster-wide only when fetching by message_ids: those can live on
	// a peer. Room reads and paged reads are room-scoped and stay local.
	if args.ClusterWide != "true" {
		if ok, res, out, err := r.routeClusterRead(args.ClusterWide, args.MessageIDs != "" && args.History != "true", // revision chains are node-local
			func() (*mcp.CallToolResult, ToolOutput, error) { return r.handleGetMessagesCluster(args) },
			func() (*mcp.CallToolResult, ToolOutput, error) {
				a := args
				a.ClusterWide = "false"
				return r.handleGetMessages(ctx, req, a)
			}); ok {
			return res, out, err
		}
	}
	if args.ClusterWide == "true" {
		return r.handleGetMessagesCluster(args)
	}

	msg := textResult

	var messages []council.Message
	var requestedIDs []string // set only in mode 1, to report which requested IDs missed
	var moreCursor string     // set when a paged read stopped short of the room's end

	if args.MessageIDs != "" {
		// Mode 1: fetch by explicit IDs
		ids := splitIDList(args.MessageIDs)
		ids, idErr := r.resolveIDList(ids)
		if idErr != nil {
			return msg(fmt.Sprintf("Error: %s", idErr.Error()))
		}
		requestedIDs = ids

		// history=true: show each message's full append-only edit chain (every
		// preserved version, oldest → newest) instead of just the current head.
		if args.History == "true" {
			var b strings.Builder
			for _, id := range ids {
				chain, err := r.Server.GetRevisionHistory(id)
				if err != nil {
					fmt.Fprintf(&b, "#%s — not found\n\n", id)
					continue
				}
				fmt.Fprintf(&b, "## #%s — %d version(s)\n\n", chain[len(chain)-1].ID, len(chain))
				for i, m := range chain {
					marker := "original"
					if i == len(chain)-1 {
						marker = "current"
					} else if i > 0 {
						marker = "revision"
					}
					ts := m.Timestamp.Format("2006-01-02 15:04:05")
					fmt.Fprintf(&b, "**v%d (%s)** — #%s [%s] %s (%s):\n\n%s\n\n", i+1, marker, m.ID, ts, m.Author, m.MessageType, m.Content)
				}
				b.WriteString("---\n\n")
			}
			return msg(b.String())
		}

		var err error
		messages, err = r.Server.GetMessagesByIDs(ids)
		if err != nil {
			r.Server.Logger.Error("Failed to get messages", "error", err)
			return nil, ToolOutput{}, err
		}
	} else if args.RoomID != "" && (args.AfterID != "" || args.BeforeID != "") {
		// Mode 2: paged read — forward from after_id (delta read) or backward from
		// before_id. Capped (default 50, max 200) because an uncapped delta read
		// returned 87k chars in one reply.
		limit := 50
		if args.Limit != "" {
			if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil || limit <= 0 {
				limit = 50
			}
		}
		if limit > 200 {
			limit = 200
		}
		if _, err := r.Server.GetRoom(args.RoomID); err != nil {
			return msg(fmt.Sprintf("Error: room '%s' not found", args.RoomID))
		}
		var more bool
		var err error
		messages, more, err = r.Server.GetMessagesPage(args.RoomID, args.AfterID, args.BeforeID, limit)
		if err != nil {
			return msg(fmt.Sprintf("Error: %s", err.Error()))
		}
		if more && len(messages) > 0 {
			if args.AfterID != "" {
				moreCursor = "more messages follow — continue with after_id=" + messages[len(messages)-1].ID
			} else {
				moreCursor = "older messages exist — continue with before_id=" + messages[0].ID
			}
		}
	} else if args.RoomID != "" {
		// Mode 3: browse room messages by last_n (limit is an accepted alias)
		lastNStr, aliasErr := resolveAlias(args.LastN, "last_n", args.Limit, "limit")
		if aliasErr != nil {
			return msg("Error: " + aliasErr.Error())
		}
		limit := 10
		if lastNStr != "" {
			if _, err := fmt.Sscanf(lastNStr, "%d", &limit); err != nil {
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
		return msg("Error: provide either message_ids, or room_id (with optional after_id, before_id, last_n or limit).")
	}

	// Name which requested IDs missed instead of a bare "No messages found" —
	// silently swallowing 5 requested IDs down to 0 (or 3) results with no
	// indication of *which* ones failed was the actionable-empty-result gap.
	var missing []string
	if requestedIDs != nil {
		found := make(map[string]bool, len(messages))
		for _, m := range messages {
			found[m.ID] = true
		}
		for _, id := range requestedIDs {
			if !found[id] {
				missing = append(missing, id)
			}
		}
	}

	if len(messages) == 0 {
		if len(missing) > 0 {
			return msg(fmt.Sprintf("No messages found. Not found: %s.", strings.Join(missing, ", ")))
		}
		return msg("No messages found.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s):\n\n", len(messages))
	for _, m := range messages {
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		fmt.Fprintf(&b, "---\n**#%s** [%s] %s in **%s** (%s):\n\n%s\n\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, council.DisplayContent(m))
	}
	if len(missing) > 0 {
		fmt.Fprintf(&b, "Not found: %s.\n", strings.Join(missing, ", "))
	}
	if moreCursor != "" {
		fmt.Fprintf(&b, "(%s)\n", moreCursor)
	}

	return msg(b.String())
}

func (r *Registry) handleGetMentions(ctx context.Context, req *mcp.CallToolRequest, args GetMentionsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.Author == "" {
		return msg("Error: author is required.")
	}

	limit := 20
	if args.Limit != "" {
		if _, err := fmt.Sscanf(args.Limit, "%d", &limit); err != nil {
			limit = 20
		}
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if ok, res, out, err := r.routeClusterRead(args.ClusterWide, true,
		func() (*mcp.CallToolResult, ToolOutput, error) { return r.handleGetMentionsCluster(args, limit) },
		func() (*mcp.CallToolResult, ToolOutput, error) {
			a := args
			a.ClusterWide = "false"
			return r.handleGetMentions(ctx, req, a)
		}); ok {
		return res, out, err
	}

	messages, err := r.Server.GetMentions(args.Author, args.Project, limit)
	if err != nil {
		r.Server.Logger.Error("Failed to get mentions", "author", args.Author, "error", err)
		return nil, ToolOutput{}, err
	}

	note := r.localOnlyNote()
	if note != "" {
		note = "\n" + note + "\n"
	}

	if len(messages) == 0 {
		if args.Project != "" {
			return msg(fmt.Sprintf("No messages mention @%s in project '%s'.%s", args.Author, args.Project, note))
		}
		return msg(fmt.Sprintf("No messages mention @%s.%s", args.Author, note))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s) mentioning @%s:\n\n", len(messages), args.Author)
	for _, m := range messages {
		ts := m.Timestamp.Format("2006-01-02 15:04:05")
		excerpt := council.DisplayContent(m)
		excerpt = council.TruncateRunes(excerpt, 200, " ", 150)
		excerpt = strings.ReplaceAll(excerpt, "\n", " ")
		fmt.Fprintf(&b, "- **#%s** [%s] %s in **%s** (%s): %s\n", m.ID, ts, m.Author, m.RoomID, m.MessageType, excerpt)
	}
	b.WriteString(note)

	return msg(b.String())
}
