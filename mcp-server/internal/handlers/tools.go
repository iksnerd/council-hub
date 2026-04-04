package handlers

import (
	"context"
	"council-hub/internal/council"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input size limits to prevent DoS and unbounded database growth.
const (
	maxIDLen       = 255
	maxAuthorLen   = 255
	maxContentLen  = 100_000 // ~100KB
	maxMetadataLen = 10_000  // topic, project, tech_stack, tags, system_prompt
)

// validateSize returns an error if value exceeds max characters.
func validateSize(field, value string, max int) error {
	if len(value) > max {
		return fmt.Errorf("%s exceeds maximum length (%d chars, limit %d)", field, len(value), max)
	}
	return nil
}

// validateRoomMetadata checks size limits on all room metadata fields.
func validateRoomMetadata(topic, project, techStack, tags, systemPrompt string) error {
	for _, check := range []struct{ name, val string }{
		{"topic", topic}, {"project", project}, {"tech_stack", techStack},
		{"tags", tags}, {"system_prompt", systemPrompt},
	} {
		if err := validateSize(check.name, check.val, maxMetadataLen); err != nil {
			return err
		}
	}
	return nil
}

// Registry holds the council server and handles MCP tool registration.
type Registry struct {
	Server     *council.Server
	HTTPClient *http.Client // for cluster-wide queries via Phoenix internal API
	PhoenixURL string       // e.g. "http://127.0.0.1:4000"
}

// toolResultText extracts the text content from a CallToolResult.
func toolResultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	if tc, ok := r.Content[0].(*mcp.TextContent); ok && tc != nil {
		return tc.Text
	}
	return ""
}

// ToolOutput is the structured output type for tool results.
type ToolOutput struct {
	Message string `json:"message"`
}

var validMessageTypes = map[string]bool{
	"message":   true,
	"thought":   true,
	"decision":  true,
	"code":      true,
	"review":    true,
	"action":    true,
	"critique":  true,
	"synthesis": true,
}

