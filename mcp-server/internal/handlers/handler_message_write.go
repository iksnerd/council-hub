package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PostToRoomInput represents the parameters for posting a message.
type PostToRoomInput struct {
	RoomID  string `json:"room_id"`
	Author  string `json:"author"`
	Message string `json:"message"`
	// Content is an accepted alias for Message: it is the natural guess beside
	// message_type and the spelling several neighbouring MCP servers use. This
	// is the one alias in the set that is expensive to get wrong, because the
	// rejection arrives only after the whole body has been transmitted.
	Content      string `json:"content"`
	MessageType  string `json:"message_type"`
	ReplyTo      string `json:"reply_to"`
	Mentions     string `json:"mentions"`
	Supersedes   string `json:"supersedes"`
	MarkReadSelf string `json:"mark_read_self"`
	Pin          string `json:"pin"`
	Workspace    string `json:"workspace"`
	// Force lets pin=true replace another author's fresh pin (see PinFreshWindow).
	Force string `json:"force"`
	// Reopen flips a resolved room back to active as part of the post.
	Reopen string `json:"reopen"`
	// Tags is a CSV of message-level tags (e.g. "meta-feedback").
	Tags string `json:"tags"`
	// Links is "relation:ref,relation:ref" (e.g. "refines:#01a0b156"), validated
	// before anything is written.
	Links string `json:"links"`
	// IdempotencyKey makes a retry after a timeout return the first message.
	IdempotencyKey string `json:"idempotency_key"`
}

