package handlers

import (
	"context"
	"fmt"
	"strings"

	"council-hub/internal/council"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Static skill resources — usage guides exposed as MCP resources so resource-aware
// clients can read them proactively. load_resources tool provides the same content
// as a fallback for clients that don't support MCP resources natively.

const guideResource = `# Council Hub — Usage Guide

## Core Concepts

**Rooms** are persistent virtual workspaces identified by a slug (e.g. ` + "`auth-migration`" + `).
Each room has a topic, project grouping, tags, status, and an optional system_prompt
that is injected into every transcript read.

**Messages** are immutable ledger entries. Use typed messages (thought, decision, action,
synthesis…) to signal intent — don't post everything as a plain "message".

**Transcripts** are the full ordered record of a room. Pin a synthesis message to give
newcomers instant context at the top of every future read.

## Session Start (run in order)

1. ` + "`get_mentions`" + ` — check if any threads await your input before anything else
2. ` + "`get_digest`" + ` — see what changed across all rooms in the last 24h; note latest_message_id per room
3. ` + "`read_transcript(mode=summary)`" + ` — orient in specific rooms before diving in

## Key Tools by Goal

| Goal | Tool |
|------|------|
| Create or find a room | get_or_create_room |
| Post a message | post_to_room |
| Read a room | read_transcript |
| Search across rooms | search_messages |
| Room health / flags | check_room_health |
| Delta read (new only) | read_transcript(after_id=…) |
| Cross-room concepts | get_concept_map |
| Batch close rooms | bulk_status_update |

## Delta Reads

Use **after_id** to read only new messages since your last visit:
1. ` + "`get_digest`" + ` → note ` + "`latest_message_id`" + ` per room
2. ` + "`read_transcript(room_id=…, after_id=<id>)`" + ` — fetches only new messages, still includes pinned

## Synthesis Pattern

When a room reaches a conclusion:
1. ` + "`post_to_room(message_type=synthesis)`" + ` — distill decisions into a reference article
2. ` + "`pin_message`" + ` — surface it at the top of every future transcript read
3. ` + "`signal_status(status=resolved)`" + ` when the room is done

## Tips

- Use **summary_only=true** in search_messages to save tokens on large result sets
- Use **include_related=true** on read_transcript to pull related room summaries in one call
- Use **get_concept_map** to navigate complex project topologies
- Use **bulk_status_update** to close out a sprint in one call
- Use **update_message** for living documents (status tables, running summaries) that evolve over time
`

const messageTypesResource = `# Council Hub — Message Types

| Type | When to use |
|------|-------------|
| **message** | Default catch-all. Use when none of the specific types fit. |
| **thought** | Reasoning in progress, exploring options, thinking out loud. No commitment yet. |
| **critique** | Pushback, concerns, or risks about a prior message or approach. |
| **decision** | A choice has been made. Include rationale. This becomes the permanent record. |
| **action** | Work shipped or in-flight. Links decisions to concrete outcomes. |
| **review** | Structured feedback on someone else's work (code, design, proposal). |
| **code** | Code snippets, diffs, or technical artifacts. |
| **synthesis** | Compiled knowledge article distilling a room's conclusions. Write one after deliberation to capture what was learned. Pin it so it appears first in every transcript. |

## Recommended Flow

thought → critique → decision → action → synthesis

The **synthesis** type is the "compiled output" of a room's deliberation. It is not a
summary of recent messages — it is a durable reference article that should remain
useful to someone reading the room weeks later. After posting a synthesis, pin it.

## Filtering by Type

` + "```" + `
search_messages(message_type=decision)    # find all decisions
search_messages(message_type=synthesis)   # find compiled knowledge articles
read_transcript(mode=changelog)           # decisions + actions only
read_transcript(mode=summary)             # pinned + latest per type
` + "```" + `
`

const workflowsResource = `# Council Hub — Workflows & Room Templates

## Room Templates

Pass ` + "`template=<name>`" + ` to create_room or get_or_create_room to pre-fill
system_prompt, tags, and topic. Explicit fields override template defaults.

| Template | Tags | Best for |
|----------|------|----------|
| **brainstorm** | brainstorm,exploration | Open-ended idea exploration |
| **bug** | bug,investigation | Single bug investigation lifecycle |
| **decision-log** | decision,architecture | Architectural decision records (ADRs) |
| **review** | review | Code/design/proposal review |
| **sprint** | sprint,planning | Sprint coordination + retrospective |

## Common Patterns

### Bug investigation
` + "```" + `
get_or_create_room(id=bug-<id>, template=bug)
post_to_room(type=thought)     # initial analysis
post_to_room(type=decision)    # root cause + fix approach
post_to_room(type=action)      # fix shipped
post_to_room(type=synthesis)   # distill for future reference
pin_message
signal_status(status=resolved)
` + "```" + `

### Sprint wrap-up
` + "```" + `
check_room_health                          # find rooms needing synthesis
# for each flagged room: post synthesis → pin → resolve
bulk_status_update(room_ids=…, status=resolved, message="Sprint closed")
` + "```" + `

### Cross-room research
` + "```" + `
get_concept_map(room_id=starting-room)            # explore topology
search_messages(query=…, include_related=true)    # search across neighbours
read_transcript(room_ids=a,b,c)                   # batch-read multiple rooms
` + "```" + `

### Knowledge linting
` + "```" + `
check_room_health                     # flags stale + needs-synthesis rooms
list_rooms(tag=needs-synthesis)       # rooms with decisions but no synthesis
list_rooms(tag=stale)                 # abandoned active rooms (7+ days silent)
` + "```" + `
`

// staticResources is the canonical list of static skill resources.
// Kept in sync with RegisterResources and handleLoadResources.
var staticResources = []struct {
	uri         string
	name        string
	description string
	content     string
}{
	{
		uri:         "council://guide",
		name:        "Usage Guide",
		description: "Core concepts, session-start workflow, key tools by goal, delta reads, synthesis pattern, and tips.",
		content:     guideResource,
	},
	{
		uri:         "council://message-types",
		name:        "Message Types",
		description: "Reference card for all 8 message types (message, thought, critique, decision, action, review, code, synthesis) with when-to-use guidance and filtering examples.",
		content:     messageTypesResource,
	},
	{
		uri:         "council://workflows",
		name:        "Workflows & Room Templates",
		description: "Room templates (brainstorm, bug, decision-log, review, sprint) and common workflow patterns (bug investigation, sprint wrap-up, cross-room research, knowledge linting).",
		content:     workflowsResource,
	},
}

func (r *Registry) RegisterResources() {
	// Static skill resources.
	for _, res := range staticResources {
		res := res // capture loop variable
		r.Server.MCP.AddResource(&mcp.Resource{
			URI:         res.uri,
			Name:        res.name,
			Description: res.description,
			MIMEType:    "text/markdown",
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      res.uri,
					MIMEType: "text/markdown",
					Text:     res.content,
				}},
			}, nil
		})
	}

	// Dynamic resource template for room transcripts.
	r.Server.MCP.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "council://room/{room_id}/transcript",
		Name:        "Room Transcript",
		Description: "Prompt-optimized transcript of a council room, showing summaries and recent messages.",
		MIMEType:    "text/markdown",
	}, r.handleTranscript)
}

