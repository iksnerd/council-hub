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
	RoomID       string `json:"room_id"`
	Author       string `json:"author"`
	Message      string `json:"message"`
	MessageType  string `json:"message_type"`
	ReplyTo      string `json:"reply_to"`
	Mentions     string `json:"mentions"`
	Supersedes   string `json:"supersedes"`
	MarkReadSelf string `json:"mark_read_self"`
	Pin          string `json:"pin"`
}

// UpdateMessageInput represents the parameters for editing a message. The edit is
// append-only: it posts a new revision and preserves the prior version.
type UpdateMessageInput struct {
	MessageID       string `json:"message_id"`
	Content         string `json:"content"`
	MessageType     string `json:"message_type"`
	ExpectedContent string `json:"expected_content"`
	Author          string `json:"author"`
}

// DeleteMessagesInput represents the parameters for retracting, restoring, or
// purging messages.
type DeleteMessagesInput struct {
	MessageIDs string `json:"message_ids"`
	DryRun     string `json:"dry_run"`
	Author     string `json:"author"`
	Purge      string `json:"purge"`
	Restore    string `json:"restore"`
}

// MoveMessagesInput represents the parameters for moving messages between rooms.
type MoveMessagesInput struct {
	MessageIDs   string `json:"message_ids"`
	TargetRoomID string `json:"target_room_id"`
}

// ForkThreadInput represents the parameters for forking a thread into a new room.
type ForkThreadInput struct {
	StartMessageID string `json:"start_message_id"`
	NewRoomID      string `json:"new_room_id"`
	Topic          string `json:"topic"`
	Project        string `json:"project"`
	Tags           string `json:"tags"`
}

func (r *Registry) handlePostToRoom(ctx context.Context, req *mcp.CallToolRequest, args PostToRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

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
		return msg(fmt.Sprintf("Error: Invalid message_type '%s'. Must be one of: message, thought, draft, decision, plan, review, action, critique, synthesis, note.", args.MessageType))
	}

	// Verify room exists locally. If it doesn't and we're clustered, the room may
	// be owned by a peer node — locate the owner and proxy the write there rather
	// than silently creating a local shadow.
	if _, err := r.Server.GetRoom(args.RoomID); err != nil {
		if owner, lerr := r.locateRoomOwner(args.RoomID); lerr == nil && owner != "" {
			msgID, perr := r.proxyPostToRoom(owner, args)
			if perr != nil {
				return msg(fmt.Sprintf("Error: room '%s' is owned by cluster node '%s' but the write could not be forwarded: %s", args.RoomID, owner, perr.Error()))
			}
			r.Server.Logger.Info("Message proxied to owner", "room_id", args.RoomID, "owner", owner, "msg_id", msgID)
			return msg(fmt.Sprintf("Message #%.8s posted to room '%s' (on cluster node %s) by %s.\n\n```json\n{\"message_id\": \"%s\", \"room_id\": \"%s\", \"latest_message_id\": \"%s\", \"owner_node\": \"%s\"}\n```", msgID, args.RoomID, owner, args.Author, msgID, args.RoomID, msgID, owner))
		}
		return msg(fmt.Sprintf("Error: Room '%s' not found. Create it first with create_room.", args.RoomID))
	}

	// Resolve reply_to/supersedes to full IDs before storing them. PostMessageWithRefs
	// stores these refs verbatim and downstream matching (backlinks, revision chains,
	// get_links) is exact — a stored prefix or typo'd ID would create a silently
	// dangling edge, so unlike resolveSingleID's pass-through this errors on
	// not-found too. Local-only: on the cluster-proxy path above the referenced
	// message lives on the owning node, so the owner resolves it there.
	for _, ref := range []struct {
		name  string
		value *string
	}{{"reply_to", &args.ReplyTo}, {"supersedes", &args.Supersedes}} {
		if *ref.value == "" {
			continue
		}
		resolved, rerr := r.Server.ResolveMessageID(*ref.value)
		if rerr != nil {
			return msg(fmt.Sprintf("Error: %s: %s", ref.name, rerr.Error()))
		}
		*ref.value = resolved
	}

	msgID, err := r.Server.PostMessageWithRefs(args.RoomID, args.Author, args.Message, args.MessageType, args.ReplyTo, args.Mentions, args.Supersedes)
	if err != nil {
		r.Server.Logger.Error("Failed to post message", "room_id", args.RoomID, "error", err)
		return nil, ToolOutput{}, err
	}

	// mark_read_self folds the end-of-session mark_read round-trip into the write:
	// the poster just authored this message, so advance their own cursor to it.
	if args.MarkReadSelf == "true" {
		if err := r.Server.MarkRead(args.Author, args.RoomID, msgID); err != nil {
			r.Server.Logger.Warn("mark_read_self failed", "room_id", args.RoomID, "author", args.Author, "error", err)
		}
	}

	// pin folds the post→pin dance into one call: pin the just-authored message
	// (auto-unpinning the room's previous pin, identical to pin_message). The
	// near-universal flow for synthesis/decision is "write after deliberation,
	// then pin it" — this saves a round-trip and the manual message_id plumbing.
	pinNote := ""
	if args.Pin == "true" {
		if _, perr := r.Server.PinMessage(args.RoomID, msgID); perr != nil {
			r.Server.Logger.Warn("pin-on-post failed", "room_id", args.RoomID, "msg_id", msgID, "error", perr)
			pinNote = " (pin failed — pin it manually with pin_message)"
		} else {
			pinNote = " 📌 pinned (previous pin replaced)"
		}
	}

	// Surface any Knowledge-Linter health flags that this post didn't clear, so the
	// flag is actionable. Read tags AFTER the post/pin (a synthesis clears
	// needs-synthesis, a non-system post clears stale, a pin clears stale-pin) — we
	// only nudge about what genuinely remains.
	healthHint := ""
	if room, gerr := r.Server.GetRoom(args.RoomID); gerr == nil {
		healthHint = healthTagHint(room.Tags)
	}

	r.Server.Logger.Info("Message posted", "room_id", args.RoomID, "author", args.Author, "type", args.MessageType, "msg_id", msgID, "pinned", args.Pin == "true")
	return msg(fmt.Sprintf("Message #%.8s posted to room '%s' by %s.%s\n\n```json\n{\"message_id\": \"%s\", \"room_id\": \"%s\", \"latest_message_id\": \"%s\"}\n```%s", msgID, args.RoomID, args.Author, pinNote, msgID, args.RoomID, msgID, healthHint))
}

