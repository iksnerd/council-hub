package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PinMessageInput represents the parameters for pinning/unpinning a message.
type PinMessageInput struct {
	RoomID    string `json:"room_id"`
	MessageID string `json:"message_id"`
}

// ReactInput represents the parameters for adding/removing a reaction.
type ReactInput struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
	Author    string `json:"author"`
}

func (r *Registry) handlePinMessage(ctx context.Context, req *mcp.CallToolRequest, args PinMessageInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}
	if args.MessageID == "" {
		return msg("Error: message_id is required.")
	}
	resolved, err := r.resolveSingleID(args.MessageID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}
	args.MessageID = resolved

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

func (r *Registry) handleReactToMessage(ctx context.Context, req *mcp.CallToolRequest, args ReactInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.MessageID == "" {
		return msg("Error: message_id is required.")
	}
	if args.Emoji == "" {
		return msg("Error: emoji is required.")
	}
	if args.Author == "" {
		return msg("Error: author is required.")
	}
	resolved, err := r.resolveSingleID(args.MessageID)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}
	args.MessageID = resolved

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