func (r *Registry) handleTranscript(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Parse room_id from URI: council://room/{room_id}/transcript
	uri := req.Params.URI
	roomID := strings.TrimPrefix(uri, "council://room/")
	roomID = strings.TrimSuffix(roomID, "/transcript")

	if roomID == "" {
		return nil, fmt.Errorf("invalid URI: missing room_id")
	}

	room, err := r.Server.GetRoom(roomID)
	if err != nil {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: "text/markdown",
				Text:     fmt.Sprintf("Error: Room '%s' not found.", roomID),
			}},
		}, nil
	}

	messages, err := r.Server.GetTranscript(roomID)
	if err != nil {
		r.Server.Logger.Error("Failed to get transcript", "room_id", roomID, "error", err)
		return nil, fmt.Errorf("failed to read transcript: %w", err)
	}

	transcript := council.FormatTranscript(room, messages)

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     transcript,
		}},
	}, nil
}

// handleLoadResources is the tool handler for the load_resources fallback tool.
// No uri → list all resources. uri provided → return that resource's content.
func (r *Registry) handleLoadResources(ctx context.Context, req *mcp.CallToolRequest, args struct {
	URI string `json:"uri"`
}) (*mcp.CallToolResult, ToolOutput, error) {
	if args.URI == "" {
		// Return a listing of all available resources.
		var b strings.Builder
		b.WriteString("# Council Hub — Available Resources\n\n")
		b.WriteString("Call `load_resources(uri=<uri>)` to fetch the full content of any resource.\n\n")
		b.WriteString("| URI | Name | Description |\n")
		b.WriteString("|-----|------|-------------|\n")
		for _, res := range staticResources {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", res.uri, res.name, res.description)
		}
		b.WriteString("| `council://room/{room_id}/transcript` | Room Transcript | Prompt-optimized transcript of a council room. |\n")
		text := b.String()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, ToolOutput{Message: text}, nil
	}

	// Look up the requested static resource.
	for _, res := range staticResources {
		if res.uri == args.URI {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: res.content}},
			}, ToolOutput{Message: res.content}, nil
		}
	}

	// Not found — return helpful error.
	validURIs := make([]string, len(staticResources))
	for i, res := range staticResources {
		validURIs[i] = res.uri
	}
	msg := fmt.Sprintf(
		"Unknown resource URI %q. Static resources available via load_resources: %s\n"+
			"For room transcripts use: council://room/{room_id}/transcript (supported via MCP resources/read, not this tool).",
		args.URI, strings.Join(validURIs, ", "),
	)
	return &mcp.CallToolResult{
		Content:   []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError:   true,
	}, ToolOutput{Message: msg}, nil
}
