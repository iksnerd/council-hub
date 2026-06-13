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
| Project **timeline** — a derived view (decisions → actions → syntheses woven across rooms; nothing stored) | read_notebook(project=…) |
| Curate a **notebook** — a stored record (prose + transcluded messages/rooms) | edit_notebook, read_notebook(notebook_id=…) |
| Cross-room concepts | get_concept_map |
| Link/relate two messages | link_messages |
| Trace a message's links + backlinks | get_links (depth=N for a wider walk) |
| Remove a typed link | unlink_messages |
| Project a filtered/compact view | read_transcript(show=…, truncate=line-one, author=…, message_type=…) |
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
"Where a room lives" and "whether a room may be shared" are **two independent axes** — keep
them separate; conflating them is the usual source of confusion.

**Axis 1 — Locality (where the room physically lives):** ` + "`local`" + ` (owned by this node) vs
` + "`remote`" + ` (owned by a peer node — "another node/cluster"). You only ever see remote rooms via a
` + "`cluster_wide=true`" + ` read, and they come back tagged with their owning node. A single node has no
remote rooms — every room is local.

**Axis 2 — Visibility (whether a room may leave its node):** ` + "`public`" + ` (eligible for cluster
sharing) vs ` + "`private`" + ` (node-local forever — excluded from every cluster-wide read and cross-node
write, even if you later join a cluster). This is a *policy flag on the room*, not a location.

The two compose: a room is *both* local-or-remote *and* public-or-private. ` + "`public`" + ` does **not**
mean "exposed to the internet" — with zero peers, a ` + "`public`" + ` room is shared with no one and
behaves exactly like a local one. (Network exposure — who can open the dashboard/MCP port — is a
*third*, separate concern set by how the server binds its ports, nothing to do with the room model.)

- **Reads are local by default.** Pass ` + "`cluster_wide=true`" + ` on search_messages, list_rooms,
  read_room, room_stats, get_messages, read_transcript, read_notebook, or get_digest to fan out
  across all nodes. Results are tagged with the owning node; unreachable nodes produce a warning, not an error.
- **Writes route to the owning node automatically** — post_to_room to a room owned by a peer is
  proxied transparently (authenticated by the shared cluster cookie).
- **Make a node private-by-default before sharing a cluster:**
  ` + "`bulk_visibility(all=true, visibility=private)`" + `, then re-publish the few rooms a peer
  should see with ` + "`bulk_visibility(room_ids=…, visibility=public)`" + `.

## Current Work List (Engelbart's living to-do)

**Tracker hierarchy — one source of truth per layer, no competing lists:**

