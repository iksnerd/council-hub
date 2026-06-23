package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateRoomInput represents the parameters for creating a room.
type CreateRoomInput struct {
	ID           string `json:"id"`
	Template     string `json:"template"`
	Topic        string `json:"topic"`
	Project      string `json:"project"`
	TechStack    string `json:"tech_stack"`
	Tags         string `json:"tags"`
	SystemPrompt string `json:"system_prompt"`
	RelatedRooms string `json:"related_rooms"`
	Visibility   string `json:"visibility"`
	Repo         string `json:"repo"`
}

// GetOrCreateRoomInput represents the parameters for upserting a room.
type GetOrCreateRoomInput struct {
	ID           string `json:"id"`
	Topic        string `json:"topic"`
	Project      string `json:"project"`
	TechStack    string `json:"tech_stack"`
	Tags         string `json:"tags"`
	SystemPrompt string `json:"system_prompt"`
	RelatedRooms string `json:"related_rooms"`
	Visibility   string `json:"visibility"`
	Repo         string `json:"repo"`
	LastN        string `json:"last_n"`
}

// UpdateRoomInput represents the parameters for updating a room's metadata.
type UpdateRoomInput struct {
	RoomID       string `json:"room_id"`
	RoomIDs      string `json:"room_ids"`
	WhereProject string `json:"where_project"`
	Topic        string `json:"topic"`
	Project      string `json:"project"`
	TechStack    string `json:"tech_stack"`
	Tags         string `json:"tags"`
	AddTags      string `json:"add_tags"`
	RemoveTags   string `json:"remove_tags"`
	SystemPrompt string `json:"system_prompt"`
	RelatedRooms string `json:"related_rooms"`
	Visibility   string `json:"visibility"`
	Repo         string `json:"repo"`
}

// ReadRoomInput represents the parameters for reading a room's metadata.
type ReadRoomInput struct {
	RoomID                  string `json:"room_id"`
	ClusterWide             string `json:"cluster_wide"`
	IncludeRelatedSummaries string `json:"include_related_summaries"`
	IncludeLastN            string `json:"include_last_n"`
}

// DeleteRoomInput represents the parameters for deleting a room.
type DeleteRoomInput struct {
	RoomID string `json:"room_id"`
}

