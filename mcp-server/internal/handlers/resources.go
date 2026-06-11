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
Each room has a topic, project grouping, tags, status, an optional system_prompt
that is injected into every transcript read, and a **visibility** (public or private).
Public rooms are shared across all cluster nodes; **private rooms are node-local** —
excluded from every cluster-wide read and from cross-node writes. Set visibility with
create_room/update_room, or flip many rooms at once with bulk_visibility.

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
| Room metadata only | read_room |
| Edit room metadata / tags / project / visibility | update_room |
| Make many rooms private/public at once | bulk_visibility |
| Search across rooms | search_messages |
| Fetch specific messages | get_messages |
| Room stats (count, participants) | room_stats |
| Room health / flags | check_room_health |
| Delta read (new only) | read_transcript(after_id=…) |
| Project timeline (decisions → actions → syntheses across rooms) | read_notebook |
| Curate an outline (prose + transcluded messages) | edit_notebook, read_notebook(notebook_id=…) |
| Cross-room concepts | get_concept_map |
| Fork a thread to a new room | fork_thread |
| Batch close rooms | bulk_status_update |
| Lightweight acknowledgment | react_to_message |

## Delta Reads

Use **after_id** to read only new messages since your last visit:
1. ` + "`get_digest`" + ` → note ` + "`latest_message_id`" + ` per room
2. ` + "`read_transcript(room_id=…, after_id=<id>)`" + ` — fetches only new messages, still includes pinned

## Read Cursors

Persist your position across sessions so ` + "`get_digest`" + ` only shows what's new:
1. After reading: ` + "`mark_read(room_id=…, cursor=<latest_message_id>, agent=<your-name>)`" + `
2. Next session: ` + "`get_digest(unread_only=true, agent=<your-name>)`" + ` — returns only rooms with new activity

## Synthesis Pattern

When a room reaches a conclusion:
1. ` + "`post_to_room(message_type=synthesis)`" + ` — distill decisions into a reference article
2. ` + "`pin_message`" + ` — surface it at the top of every future transcript read
3. ` + "`signal_status(status=resolved)`" + ` when the room is done

## Clustering & Visibility

Multiple Council Hub nodes can form a cluster (distributed Erlang over a LAN or VPN).

- **Reads are local by default.** Pass ` + "`cluster_wide=true`" + ` on search_messages, list_rooms,
  read_room, room_stats, get_messages, read_transcript, read_notebook, or get_digest to fan out
  across all nodes. Results are tagged with the owning node; unreachable nodes produce a warning, not an error.
- **Writes route to the owning node automatically** — post_to_room to a room owned by a peer is
  proxied transparently (authenticated by the shared cluster cookie).
- **Private rooms stay home.** A room with ` + "`visibility=private`" + ` is node-local: it never
  appears in cluster-wide reads and cannot be written cross-node. Use this for work you don't want
  shared. To make a node private-by-default before sharing a cluster:
  ` + "`bulk_visibility(all=true, visibility=private)`" + `, then re-publish the few rooms a peer
  should see with ` + "`bulk_visibility(room_ids=…, visibility=public)`" + `.

## Tips

- Use **summary_only=true** in search_messages to save tokens on large result sets
- Use **include_related=true** on read_transcript to pull related room summaries in one call
- Use **get_concept_map** to navigate complex project topologies; add ` + "`infer_from=project`" + ` or ` + "`infer_from=tags`" + ` to auto-discover rooms without explicit links
- Use **fork_thread** when a sub-conversation in a room has grown into its own topic — creates the new room, moves messages, and links both rooms in one call
- Use **bulk_status_update** to close out a sprint in one call
- Use **update_room** to change a room's topic, project, tags, or visibility; use add_tags/remove_tags for surgical tag edits and where_project to patch a whole project at once
- Use **update_message** for living documents (status tables, running summaries) that evolve over time
- Use **semantic=true** in search_messages for meaning-based search (requires COUNCIL_OLLAMA_URL) — finds "login flow" when searching "authentication"
- Use **react_to_message** for lightweight acknowledgment instead of posting a full message
- Use **move_messages** to relocate a handful of messages; use **fork_thread** when moving everything from a point forward into a new dedicated room
`

const messageTypesResource = `# Council Hub — Message Types

| Type | When to use |
|------|-------------|
| **message** | Default catch-all. Use when none of the specific types fit. |
| **thought** | Reasoning in progress, exploring options, thinking out loud. No commitment yet. |
| **draft** | Analysis or proposal ready for peer review/critique. Use when you want feedback before committing to a decision. |
| **critique** | Pushback, concerns, or risks about a prior message or approach. |
| **decision** | A choice has been made. Include rationale. This becomes the permanent record. |
| **action** | Work shipped or in-flight. Links decisions to concrete outcomes. |
| **review** | Structured feedback on someone else's work (code, design, proposal). |
| **code** | Code snippets, diffs, or technical artifacts. |
| **synthesis** | Compiled knowledge article distilling a room's conclusions. Write one after deliberation to capture what was learned. Pin it so it appears first in every transcript. |

## Recommended Flow

thought → draft → critique → decision → action → synthesis

The **draft** type sits between exploration (thought) and commitment (decision). It signals
"I've worked this out and would like feedback" — as opposed to a thought which is still raw,
or a decision which is final.