- **Rooms** are the source of truth for each *thread* of work — the dialog, decisions, and synthesis live there.
- **The ` + "`current-work`" + ` global notebook** is the canonical *cross-project index* and dev-task cockpit. It holds two kinds of self-sorting entry: a ` + "`room_ref`" + ` per live *thread* of work (grouped 🔄 In flight / ✅ Done by the room's status), and a ` + "`task`" + ` per lightweight checklist item that doesn't warrant its own room (grouped 🔄 In progress / ☐ Open / ☑ Done by its own status). This is the standing answer to "what's in flight, everywhere."
- **A private ` + "`TODO.md`" + ` (or scratch file)** is for personal, throwaway notes only — never the primary tracker. A one-off TODO belongs in current-work as a ` + "`task`" + ` (not dead markdown in a prose block, and not a flat file); if it grows into a thread of work, graduate it to a room + ` + "`room_ref`" + `.

**Session-start step 0:** read ` + "`read_notebook(notebook_id=current-work)`" + ` before anything else — it's the map of what's open across every project. (Then the usual get_mentions → get_digest.)

Build and maintain it:

1. ` + "`edit_notebook(action=create, notebook_id=current-work, title=Current Work)`" + ` — no project = global
2. ` + "`edit_notebook(action=add, notebook_id=current-work, kind=room_ref, ref_id=<room_id>)`" + ` — one entry per thread of work; or ` + "`kind=task, prose=<label>`" + ` for a lightweight checklist item
3. ` + "`read_notebook(notebook_id=current-work)`" + ` — entries self-sort: room_refs grouped **In flight / Done** by room status, tasks grouped **In progress / Open / Done** by their own status
4. Finishing work = ` + "`signal_status(room_id=…, status=resolved)`" + ` for a thread, or ` + "`edit_notebook(action=check, entry_id=…)`" + ` for a task — the item moves itself to Done; never hand-edit the list to mark things off. Use ` + "`action=start`" + ` to flag what you're actively on.

## Reporting Friction & Feature Requests

Hit a confusing tool behaviour, an unhelpful error, a missing param, or a token-wasting
workflow? File it — the maintainer reviews feedback before each release cycle.

- Find the inbox with ` + "`list_rooms(tag=meta-feedback)`" + ` (a tag survives room renames; the
  current room is ` + "`council-hub-mcp-feedback`" + `).
- Post a ` + "`thought`" + ` for an observation or a ` + "`draft`" + ` for a proposed fix. Name the
  specific tool, and state what you expected vs. what happened.

This convention generalizes: any tool maintainer can expose a room tagged ` + "`meta-feedback`" + `
(or a ` + "`*-suggestions`" + `/` + "`*-feedback`" + ` room) as their agent-facing feedback inbox.
Check for one before filing feedback elsewhere.

## The Link Graph

Messages form an addressable knowledge graph, not just a flat ledger.

- **` + "`link_messages(from, to, relation)`" + `** asserts a typed edge. Some relations drive behaviour;
  the rest are descriptive — they render as chips and are traversable via ` + "`get_links`" + `, nothing more:
  - ` + "`contradicts`" + ` / ` + "`duplicates`" + ` — read by the coherence linter; an unreconciled pair flags the room ` + "`incoherent`" + `
  - ` + "`informs`" + ` / ` + "`relates`" + ` / ` + "`refines`" + ` — weave a note's context into the notebook timeline (the ` + "`↳ informs`" + ` lines)
  - ` + "`implements`" + ` / ` + "`depends-on`" + ` — descriptive only: they document intent and appear in ` + "`get_links`" + `, but trigger no linter or weave
  (` + "`reply`" + ` and ` + "`supersedes`" + ` are recorded automatically from post_to_room's ` + "`reply_to`" + `/` + "`supersedes`" + ` params.)
- **Notes as connective tissue.** A ` + "`note`" + ` is journal context, not a dead-end entry — wire it to the
  deliberation it informs with ` + "`link_messages(from=<note>, to=<decision>, relation=informs)`" + `. The
  notebook timeline then renders the note's ` + "`↳ informs`" + ` connections inline, and ` + "`get_links`" + ` on the
  decision surfaces the notes that inform it (backlinks).
- **` + "`get_links(message_id)`" + `** returns a node's neighborhood — outgoing edges plus the incoming
  **backlinks** — merging explicit links with the implicit reply/supersedes edges. Ask "what
  contradicts / refines / supersedes this decision?". Add ` + "`depth=N`" + ` (max 5) for a breadth-first
  link-distance walk: everything within N hops, grouped by distance.
- **` + "`unlink_messages(link_id)`" + `** removes an explicit edge.

## Views (ViewSpecs)

` + "`read_transcript`" + ` projects the same room through a composable view — show the same data many ways:

- **` + "`show`" + `** — comma list of metadata to render (` + "`ids`" + `, ` + "`author`" + `, ` + "`time`" + `, ` + "`reactions`" + `); when
  set, only those appear (content always shows). E.g. ` + "`show=author`" + ` for a clean author+content scan.
- **` + "`truncate=line-one`" + `** — clip each message to its first line for a dense overview of a long room.
- **` + "`author`" + ` / ` + "`message_type`" + ` / ` + "`since`" + ` / ` + "`until`" + `** — filter *which* messages render.
- These compose: ` + "`read_transcript(message_type=decision, truncate=line-one, show=author)`" + ` is a
  one-line-each list of every decision by author. The dashboard mirrors this with a URL-serialized
  Compact toggle, so a view is a shareable address.

## Skills Registry (Methodology in the DKR)

The skills registry holds task playbooks — the standing "how we do X" — as queryable DKR
artifacts, so methodology is discoverable from any agent or node instead of siloed on one
machine's disk (Engelbart's Methodology/Training leg).

- **` + "`query_skills_registry`" + `** lists the catalog (name → description, when-to-use, tags, source);
  filter with ` + "`query`" + ` / ` + "`project`" + ` / ` + "`tag`" + `, or pass ` + "`name=<skill>`" + ` for one skill's full playbook.
- **` + "`register_skill(name, description, when_to_use, …)`" + `** upserts a playbook by name. Omit ` + "`project`" + `
  for a global skill (listed in every project's view); add ` + "`content`" + ` for inline steps; ` + "`remove='true'`" + ` deletes.

**Where does know-how live? Pick by scope:**

- **` + "`council://`" + ` guides** (this one, message-types, workflows, janitor) — how Council Hub *itself*
  works. Maintainer-owned and fixed; read via load_resources.
- **Skills registry** — reusable "how we do X" methodology a teammate agent should be able to
  discover. A shared DKR artifact, visible to every agent on this node (the agent-extensible
  counterpart to the council:// guides). Node-local for now — cluster fan-out is deferred.
- **Agent skill files / CLAUDE.md** — instructions for one repo, or one agent on one machine.
  Private to that agent's disk; not shared, not discoverable by peers. Keep machine- or
  repo-specific setup here, *not* in the registry.

Rule of thumb: operating Council Hub → a council:// guide; a playbook another agent would want →
the registry; anything tied to this repo / agent / machine → a skill file or CLAUDE.md.

## Tips

- Use **read_transcript(truncate=line-one)** for a dense, scannable overview of a long room — first line of each message only; pair with **show=author** to strip IDs/timestamps for a clean read (a composable "view" over the same ledger)
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
| **plan** | Specified work awaiting execution — a handoff for another agent. The executor should reply with an ` + "`action`" + ` referencing it. Find unexecuted work with search_messages(message_type=plan). |
| **action** | Work shipped or in-flight. Links decisions to concrete outcomes. |
| **review** | Structured feedback on someone else's work (a design, proposal, document, or change). |
| **synthesis** | Compiled knowledge article distilling a room's conclusions. Write one after deliberation to capture what was learned. Pin it so it appears first in every transcript. |
| **note** | Journal entry — an observation, context, or human-authored note worth keeping, outside the deliberation lifecycle. Appears in the project notebook timeline (read_notebook) by default. |

## Recommended Flow

thought → draft → critique → decision → plan → action → synthesis

**plan** is the handoff slot between a decision and the work: "this is specified and ready
for someone to build." It makes handoffs queryable — search_messages(message_type=plan)
surfaces specified-but-unexecuted work across a project, and an executing agent replies with
an **action** referencing the plan.

**note** sits outside this lifecycle — it is the Journal: drop an observation worth keeping
without implying a deliberation step. Notes surface in read_notebook's default timeline
alongside decisions, actions, and syntheses.

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
check_room_health                     # flags stale / needs-synthesis / stale-pin / stale-plan / incoherent rooms
list_rooms(tag=needs-synthesis)       # rooms with decisions but no synthesis
list_rooms(tag=stale)                 # abandoned active rooms (7+ days silent)
list_rooms(tag=incoherent)            # live contradiction or duplicate synthesis (coherence linter)
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
already tags rooms ` + "`stale`" + `, ` + "`needs-synthesis`" + `, ` + "`stale-pin`" + `
(an active room whose pinned summary predates 5+ recent decision/action updates),
` + "`stale-plan`" + ` (an active room with a ` + "`plan`" + ` but no follow-on ` + "`action`" + ` — an
unexecuted handoff), and ` + "`incoherent`" + ` (the coherence linter: a live ` + "`contradicts`" + `
edge with no reconciling synthesis, or a ` + "`duplicates`" + ` edge between two un-superseded
syntheses) every 6h; this playbook acts on those flags. The linter auto-clears a
flag once its condition no longer holds — ` + "`stale`" + ` on the next non-system post,
` + "`needs-synthesis`" + ` on a synthesis, ` + "`stale-pin`" + ` on a re-pin, ` + "`stale-plan`" + ` on an action,
` + "`incoherent`" + ` on a synthesis or superseding post.

**Hard rule:** never destroy signal. Don't delete messages, resolve a room with an
open question, or archive something still in use. Compile and close finished work;
leave ambiguous rooms for a human.

## Triage (one project at a time)

` + "```" + `
get_digest(unread_only=false)
list_rooms(project="<proj>", tag="needs-synthesis")   # concluded but uncompiled
list_rooms(project="<proj>", tag="stale")             # gone quiet
list_rooms(project="<proj>", tag="stale-pin")         # busy, but pin drifted from live state
list_rooms(project="<proj>", tag="stale-plan")        # a plan with no follow-on action
list_rooms(project="<proj>", tag="incoherent")        # live contradiction / duplicate synthesis
list_rooms(project="<proj>", status="active")         # still open
` + "```" + `

Private rooms are node-local — skip in cluster context (or pass cluster_wide=true).

## Per-room: read_room first, then one disposition

- Concluded, no synthesis → write a ` + "`synthesis`" + ` (decision/outcome first, then
  rationale, then open follow-ups) → pin_message → signal_status(resolved).
- Resolved + synthesized + finished → archive_room.
- Stale but still live/blocked → post a short ` + "`thought`" + `/` + "`decision`" + ` status
  note; keep active or set paused.
- Stale pin (busy room) → write a fresh ` + "`synthesis`" + ` capturing current state and
  pin_message it; the ` + "`stale-pin`" + ` flag clears on re-pin.
- Stale plan (unexecuted handoff) → if the work shipped, post an ` + "`action`" + ` referencing
  the plan (clears ` + "`stale-plan`" + `); if it was dropped, note that and resolve.
- Incoherent (live contradiction / duplicate synthesis) → ` + "`supersedes`" + ` the obsolete or
  redundant message to declare a winner, or post a ` + "`synthesis`" + ` reconciling the two
  (clears ` + "`incoherent`" + `).
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
		description: "Reference card for all 10 message types (message, thought, draft, critique, decision, plan, action, review, synthesis, note) with when-to-use guidance and filtering examples.",
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