func (r *Registry) handleCreateRoom(ctx context.Context, req *mcp.CallToolRequest, args CreateRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.ID == "" {
		return msg("Error: room id is required.")
	}
	if err := validateSize("id", args.ID, maxIDLen); err != nil {
		return msg("Error: " + err.Error())
	}
	if err := validateRoomMetadata(args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt); err != nil {
		return msg("Error: " + err.Error())
	}

	// Apply template defaults (explicit user args take precedence)
	if args.Template != "" {
		tpl, ok := roomTemplates[args.Template]
		if !ok {
			return msg(fmt.Sprintf("Error: unknown template '%s'. Available: %s",
				args.Template, strings.Join(templateNames(), ", ")))
		}
		if args.Topic == "" {
			args.Topic = tpl.Topic
		}
		if args.Tags == "" {
			args.Tags = tpl.Tags
		}
		if args.TechStack == "" {
			args.TechStack = tpl.TechStack
		}
		if args.SystemPrompt == "" {
			args.SystemPrompt = tpl.SystemPrompt
		}
	}

	// Pre-check existence so we know whether to post the initial message
	_, roomAlreadyExists := r.Server.GetRoom(args.ID)

	// Z1 conflict guard: if the room doesn't exist locally but a public peer owns
	// the same ID, refuse to create a local shadow — name the owner instead.
	if roomAlreadyExists != nil {
		if owner, lerr := r.locateRoomOwner(args.ID); lerr == nil && owner != "" {
			return msg(fmt.Sprintf("Error: room '%s' already exists on cluster node '%s'. Use post_to_room to participate in it, read_room(cluster_wide=true) to view it, or choose a different id.", args.ID, owner))
		}
	}

	if err := r.Server.CreateRoom(args.ID, args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt, args.RelatedRooms); err != nil {
		r.Server.Logger.Error("Failed to create room", "id", args.ID, "error", err)
		return nil, ToolOutput{}, err
	}

	// Apply visibility (defaults to public at the DB level; only act on private).
	if args.Visibility != "" {
		if err := r.Server.SetVisibility(args.ID, args.Visibility); err != nil {
			r.Server.Logger.Error("Failed to set room visibility", "id", args.ID, "error", err)
		}
	}

	// Apply repo (used for {sha:...} commit-link resolution at render time).
	if args.Repo != "" {
		if err := r.Server.SetRepo(args.ID, args.Repo); err != nil {
			r.Server.Logger.Error("Failed to set room repo", "id", args.ID, "error", err)
		}
	}

	// Post initial message for new rooms created from a template
	if args.Template != "" && roomAlreadyExists != nil {
		tpl := roomTemplates[args.Template]
		if tpl.InitialMsg != "" {
			r.Server.PostMessage(args.ID, "system", tpl.InitialMsg, "thought", "") //nolint:errcheck
		}
	}

	r.Server.Logger.Info("Room created", "id", args.ID, "project", args.Project, "topic", args.Topic)

	var b strings.Builder
	fmt.Fprintf(&b, "Room '%s' created.\n", args.ID)
	if args.Template != "" {
		fmt.Fprintf(&b, "**Template:** %s\n", args.Template)
	}
	if args.Topic != "" {
		fmt.Fprintf(&b, "**Topic:** %s\n", args.Topic)
	}
	if args.Project != "" {
		fmt.Fprintf(&b, "**Project:** %s\n", args.Project)
	}
	if args.Tags != "" {
		fmt.Fprintf(&b, "**Tags:** %s\n", args.Tags)
	}
	if args.RelatedRooms != "" {
		fmt.Fprintf(&b, "**Related rooms:** %s (bidirectional links created)\n", args.RelatedRooms)
	}
	if args.Repo != "" {
		fmt.Fprintf(&b, "**Repo:** %s ({sha:...} tokens resolve to commit links)\n", args.Repo)
	}
	if strings.EqualFold(strings.TrimSpace(args.Visibility), "private") {
		fmt.Fprintf(&b, "**Visibility:** private (node-local — excluded from cluster fan-out)\n")
	}

	if args.RelatedRooms == "" {
		fmt.Fprintf(&b, "\n**Tip:** No related_rooms set — link parent/sibling rooms for cross-room navigation.\n")
	}

	// Advisory duplicate check — never blocks creation.
	r.appendSimilarRooms(&b, args.ID, args.Topic, args.Project, args.Tags)

	return msg(b.String())
}

