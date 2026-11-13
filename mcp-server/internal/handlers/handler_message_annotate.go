package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PinMessageInput represents the parameters for pinning/unpinning a message.
type PinMessageInput struct {
	RoomID    string `json:"room_id"`
	MessageID string `json:"message_id"`
	// Force lets a pin replace another author's fresh pin (see PinFreshWindow).
	Force string `json:"force"`
	// Author is who is pinning; it defaults to the client name given at initialize,
	// like post_to_room. The fresh-pin rule compares it with the current pin's author.
	Author string `json:"author"`
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
	if args.Author == "" {
		args.Author = clientIdentity(req)
	}
	// A room on a peer: forward to the owner and say so, rather than failing
	// locally and leaving the caller to guess where the write went.
	if _, gerr := r.Server.GetRoom(args.RoomID); gerr != nil {
		if owner, lerr := r.locateRoomOwner(args.RoomID); lerr == nil && owner != "" {
			out, perr := r.proxyPinMessage(owner, args.RoomID, args.MessageID, args.Force, args.Author)
			if errors.Is(perr, errPeerLacksEndpoint) {
				return msg(fmt.Sprintf("Error: room '%s' lives on cluster node '%s', which runs an older version that cannot take a forwarded pin. Nothing was changed. Use post_to_room(pin=true), or run pin_message from a session on that node.", args.RoomID, owner))
			}
			if perr != nil {
				return msg(fmt.Sprintf("Error: room '%s' is owned by cluster node '%s' but the pin could not be forwarded (nothing was changed): %s", args.RoomID, owner, perr.Error()))
			}
			if out.Refusal != "" {
				return msg(fmt.Sprintf("%s (room '%s' is on cluster node %s; nothing was changed)", out.Refusal, args.RoomID, owner))
			}
			verb := "pinned"
			if !out.Pinned {
				verb = "unpinned"
			}
			return msg(fmt.Sprintf("Message #%.8s %s in room '%s' (forwarded to cluster node %s).", out.MessageID, verb, args.RoomID, owner))
		}
	}
	// Room-scoped first: a prefix shared with a message in another room is not
	// ambiguity. A genuine in-room collision still lists each match with its room
	// and type (council.ErrAmbiguousMessageID).
	if resolved, rerr := r.resolveRef(args.MessageID, args.RoomID); rerr != nil {
		if _, ambiguous := rerr.(*council.ErrAmbiguousMessageID); ambiguous {
			return msg(fmt.Sprintf("Error: %s", rerr.Error()))
		}
	} else {
		args.MessageID = resolved
	}

	if kept := r.pinGuard(args.RoomID, args.MessageID, args.Force, args.Author); kept != nil {
		return msg(pinRefusalText(kept, args.RoomID, args.MessageID))
	}

	pinned, err := r.Server.PinMessage(args.RoomID, args.MessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			// The message may be missing because the whole room is on a peer —
			// post_to_room(pin=true) is the cross-node way to pin.
			return msg(fmt.Sprintf("Error: message #%.8s not found.%s", args.MessageID,
				r.remoteRoomNote(args.RoomID, "pin it there, or use post_to_room(pin=true) which routes automatically")))
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
	if err := r.resolveInto(&args.MessageID); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
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