func (r *Registry) handleUpdateMessage(ctx context.Context, req *mcp.CallToolRequest, args UpdateMessageInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

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
		return msg(fmt.Sprintf("Error: invalid message_type '%s'. Valid types: message, thought, draft, decision, plan, review, action, critique, synthesis, note.", args.MessageType))
	}
	if err := r.resolveInto(&args.MessageID); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	m, err := r.Server.UpdateMessageWithExpected(args.MessageID, args.Content, args.MessageType, args.ExpectedContent, args.Author)
	if err != nil {
		if err == sql.ErrNoRows {
			return msg(fmt.Sprintf("Error: message #%.8s not found.", args.MessageID))
		}
		if changed, ok := err.(*council.ErrContentChanged); ok {
			return msg(fmt.Sprintf("Error: content changed since last read — re-read before updating.\n\nCurrent content:\n%s", changed.CurrentContent))
		}
		if revised, ok := err.(*council.ErrAlreadyRevised); ok {
			return msg(fmt.Sprintf("Error: message #%.8s was already revised — edit the current version #%.8s instead.", args.MessageID, revised.HeadID))
		}
		r.Server.Logger.Error("Failed to update message", "id", args.MessageID, "error", err)
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Message revised", "from", args.MessageID, "to", m.ID, "room", m.RoomID)
	return msg(fmt.Sprintf("Message #%.8s edited — posted revision #%.8s (the prior version is preserved and stays linked via `revises`). Author: %s, Room: %s, Type: %s.\n\n```json\n{\"message_id\": \"%s\", \"revises\": \"%s\", \"room_id\": \"%s\"}\n```", args.MessageID, m.ID, m.Author, m.RoomID, m.MessageType, m.ID, args.MessageID, m.RoomID))
}