func (r *Registry) handleGetOrCreateRoom(ctx context.Context, req *mcp.CallToolRequest, args GetOrCreateRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.ID == "" {
		return msg("Error: id is required.")
	}
	if err := validateSize("id", args.ID, maxIDLen); err != nil {
		return msg("Error: " + err.Error())
	}
	if err := validateRoomMetadata(args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt); err != nil {
		return msg("Error: " + err.Error())
	}

	// Try to get existing room
	room, err := r.Server.GetRoom(args.ID)
	created := false
	if err != nil {
		// Z1 conflict guard: don't create a local shadow of a room a public peer owns.
		if owner, lerr := r.locateRoomOwner(args.ID); lerr == nil && owner != "" {
			return msg(fmt.Sprintf("Error: room '%s' already exists on cluster node '%s'. Use post_to_room to participate in it, or read_room(cluster_wide=true) to view it.", args.ID, owner))
		}
		// Room doesn't exist — create it
		if err := r.Server.CreateRoom(args.ID, args.Topic, args.Project, args.TechStack, args.Tags, args.SystemPrompt, args.RelatedRooms); err != nil {
			r.Server.Logger.Error("Failed to create room", "id", args.ID, "error", err)
			return nil, ToolOutput{}, err
		}
		if args.Visibility != "" {
			if err := r.Server.SetVisibility(args.ID, args.Visibility); err != nil {
				r.Server.Logger.Error("Failed to set room visibility", "id", args.ID, "error", err)
			}
		}
		if args.Repo != "" {
			if err := r.Server.SetRepo(args.ID, args.Repo); err != nil {
				r.Server.Logger.Error("Failed to set room repo", "id", args.ID, "error", err)
			}
		}
		room, _ = r.Server.GetRoom(args.ID)
		created = true
	}

	// Backfill: on an *existing* room, fill metadata fields that are still empty
	// from the matching creation args (idempotent upsert). This lets a room created
	// before a project/tag convention adopt it without a separate update_room call —
	// the gap that left early rooms untagged forever. Gap-fill only: a field the room
	// already has is never overwritten (that's update_room's job), and visibility is
	// excluded (it DB-defaults to "public", so it's never "empty" to fill).
	var backfilled []string
	if !created {
		var bfTopic, bfProject, bfTechStack, bfTags, bfSystemPrompt, bfRelated string
		if args.Topic != "" && room.Description == "" {
			bfTopic = args.Topic
			backfilled = append(backfilled, "topic")
		}
		if args.Project != "" && room.Project == "" {
			bfProject = args.Project
			backfilled = append(backfilled, "project")
		}
		if args.TechStack != "" && room.TechStack == "" {
			bfTechStack = args.TechStack
			backfilled = append(backfilled, "tech_stack")
		}
		if args.Tags != "" && room.Tags == "" {
			bfTags = args.Tags
			backfilled = append(backfilled, "tags")
		}
		if args.SystemPrompt != "" && room.SystemPrompt == "" {
			bfSystemPrompt = args.SystemPrompt
			backfilled = append(backfilled, "system_prompt")
		}
		if args.RelatedRooms != "" && room.RelatedRooms == "" {
			bfRelated = args.RelatedRooms
			backfilled = append(backfilled, "related_rooms")
		}
		if bfTopic != "" || bfProject != "" || bfTechStack != "" || bfTags != "" || bfSystemPrompt != "" || bfRelated != "" {
			if err := r.Server.UpdateRoom(args.ID, bfTopic, bfProject, bfTechStack, bfTags, "", "", bfSystemPrompt, bfRelated); err != nil {
				r.Server.Logger.Error("Failed to backfill room metadata", "id", args.ID, "error", err)
			}
		}
		if args.Repo != "" && room.Repo == "" {
			if err := r.Server.SetRepo(args.ID, args.Repo); err != nil {
				r.Server.Logger.Error("Failed to backfill room repo", "id", args.ID, "error", err)
			} else {
				backfilled = append(backfilled, "repo")
			}
		}
		if len(backfilled) > 0 {
			room, _ = r.Server.GetRoom(args.ID)
		}
	}

	limit := 5
	if args.LastN != "" {
		if _, err := fmt.Sscanf(args.LastN, "%d", &limit); err != nil {
			limit = 5
		}
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	var b strings.Builder
	if created {
		fmt.Fprintf(&b, "**Created** room '%s'.\n", room.ID)
	} else {
		fmt.Fprintf(&b, "**Found** room '%s'.\n", room.ID)
		if len(backfilled) > 0 {
			fmt.Fprintf(&b, "**Backfilled** (were empty): %s\n", strings.Join(backfilled, ", "))
		}
	}
	fmt.Fprintf(&b, "**%s** [%s]\n", room.ID, room.Status)
	fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
	if room.SystemPrompt != "" {
		fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
	}
	if room.Visibility == "private" {
		fmt.Fprintf(&b, "**Visibility:** private (node-local)\n")
	}
	if room.Repo != "" {
		fmt.Fprintf(&b, "**Repo:** %s\n", room.Repo)
	}

	if !created {
		messages, _ := r.Server.GetRecentMessages(args.ID, limit)
		if len(messages) > 0 {
			fmt.Fprintf(&b, "---\n**Recent messages (%d):**\n", len(messages))
			for _, m := range messages {
				appendMessageBlock(&b, m.ID, m.Timestamp.Format("2006-01-02 15:04:05"), m.Author, m.MessageType, m.Content, room.Repo)
			}
		} else {
			b.WriteString("No messages yet.\n")
		}
	}

	// Hint to link related rooms on creation
	if created && args.RelatedRooms == "" {
		fmt.Fprintf(&b, "\n**Tip:** No related_rooms set — link parent/sibling rooms for cross-room navigation.\n")
	}

	// Advisory checks on newly created rooms only.
	if created {
		r.appendSimilarRooms(&b, args.ID, args.Topic, args.Project, args.Tags)
		b.WriteString(r.repoProjectHint(args.Repo, room.ID, room.Project))
	}

	r.Server.Logger.Info("get_or_create_room", "id", args.ID, "created", created)
	return msg(b.String())
}

// repoProjectHint nudges a new room toward the project its repo's other rooms
// already use, so a standalone repo's rooms don't scatter across projects.
// Returns "" when there's no repo, no other rooms share it, or the chosen project
// already matches the dominant one. chosenProject must be the stored (normalized)
// project so the comparison lines up.
func (r *Registry) repoProjectHint(repo, excludeID, chosenProject string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ""
	}
	var proj string
	var n int
	err := r.Server.DB.QueryRow(
		`SELECT project, COUNT(*) c FROM rooms WHERE repo = ? AND id != ? AND project != '' GROUP BY project ORDER BY c DESC LIMIT 1`,
		repo, excludeID,
	).Scan(&proj, &n)
	if err != nil || proj == "" || proj == chosenProject {
		return ""
	}
	return fmt.Sprintf("\n**Tip:** %d existing room(s) with repo `%s` are grouped under project `%s` — consider update_room(room_id=%s, project=%s) to keep this repo's rooms together.\n", n, repo, proj, excludeID, proj)
}

