package handlers

import (
	"context"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReadTranscriptInput represents the parameters for reading a room transcript.
type ReadTranscriptInput struct {
	RoomID         string `json:"room_id"`
	RoomIDs        string `json:"room_ids"`
	LastN          string `json:"last_n"`
	AfterID        string `json:"after_id"`
	Mode           string `json:"mode"`
	IncludeRelated string `json:"include_related"`
	ClusterWide    string `json:"cluster_wide"`
}

// ArchiveRoomInput represents the parameters for archiving a room.
type ArchiveRoomInput struct {
	RoomID string `json:"room_id"`
	Delete string `json:"delete"`
}

func (r *Registry) handleReadTranscript(ctx context.Context, req *mcp.CallToolRequest, args ReadTranscriptInput) (*mcp.CallToolResult, ToolOutput, error) {
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
				RoomID:      id,
				LastN:       args.LastN,
				AfterID:     args.AfterID,
				Mode:        args.Mode,
				ClusterWide: args.ClusterWide,
			}
			
			var result *mcp.CallToolResult
			var err error
			if args.ClusterWide == "true" {
				result, _, err = r.handleReadTranscriptCluster(singleArgs, id)
			} else {
				result, _, err = r.readSingleTranscript(singleArgs)
			}

			if err != nil {
				fmt.Fprintf(&combined, "# %s \u2014 Error: %s\n", id, err.Error())
			} else {
				combined.WriteString(toolResultText(result))
			}
		}
		return msg(combined.String())
	}

	if args.RoomID == "" {
		return msg("Error: room_id or room_ids is required.")
	}

	var result *mcp.CallToolResult
	var output ToolOutput
	var err error

	if args.ClusterWide == "true" {
		result, output, err = r.handleReadTranscriptCluster(args, args.RoomID)
	} else {
		result, output, err = r.readSingleTranscript(args)
	}

	if err != nil {
		return nil, output, err
	}

	// Append related room summaries if requested (local only for now, as related traversal is complex across cluster)
	if args.IncludeRelated == "true" && args.ClusterWide != "true" {
		room, roomErr := r.Server.GetRoom(args.RoomID)
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
				sResult, _, sErr := r.readSingleTranscript(summaryArgs)
				if sErr != nil {
					fmt.Fprintf(&related, "# %s (related) \u2014 Error: %s\n", rid, sErr.Error())
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
func (r *Registry) readSingleTranscript(args ReadTranscriptInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	room, err := r.Server.GetRoom(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: Room '%s' not found.", args.RoomID))
	}

	// Mode: summary \u2014 return system_prompt + latest message per type
	if args.Mode == "summary" {
		latestMsgs, err := r.Server.GetLatestPerType(args.RoomID)
		if err != nil {
			r.Server.Logger.Error("Failed to get summary", "room_id", args.RoomID, "error", err)
			return nil, ToolOutput{}, err
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# %s [%s] \u2014 summary\n", room.ID, room.Status)
		if room.Description != "" {
			fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
		}
		if room.SystemPrompt != "" {
			fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
		}
		b.WriteString("---\n")

		// Show pinned message prominently in summary
		pinned, _ := r.Server.GetPinnedMessage(args.RoomID)
		if pinned != nil {
			ts := pinned.Timestamp.Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "**PINNED [#%.8s %s] %s:**\n%s\n---\n", pinned.ID, ts, pinned.Author, pinned.Content)
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
				fmt.Fprintf(&b, "**%s %s** [#%.8s %s] %s:\n  %s\n\n", label, m.MessageType, m.ID, ts, m.Author, snippet)
			}
		}
		return msg(b.String())
	}

	// Mode: changelog \u2014 only decision + action messages, chronological
	if args.Mode == "changelog" {
		messages, err := r.Server.GetTranscript(args.RoomID)
		if err != nil {
			r.Server.Logger.Error("Failed to get transcript", "room_id", args.RoomID, "error", err)
			return nil, ToolOutput{}, err
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# %s \u2014 changelog\n", room.ID)
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
			fmt.Fprintf(&b, "\n**[#%.8s %s] %s (%s):**\n%s\n", m.ID, ts, m.Author, m.MessageType, m.Content)
			count++
		}
		if count == 0 {
			b.WriteString("\nNo decisions or actions recorded yet.\n")
		}
		return msg(b.String())
	}

	// Mode: after_id \u2014 delta read
	if args.AfterID != "" {
		messages, err := r.Server.GetMessagesAfterID(args.RoomID, args.AfterID)
		if err != nil {
			r.Server.Logger.Error("Failed to get messages after ID", "room_id", args.RoomID, "after_id", args.AfterID, "error", err)
			return nil, ToolOutput{}, err
		}

		// Find the latest message ID in the room for cursor tracking
		latestID := ""
		for _, m := range messages {
			if m.ID > latestID {
				latestID = m.ID
			}
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# %s \u2014 %d message(s) after #%.8s", room.ID, len(messages), args.AfterID)
		if latestID != "" {
			fmt.Fprintf(&b, " (latest: #%.8s)", latestID)
		}
		b.WriteString("\n")
		if room.SystemPrompt != "" {
			fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
		}
		b.WriteString("---\n")

		// Include pinned message at top of delta reads for context
		pinned, _ := r.Server.GetPinnedMessage(args.RoomID)
		if pinned != nil {
			ts := pinned.Timestamp.Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "\n**PINNED [#%.8s %s] %s:**\n%s\n---\n", pinned.ID, ts, pinned.Author, pinned.Content)
		}

		for _, m := range messages {
			ts := m.Timestamp.Format("2006-01-02 15:04:05")
			replyTag := ""
			if m.ReplyTo != "" {
				replyTag = fmt.Sprintf(", re: #%.8s", m.ReplyTo)
			}
			if m.IsSummary {
				fmt.Fprintf(&b, "\n**[%s] SUMMARY:**\n%s\n", ts, m.Content)
			} else if m.MessageType != "" && m.MessageType != "message" {
				fmt.Fprintf(&b, "\n**[#%.8s %s] %s (%s%s):**\n%s\n", m.ID, ts, m.Author, m.MessageType, replyTag, m.Content)
			} else {
				fmt.Fprintf(&b, "\n**[#%.8s %s] %s:**\n%s\n", m.ID, ts, m.Author, m.Content)
			}
		}
		return msg(b.String())
	}

	// Default: full transcript (with optional last_n)
	messages, err := r.Server.GetTranscript(args.RoomID)
	if err != nil {
		r.Server.Logger.Error("Failed to get transcript", "room_id", args.RoomID, "error", err)
		return nil, ToolOutput{}, err
	}

	// Apply last_n: keep only the last N non-summary messages (summaries always included)
	if args.LastN != "" {
		var lastN int
		if _, err := fmt.Sscanf(args.LastN, "%d", &lastN); err == nil && lastN > 0 {
			var summaries, regular []council.Message
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

	transcript := council.FormatTranscript(room, messages)
	return msg(transcript)
}

// ListArchivesInput is the (empty) parameter struct for list_archives.
type ListArchivesInput struct{}

// ReadArchiveInput holds parameters for read_archive.
type ReadArchiveInput struct {
	RoomID string `json:"room_id"`
}

func (r *Registry) handleListArchives(ctx context.Context, req *mcp.CallToolRequest, args ListArchivesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	archives, err := r.Server.ListArchives()
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	if len(archives) == 0 {
		return msg("No archives found.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d archive(s):\n\n", len(archives))
	for _, a := range archives {
		sizeKB := float64(a.Size) / 1024.0
		date := a.ArchivedAt.Format("2006-01-02 15:04")
		fmt.Fprintf(&b, "- **%s** | %.1f KB | archived %s\n", a.RoomID, sizeKB, date)
	}
	return msg(b.String())
}

func (r *Registry) handleReadArchive(ctx context.Context, req *mcp.CallToolRequest, args ReadArchiveInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	content, err := r.Server.ReadArchive(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}
	return msg(content)
}

func (r *Registry) handleArchiveRoom(ctx context.Context, req *mcp.CallToolRequest, args ArchiveRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := func(text string) (*mcp.CallToolResult, ToolOutput, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	archivePath, err := r.Server.ArchiveRoom(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	result := fmt.Sprintf("Room '%s' archived to %s.", args.RoomID, archivePath)

	if args.Delete == "true" {
		if err := r.Server.DeleteRoom(args.RoomID); err != nil {
			return msg(fmt.Sprintf("Archived successfully but failed to delete: %s", err.Error()))
		}
		result += " Room and messages deleted."
		r.Server.Logger.Info("Room archived and deleted", "room_id", args.RoomID, "path", archivePath)
	} else {
		r.Server.Logger.Info("Room archived", "room_id", args.RoomID, "path", archivePath)
	}

	return msg(result)
}