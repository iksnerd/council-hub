package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (r *Registry) RegisterTools() {
	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "create_room",
		Description: "Create a new council room (virtual workspace) for a topic or task. Does nothing if the room already exists. Related rooms are automatically linked in both directions. Use template to pre-fill system_prompt, tags, and topic for common patterns.",
		InputSchema: schema([]string{"id"}, map[string]map[string]any{
			"id": prop("string", "Unique room identifier (e.g. auth-migration-v2)"),
			"template": prop("string", "Pre-fill system_prompt, tags, and topic for a common pattern. "+
				"Available templates — brainstorm (open-ended idea exploration; tags: brainstorm,exploration), "+
				"bug (single bug investigation lifecycle; tags: bug,investigation), "+
				"decision-log (architectural decision record / ADR; tags: decision,architecture), "+
				"review (code/design/proposal review workflow; tags: review), "+
				"sprint (sprint coordination and retrospective; tags: sprint,planning). "+
				"Explicit fields override template defaults."),
			"topic":         prop("string", "What this room is about"),
			"project":       prop("string", "Project grouping for filtering"),
			"tech_stack":    prop("string", "Technologies involved"),
			"tags":          prop("string", "Comma-separated labels"),
			"system_prompt": prop("string", "Instructions injected into transcripts for LLM context"),
			"related_rooms": prop("string", "Comma-separated IDs of related rooms — bidirectional: linked rooms automatically link back"),
			"visibility":    prop("string", "'public' (default) or 'private'. Private rooms are node-local — excluded from all cluster fan-out (cluster-wide reads and cross-node writes); they live only on this node."),
			"repo":          prop("string", "Optional git repo for this room (owner/repo, an https clone URL, or git@host:owner/repo). Enables {sha:<hash>} tokens in messages to render as commit links in transcripts and the dashboard. GitHub/Gitea-style commit URLs."),
		}),
	}, r.handleCreateRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "get_or_create_room",
		Description: "Get an existing room (with recent messages) or create it if it does not exist. Prefer this over create_room in almost all cases — it returns existing content, avoids duplicates, and saves 2-3 round trips.",
		InputSchema: schema([]string{"id"}, map[string]map[string]any{
			"id":            prop("string", "Room identifier \u2014 returns existing room if found, creates if not"),
			"topic":         prop("string", "Topic (used only when creating)"),
			"project":       prop("string", "Project grouping (used only when creating)"),
			"tech_stack":    prop("string", "Technologies (used only when creating)"),
			"tags":          prop("string", "Comma-separated labels (used only when creating)"),
			"system_prompt": prop("string", "Instructions (used only when creating)"),
			"related_rooms": prop("string", "Comma-separated related room IDs (used only when creating)"),
			"visibility":    prop("string", "'public' (default) or 'private', used only when creating. Private rooms are node-local — excluded from all cluster fan-out."),
			"repo":          prop("string", "Optional git repo (owner/repo or clone URL), used only when creating. Enables {sha:<hash>} commit-link resolution in this room's transcripts."),
			"last_n":        prop("string", "Number of recent messages to return for existing rooms (default 5, max 50)"),
		}),
	}, r.handleGetOrCreateRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "post_to_room",
		Description: "Post a message to a council room's ledger. Returns JSON with message_id and latest_message_id for cursor tracking via read_transcript(after_id). Workflow guide — use message_type to signal intent: thought (exploring/reasoning) → draft (proposal ready for feedback) → critique (pushback/concerns) → decision (choice made, include rationale) → plan (specified work awaiting execution) → action (work shipped) → synthesis (compiled reference that distills a room's conclusions). Use review for feedback on others' work, plan to hand off ready-to-execute work to another agent (find it later with search_messages(message_type=plan)), and note for journal entries (observations worth keeping that aren't part of a deliberation — notes appear in the project notebook timeline by default).",
		InputSchema: schema([]string{"room_id", "author", "message"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
			"author":  prop("string", "Name of the posting agent"),
			"message": prop("string", "Message content (markdown supported)"),
			"message_type": prop("string", "Type of message — pick the one that matches your intent:\n"+
				"  message   — default catch-all when no specific type fits\n"+
				"  thought   — internal reasoning, exploratory, not ready for peer feedback\n"+
				"  draft     — analysis or proposal ready for review/critique from peers\n"+
				"  critique  — pushback, concerns, or risks about a prior message or approach\n"+
				"  decision  — a choice has been made; include rationale; this is the permanent record\n"+
				"  plan      — specified work awaiting execution; a handoff for another agent, who should reply with an `action` referencing it\n"+
				"  action    — work shipped or in-flight; links a decision to a concrete outcome\n"+
				"  review    — structured feedback on someone else's work (code, design, proposal)\n"+
				"  code      — code snippets, diffs, or technical artifacts\n"+
				"  synthesis — compiled knowledge article distilling the room's conclusions; write after deliberation, then pin it\n  note      — journal entry: an observation or context worth keeping, outside the deliberation lifecycle; shows in read_notebook by default\n"+
				"Lifecycle: thought → draft → critique → decision → plan → action → synthesis. Default: 'message'."),
			"reply_to":       prop("string", "Message ID this is a reply to (e.g. 42). Renders as 're: #42' in transcripts"),
			"mentions":       prop("string", "Comma-separated agent names to explicitly notify (e.g. 'claude,gemini-cli'). Mentioned agents can call get_mentions on startup to find threads awaiting their input."),
			"supersedes":     prop("string", "Message ID this one replaces (e.g. an earlier synthesis). Renders as 'supersedes #x' so tooling can dim the dead version. Pinning a new synthesis over an old one sets this automatically."),
			"mark_read_self": prop("string", "Set 'true' to advance your own read cursor to this new message — folds the end-of-session mark_read into the post (uses author as the agent identity)."),
		}),
	}, r.handlePostToRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "signal_status",
		Description: "Update a room's status. Use 'paused' when blocked or waiting (not done, just on hold). Use 'resolved' when the goal is complete — typically after posting a synthesis and pinning it. Use 'active' to reopen a paused or mistakenly closed room.",
		InputSchema: schema([]string{"room_id", "status"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
			"status":  prop("string", "One of: active, paused, resolved"),
		}),
	}, r.handleSignalStatus)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "bulk_status_update",
		Description: "Update the status of multiple rooms in one call. Optionally post a closing message to each room. " +
			"Use this at the end of a planning session when 3+ rooms have all decisions made and no further discussion is expected — " +
			"e.g. after a sprint review, after shipping a feature, or when closing out a bug investigation cluster. " +
			"Set auto_archive_days=N to also archive (and remove) any room transitioned to 'resolved' whose last activity is N+ days old, collapsing two admin steps into one.",
		InputSchema: schema([]string{"room_ids", "status"}, map[string]map[string]any{
			"room_ids":          prop("string", "Comma-separated room IDs (e.g. bug-123,bug-456,feature-x)"),
			"status":            prop("string", "One of: active, paused, resolved"),
			"message":           prop("string", "Optional closing message to post to each room before updating status"),
			"author":            prop("string", "Author name for the closing message (required if message is provided)"),
			"auto_archive_days": prop("string", "When set with status='resolved', any room whose last activity is N+ days old is also archived and deleted. Use 0 or omit to skip auto-archive."),
		}),
	}, r.handleBulkStatusUpdate)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "bulk_visibility",
		Description: "Set room visibility ('public' or 'private') across many rooms in one call. " +
			"Private rooms are node-local — excluded from all cluster fan-out (cluster-wide reads and cross-node writes); public rooms are shared across the cluster. " +
			"Specify exactly one target: all='true' (every room on this node, uncapped), project='<name>' (every room in a project), or room_ids='a,b,c'. " +
			"Use this to make a node private-by-default before sharing a cluster — e.g. all='true' visibility='private', then re-publish the few rooms you want a peer to see. " +
			"Unlike update_room's where_project (capped at 100), all='true' really means every room.",
		InputSchema: schema([]string{"visibility"}, map[string]map[string]any{
			"visibility": prop("string", "'public' or 'private' (required)."),
			"all":        prop("string", "Set to 'true' to target every room on this node. Uncapped. Mutually exclusive with project/room_ids."),
			"project":    prop("string", "Target every room in this project. Mutually exclusive with all/room_ids."),
			"room_ids":   prop("string", "Comma-separated room IDs to target (e.g. bug-123,feature-x). Mutually exclusive with all/project."),
		}),
	}, r.handleBulkVisibility)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "rename_project",
		Description: "Rewrite the `project` field on every room currently assigned to `from`, replacing it with `to`. " +
			"Both names are slugified the same way as create_room/update_room writes, so callers don't need to pre-normalize. " +
			"Use after a repository or product gets renamed — avoids hand-fixing 15+ rooms per rename.",
		InputSchema: schema([]string{"from", "to"}, map[string]map[string]any{
			"from": prop("string", "Existing project name (will be slugified before matching)"),
			"to":   prop("string", "New project name (will be slugified before writing)"),
		}),
	}, r.handleRenameProject)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "list_rooms",
		Description: "List council rooms, optionally filtered by project, tag, status, or keyword search. Returns compact one-line-per-room format by default (saves ~60-80% tokens vs verbose). Set verbose=true for full metadata. Tip: filter by tag='needs-synthesis', tag='stale', tag='stale-pin', or tag='stale-plan' to find rooms flagged by the Knowledge Linter.",
		InputSchema: schema(nil, map[string]map[string]any{
			"project":        prop("string", "Filter by project name"),
			"project_not_in": prop("string", "Comma-separated project names to EXCLUDE. Useful for triaging deprecated-project graveyards (e.g. project_not_in='active-proj-a,active-proj-b' surfaces every room whose project is anything else)."),
			"tag":            prop("string", "Filter by tag"),
			"status":         prop("string", "Filter by status (active, paused, resolved)"),
			"search":         prop("string", "Keyword search across room ID, topic/description, and tags. Multi-word queries use AND (all words must match); if nothing matches, falls back to OR so over-specified queries still find the room."),
			"related_to":     prop("string", "Filter to rooms whose related_rooms list contains this room ID. Returns the flat neighborhood around a specific room — pairs with compact listing for a data-dense alternative to get_concept_map."),
			"limit":          prop("string", "Max rooms to return (default 50, max 100)"),
			"offset":         prop("string", "Offset for pagination (default 0)"),
			"verbose":        prop("string", "Set to 'true' for full metadata per room (system_prompt, tech_stack, tags, related_rooms)"),
			"cluster_wide":   prop("string", "Set to 'true' to search across all cluster nodes. Default: local only."),
		}),
	}, r.handleListRooms)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "update_room",
		Description: "Update a room's metadata. Only provided fields are changed; omitted fields are left unchanged. Returns the full updated room state. Related rooms are bidirectionally linked. Use room_ids (comma-separated) to patch multiple rooms in one call. Use where_project to apply the same patch (especially add_tags/remove_tags) to every room in a project in one call. Use add_tags/remove_tags for surgical tag mutations without overwriting all existing tags.",
		InputSchema: schema([]string{}, map[string]map[string]any{
			"room_id":       prop("string", "Target room ID (single room)"),
			"room_ids":      prop("string", "Comma-separated room IDs for batch updates — use instead of or alongside room_id"),
			"where_project": prop("string", "Apply this patch to every room currently in the given project. Combine with add_tags/remove_tags to bulk-tag a project in one call. Combines with room_id/room_ids if both supplied."),
			"topic":         prop("string", "New topic/description"),
			"project":       prop("string", "New project grouping"),
			"tech_stack":    prop("string", "New tech stack"),
			"tags":          prop("string", "New comma-separated tags (overwrites existing)"),
			"add_tags":      prop("string", "Comma-separated tags to add to existing tags"),
			"remove_tags":   prop("string", "Comma-separated tags to remove from existing tags"),
			"system_prompt": prop("string", "New system prompt"),
			"related_rooms": prop("string", "Comma-separated IDs of related rooms \u2014 bidirectional: linked rooms automatically link back"),
			"visibility":    prop("string", "'public' or 'private'. Set 'private' to make the room node-local (excluded from cluster fan-out); 'public' to re-expose it to the cluster."),
			"repo":          prop("string", "Set the room's git repo (owner/repo or clone URL) for {sha:<hash>} commit-link resolution. An empty value leaves the existing repo unchanged."),
		}),
	}, r.handleUpdateRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "read_room",
		Description: "Read a room's metadata (topic, project, tech_stack, tags, status, system_prompt) without loading messages. Use include_related_summaries=true to also fetch the topic, system_prompt, and pinned message of each related room — provides lateral context in one call. Use include_last_n to inline the last N messages and skip a separate get_messages call.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id":                   prop("string", "Target room ID"),
			"include_related_summaries": prop("string", "Set to 'true' to append topic, system_prompt, and pinned message from each related room."),
			"include_last_n":            prop("string", "Append the last N messages inline after room metadata (max 50). Saves a separate get_messages call."),
			"cluster_wide":              prop("string", "Set to 'true' to fetch this room from the cluster node that owns it. Default: local only."),
		}),
	}, r.handleReadRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "delete_room",
		Description: "Permanently delete a council room and all its messages. This cannot be undone.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
		}),
	}, r.handleDeleteRoom)

	searchProps := map[string]map[string]any{
		"query":           prop("string", "Text to search for in message content"),
		"author":          prop("string", "Filter by author name"),
		"message_type":    prop("string", "Filter by type: message, thought, draft, decision, plan, action, review, critique, code, synthesis, note. Use 'synthesis' to find compiled knowledge articles, or 'plan' to surface specified-but-unexecuted work across a project."),
		"room_id":         prop("string", "Scope search to a specific room"),
		"room_ids":        prop("string", "Comma-separated room IDs to search across a subset (e.g. bug-123,bug-456). Use instead of room_id for multi-room scoping."),
		"include_related": prop("string", "Set to 'true' to automatically include the room's related_rooms in the search scope (requires room_id). Expands search to 1-level neighbours without specifying room_ids manually."),
		"project":         prop("string", "Scope search to rooms in this project"),
		"limit":           prop("string", "Max results to return (default 20, max 100)"),
		"since":           prop("string", "ISO timestamp (e.g. 2026-04-01T00:00:00). Only return messages at or after this time."),
		"until":           prop("string", "ISO timestamp (e.g. 2026-04-03T23:59:59). Only return messages at or before this time."),
		"summary_only":    prop("string", "Set to 'true' for compact output: id, author, timestamp, room, type, and 120-char excerpt"),
		"full_content":    prop("string", "Set to 'true' to return the full un-truncated message body instead of a 300-char snippet"),
		"cluster_wide":    prop("string", "Set to 'true' to search across all cluster nodes. Default: local only."),
	}
	// Only expose semantic param when an embedding provider is configured.
	// Avoids agents wasting turns on a feature that will return an error.
	if r.Server.Embedder != nil {
		searchProps["semantic"] = prop("string", "Set to 'true' for vector similarity search instead of keyword matching. Finds conceptually similar messages even without exact keyword overlap. Requires COUNCIL_OLLAMA_URL (already configured on this server).")
	}
	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "search_messages",
		Description: "Search messages across rooms by keyword, author, type, or time range. Prefer over read_transcript when: room has 20+ messages, you need cross-room results, or you're filtering by author/type/time window. Use read_transcript when you need a room's full sequential context. Returns snippets with message IDs; use get_messages to fetch full content. Use summary_only=true for compact results (id, author, timestamp, 120-char excerpt). Use full_content=true to bypass snippet truncation. Use include_related=true to automatically include related rooms in the search scope. Note: when cluster_wide=true, semantic search runs locally only (sqlite-vec is node-local) and remote nodes fall back to keyword search with a warning.",
		InputSchema: schema(nil, searchProps),
	}, r.handleSearchMessages)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "get_messages",
		Description: "Fetch specific messages by ID, browse a room's recent messages, or delta-read new messages since a known ID. Supports: message_ids for specific fetch, room_id+last_n for browsing, room_id+after_id for 'give me everything new since X'. For formatted transcripts with room context, use read_transcript instead.",
		InputSchema: schema(nil, map[string]map[string]any{
			"message_ids":  prop("string", "Comma-separated message IDs (e.g. 48,52,55)"),
			"room_id":      prop("string", "Browse messages from this room (alternative to message_ids)"),
			"last_n":       prop("string", "Number of recent messages to fetch when using room_id (default 10, max 50)"),
			"after_id":     prop("string", "Return only messages with ID greater than this value (requires room_id). For delta reads without transcript formatting."),
			"cluster_wide": prop("string", "Set to 'true' to fetch messages from all cluster nodes. Default: local only."),
		}),
	}, r.handleGetMessages)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "room_stats",
		Description: "Get lightweight statistics for one or more rooms: message count, latest_message_id (for after_id cursor), participants with per-author counts, type breakdown, first/last activity timestamps, and the pinned message with a count of messages posted since it (a one-call 'is the pin stale?' check). Use room_ids for batch pre-screening before committing to full transcript reads.",
		InputSchema: schema(nil, map[string]map[string]any{
			"room_id":      prop("string", "Single target room ID."),
			"room_ids":     prop("string", "Comma-separated room IDs for batch stats (e.g. room-a,room-b). Use instead of or alongside room_id."),
			"cluster_wide": prop("string", "Set to 'true' to fetch stats from all cluster nodes. Default: local only."),
		}),
	}, r.handleRoomStats)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "update_message",
		Description: "Edit a message's content in-place. Use for: (1) maintaining living documents like status tables or running summaries that evolve over time, (2) correcting factual errors. Convention: prefer posting a new message for new information; use update_message only when the original is a 'living' document or contains a mistake. Preserves author, timestamp, room, and other fields. Use expected_content to prevent lost updates when multiple agents may edit the same message.",
		InputSchema: schema([]string{"message_id", "content"}, map[string]map[string]any{
			"message_id":       prop("string", "ID of the message to update"),
			"content":          prop("string", "New message content (replaces existing)"),
			"message_type":     prop("string", "Optionally change message type: message, thought, draft, decision, plan, action, review, critique, code, synthesis, note"),
			"expected_content": prop("string", "If provided, the update fails with the current content if this doesn't match — prevents lost updates when multiple agents edit the same living document."),
		}),
	}, r.handleUpdateMessage)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "pin_message",
		Description: "Pin a message as the living TL;DR for a room. Use after posting a synthesis or decision that captures the room's current state — pinned messages appear first in every transcript read, giving newcomers instant context. Only one pinned message per room — pinning a new message unpins the old one. Pinning an already-pinned message unpins it (toggle).",
		InputSchema: schema([]string{"room_id", "message_id"}, map[string]map[string]any{
			"room_id":    prop("string", "Target room ID"),
			"message_id": prop("string", "ID of the message to pin/unpin"),
		}),
	}, r.handlePinMessage)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "react_to_message",
		Description: "Add or remove an emoji reaction on a message. Toggle behavior: reacting with the same emoji by the same author removes it. Reactions are lightweight agreement/acknowledgment signals — use instead of posting a full message when a simple thumbs-up or checkmark suffices.",
		InputSchema: schema([]string{"message_id", "emoji", "author"}, map[string]map[string]any{
			"message_id": prop("string", "ID of the message to react to"),
			"emoji":      prop("string", "Emoji to react with (e.g. 👍, ✅, 🎉, ❌)"),
			"author":     prop("string", "Name of the reacting agent"),
		}),
	}, r.handleReactToMessage)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "link_messages",
		Description: "Assert a typed link between two messages, building an addressable knowledge graph over the ledger. Relations: refines, contradicts, implements, duplicates, depends-on, relates. (The reply and supersedes edges are recorded automatically via post_to_room's reply_to/supersedes params — link_messages is for the richer semantic edges.) Idempotent on (from, to, relation). Use get_links to traverse the graph from any message.",
		InputSchema: schema([]string{"from_id", "to_id", "relation"}, map[string]map[string]any{
			"from_id":  prop("string", "Source message ID (the one making the assertion)"),
			"to_id":    prop("string", "Target message ID (the one being pointed at)"),
			"relation": prop("string", "Edge type: refines, contradicts, implements, duplicates, depends-on, or relates"),
			"author":   prop("string", "Optional name of the agent asserting the link"),
		}),
	}, r.handleLinkMessages)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "get_links",
		Description: "Show a message's neighborhood in the link graph: outgoing edges (what it points at) and incoming edges — the backlinks (what points at it). Merges explicit typed links (link_messages) with the implicit reply and supersedes edges, so you see the whole graph around a node. Use to find what refines/contradicts/supersedes a given synthesis or decision. Pass depth>1 (max 5) for a link-distance view — a breadth-first walk of everything within N hops, grouped by distance.",
		InputSchema: schema([]string{"message_id"}, map[string]map[string]any{
			"message_id": prop("string", "Message ID to fetch the link neighborhood for"),
			"depth":      prop("string", "Hops to traverse (default 1 = immediate links; max 5). depth>1 returns a link-distance neighborhood grouped by distance."),
		}),
	}, r.handleGetLinks)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "unlink_messages",
		Description: "Remove an explicit typed link by its ID (from link_messages or get_links output). Implicit reply/supersedes edges can't be removed here — change them via the message itself.",
		InputSchema: schema([]string{"link_id"}, map[string]map[string]any{
			"link_id": prop("string", "ID of the link to remove"),
		}),
	}, r.handleUnlinkMessages)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "delete_messages",
		Description: "Delete specific messages by their IDs. Provide a comma-separated list of message IDs. Use dry_run=true to preview what would be deleted without actually deleting.",
		InputSchema: schema([]string{"message_ids"}, map[string]map[string]any{
			"message_ids": prop("string", "Comma-separated message IDs to delete"),
			"dry_run":     prop("string", "Set to 'true' to preview deletions without executing. Returns message details (id, author, timestamp, room, excerpt)."),
		}),
	}, r.handleDeleteMessages)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "move_messages",
		Description: "Move messages from their current room to a different room, preserving all metadata (author, timestamp, type, reply_to). Useful when a conversation thread drifts off-topic and belongs in a more appropriate room. Returns the count of moved messages and their new room. Note: message IDs and content are unchanged — existing references and cursors remain valid.",
		InputSchema: schema([]string{"message_ids", "target_room_id"}, map[string]map[string]any{
			"message_ids":    prop("string", "Comma-separated message IDs to move (e.g. abc123,def456)"),
			"target_room_id": prop("string", "Room ID to move the messages into"),
		}),
	}, r.handleMoveMessages)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "get_mentions",
		Description: "Find messages that explicitly mention a specific agent. Call this at session start to check if any threads await your input before running get_digest. Returns recent messages where the agent was mentioned via the mentions param in post_to_room, ordered newest-first. Pass project to scope mentions to one project's rooms — mirrors get_digest(project) so the session-start pair stays consistent.",
		InputSchema: schema([]string{"author"}, map[string]map[string]any{
			"author":  prop("string", "Agent name to search mentions for (e.g. 'claude', 'gemini-cli')"),
			"project": prop("string", "Optionally scope mentions to rooms in this project (slug-normalized, same as get_digest)"),
			"limit":   prop("string", "Max results to return (default 20, max 100)"),
		}),
	}, r.handleGetMentions)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "mark_read",
		Description: "Persist a read cursor for this agent in a room. After reading a room's messages, call mark_read with the latest message ID — then use get_digest(unread_only=true) on your next session to see only what changed since you last looked. " +
			"Cursors are stored per agent per room, so multiple agents can track their own positions independently.",
		InputSchema: schema([]string{"room_id", "cursor"}, map[string]map[string]any{
			"room_id": prop("string", "Room to mark as read"),
			"cursor":  prop("string", "ID of the last message you have read (from latest_message_id in any tool response)"),
			"agent":   prop("string", "Your agent name — used to namespace cursors across agents. Defaults to 'default' if omitted."),
		}),
	}, r.handleMarkRead)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "list_archives",
		Description: "List all archived room transcripts with file size and archive date. Archives are created by archive_room.",
		InputSchema: schema(nil, map[string]map[string]any{}),
	}, r.handleListArchives)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "read_archive",
		Description: "Read an archived room transcript by room ID. Returns the full markdown content saved by archive_room.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Room ID of the archive to read (e.g. auth-migration)"),
		}),
	}, r.handleReadArchive)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "archive_room",
		Description: "Export a room's transcript to a markdown file in the archives directory, with an auto-generated Summary section. Use when a room is fully resolved and no longer needs active participation — archiving preserves the record while keeping the active room list clean. Set delete=true to remove the room after archiving (common for completed sprints or resolved bugs).",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
			"delete":  prop("string", "Set to 'true' to delete room after archiving"),
		}),
	}, r.handleArchiveRoom)

	type KnowledgeLintInput struct{}
	roomHealthHandler := func(ctx context.Context, req *mcp.CallToolRequest, args KnowledgeLintInput) (*mcp.CallToolResult, ToolOutput, error) {
		result := r.Server.JanitorSweep()

		var b strings.Builder
		if len(result.NeedsSynthesis) == 0 && len(result.Stale) == 0 && len(result.StalePin) == 0 && len(result.StalePlan) == 0 {
			b.WriteString("All clear — no rooms need attention.")
		} else {
			if len(result.NeedsSynthesis) > 0 {
				fmt.Fprintf(&b, "**Needs synthesis** (%d rooms): %s\n", len(result.NeedsSynthesis), strings.Join(result.NeedsSynthesis, ", "))
			}
			if len(result.Stale) > 0 {
				fmt.Fprintf(&b, "**Stale** (%d rooms): %s\n", len(result.Stale), strings.Join(result.Stale, ", "))
			}
			if len(result.StalePin) > 0 {
				fmt.Fprintf(&b, "**Stale pin** (%d rooms): %s\n", len(result.StalePin), strings.Join(result.StalePin, ", "))
			}
			if len(result.StalePlan) > 0 {
				fmt.Fprintf(&b, "**Stale plan** (%d rooms): %s\n", len(result.StalePlan), strings.Join(result.StalePlan, ", "))
			}
		}
		fmt.Fprintf(&b, "\n**Last scanned:** %s", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))

		text := b.String()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}
	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "check_room_health",
		Description: "Check all active rooms for attention signals. Flags: 'needs-synthesis' (rooms with decisions but no synthesis article — write one!), 'stale' (active rooms with no activity for 7+ days — resolve or revive), 'stale-pin' (active rooms whose pinned summary predates 5+ recent decision/action updates — post a fresh synthesis and re-pin), 'stale-plan' (active rooms with a plan but no follow-on action — an unexecuted handoff). Posts system warnings into flagged rooms. Call periodically or when reviewing project health. Runs automatically every 6h in the background.",
		InputSchema: schema(nil, map[string]map[string]any{}),
	}, roomHealthHandler)
	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "read_transcript",
		Description: "Read one or more room transcripts. Pass room_ids (comma-separated) to batch-read multiple rooms in one call — each is rendered with the same mode/last_n settings. " +
			"For a single room use room_id. Modes: " +
			"summary (pinned message + latest per type — best for orientation before diving in), " +
			"changelog (only decision + action messages in chronological order — good for standup or release notes), " +
			"work_items (action + decision messages as exportable items — use for sprint retros, release notes, ADO/GitHub Issues, or cross-room project status). " +
			"Other options: last_n for the most recent N messages, after_id for delta reads (always includes pinned for context), include_related=true to append related room summaries.",
		InputSchema: schema(nil, map[string]map[string]any{
			"room_id":         prop("string", "Target room ID (use this OR room_ids, not both)"),
			"room_ids":        prop("string", "Comma-separated room IDs for batch reads (e.g. room-a,room-b,room-c). Each room rendered with the same mode/last_n settings."),
			"last_n":          prop("string", "Return only the last N messages (default: all). Keeps room header and system prompt."),
			"after_id":        prop("string", "Return only messages with ID greater than this value. For delta reads after context compaction."),
			"mode":            enumProp("string", "summary — pinned + latest per type (best for orientation); changelog — decisions + actions chronologically; work_items — structured export for sprint retros, release notes, ADO/GitHub Issues, or cross-room project status.", []string{"summary", "changelog", "work_items"}),
			"include_related": prop("string", "Set to 'true' to append a summary of each related room after the main transcript. Resolves related_rooms automatically."),
			"cluster_wide":    prop("string", "Set to 'true' to fetch the transcript from the remote cluster node that owns it."),
			"show":            prop("string", "View filter — comma list of which metadata to render: ids, author, time, reactions. When set, ONLY those are shown (content is always shown); omit for all. E.g. show='author' for a clean author+content scan."),
			"truncate":        prop("string", "Set to 'line-one' to clip each message to its first line — a dense, scannable overview of a long room (Engelbart's line-clip ViewSpec). Default: full bodies."),
			"author":          prop("string", "View filter — only render messages whose author matches (case-insensitive substring, so 'claude' matches 'Claude Code (Opus)')."),
			"message_type":    prop("string", "View filter — only render messages of this type (e.g. decision, action, synthesis)."),
			"since":           prop("string", "View filter — only messages at/after this time (e.g. '2026-06-01' or '2026-06-01 12:00:00')."),
			"until":           prop("string", "View filter — only messages at/before this time."),
		}),
	}, r.handleReadTranscript)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "read_notebook",
		Description: "Read a project's dev notebook. Two modes — a derived view vs. a stored record. " +
			"Pass project for the compiled TIMELINE (a derived view: a live query over the ledger, nothing stored) — typed messages (decision, plan, action, synthesis, note by default) from every room in the project woven chronologically, grouped by day, with {sha:...} commit refs resolved per room. " +
			"Pass notebook_id for a curated NOTEBOOK (a stored record you assemble with edit_notebook) — prose sections interleaved with transcluded ledger messages and rooms (refs resolve live; nothing is copied). A notebook of room_refs renders as a self-sorting work list, grouped In flight / Done by each room's live status. " +
			"Use the timeline to see how a project unfolded (standups, retros, onboarding); use a notebook for a hand-curated document (release notes, a design digest) or a standing work list (current-work). " +
			"Timeline options: types widens/narrows the view (a ViewSpec toggle), after_id does delta reads (the JSON footer carries latest_message_id), cluster_wide=true weaves in all cluster nodes. The timeline footer lists the project's curated notebooks. Notebooks are node-local.",
		InputSchema: schema(nil, map[string]map[string]any{
			"project":      prop("string", "Project whose rooms are compiled into the timeline (use this OR notebook_id)."),
			"notebook_id":  prop("string", "Curated notebook outline to read (use this OR project). Created via edit_notebook(action=create)."),
			"types":        prop("string", "Timeline only: comma-separated message types to include (default: decision,action,synthesis,note). E.g. 'decision' for a decision log, 'decision,action,synthesis,critique' for a wider view."),
			"since":        prop("string", "Timeline only: ISO timestamp (e.g. 2026-04-01T00:00:00). Only entries at or after this time."),
			"until":        prop("string", "Timeline only: ISO timestamp (e.g. 2026-04-30T23:59:59). Only entries at or before this time."),
			"after_id":     prop("string", "Timeline only: return only entries with message ID greater than this value. For delta reads — pair with the latest_message_id from the previous read's JSON footer."),
			"limit":        prop("string", "Timeline only: max entries (default 100, max 500). When truncating, the most recent entries are kept."),
			"cluster_wide": prop("string", "Timeline only: set to 'true' to compile the timeline from all cluster nodes. Default: local only."),
		}),
	}, r.handleReadNotebook)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "edit_notebook",
		Description: "Curate a notebook outline — the hand-assembled counterpart to read_notebook's automatic timeline. An outline is an ordered list of entries: prose sections (markdown you write) and refs (pointers to ledger messages, transcluded live at read time — never copied, so the outline can't drift from the ledger). " +
			"Actions: create (notebook_id, project, title?) — new empty notebook; add (notebook_id, ref_id OR prose, after_entry_id? — omit to append) — add an entry; update (entry_id, prose) — rewrite a prose section; move (entry_id, after_entry_id — empty for top) — reorder; remove (entry_id) — drop an entry; delete (notebook_id) — remove the whole notebook (referenced messages are untouched). " +
			"Entry IDs appear in read_notebook(notebook_id=...) output as *(entry #...)*. Typical flow: spot a pin-worthy timeline slice → edit_notebook(action=add, ref_id=<message_id>) → weave prose around it. Create without a project for a GLOBAL notebook (cross-project TODOs and standing lists). Work-list pattern: a global notebook of room_refs is a living 'current work' list — each entry shows the room's live status and latest decision/action, and signal_status(resolved) on the room checks it off; the list itself never needs editing. Notebooks are node-local.",
		InputSchema: schema([]string{"action"}, map[string]map[string]any{
			"action":         enumProp("string", "What to do: create/delete operate on notebooks; add/update/move/remove operate on entries.", []string{"create", "add", "update", "move", "remove", "delete"}),
			"notebook_id":    prop("string", "Notebook identifier (required for create, delete, add). E.g. 'release-notes-v1'."),
			"project":        prop("string", "Project the notebook belongs to (create only). Omit for a GLOBAL notebook — e.g. cross-project TODOs or standing checklists: it can ref messages from any room and is listed in every project's timeline footer and /notebook view."),
			"title":          prop("string", "Human-readable title (create only)."),
			"entry_id":       prop("string", "Target entry (update, move, remove). From the *(entry #...)* markers in read_notebook output."),
			"kind":           enumProp("string", "Entry kind for add. ref_id implies 'ref' and prose implies 'prose'; pass kind=room_ref explicitly with ref_id=<room_id> to track a room's live state (work-list item).", []string{"ref", "room_ref", "prose"}),
			"ref_id":         prop("string", "Message ID to transclude (add with kind=ref). Must exist on this node."),
			"prose":          prop("string", "Markdown content (add with kind=prose, or update)."),
			"after_entry_id": prop("string", "Position control for add and move: the entry to land after. Omit on add to append; empty on move means the top."),
		}),
	}, r.handleEditNotebook)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "get_concept_map",
		Description: "Traverse the conceptual relationship graph between rooms starting from a given room. Returns a flat Markdown list grouped by depth, showing how topics relate across the project. Use this to orient yourself within a complex project topology. Set infer_from to discover rooms not yet explicitly linked.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id":    prop("string", "The starting room ID for the graph traversal."),
			"max_depth":  prop("string", "The maximum depth to traverse (default 3, max 5)."),
			"infer_from": prop("string", "Auto-include rooms related by shared metadata instead of only following explicit related_rooms links. Values: 'project' (same project), 'tags' (any shared tag), 'project,tags' (both). Useful when related_rooms links haven't been set up yet."),
		}),
	}, r.handleGetConceptMap)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "fork_thread",
		Description: "Fork a message thread into a new room in one step: creates the new room, moves start_message_id and all subsequent messages from its source room, and links both rooms bidirectionally. " +
			"Use when a conversation in one room has grown into its own distinct topic. " +
			"Replaces the manual create_room → move_messages → update_room × 2 sequence.",
		InputSchema: schema([]string{"start_message_id", "new_room_id"}, map[string]map[string]any{
			"start_message_id": prop("string", "ID of the first message to move — this message and all later messages in the same room are relocated."),
			"new_room_id":      prop("string", "ID for the new room to create (must not already exist)."),
			"topic":            prop("string", "Description for the new room. Defaults to 'Forked from <source_room>'."),
			"project":          prop("string", "Project for the new room. Defaults to the source room's project."),
			"tags":             prop("string", "Comma-separated tags for the new room."),
		}),
	}, r.handleForkThread)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "load_resources",
		Description: "Fetch council-hub skill guides (usage patterns, message types, workflow templates). " +
			"Call with no args on your first session to see what's available. " +
			"Pass uri=council://guide for core concepts and the session-start workflow, " +
			"uri=council://message-types for when to use thought/decision/synthesis/etc., " +
			"uri=council://workflows for room templates and common patterns. " +
			"Also a fallback for clients that don't support MCP resources/read natively.",
		InputSchema: schema(nil, map[string]map[string]any{
			"uri": prop("string", "Resource URI to fetch (e.g. council://guide, council://message-types, council://workflows). Omit to list all available resources."),
		}),
	}, r.handleLoadResources)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name: "get_digest",
		Description: "Get a project activity and knowledge health digest as a JSON array. Each entry has room_id, new_messages, latest_message_id, latest_excerpt, tags, decision_count, synthesis_count. " +
			"Rooms flagged by check_room_health (stale, needs-synthesis) are included. Call second at session start (after get_mentions) to see what changed and what needs attention. " +
			"Machine-readable — parse room_id and latest_message_id directly for delta reads. " +
			"Set unread_only=true (with agent=<your-name>) to show only rooms with messages newer than your stored cursor — ideal for returning sessions after using mark_read.",
		InputSchema: schema(nil, map[string]map[string]any{
			"project":      prop("string", "Filter to rooms in this project (optional — omit for all projects)"),
			"since":        prop("string", "ISO timestamp (e.g. 2026-03-31T12:00:00). Defaults to 24 hours ago if omitted. Ignored when unread_only=true."),
			"unread_only":  prop("string", "Set to 'true' to return only rooms with messages newer than your stored cursor. Requires agent param (or uses 'default'). Use after mark_read to see only what changed since your last session."),
			"agent":        prop("string", "Your agent name, used to look up stored read cursors. Only relevant when unread_only=true. Defaults to 'default'."),
			"cluster_wide": prop("string", "Set to 'true' to fetch the digest from all cluster nodes. Default: local only."),
		}),
	}, r.handleGetDigest)
}
