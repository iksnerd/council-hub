package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SignalStatusInput represents the parameters for signaling room status.
type SignalStatusInput struct {
	RoomID string `json:"room_id"`
	Status string `json:"status"`
}

// BulkStatusInput represents the parameters for updating multiple rooms' status at once.
type BulkStatusInput struct {
	RoomIDs         string `json:"room_ids"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	Author          string `json:"author"`
	AutoArchiveDays string `json:"auto_archive_days"`
}

// RenameProjectInput represents the parameters for rewriting a project name across rooms.
type RenameProjectInput struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// BulkVisibilityInput represents the parameters for setting visibility on many rooms at once.
type BulkVisibilityInput struct {
	Visibility string `json:"visibility"`
	All        string `json:"all"`
	Project    string `json:"project"`
	RoomIDs    string `json:"room_ids"`
}

// RegenerateEmbeddingsInput represents the parameters for triggering an
// on-demand embedding backfill or full re-embed.
type RegenerateEmbeddingsInput struct {
	Full string `json:"full"`
}

func (r *Registry) handleRegenerateEmbeddings(ctx context.Context, req *mcp.CallToolRequest, args RegenerateEmbeddingsInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if r.Server.Embedder == nil {
		return msg("Error: semantic search is not enabled (COUNCIL_OLLAMA_URL not set) — nothing to embed.")
	}

	full := args.Full == "true"
	msgTotal, msgIndexed, roomTotal, roomIndexed := r.Server.EmbeddingCoverage()

	// A context tied to this request would be canceled the moment the tool
	// call returns; the job runs in the background well past that.
	if !r.Server.TriggerEmbedJob(context.Background(), full) {
		return msg(fmt.Sprintf(
			"A backfill/re-embed is already running — try again shortly. Current coverage: %d/%d messages, %d/%d rooms.",
			msgIndexed, msgTotal, roomIndexed, roomTotal,
		))
	}

	mode := "backfill (missing vectors only)"
	if full {
		mode = "full re-embed (clearing all existing vectors first, then recomputing everything)"
	}
	r.Server.Logger.Info("Embedding regeneration triggered on demand", "full", full)
	return msg(fmt.Sprintf(
		"Started a %s in the background. Coverage before: %d/%d messages, %d/%d rooms. "+
			"Check again shortly, or watch server logs / GET /health for progress.",
		mode, msgIndexed, msgTotal, roomIndexed, roomTotal,
	))
}

func (r *Registry) handleSignalStatus(ctx context.Context, req *mcp.CallToolRequest, args SignalStatusInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if !validRoomStatuses[args.Status] {
		return msg(fmt.Sprintf("Error: Invalid status '%s'. Must be one of: active, paused, resolved.", args.Status))
	}

	// Status is room metadata, so it has to land on the node that owns the room.
	// If it isn't here and we're clustered, locate the owner and forward — the same
	// thing post_to_room does for a write. Without this a room could be closed out
	// from any node (synthesis posted, decision recorded) while the status flip
	// 404'd, leaving the Knowledge Linter flagging work that was already finished.
	if _, err := r.Server.GetRoom(args.RoomID); err != nil {
		if owner, lerr := r.locateRoomOwner(args.RoomID); lerr == nil && owner != "" {
			if perr := r.proxyStatusUpdate(owner, args.RoomID, args.Status); perr != nil {
				return msg(fmt.Sprintf("Error: room '%s' is owned by cluster node '%s' but the status change could not be forwarded: %s", args.RoomID, owner, perr.Error()))
			}
			r.Server.Logger.Info("Status change proxied to owner", "room_id", args.RoomID, "owner", owner, "status", args.Status)
			return msg(fmt.Sprintf("Room '%s' status \u2192 **%s** (on cluster node %s).", args.RoomID, args.Status, owner))
		}
		// No owner found — fall through so the caller gets the canonical
		// "room not found" error from the local update.
	}

	if err := r.Server.UpdateStatus(args.RoomID, args.Status); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	r.Server.Logger.Info("Status updated", "room_id", args.RoomID, "status", args.Status)

	var b strings.Builder
	fmt.Fprintf(&b, "Room '%s' status \u2192 **%s**.", args.RoomID, args.Status)
	if room, err := r.Server.GetRoom(args.RoomID); err == nil {
		if room.Description != "" {
			fmt.Fprintf(&b, "\n**Topic:** %s", room.Description)
		}
		if room.Project != "" {
			fmt.Fprintf(&b, "\n**Project:** %s", room.Project)
		}
	}
	return msg(b.String())
}

func (r *Registry) handleBulkStatusUpdate(ctx context.Context, req *mcp.CallToolRequest, args BulkStatusInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if !validRoomStatuses[args.Status] {
		return msg(fmt.Sprintf("Error: Invalid status '%s'. Must be one of: active, paused, resolved.", args.Status))
	}

	if args.RoomIDs == "" {
		return msg("Error: room_ids is required (comma-separated list of room IDs).")
	}

	if args.Message != "" && args.Author == "" {
		return msg("Error: author is required when message is provided.")
	}

	autoArchiveDays := 0
	if args.AutoArchiveDays != "" {
		if _, err := fmt.Sscanf(args.AutoArchiveDays, "%d", &autoArchiveDays); err != nil || autoArchiveDays < 0 {
			return msg("Error: auto_archive_days must be a non-negative integer.")
		}
	}

	parts := strings.Split(args.RoomIDs, ",")
	var updated, notFound, archived []string
	for _, p := range parts {
		roomID := strings.TrimSpace(p)
		if roomID == "" {
			continue
		}
		// Validate existence before posting — messages carries no FK on room_id, so
		// posting first (the old order) left an orphan message row for any nonexistent
		// room ID instead of just reporting it not found.
		if _, err := r.Server.GetRoom(roomID); err != nil {
			notFound = append(notFound, roomID)
			continue
		}

		if args.Message != "" {
			_, _ = r.Server.PostMessage(roomID, args.Author, args.Message, "decision", "")
		}
		if err := r.Server.UpdateStatus(roomID, args.Status); err != nil {
			notFound = append(notFound, roomID)
			continue
		}
		updated = append(updated, roomID)

		// Auto-archive: only on resolved transitions, only if last activity is old enough
		if autoArchiveDays > 0 && args.Status == "resolved" {
			stats, err := r.Server.GetRoomStats(roomID)
			if err != nil {
				continue
			}
			cutoff := time.Now().Add(-time.Duration(autoArchiveDays) * 24 * time.Hour)
			if stats.MessageCount == 0 || stats.LastMessage.Before(cutoff) {
				if _, err := r.Server.ArchiveRoom(roomID); err == nil {
					if delErr := r.Server.DeleteRoom(roomID); delErr == nil {
						archived = append(archived, roomID)
					}
				}
			}
		}
	}

	var b strings.Builder
	if len(updated) > 0 {
		latestIDs := r.Server.GetLatestMessageIDs()
		fmt.Fprintf(&b, "Updated %d room(s) to '%s':\n", len(updated), args.Status)
		for _, id := range updated {
			if lid, ok := latestIDs[id]; ok {
				fmt.Fprintf(&b, "- %s (latest_message_id: %.8s)\n", id, lid)
			} else {
				fmt.Fprintf(&b, "- %s\n", id)
			}
		}
	}
	if len(archived) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Auto-archived %d room(s) inactive for \u2265%d day(s): %s\n", len(archived), autoArchiveDays, strings.Join(archived, ", "))
	}
	if len(notFound) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Not found: %s", strings.Join(notFound, ", "))
		// No per-room owner lookup here: that would be one locate_room round trip
		// per missing ID. bulk_status_update is local-only by design — a batch
		// spanning several owners has no single target and would need per-room
		// routing plus a partial-success report.
		if r.PhoenixURL != "" {
			b.WriteString("\n(Any of these owned by another cluster node cannot be updated from here — " +
				"bulk_status_update is local-only. Check with list_rooms(cluster_wide=true), then use " +
				"signal_status per room, which routes to the owner automatically.)")
		}
	}
	if b.Len() == 0 {
		return msg("No valid room IDs provided.")
	}

	r.Server.Logger.Info("Bulk status update", "status", args.Status, "updated", len(updated), "archived", len(archived), "not_found", len(notFound))
	return msg(b.String())
}

func (r *Registry) handleBulkVisibility(ctx context.Context, req *mcp.CallToolRequest, args BulkVisibilityInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	v := strings.ToLower(strings.TrimSpace(args.Visibility))
	if v != "public" && v != "private" {
		return msg("Error: visibility is required and must be 'public' or 'private'.")
	}

	all := strings.EqualFold(strings.TrimSpace(args.All), "true")
	project := strings.TrimSpace(args.Project)

	var roomIDs []string
	for _, p := range strings.Split(args.RoomIDs, ",") {
		if id := strings.TrimSpace(p); id != "" {
			roomIDs = append(roomIDs, id)
		}
	}

	// Exactly one targeting mode.
	targets := 0
	for _, set := range []bool{all, project != "", len(roomIDs) > 0} {
		if set {
			targets++
		}
	}
	if targets == 0 {
		return msg("Error: specify exactly one target — all='true', project='<name>', or room_ids='a,b,c'.")
	}
	if targets > 1 {
		return msg("Error: specify only one target — all, project, or room_ids (not more than one).")
	}

	count, err := r.Server.BulkSetVisibility(v, roomIDs, project, all)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	var scope string
	switch {
	case all:
		scope = "all rooms"
	case project != "":
		scope = fmt.Sprintf("project '%s'", project)
	default:
		scope = fmt.Sprintf("%d listed room(s)", len(roomIDs))
	}

	r.Server.Logger.Info("Bulk visibility update", "visibility", v, "scope", scope, "rooms_changed", count)

	note := ""
	if v == "private" {
		note = " They are now node-local — excluded from all cluster fan-out (cluster-wide reads and cross-node writes)."
	} else {
		note = " They are now public — visible across the cluster again."
	}
	return msg(fmt.Sprintf("Set %d room(s) to **%s** (%s).%s", count, v, scope, note))
}

func (r *Registry) handleRenameProject(ctx context.Context, req *mcp.CallToolRequest, args RenameProjectInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.From == "" || args.To == "" {
		return msg("Error: both 'from' and 'to' are required.")
	}

	count, err := r.Server.RenameProject(args.From, args.To)
	if err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	r.Server.Logger.Info("Project renamed", "from", args.From, "to", args.To, "rooms_updated", count)
	return msg(fmt.Sprintf("Renamed project '%s' \u2192 '%s' across %d room(s).", args.From, args.To, count))
}