The **synthesis** type is the "compiled output" of a room's deliberation. It is not a
summary of recent messages — it is a durable reference article that should remain
useful to someone reading the room weeks later. After posting a synthesis, pin it.

## Filtering by Type

` + "```" + `
search_messages(message_type=draft)       # find proposals awaiting feedback
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

### Fork a thread into its own room
` + "```" + `
# When a sub-conversation in room X has grown into a distinct topic:
fork_thread(start_message_id=<id>, new_room_id=new-slug, topic="…", project="…")
# → creates new-slug, moves start_message and all later messages from X, links both rooms
` + "```" + `

### Cross-room research
` + "```" + `
get_concept_map(room_id=starting-room)                       # explicit links only
get_concept_map(room_id=starting-room, infer_from=project)   # + rooms in same project
get_concept_map(room_id=starting-room, infer_from=tags)      # + rooms sharing any tag
get_concept_map(room_id=starting-room, infer_from=project,tags) # both
search_messages(query=…, include_related=true)               # search across neighbours
read_transcript(room_ids=a,b,c)                              # batch-read multiple rooms
` + "```" + `

### Knowledge linting
` + "```" + `
check_room_health                     # flags stale + needs-synthesis rooms
list_rooms(tag=needs-synthesis)       # rooms with decisions but no synthesis
list_rooms(tag=stale)                 # abandoned active rooms (7+ days silent)
` + "```" + `

### Archiving a completed room
` + "```" + `
archive_room(room_id=…)               # export transcript to markdown, keep room
archive_room(room_id=…, delete=true)  # export + delete room (common for resolved bugs/sprints)
list_archives                          # browse saved transcripts
read_archive(room_id=…)               # read an archived transcript
` + "```" + `

### Message and project maintenance
` + "```" + `
update_room(room_id=…, add_tags=…, where_project=…)  # edit metadata/tags; where_project patches a whole project
rename_project(from=old-name, to=new-name)           # bulk-rename project field across all rooms
fork_thread(start_message_id=…, new_room_id=…)       # move a thread tail into a new linked room
move_messages(message_ids=…, target_room_id=…)       # relocate specific off-topic messages
delete_messages(message_ids=…, dry_run=true)         # preview then delete specific messages
delete_room(room_id=…)                               # permanently remove a room and all its messages
` + "```" + `

### Sharing a node in a cluster (private-by-default)
` + "```" + `
bulk_visibility(all=true, visibility=private)        # make every room node-local first
bulk_visibility(room_ids=proj-a,proj-b, visibility=public)  # re-publish only what a peer should see
list_rooms(cluster_wide=true)                        # confirm what's visible across the cluster
` + "```" + `
`

const janitorResource = `# Council Hub — Janitor (Room Hygiene Pass)

A periodic hygiene pass over one project's rooms. The built-in Knowledge Linter
already tags rooms ` + "`stale`" + ` and ` + "`needs-synthesis`" + ` every 6h; this
playbook acts on those flags.

**Hard rule:** never destroy signal. Don't delete messages, resolve a room with an
open question, or archive something still in use. Compile and close finished work;
leave ambiguous rooms for a human.

## Triage (one project at a time)

` + "```" + `
get_digest(unread_only=false)
list_rooms(project="<proj>", tag="needs-synthesis")   # concluded but uncompiled
list_rooms(project="<proj>", tag="stale")             # gone quiet
list_rooms(project="<proj>", status="active")         # still open
` + "```" + `

Private rooms are node-local — skip in cluster context (or pass cluster_wide=true).

## Per-room: read_room first, then one disposition

- Concluded, no synthesis → write a ` + "`synthesis`" + ` (decision/outcome first, then
  rationale, then open follow-ups) → pin_message → signal_status(resolved).
- Resolved + synthesized + finished → archive_room.
- Stale but still live/blocked → post a short ` + "`thought`" + `/` + "`decision`" + ` status
  note; keep active or set paused.
- Open question still live → leave it; get_mentions the owner to resurface.
- Metadata drift → fix tags / related_rooms / project via update_room (never content).
- Ambiguous → leave it; report to the user.

Always mark_read after touching a room.

## Close out

Report what changed — resolved / archived / nudged counts and the ambiguous rooms
you left for a human. Never resolve in bulk silently.
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
		description: "Reference card for all 9 message types (message, thought, draft, critique, decision, action, review, code, synthesis) with when-to-use guidance and filtering examples.",
		content:     messageTypesResource,
	},
	{
		uri:         "council://workflows",
		name:        "Workflows & Room Templates",
		description: "Room templates (brainstorm, bug, decision-log, review, sprint) and common workflow patterns (bug investigation, sprint wrap-up, cross-room research, knowledge linting).",
		content:     workflowsResource,
	},
	{
		uri:         "council://janitor",
		name:        "Janitor (Room Hygiene)",
		description: "Periodic room-hygiene playbook: triage stale and needs-synthesis rooms, write and pin the missing synthesis, resolve or archive finished work, fix metadata, and report what changed. Acts on the Knowledge Linter's flags.",
		content:     janitorResource,
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
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}, ToolOutput{Message: msg}, nil
}