func (r *Registry) handleUpdateRoom(ctx context.Context, req *mcp.CallToolRequest, args UpdateRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	// Collect all target room IDs (room_ids takes comma-separated list; room_id is single/legacy)
	seen := map[string]bool{}
	var ids []string
	for _, id := range strings.Split(args.RoomIDs, ",") {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if args.RoomID != "" && !seen[args.RoomID] {
		seen[args.RoomID] = true
		ids = append(ids, args.RoomID)
	}

	// where_project expands the target set to every room currently in that project
	if args.WhereProject != "" {
		matched, listErr := r.Server.ListRooms(args.WhereProject, "", "", "", 100, 0)
		if listErr != nil {
			return msg(fmt.Sprintf("Error: failed to expand where_project: %s", listErr.Error()))
		}
		for _, rm := range matched {
			if !seen[rm.ID] {
				seen[rm.ID] = true
				ids = append(ids, rm.ID)
			}
		}
	}

	if len(ids) == 0 {
		return msg("Error: room_id, room_ids, or where_project is required.")
	}

	if args.Topic == "" && args.Project == "" && args.TechStack == "" && args.Tags == "" && args.AddTags == "" && args.RemoveTags == "" && args.SystemPrompt == "" && args.RelatedRooms == "" && args.Visibility == "" && args.Repo == "" {
		return msg("Error: at least one field to update must be provided (topic, project, tech_stack, tags, add_tags, remove_tags, system_prompt, related_rooms, visibility, repo).")
	}

	var updated []string
	if args.Topic != "" {
		updated = append(updated, "topic")
	}
	if args.Project != "" {
		updated = append(updated, "project")
	}
	if args.TechStack != "" {
		updated = append(updated, "tech_stack")
	}
	if args.Tags != "" {
		updated = append(updated, "tags")
	}
	if args.AddTags != "" {
		updated = append(updated, "add_tags")
	}
	if args.RemoveTags != "" {
		updated = append(updated, "remove_tags")
	}
	if args.SystemPrompt != "" {
		updated = append(updated, "system_prompt")
	}
	if args.RelatedRooms != "" {
		updated = append(updated, "related_rooms")
	}
	if args.Visibility != "" {
		updated = append(updated, "visibility")
	}
	if args.Repo != "" {
		updated = append(updated, "repo")
	}
	fieldsLabel := strings.Join(updated, ", ")

	var b strings.Builder
	for _, id := range ids {
		if err := r.Server.UpdateRoom(id, args.Topic, args.Project, args.TechStack, args.Tags, args.AddTags, args.RemoveTags, args.SystemPrompt, args.RelatedRooms); err != nil {
			fmt.Fprintf(&b, "Error updating '%s': %s\n", id, err.Error())
			continue
		}
		if args.Visibility != "" {
			if err := r.Server.SetVisibility(id, args.Visibility); err != nil {
				fmt.Fprintf(&b, "Error setting visibility on '%s': %s\n", id, err.Error())
				continue
			}
		}
		if args.Repo != "" {
			if err := r.Server.SetRepo(id, args.Repo); err != nil {
				fmt.Fprintf(&b, "Error setting repo on '%s': %s\n", id, err.Error())
				continue
			}
		}
		r.Server.Logger.Info("Room updated", "room_id", id, "fields", fieldsLabel)
		fmt.Fprintf(&b, "Room '%s' updated: %s.\n", id, fieldsLabel)
	}

	// For single-room updates, append current state (backward-compatible behaviour)
	if len(ids) == 1 {
		if room, err := r.Server.GetRoom(ids[0]); err == nil {
			fmt.Fprintf(&b, "\n**Current state:**")
			if room.Description != "" {
				fmt.Fprintf(&b, "\n- Topic: %s", room.Description)
			}
			if room.Project != "" {
				fmt.Fprintf(&b, "\n- Project: %s", room.Project)
			}
			if room.Tags != "" {
				fmt.Fprintf(&b, "\n- Tags: %s", room.Tags)
			}
			if room.RelatedRooms != "" {
				fmt.Fprintf(&b, "\n- Related rooms: %s", room.RelatedRooms)
			}
			if room.Repo != "" {
				fmt.Fprintf(&b, "\n- Repo: %s", room.Repo)
			}
			fmt.Fprintf(&b, "\n- Status: %s", room.Status)
		}
	}

	return msg(strings.TrimRight(b.String(), "\n"))
}

func (r *Registry) handleReadRoom(ctx context.Context, req *mcp.CallToolRequest, args ReadRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	if args.ClusterWide == "true" {
		return r.handleReadRoomCluster(args)
	}

	msg := textResult

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	room, err := r.Server.GetRoom(args.RoomID)
	if err != nil {
		return msg(fmt.Sprintf("Error: Room '%s' not found.", args.RoomID))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%s** [%s]\n", room.ID, room.Status)
	fmt.Fprintf(&b, "**Topic:** %s\n", room.Description)
	if room.Project != "" {
		fmt.Fprintf(&b, "**Project:** %s\n", room.Project)
	}
	if room.TechStack != "" {
		fmt.Fprintf(&b, "**Tech Stack:** %s\n", room.TechStack)
	}
	if room.Tags != "" {
		fmt.Fprintf(&b, "**Tags:** %s\n", room.Tags)
	}
	if room.SystemPrompt != "" {
		fmt.Fprintf(&b, "**System Prompt:** %s\n", room.SystemPrompt)
	}
	if room.RelatedRooms != "" {
		fmt.Fprintf(&b, "**Related Rooms:** %s\n", room.RelatedRooms)
	}
	if room.Visibility == "private" {
		fmt.Fprintf(&b, "**Visibility:** private (node-local — excluded from cluster fan-out)\n")
	}
	if room.Repo != "" {
		fmt.Fprintf(&b, "**Repo:** %s\n", room.Repo)
	}
	fmt.Fprintf(&b, "**Created:** %s\n", room.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "**Updated:** %s\n", room.UpdatedAt.Format("2006-01-02 15:04:05"))

	// Content. include_last_n requests the raw recent feed; otherwise read_room
	// orients by default — folding in the pinned message + latest-per-type (the
	// read_transcript(mode=summary) view) so the call shows the room, not just its
	// header. (The bare-header behaviour read like a no-op and cost a round-trip.)
	if args.IncludeLastN != "" {
		lastN := 0
		_, _ = fmt.Sscanf(args.IncludeLastN, "%d", &lastN)
		if lastN > 50 {
			lastN = 50
		}
		if lastN > 0 {
			messages, _ := r.Server.GetRecentMessages(args.RoomID, lastN)
			if len(messages) > 0 {
				fmt.Fprintf(&b, "\n---\n**Recent messages (%d):**\n", len(messages))
				for _, m := range messages {
					appendMessageBlock(&b, m.ID, m.Timestamp.Format("2006-01-02 15:04:05"), m.Author, m.MessageType, m.Content, room.Repo)
				}
			}
		}
	} else {
		r.appendRoomSummary(&b, args.RoomID, room.Repo)
	}

	// Append related room summaries if requested
	if args.IncludeRelatedSummaries == "true" && room.RelatedRooms != "" {
		relatedIDs := strings.Split(room.RelatedRooms, ",")
		for _, rid := range relatedIDs {
			rid = strings.TrimSpace(rid)
			if rid == "" {
				continue
			}
			relRoom, err := r.Server.GetRoom(rid)
			if err != nil {
				fmt.Fprintf(&b, "\n---\n**%s** — (not found)\n", rid)
				continue
			}
			fmt.Fprintf(&b, "\n---\n**%s** [%s]\n", relRoom.ID, relRoom.Status)
			fmt.Fprintf(&b, "**Topic:** %s\n", relRoom.Description)
			if relRoom.SystemPrompt != "" {
				fmt.Fprintf(&b, "**System Prompt:** %s\n", relRoom.SystemPrompt)
			}
			pinned, _ := r.Server.GetPinnedMessage(rid)
			if pinned != nil {
				excerpt := pinned.Content
				if len(excerpt) > 200 {
					excerpt = excerpt[:200]
					if i := strings.LastIndex(excerpt, " "); i > 120 {
						excerpt = excerpt[:i]
					}
					excerpt += "..."
				}
				fmt.Fprintf(&b, "📌 %s\n", excerpt)
			}
		}
	}

	// Make any Knowledge-Linter health flags actionable rather than just visible.
	b.WriteString(healthTagHint(room.Tags))

	return msg(b.String())
}

// appendSimilarRooms renders the advisory "similar room(s) already exist" notice
// with enough content to decide use-vs-create in one call: per room, its message
// count plus a one-line excerpt (the pinned message, else the latest message).
// Without the excerpt an agent had to make the call blind and discovered the
// overlap only later. Never blocks creation.
func (r *Registry) appendSimilarRooms(b *strings.Builder, excludeID, topic, project, tags string) {
	similar, err := r.Server.FindSimilarRooms(excludeID, topic, project, tags, 3)
	if err != nil || len(similar) == 0 {
		return
	}
	counts := r.Server.GetMessageCounts()
	b.WriteString("\n**Note:** Similar room(s) already exist — check before duplicating:\n")
	for _, sr := range similar {
		fmt.Fprintf(b, "- **%s** | %d msgs — %s (%s)\n", sr.ID, counts[sr.ID], sr.Description, sr.MatchReason)
		if pinned, _ := r.Server.GetPinnedMessage(sr.ID); pinned != nil {
			fmt.Fprintf(b, "    📌 %s\n", excerptLine(pinned.Content, 160))
		} else if recent, _ := r.Server.GetRecentMessages(sr.ID, 1); len(recent) > 0 {
			m := recent[0]
			fmt.Fprintf(b, "    latest [%s]: %s\n", m.MessageType, excerptLine(m.Content, 160))
		}
	}
	b.WriteString("  → To use one, post_to_room there (and link it via related_rooms); otherwise keep this new room.\n")
}

// excerptLine collapses content to its first non-empty line, clipped to max runes.
func excerptLine(content string, max int) string {
	line := firstNonEmptyLine(content)
	if len(line) > max {
		line = line[:max] + "…"
	}
	return line
}

// appendRoomSummary folds the pinned message + latest-per-type into a read_room
// response so one call orients a newcomer (the read_transcript(mode=summary)
// view). The pinned message, if any, is shown once and not repeated in the
// per-type list.
func (r *Registry) appendRoomSummary(b *strings.Builder, roomID, repo string) {
	pinned, _ := r.Server.GetPinnedMessage(roomID)
	latest, _ := r.Server.GetLatestPerType(roomID)

	if pinned == nil && len(latest) == 0 {
		b.WriteString("\n---\nNo messages yet. Post the first with post_to_room.\n")
		return
	}

	b.WriteString("\n---\n")
	if pinned != nil {
		b.WriteString("**📌 Pinned:**\n")
		appendMessageBlock(b, pinned.ID, pinned.Timestamp.Format("2006-01-02 15:04:05"), pinned.Author, pinned.MessageType, pinned.Content, repo)
	}

	var shown int
	for _, m := range latest {
		if pinned != nil && m.ID == pinned.ID {
			continue
		}
		shown++
	}
	if shown > 0 {
		fmt.Fprintf(b, "**Latest per type (%d):**\n", shown)
		for _, m := range latest {
			if pinned != nil && m.ID == pinned.ID {
				continue
			}
			appendMessageBlock(b, m.ID, m.Timestamp.Format("2006-01-02 15:04:05"), m.Author, m.MessageType, m.Content, repo)
		}
	}

	b.WriteString("\n*Full sequential history: read_transcript. Raw recent feed: read_room(include_last_n=N).*\n")
}

func (r *Registry) handleDeleteRoom(ctx context.Context, req *mcp.CallToolRequest, args DeleteRoomInput) (*mcp.CallToolResult, ToolOutput, error) {
	msg := textResult

	if args.RoomID == "" {
		return msg("Error: room_id is required.")
	}

	if err := r.Server.DeleteRoom(args.RoomID); err != nil {
		return msg(fmt.Sprintf("Error: %s", err.Error()))
	}

	r.Server.Logger.Info("Room deleted", "room_id", args.RoomID)
	return msg(fmt.Sprintf("Room '%s' and all its messages have been permanently deleted.", args.RoomID))
}