// schema builds a JSON Schema object with additionalProperties: true.
// required lists the field names that are mandatory; all others are optional.
func schema(required []string, props map[string]map[string]any) map[string]any {
	s := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": true,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func prop(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

func enumProp(typ, desc string, enum []string) map[string]any {
	return map[string]any{"type": typ, "description": desc, "enum": enum}
}

func (r *Registry) RegisterTools() {
	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "create_room",
		Description: "Create a new council room (virtual workspace) for a topic or task. Does nothing if the room already exists. Related rooms are automatically linked in both directions. Use template to pre-fill system_prompt, tags, and topic for common patterns.",
		InputSchema: schema([]string{"id"}, map[string]map[string]any{
			"id":            prop("string", "Unique room identifier (e.g. auth-migration-v2)"),
			"template":      prop("string", "Pre-fill system_prompt, tags, and topic for a common pattern. Available: brainstorm, bug, decision-log, review, sprint. Explicit fields override template defaults."),
			"topic":         prop("string", "What this room is about"),
			"project":       prop("string", "Project grouping for filtering"),
			"tech_stack":    prop("string", "Technologies involved"),
			"tags":          prop("string", "Comma-separated labels"),
			"system_prompt": prop("string", "Instructions injected into transcripts for LLM context"),
			"related_rooms": prop("string", "Comma-separated IDs of related rooms — bidirectional: linked rooms automatically link back"),
		}),
	}, r.handleCreateRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "get_or_create_room",
		Description: "Get an existing room (with recent messages) or create it if it doesn't exist. Saves 2-3 round trips vs list_rooms \u2192 check \u2192 read_recent or create_room.",
		InputSchema: schema([]string{"id"}, map[string]map[string]any{
			"id":            prop("string", "Room identifier \u2014 returns existing room if found, creates if not"),
			"topic":         prop("string", "Topic (used only when creating)"),
			"project":       prop("string", "Project grouping (used only when creating)"),
			"tech_stack":    prop("string", "Technologies (used only when creating)"),
			"tags":          prop("string", "Comma-separated labels (used only when creating)"),
			"system_prompt": prop("string", "Instructions (used only when creating)"),
			"related_rooms": prop("string", "Comma-separated related room IDs (used only when creating)"),
			"last_n":        prop("string", "Number of recent messages to return for existing rooms (default 5, max 50)"),
		}),
	}, r.handleGetOrCreateRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "post_to_room",
		Description: "Post a message to a council room's ledger. Returns JSON with message_id and latest_message_id for delta-read cursor tracking via read_transcript(after_id).",
		InputSchema: schema([]string{"room_id", "author", "message"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
			"author":  prop("string", "Name of the posting agent"),
			"message": prop("string", "Message content (markdown supported)"),
			"message_type": prop("string", "Type of message. Use: 'thought' for reasoning/exploration, 'decision' for choices made, "+
				"'action' for work done/shipped, 'review' for feedback on others' work, 'critique' for pushback/concerns, "+
				"'code' for code snippets, 'synthesis' for distilled knowledge articles that compile a room's conclusions "+
				"(the 'compiled output' — use after deliberation to capture what was learned). Default: 'message'."),
			"reply_to": prop("string", "Message ID this is a reply to (e.g. 42). Renders as 're: #42' in transcripts"),
		}),
	}, r.handlePostToRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "signal_status",
		Description: "Update a room's status to coordinate work between agents (active, paused, or resolved).",
		InputSchema: schema([]string{"room_id", "status"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
			"status":  prop("string", "One of: active, paused, resolved"),
		}),
	}, r.handleSignalStatus)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "bulk_status_update",
		Description: "Update the status of multiple rooms in one call. Optionally post a closing message to each room. Useful for closing out a sprint or batch-resolving rooms.",
		InputSchema: schema([]string{"room_ids", "status"}, map[string]map[string]any{
			"room_ids": prop("string", "Comma-separated room IDs (e.g. bug-123,bug-456,feature-x)"),
			"status":   prop("string", "One of: active, paused, resolved"),
			"message":  prop("string", "Optional closing message to post to each room before updating status"),
			"author":   prop("string", "Author name for the closing message (required if message is provided)"),
		}),
	}, r.handleBulkStatusUpdate)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "list_rooms",
		Description: "List council rooms, optionally filtered by project, tag, status, or keyword search. Returns compact one-line-per-room format by default (saves ~60-80% tokens vs verbose). Set verbose=true for full metadata. Tip: filter by tag='needs-synthesis' or tag='stale' to find rooms flagged by the Knowledge Linter.",
		InputSchema: schema(nil, map[string]map[string]any{
			"project":      prop("string", "Filter by project name"),
			"tag":          prop("string", "Filter by tag"),
			"status":       prop("string", "Filter by status (active, paused, resolved)"),
			"search":       prop("string", "Keyword search across room ID, topic/description, and tags"),
			"verbose":      prop("string", "Set to 'true' for full metadata per room (system_prompt, tech_stack, tags, related_rooms)"),
			"cluster_wide": prop("string", "Set to 'true' to search across all cluster nodes. Default: local only."),
		}),
	}, r.handleListRooms)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "update_room",
		Description: "Update a room's metadata. Only provided fields are changed; omitted fields are left unchanged. Returns the full updated room state. Related rooms are bidirectionally linked. Use room_ids (comma-separated) to patch multiple rooms in one call.",
		InputSchema: schema([]string{}, map[string]map[string]any{
			"room_id":       prop("string", "Target room ID (single room)"),
			"room_ids":      prop("string", "Comma-separated room IDs for batch updates — use instead of or alongside room_id"),
			"topic":         prop("string", "New topic/description"),
			"project":       prop("string", "New project grouping"),
			"tech_stack":    prop("string", "New tech stack"),
			"tags":          prop("string", "New comma-separated tags (overwrites existing)"),
			"add_tags":      prop("string", "Comma-separated tags to add to existing tags"),
			"remove_tags":   prop("string", "Comma-separated tags to remove from existing tags"),
			"system_prompt": prop("string", "New system prompt"),
			"related_rooms": prop("string", "Comma-separated IDs of related rooms \u2014 bidirectional: linked rooms automatically link back"),
		}),
	}, r.handleUpdateRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "read_room",
		Description: "Read a room's metadata (topic, project, tech_stack, tags, status, system_prompt) without loading messages.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id":      prop("string", "Target room ID"),
			"cluster_wide": prop("string", "Set to 'true' to search across all cluster nodes. Default: local only."),
		}),
	}, r.handleReadRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "delete_room",
		Description: "Permanently delete a council room and all its messages. This cannot be undone.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
		}),
	}, r.handleDeleteRoom)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "search_messages",
		Description: "Search messages across rooms. All filter params are optional \u2014 omit or leave empty to skip. Returns snippets with message IDs; use get_messages to fetch full content. Use summary_only=true for compact results (id, author, timestamp, 120-char excerpt). Use full_content=true to bypass snippet truncation.",
		InputSchema: schema(nil, map[string]map[string]any{
			"query":        prop("string", "Text to search for in message content"),
			"author":       prop("string", "Filter by author name"),
			"message_type": prop("string", "Filter by type: message, thought, decision, action, review, critique, code, synthesis. Use 'synthesis' to find compiled knowledge articles."),
			"room_id":      prop("string", "Scope search to a specific room"),
			"project":      prop("string", "Scope search to rooms in this project"),
			"limit":        prop("string", "Max results to return (default 20, max 100)"),
			"since":        prop("string", "ISO timestamp (e.g. 2026-04-01T00:00:00). Only return messages at or after this time."),
			"until":        prop("string", "ISO timestamp (e.g. 2026-04-03T23:59:59). Only return messages at or before this time."),
			"summary_only": prop("string", "Set to 'true' for compact output: id, author, timestamp, room, type, and 120-char excerpt"),
			"full_content": prop("string", "Set to 'true' to return the full un-truncated message body instead of a 300-char snippet"),
			"cluster_wide": prop("string", "Set to 'true' to search across all cluster nodes. Default: local only."),
		}),
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
		Description: "Get lightweight statistics for a room: message count, latest_message_id (for after_id cursor), participants with per-author counts, type breakdown, and first/last activity timestamps. Use before read_transcript to decide whether to read.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id":      prop("string", "Target room ID"),
			"cluster_wide": prop("string", "Set to 'true' to search across all cluster nodes. Default: local only."),
		}),
	}, r.handleRoomStats)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "update_message",
		Description: "Edit a message's content in-place. Useful for maintaining living status tables or correcting mistakes. Preserves author, timestamp, room, and other fields.",
		InputSchema: schema([]string{"message_id", "content"}, map[string]map[string]any{
			"message_id":   prop("string", "ID of the message to update"),
			"content":      prop("string", "New message content (replaces existing)"),
			"message_type": prop("string", "Optionally change message type: message, thought, decision, action, review, critique, code, synthesis"),
		}),
	}, r.handleUpdateMessage)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "pin_message",
		Description: "Pin a message as the living TL;DR for a room. Only one pinned message per room \u2014 pinning a new message unpins the old one. Pinning an already-pinned message unpins it (toggle). Pinned messages appear first in transcripts.",
		InputSchema: schema([]string{"room_id", "message_id"}, map[string]map[string]any{
			"room_id":    prop("string", "Target room ID"),
			"message_id": prop("string", "ID of the message to pin/unpin"),
		}),
	}, r.handlePinMessage)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "delete_messages",
		Description: "Delete specific messages by their IDs. Provide a comma-separated list of message IDs. Use dry_run=true to preview what would be deleted without actually deleting.",
		InputSchema: schema([]string{"message_ids"}, map[string]map[string]any{
			"message_ids": prop("string", "Comma-separated message IDs to delete"),
			"dry_run":     prop("string", "Set to 'true' to preview deletions without executing. Returns message details (id, author, timestamp, room, excerpt)."),
		}),
	}, r.handleDeleteMessages)

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
		Description: "Export a room's transcript to a markdown file in the archives directory, with an auto-generated Summary section (last decision + last action). Optionally delete the room after archiving.",
		InputSchema: schema([]string{"room_id"}, map[string]map[string]any{
			"room_id": prop("string", "Target room ID"),
			"delete":  prop("string", "Set to 'true' to delete room after archiving"),
		}),
	}, r.handleArchiveRoom)

	type KnowledgeLintInput struct{}
	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "knowledge_lint",
		Description: "Run the Knowledge Linter on demand. Scans all active rooms and flags: 'needs-synthesis' (rooms with decisions but no synthesis article), 'stale' (active rooms with no activity for 7+ days). Posts system warnings into newly flagged rooms. Returns which rooms were flagged. Also runs automatically every hour.",
		InputSchema: schema(nil, map[string]map[string]any{}),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args KnowledgeLintInput) (*mcp.CallToolResult, ToolOutput, error) {
		result := r.Server.JanitorSweep()

		var b strings.Builder
		if len(result.NeedsSynthesis) == 0 && len(result.Stale) == 0 {
			b.WriteString("All clear — no rooms need attention.")
		} else {
			if len(result.NeedsSynthesis) > 0 {
				fmt.Fprintf(&b, "**Needs synthesis** (%d rooms): %s\n", len(result.NeedsSynthesis), strings.Join(result.NeedsSynthesis, ", "))
			}
			if len(result.Stale) > 0 {
				fmt.Fprintf(&b, "**Stale** (%d rooms): %s\n", len(result.Stale), strings.Join(result.Stale, ", "))
			}
		}

		text := b.String()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	})

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "read_transcript",
		Description: "Read a room's transcript with full context (room header, system_prompt, pinned message, messages). The primary tool for reading rooms. Supports: last_n for recent messages, after_id for delta reads (includes pinned message for context), mode=summary for orientation (pinned + latest per type), mode=changelog for decisions+actions only, mode=work_items for exportable action/decision list. Use room_ids for batch multi-room reads. Use include_related=true to auto-append related room summaries.",
		InputSchema: schema(nil, map[string]map[string]any{
			"room_id":         prop("string", "Target room ID (use this OR room_ids, not both)"),
			"room_ids":        prop("string", "Comma-separated room IDs for batch reads (e.g. room-a,room-b,room-c). Each room rendered with the same mode/last_n settings."),
			"last_n":          prop("string", "Return only the last N messages (default: all). Keeps room header and system prompt."),
			"after_id":        prop("string", "Return only messages with ID greater than this value. For delta reads after context compaction."),
			"mode":            enumProp("string", "Set to 'summary' for system_prompt + latest per type, 'changelog' for only decision + action messages chronologically, or 'work_items' to export actions and decisions as structured work items (useful for ADO/GitHub Issues).", []string{"summary", "changelog", "work_items"}),
			"include_related": prop("string", "Set to 'true' to append a summary of each related room after the main transcript. Resolves related_rooms automatically."),
			"cluster_wide":    prop("string", "Set to 'true' to fetch the transcript from the remote cluster node that owns it."),
		}),
	}, r.handleReadTranscript)

	mcp.AddTool(r.Server.MCP, &mcp.Tool{
		Name:        "get_digest",
		Description: "Get a project activity and knowledge health digest. Shows rooms with new messages since a timestamp, plus rooms flagged by the Knowledge Linter (stale, needs-synthesis). Rooms with compiled synthesis articles show a [Compiled] badge. Perfect for start-of-session orientation: 'what changed?' + 'what needs attention?'.",
		InputSchema: schema([]string{"since"}, map[string]map[string]any{
			"project":      prop("string", "Filter to rooms in this project (optional \u2014 omit for all projects)"),
			"since":        prop("string", "ISO timestamp (e.g. 2026-03-31T12:00:00). Returns only rooms with messages after this time."),
			"cluster_wide": prop("string", "Set to 'true' to fetch the digest from all cluster nodes. Default: local only."),
		}),
	}, r.handleGetDigest)

}