func (r *Registry) handleDeleteMessages(ctx context.Context, req *mcp.CallToolRequest, args DeleteMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.MessageIDs == "" {
		return msg("Error: message_ids is required (comma-separated list of message IDs).")
	}

	ids := splitIDList(args.MessageIDs)
	// Purge is irreversible, so prefixes are never resolved for it: a typo'd
	// prefix that happens to uniquely match a *different* message must not
	// hard-delete it. Only exact full IDs purge; everything else reports not found.
	purge := args.Purge == "true"
	if !purge {
		var idErr error
		ids, idErr = r.resolveIDList(ids)
		if idErr != nil {
			return msg(fmt.Sprintf("Error: %s", idErr.Error()))
		}
	}

	// restore reverses a retraction — bring tombstoned messages back.
	if args.Restore == "true" {
		count, err := r.Server.RestoreMessages(ids)
		if err != nil {
			r.Server.Logger.Error("Failed to restore messages", "error", err)
			return nil, ToolOutput{}, err
		}
		r.Server.Logger.Info("Messages restored", "count", count, "ids", args.MessageIDs)
		return msg(fmt.Sprintf("Restored %d message(s) — retraction cleared, they render normally again.", count))
	}

	if args.DryRun == "true" {
		msgs, err := r.Server.GetMessagesByIDs(ids)
		if err != nil {
			r.Server.Logger.Error("Failed to fetch messages for dry run", "error", err)
			return nil, ToolOutput{}, err
		}

		verb := "retracted (tombstoned; content + links preserved)"
		if purge {
			verb = "PURGED (permanently destroyed; links cascade-deleted)"
		}
		var b strings.Builder
		fmt.Fprintf(&b, "DRY RUN — %d message(s) would be %s:\n\n", len(msgs), verb)
		foundIDs := make(map[string]bool)
		for _, m := range msgs {
			foundIDs[m.ID] = true
			excerpt := council.TruncateRunes(m.Content, 120, " ", 80)
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

	if purge {
		count, err := r.Server.PurgeMessages(ids)
		if err != nil {
			r.Server.Logger.Error("Failed to purge messages", "error", err)
			return nil, ToolOutput{}, err
		}
		r.Server.Logger.Info("Messages purged", "count", count, "ids", args.MessageIDs)
		return msg(fmt.Sprintf("Purged %d message(s) — permanently destroyed. Use this only for content that must not persist (a leaked secret, PII); everything else should be retracted.", count))
	}

	count, err := r.Server.RetractMessages(ids, args.Author)
	if err != nil {
		r.Server.Logger.Error("Failed to retract messages", "error", err)
		return nil, ToolOutput{}, err
	}

	r.Server.Logger.Info("Messages retracted", "count", count, "ids", args.MessageIDs)
	return msg(fmt.Sprintf("Retracted %d message(s) — tombstoned, content and links preserved (they render as \"[retracted]\"). Pass purge=true to permanently destroy instead.", count))
}

func (r *Registry) handleMoveMessages(ctx context.Context, req *mcp.CallToolRequest, args MoveMessagesInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.MessageIDs == "" {
		return msg("Error: message_ids is required (comma-separated list of message IDs).")
	}
	if args.TargetRoomID == "" {
		return msg("Error: target_room_id is required.")
	}
	if err := validateSize("target_room_id", args.TargetRoomID, maxIDLen); err != nil {
		return msg("Error: " + err.Error())
	}

	ids := splitIDList(args.MessageIDs)
	if len(ids) == 0 {
		return msg("Error: no valid message IDs provided.")
	}
	ids, idErr := r.resolveIDList(ids)
	if idErr != nil {
		return msg(fmt.Sprintf("Error: %s", idErr.Error()))
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

func (r *Registry) handleForkThread(ctx context.Context, req *mcp.CallToolRequest, args ForkThreadInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.StartMessageID == "" {
		return msg("Error: start_message_id is required.")
	}
	if args.NewRoomID == "" {
		return msg("Error: new_room_id is required.")
	}
	if err := validateSize("new_room_id", args.NewRoomID, maxIDLen); err != nil {
		return msg("Error: " + err.Error())
	}

	if err := r.resolveInto(&args.StartMessageID); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	// Look up the starting message to find the source room.
	startMsg, err := r.Server.GetMessageByID(args.StartMessageID)
	if err != nil {
		return msg(fmt.Sprintf("Error: message '%s' not found.", args.StartMessageID))
	}
	sourceRoomID := startMsg.RoomID

	// Get source room for project/description defaults.
	sourceRoom, err := r.Server.GetRoom(sourceRoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: source room '%s' not found.", sourceRoomID))
	}

	// Collect all messages from start_message_id onwards (inclusive).
	thread, err := r.Server.GetMessagesFromIDInclusive(sourceRoomID, args.StartMessageID)
	if err != nil {
		return nil, ToolOutput{}, err
	}
	if len(thread) == 0 {
		return msg(fmt.Sprintf("Error: no messages found from '%s' onwards in room '%s'.", args.StartMessageID, sourceRoomID))
	}

	// Refuse to fork into an existing room — fork_thread always creates a fresh room.
	if _, err := r.Server.GetRoom(args.NewRoomID); err == nil {
		return msg(fmt.Sprintf("Error: room '%s' already exists. fork_thread requires a new room ID.", args.NewRoomID))
	}

	// Create the new room. Passing sourceRoomID as related_rooms triggers bidirectional linking.
	topic := args.Topic
	if topic == "" {
		topic = fmt.Sprintf("Forked from %s", sourceRoomID)
	}
	project := args.Project
	if project == "" {
		project = sourceRoom.Project
	}
	if err := r.Server.CreateRoom(args.NewRoomID, topic, project, "", args.Tags, "", sourceRoomID); err != nil {
		return msg(fmt.Sprintf("Error creating room '%s': %s", args.NewRoomID, err.Error()))
	}

	// Move the messages.
	ids := make([]string, len(thread))
	for i, m := range thread {
		ids[i] = m.ID
	}
	moved, err := r.Server.MoveMessages(ids, args.NewRoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error moving messages: %s", err.Error()))
	}

	r.Server.Logger.Info("Thread forked", "from", sourceRoomID, "to", args.NewRoomID, "messages", moved)
	return msg(fmt.Sprintf(
		"Forked %d message(s) from '%s' into new room '%s'. Both rooms are now linked.\n\n```json\n{\"source_room\": \"%s\", \"new_room\": \"%s\", \"messages_moved\": %d}\n```",
		moved, sourceRoomID, args.NewRoomID, sourceRoomID, args.NewRoomID, moved,
	))
}