// UpdateMessageInput represents the parameters for editing a message. The edit is
// append-only: it posts a new revision and preserves the prior version.
type UpdateMessageInput struct {
	MessageID       string `json:"message_id"`
	Content         string `json:"content"`
	MessageType     string `json:"message_type"`
	ExpectedContent string `json:"expected_content"`
	Author          string `json:"author"`
	Append          string `json:"append"`
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

	body, aerr := resolveAlias(args.Message, "message", args.Content, "content")
	if aerr != nil {
		return msg("Error: " + aerr.Error())
	}
	args.Message = body

	// author defaults to the connected client's name — the only identity this
	// server actually knows. Without it, the rejection lands after the whole
	// body has been transmitted, so the retry pays for the content twice.
	if args.Author == "" {
		args.Author = clientIdentity(req)
	}

	if args.RoomID == "" || args.Author == "" || args.Message == "" {
		return msg("Error: room_id and message (alias: content) are required; author is required when the client did not identify itself at initialize.")
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

	// Shared-checkout detection. A working tree is a property of this machine, not
	// of the room, so this is recorded locally even when the message itself is
	// proxied to an owner on another node — the poster is here either way. Peers are
	// read before recording self, which is the order the note describes: who else is
	// in this tree, then note that we are too.
	workspaceNote := ""
	if args.Workspace != "" {
		if err := validateSize("workspace", args.Workspace, maxWorkspaceLen); err != nil {
			return msg("Error: " + err.Error())
		}
		peers, werr := r.Server.SharedWorkspacePeers(args.Author, args.Workspace)
		if werr != nil {
			r.Server.Logger.Warn("shared-workspace lookup failed", "author", args.Author, "error", werr)
		} else {
			workspaceNote = council.SharedWorkspaceWarning(council.NormalizeWorkspace(args.Workspace), peers)
		}
		if werr := r.Server.RecordWorkspace(args.Author, args.RoomID, args.Workspace); werr != nil {
			r.Server.Logger.Warn("workspace record failed", "author", args.Author, "error", werr)
		}
	}

	// posted_from separates sessions that all post under one author name: the
	// declared working tree when there is one, else the connected client's name.
	postedFrom := council.NormalizeWorkspace(args.Workspace)
	if postedFrom == "" {
		postedFrom = clientIdentity(req)
	}
	if err := validateSize("tags", args.Tags, 500); err != nil {
		return msg("Error: " + err.Error())
	}
	if err := validateSize("idempotency_key", args.IdempotencyKey, maxIDLen); err != nil {
		return msg("Error: " + err.Error())
	}
	if err := validateSize("links", args.Links, 2000); err != nil {
		return msg("Error: " + err.Error())
	}
	in := internalPostRequest{
		RoomID: args.RoomID, Author: args.Author, Message: args.Message, MessageType: args.MessageType,
		ReplyTo: args.ReplyTo, Mentions: args.Mentions, Supersedes: args.Supersedes, Pin: args.Pin,
		Force: args.Force, Reopen: args.Reopen, Tags: args.Tags, Links: args.Links,
		IdempotencyKey: args.IdempotencyKey, PostedFrom: postedFrom,
	}

	// Verify room exists locally. If it doesn't and we're clustered, the room may
	// be owned by a peer node — locate the owner and proxy the write there rather
	// than silently creating a local shadow. The owner runs the same applyPost.
	if _, err := r.Server.GetRoom(args.RoomID); err != nil {
		if owner, lerr := r.locateRoomOwner(args.RoomID); lerr == nil && owner != "" {
			out, perr := r.proxyPostToRoom(owner, in)
			if perr != nil {
				return msg(fmt.Sprintf("Error: room '%s' is owned by cluster node '%s' but the write could not be forwarded: %s", args.RoomID, owner, perr.Error()))
			}
			r.Server.Logger.Info("Message proxied to owner", "room_id", args.RoomID, "owner", owner, "msg_id", out.MessageID, "pinned", out.Pinned)
			return msg(formatPostResult(args, out, owner) + workspaceNote)
		}
		return msg(fmt.Sprintf("Error: Room '%s' not found. Create it first with create_room.", args.RoomID))
	}

	out, err := r.applyPost(in)
	if err != nil {
		if ue, ok := err.(postUserError); ok {
			return msg("Error: " + string(ue))
		}
		r.Server.Logger.Error("Failed to post message", "room_id", args.RoomID, "error", err)
		return nil, ToolOutput{}, err
	}

	// mark_read_self folds the end-of-session mark_read round-trip into the write:
	// the poster just authored this message, so advance their own cursor to it.
	if args.MarkReadSelf == "true" {
		if err := r.Server.MarkRead(args.Author, args.RoomID, out.MessageID); err != nil {
			r.Server.Logger.Warn("mark_read_self failed", "room_id", args.RoomID, "author", args.Author, "error", err)
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

	r.Server.Logger.Info("Message posted", "room_id", args.RoomID, "author", args.Author, "type", args.MessageType, "msg_id", out.MessageID, "pinned", out.Pinned)
	return msg(formatPostResult(args, out, "") + healthHint + workspaceNote)
}

// PinFreshWindow is how recent another author's pin must be for post_to_room
// (pin=true) to refuse replacing it without force=true. A pin is the room's
// living abstract and replacing a peer's mid-session is a silent overwrite.
const PinFreshWindow = 12 * time.Hour

// postUserError marks a validation failure the caller should read as "Error: ..."
// rather than an infrastructure failure to log.
type postUserError string

func (e postUserError) Error() string { return string(e) }

// postOutcome is what applyPost reports; the same shape crosses the cluster proxy.
type postOutcome struct {
	MessageID   string
	Duplicate   bool
	Pinned      bool
	PinRefused  bool
	ReplacedPin string
	Reopened    bool
	WasResolved bool // room is still resolved after the post
	Notes       []string
}

// splitRoomRef splits an optional "room#id" qualifier. A bare "#id" or "id"
// scopes to defaultRoom.
func splitRoomRef(ref, defaultRoom string) (room, id string) {
	ref = strings.TrimSpace(ref)
	if room, id, ok := strings.Cut(ref, "#"); ok {
		if room == "" {
			room = defaultRoom
		}
		return room, id
	}
	return defaultRoom, ref
}

// resolveRef resolves a message reference written while posting to roomID: a full
// id, an 8+ char prefix, or room#prefix. Lookup is room-scoped first, so a prefix
// shared with another room is not ambiguity.
func (r *Registry) resolveRef(ref, roomID string) (string, error) {
	room, id := splitRoomRef(ref, roomID)
	return r.Server.ResolveMessageIDInRoom(id, room)
}

type pendingLink struct{ relation, toID string }

// parseLinks validates "relation:ref,relation:ref" in full before anything is
// written, so a bad entry rejects the whole post instead of half-writing it.
func (r *Registry) parseLinks(spec, roomID string) ([]pendingLink, error) {
	var out []pendingLink
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		rel, ref, ok := strings.Cut(part, ":")
		rel = strings.ToLower(strings.TrimSpace(rel))
		if !ok || strings.TrimSpace(ref) == "" {
			return nil, fmt.Errorf("links: %q must look like relation:#id (e.g. refines:#01a0b156)", part)
		}
		if !council.ValidLinkRelations[rel] {
			return nil, fmt.Errorf("links: unknown relation %q (valid: refines, contradicts, implements, duplicates, depends-on, relates, informs)", rel)
		}
		id, err := r.resolveRef(ref, roomID)
		if err != nil {
			return nil, fmt.Errorf("links: %q: %s", part, err.Error())
		}
		out = append(out, pendingLink{rel, id})
	}
	return out, nil
}

// applyPost is the one write path behind post_to_room, shared by the local handler
// and the cluster InternalPostHandler so both validate and behave identically. The
// room must exist locally. Validation (refs, links, pin guard) runs before the
// first write, so a rejected post leaves nothing behind.
func (r *Registry) applyPost(in internalPostRequest) (postOutcome, error) {
	var out postOutcome
	room, err := r.Server.GetRoom(in.RoomID)
	if err != nil {
		return out, postUserError(fmt.Sprintf("room '%s' not found on this node", in.RoomID))
	}
	if in.MessageType == "" {
		in.MessageType = "message"
	}
	if !validMessageTypes[in.MessageType] {
		return out, postUserError(fmt.Sprintf("Invalid message_type '%s'.", in.MessageType))
	}

	// Resolve reply_to/supersedes to full IDs before storing them. The store keeps
	// these verbatim and downstream matching (backlinks, revision chains,
	// get_links) is exact, so a stored prefix or typo'd ID would create a silently
	// dangling edge: unlike resolveSingleID's pass-through this errors on
	// not-found too.
	for _, ref := range []struct {
		name  string
		value *string
	}{{"reply_to", &in.ReplyTo}, {"supersedes", &in.Supersedes}} {
		if *ref.value == "" {
			continue
		}
		resolved, rerr := r.resolveRef(*ref.value, in.RoomID)
		if rerr != nil {
			return out, postUserError(fmt.Sprintf("%s: %s", ref.name, rerr.Error()))
		}
		*ref.value = resolved
	}

	links, lerr := r.parseLinks(in.Links, in.RoomID)
	if lerr != nil {
		return out, postUserError(lerr.Error())
	}

	// Pin guard: another author's pin younger than PinFreshWindow is not replaced
	// silently. The message still posts (its body is already on the wire); only the
	// pin is withheld, and the response names what it would have displaced.
	var refusedPin *council.Message
	if in.Pin == "true" && in.Force != "true" {
		if cur, _ := r.Server.GetPinnedMessage(in.RoomID); cur != nil && cur.Author != in.Author && time.Since(cur.Timestamp) < PinFreshWindow {
			refusedPin = cur
		}
	}

	id, dup, err := r.Server.PostMessageOpts(in.RoomID, in.Author, in.Message, in.MessageType, in.ReplyTo, in.Mentions, in.Supersedes, council.PostOptions{
		Tags:           council.NormalizeMessageTags(in.Tags),
		PostedFrom:     in.PostedFrom,
		IdempotencyKey: in.IdempotencyKey,
	})
	if err != nil {
		return out, err
	}
	out.MessageID = id
	if dup {
		out.Duplicate = true
		out.Notes = append(out.Notes, fmt.Sprintf("Duplicate: idempotency_key matched an earlier post, so nothing was written; this is that message (#%s).", id))
		return out, nil
	}

	if room.Status == "resolved" {
		if in.Reopen == "true" {
			if uerr := r.Server.UpdateStatus(in.RoomID, "active"); uerr != nil {
				r.Server.Logger.Warn("reopen-on-post failed", "room_id", in.RoomID, "error", uerr)
				out.WasResolved = true
			} else {
				out.Reopened = true
				out.Notes = append(out.Notes, "Room was resolved; reopened to active.")
			}
		} else {
			out.WasResolved = true
			out.Notes = append(out.Notes, "⚠️ This room is resolved: your message landed, but readers who skip resolved rooms will not see it. Pass reopen=true to flip it back to active, or signal_status(active).")
		}
	}

	for _, l := range links {
		if _, cerr := r.Server.CreateLink(id, l.toID, l.relation, in.Author); cerr != nil {
			r.Server.Logger.Warn("link-on-post failed", "from", id, "to", l.toID, "relation", l.relation, "error", cerr)
			out.Notes = append(out.Notes, fmt.Sprintf("link %s:#%.8s failed: %s", l.relation, l.toID, cerr.Error()))
		}
	}

	// pin folds the post->pin dance into one call: pin the just-authored message
	// (auto-unpinning the room's previous pin, identical to pin_message).
	if in.Pin == "true" {
		if refusedPin != nil {
			out.PinRefused = true
			out.Notes = append(out.Notes, fmt.Sprintf("📌 NOT pinned: the room's current pin #%s was posted by %s %s ago (under %dh), and replacing another author's fresh pin needs force=true. To replace it anyway: pin_message(room_id=%q, message_id=%q).",
				refusedPin.ID, refusedPin.Author, time.Since(refusedPin.Timestamp).Round(time.Minute), int(PinFreshWindow.Hours()), in.RoomID, id))
		} else if _, replaced, perr := r.Server.PinMessageReplacing(in.RoomID, id); perr != nil {
			r.Server.Logger.Warn("pin-on-post failed", "room_id", in.RoomID, "msg_id", id, "error", perr)
		} else {
			out.Pinned, out.ReplacedPin = true, replaced
		}
	}
	return out, nil
}

// formatPostResult renders the post_to_room response for both the local and the
// proxied path. owner is the cluster node name when the write was forwarded.
func formatPostResult(args PostToRoomInput, out postOutcome, owner string) string {
	where := fmt.Sprintf("room '%s'", args.RoomID)
	if owner != "" {
		where = fmt.Sprintf("room '%s' (on cluster node %s)", args.RoomID, owner)
	}
	verb := "posted to"
	if out.Duplicate {
		verb = "already in"
	}
	pinNote := ""
	if args.Pin == "true" && !out.Duplicate {
		switch {
		case out.Pinned:
			pinNote = pinConfirmation(out.ReplacedPin)
		case out.PinRefused:
			// explained in Notes
		case owner != "":
			pinNote = " (pin failed on the owner node — pin it manually with pin_message there)"
		default:
			pinNote = " (pin failed — pin it manually with pin_message)"
		}
	}
	ownerField := ""
	if owner != "" {
		ownerField = fmt.Sprintf(", \"owner_node\": \"%s\"", owner)
	}
	text := fmt.Sprintf("Message #%.8s %s %s by %s.%s\n\n```json\n{\"message_id\": \"%s\", \"room_id\": \"%s\", \"latest_message_id\": \"%s\", \"ref\": \"%s#%s\"%s}\n```",
		out.MessageID, verb, where, args.Author, pinNote, out.MessageID, args.RoomID, out.MessageID, args.RoomID, out.MessageID, ownerField)
	for _, n := range out.Notes {
		text += "\n\n" + n
	}
	return text
}

func (r *Registry) handleUpdateMessage(ctx context.Context, req *mcp.CallToolRequest, args UpdateMessageInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.MessageID == "" {
		return msg("Error: message_id is required.")
	}
	if args.Content == "" && args.Append == "" {
		return msg("Error: content or append is required.")
	}
	if args.Content != "" && args.Append != "" {
		return msg("Error: pass content or append, not both — content replaces the message, append adds to it.")
	}

	if args.MessageType != "" && !validMessageTypes[args.MessageType] {
		return msg(fmt.Sprintf("Error: invalid message_type '%s'. Valid types: message, thought, draft, decision, plan, review, action, critique, synthesis, note.", args.MessageType))
	}
	if err := r.resolveInto(&args.MessageID); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	// append builds the new content from the stored one, so a caller correcting
	// a long entry does not have to re-transmit it. Re-transmitting is not just
	// wasteful: every character is a chance to corrupt a record that is supposed
	// to be immutable, and the caller usually cannot diff what it sent against
	// what was there.
	//
	// The read-then-write is made safe by the guard that already exists for it:
	// the content we read becomes expected_content, so a concurrent edit fails
	// the update rather than silently appending to a stale body. An explicit
	// expected_content from the caller wins, since theirs is the stricter claim.
	if args.Append != "" {
		cur, gerr := r.Server.GetMessageByID(args.MessageID)
		if gerr != nil {
			if gerr == sql.ErrNoRows {
				return msg(fmt.Sprintf("Error: message #%.8s not found.", args.MessageID))
			}
			return msg(fmt.Sprintf("Error: %s", gerr.Error()))
		}
		if cur.Revised {
			return msg(fmt.Sprintf("Error: message #%.8s has already been revised; append to the current head instead.", args.MessageID))
		}
		if args.ExpectedContent == "" {
			args.ExpectedContent = cur.Content
		}
		args.Content = cur.Content + "\n\n" + args.Append
	}

	if err := validateSize("content", args.Content, maxContentLen); err != nil {
		return msg("Error: " + err.Error())
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
		return msg(fmt.Sprintf("Error: source room '%s' not found.%s", sourceRoomID, r.remoteRoomNote(sourceRoomID, remedyLocalOnly)))
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

// pinConfirmation renders the pin-on-post note. It names the displaced pin only
// when there was one: a first pin replaced nothing, and saying otherwise sends
// the caller looking for a pin it never overwrote. The ID is printed in full:
// UUIDv7 IDs share their first 8 hex characters for about a minute, so a short
// prefix usually matches the new message too.
func pinConfirmation(replacedPin string) string {
	if replacedPin == "" {
		return " 📌 pinned"
	}
	return fmt.Sprintf(" 📌 pinned (replaced #%s)", replacedPin)
}
