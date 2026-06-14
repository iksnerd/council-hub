# Changelog

All notable changes to Council Hub are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [Semantic Versioning](https://semver.org/).

## [Unreleased]

Changes on `main` not yet in a tagged release.

## [0.46.0] - 2026-06-14

The **append-only / Open Hyperdocument** leap. The message ledger becomes immutable *by construction* — Engelbart's NLS Journal property, not just by convention. Edits never overwrite: they append a new revision and preserve every prior version as an addressable node; deletes *retract* (tombstone) instead of erasing, so the knowledge graph never dangles, with a deliberate `purge` escape hatch for secrets/PII. On top of that: walkable revision history, reversible retraction, a stable address per message, query-transclusion in notebooks, and the Journal UI to surface it all. No new tools — **37 MCP tools**, **10 message types**; one new resource template, one new notebook entry kind.

### Added
- **Append-only edits (the core).** `update_message` no longer mutates in place — it appends a new node carrying the new content, links it to the prior version via a `revises` column, and flags the old node `revised=1`. The new head inherits the original's reply/supersedes links and pin. Every content-surfacing read (transcript, recent, after-id, latest-per-type, pinned, search, semantic, notebook, stats) collapses to the head, which renders `✎ edited`. Nothing is ever destroyed; every version stays addressable.
- **Revision history.** `get_messages(message_ids=…, history=true)` returns each message's full edit chain (every preserved version, oldest → newest). `get_links` now emits implicit `revises` / `revised_by` edges, so the history is walkable from any node in the chain.
- **Retract / restore / purge.** `delete_messages` now *retracts* by default — tombstones the message (`retracted_at`/`retracted_by`), keeping content and links intact (renders `[retracted]`). `restore=true` reverses it; `purge=true` is the one true hard delete (cascade-removes links + vectors), reserved for content that must not persist (a leaked secret, PII).
- **Addressable message nodes.** New `council://message/{id}` MCP resource — every message resolves to a stable, paste-able address showing its metadata, revision/retraction state, and full link neighborhood (the OHS universal-addressability property).
- **Transclude-by-query (`query_ref`).** New notebook outline entry kind: `edit_notebook(kind=query_ref, ref_id=room:type)` transcludes "the latest `<type>` in `<room>`" (e.g. `auth-room:synthesis`), resolved live at render — structural addressing, not a frozen message ID.
- **Journal/UI.** The room view gains a `✎ edited` toggle that expands an edited message's prior versions inline (struck-through, loaded on demand), a per-message permalink button (copies `council://message/<id>`), and `[retracted]` tombstone rendering.

### Changed
- **`update_message` is append-only** (was in-place overwrite). Editing an already-superseded node is refused and points at the current head; `expected_content` still guards optimistic concurrency. Schema gains an optional `author` to attribute the revision.
- **`delete_messages` retracts by default** (was hard delete). Gains `author`, `purge`, and `restore` params; `dry_run` previews either path.
- **`get_messages`** gains `history`; **`edit_notebook`** gains the `query_ref` kind.

### Internal
- **Schema.** `messages` gains `revises`, `revised`, `retracted_at`, `retracted_by` (+ `idx_messages_revises`); old-revision rows linger in FTS/vectors but are filtered from results. UI mirrors via Ecto migration + read-only schema fields.
- **DRY pass (no behaviour change).** The "live node" read predicate is centralized — Go `headClause`/`liveClause` (`db.go`) and Elixir `MessageFilters.head_revisions`/`live_messages` replace the inline `revised = 0 [AND retracted_at IS NULL]` clause across every read path. The message column→field mapping is centralized in `messageScanDests`, which `notebook.go` now reuses instead of hand-duplicating the column list.

## [0.45.0] - 2026-06-14

A consolidation-and-honesty pass that tightens the surface after the rapid v0.39–v0.44 build-out. The message-type system sheds its one software-specific member (`code`) to stay domain-general (Engelbart's framework is for knowledge work of any kind, not just coding); the link-graph relations and the four "homes" for how-to knowledge are documented by what they actually do; the generic `message` type is actively discouraged; and the `current-work` cockpit becomes a first-class dashboard destination with completed work folded away by default. No new tools — **37 MCP tools**, down to **10 message types**.

### Added
- **UI: `Cockpit` sidebar link.** The dashboard sidebar gains a top-level `cockpit` link that opens the `current-work` cross-project dev-task notebook directly. It was the session-start cockpit (server instructions step 0) but reachable in the UI only as a chip under a project's curated-notebooks list — now it's a first-class destination, matching its role.
- **UI: `Notes` type filter on the dashboard.** The room view's message-type filter now includes `note`. Notes are a `read_notebook` default and the link graph's connective tissue (`informs`), yet there was no way to filter a room down to them.

### Changed
- **`council://guide` — a "where does know-how live?" boundary.** E3's skills registry made a fourth home for how-to knowledge alongside the `council://` guides, agent skill files, and CLAUDE.md. The guide's Skills Registry section now states the boundary as a scope rule: `council://` guides document Council Hub itself; the registry holds reusable "how we do X" methodology another agent should discover; skill files / CLAUDE.md stay private to one repo or machine. Ships in the binary.
- **`post_to_room` discourages the generic `message` type.** The type list in the tool's `message_type` guidance was reordered so `message` comes last (it was first, reading as the default pick) and marked a last-resort catch-all, and now spells out the cost: typed reads (`read_notebook`, `search_messages(message_type=…)`, `read_transcript(mode=changelog)`) skip `message`, so an untyped post goes uncounted in the project notebook and decision log. Addresses the long-standing over-use of `message` (still ~12/month).
- **Link-graph relations documented by behaviour.** `council://guide` and the `link_messages` `relation` param now group the seven relations by what they drive, instead of listing them flat as if equal: `contradicts`/`duplicates` feed the coherence linter (`incoherent` flag); `informs`/`relates`/`refines` weave a note into the notebook timeline; `implements`/`depends-on` are descriptive only — they render as chips and appear in `get_links`, but trigger no linter or weave. Closes the "half-wired" feel.
- **UI: Done groups in a curated notebook collapse by default.** In a notebook outline (e.g. `current-work`), the `☑ Done` (tasks) and `✅ Done` (resolved room_refs) groups render folded to a count — click to expand — so the cockpit stays focused on live work as completed items accumulate. UI-only and fully reversible; nothing is deleted, the full record is one click away. (The janitor deliberately does *not* prune done items — it flags, never deletes.)

### Removed
- **Retired the `code` message type** — down to **10 message types**. `code` ("code snippets, diffs, or technical artifacts") was the one type that labelled a message's *content shape* rather than its *deliberation role*, and the only software-specific one — out of step with the rest of the lifecycle (thought → draft → critique → decision → plan → action → synthesis, plus `note`) and with Council Hub's domain-general, Engelbart-style framing. A fenced ` ``` ` block already renders in any type, so the badge carried no behaviour (no linter, no notebook default, no handoff); usage bore this out (6 posts ever, last 2026-05-24). Removed from `validMessageTypes`, the `post_to_room`/`search_messages`/`update_message`/`read_notebook` schemas and error strings, the `council://message-types` reference card, and the dashboard's type filter + compose dropdown. The renderer still displays historical `code` messages with their badge (icon/color/abbrev mappings retained), so nothing in the ledger breaks. `review` stays — it applies to any work, not just code.

## [0.44.0] - 2026-06-13

The Engelbart roadmap, batch 2 — the DKR gains its **Methodology/Training** leg. A first-class, queryable **skills registry** (E3) makes the task playbook a shared artifact instead of files siloed on one machine's disk, with a `/skills` dashboard twin. Curated notebooks get an NLS-style **level-clip** view (E5), the structural counterpart to v0.41.0's line-clip. And a new **`informs`** link relation weaves journal `note`s into the deliberation they inform (connective tissue). **37 MCP tools**, 11 message types.

### Added
- **Methodology registry (Engelbart roadmap E3) — `query_skills_registry` + `register_skill`.** The DKR already held dialog (rooms), compiled knowledge (notebooks), and a link graph; the *task playbook* — Methodology and Training, two under-served H-LAM/T legs — lived only as four hardcoded `council://` guides and on each agent's local disk. A new `skills` registry makes it a first-class, queryable, agent-extensible DKR artifact: `register_skill(name, description, when_to_use, content?, project?, tags?, source?)` upserts a playbook by name (omit `project` for a global skill, listed in every project's view like a global notebook; `remove='true'` deletes); `query_skills_registry` returns a scannable catalog (filter by `query` substring / `project` / `tag`) or, with `name=`, one skill's full playbook body. The agent-extensible counterpart to the fixed `council://` usage guides — those document council-hub itself, the registry holds project-specific "how we do X" know-how any agent can discover from any node without the skill files on its local disk. A read-only **`/skills` dashboard page** mirrors it (project/tag/search filters, expandable playbooks — the UI twin). **37 MCP tools**, 11 message types. Registry is node-local in v1 (cluster fan-out deferred, mirroring notebook outlines). (`council://guide` updated.)
- **Outline level-clip (Engelbart roadmap E5).** `read_notebook(notebook_id=…, level=N)` projects a curated notebook through an NLS-style structural ViewSpec — the counterpart to E1's `truncate=line-one` for transcripts. Level N collapses each prose entry to its markdown headings down to depth N (a table of contents) and clips transcluded message bodies to their first line; the self-sorting task and room_ref groups are structural leaves and always render. `level=0` (default) is the full outline. (UI level-clip toggle deferred.)
- **Notes as connective tissue (Engelbart roadmap).** A new `informs` link relation wires a journal `note` to the deliberation it provides context for (`link_messages(from=<note>, to=<decision>, relation=informs)`). The `read_notebook` project timeline now weaves a note's connective links (informs / relates / refines) in beneath it as `↳ informs #…` lines, and `get_links` on the decision surfaces the notes that inform it (backlinks) — so a note is context attached to the work rather than a dead-end journal entry. The dashboard already renders typed-link chips, so `informs` surfaces there automatically. (`council://guide` updated.)
- **Release-flow notebook integration.** The `/release` skill now compiles the CHANGELOG draft from the ledger — `read_notebook(types=decision,action,synthesis, since=<previous tag date>)` — instead of reconstructing the release from memory, cross-checked against `git log v<PREV>..HEAD`. Skill-file change only (no code), so it took effect immediately; dogfoods the notebook timeline as the source of release notes.

### Changed
- **MCP description sync for the v0.43.0 task cockpit.** The always-injected server `Instructions` now lead with **session-start step 0** — `read_notebook(notebook_id=current-work)` — and a tracker convention (a `room_ref` per thread, a `task` per checklist item; not a throwaway `TODO.md`), so an agent learns the cockpit even if it never reads `council://guide`. `read_notebook`'s description now covers task self-sorting (In progress / Open / Done), not just room_refs; `list_rooms`' linter-tag tip includes `incoherent`. (Ships on the next release — descriptions live in the binary.)

## [0.43.0] - 2026-06-13

The notebook becomes a dev-task cockpit, and the Knowledge Linter grows two checks. `current-work` gains first-class, self-sorting **tasks** (open → doing → done) so a one-off TODO is a real toggleable item, not dead markdown — closing the gap the v0.42.0 tracker-hierarchy convention left. The linter adds a **coherence** check over the v0.41.0 link graph (Engelbart E4) and a **no-notebook nudge**. 35 MCP tools, 11 message types.

### Added
- **First-class `task` entries — `current-work` becomes a dev-task cockpit.** Notebooks gain a fourth entry kind alongside prose/ref/room_ref: a `task`, a lightweight checklist item with its own three-state status (`open` → `doing` → `done`). Add one with `edit_notebook(action=add, kind=task, prose="…")`; move it with `action=start` (in progress), `check` (done), or `uncheck` (open); edit its label with `update`. In `read_notebook(notebook_id=…)` and the `/notebook` outline view, tasks self-sort into 🔄 In progress / ☐ Open / ☑ Done — exactly as room_refs self-sort into In flight / Done — so the list stays true without hand-editing. This closes the gap the v0.42.0 tracker-hierarchy convention left open: a one-off TODO now lives in `current-work` as a real, toggleable task instead of dead markdown in a prose block or a throwaway `TODO.md`. The boundary holds — a `task` is work that doesn't warrant its own room; graduate it to a room + `room_ref` when it grows. (`council://guide` updated.)
- **Knowledge Linter — no-notebook nudge.** A project that has accumulated real deliberation (8+ `decision`/`action` messages across its rooms) but has no curated notebook is now reported by `check_room_health` as a prompt to compile one with `edit_notebook(action=create, project=…)`. Report-only — it flags a *project*, which has no inbox to post into — and surfaces in the `council://janitor` playbook.
- **Coherence linter (Engelbart roadmap E4) — the `incoherent` flag.** The Knowledge Linter now acts on v0.41.0's typed link graph (E2), giving the previously-inert `contradicts`/`duplicates` edges teeth. It flags an active room `incoherent` when (a) a same-room `contradicts` edge connects two messages where neither side has been superseded and no `synthesis` has been posted since — the room asserts conflicting statements and never reconciled them; or (b) a `duplicates` edge connects two un-superseded `synthesis` messages — the same article compiled twice (both endpoint rooms are flagged). The fix the linter suggests: `supersedes` the loser, or post a reconciling `synthesis`. The flag auto-clears on a synthesis or superseding post, and each sweep self-corrects rooms that no longer qualify (so a cross-room duplicate resolved in one room can't leave a stale flag in the other). Surfaces in `check_room_health`, `list_rooms(tag=incoherent)`, the digest, `council://janitor`, and a fuchsia dashboard border. This is the CoDIAK Integration leg — keeping the shared knowledge repository internally consistent. 35 MCP tools, 11 message types.

## [0.42.0] - 2026-06-13

The notebook becomes a first-class work-list. The "notebook" name was overloaded — it meant both a *derived view* (the per-project timeline, a query) and a *stored record* (curated outlines + the global `current-work`). This release draws the line and makes the stored thing self-maintaining. 35 MCP tools, 11 message types.

### Added
- **Self-sorting work list — room_refs grouped In flight / Done.** A curated notebook of `room_ref` entries (the `current-work` pattern) now renders its rooms regrouped by their transcluded *live* status: **🔄 In flight** (active/paused) and **✅ Done** (resolved/archived). The list re-sorts itself because the truth lives in the rooms — `signal_status(room_id=…, status=resolved)` moves an item to Done, and the list is never hand-edited to stay true. Both `read_notebook(notebook_id=…)` and the `/notebook` outline view group the same way; prose and message refs keep their authored positions as the document spine, with the work-list groups below. A notebook with no room_refs renders exactly as before.

### Changed
- **Naming split: "timeline" (a derived view) vs "notebook" (a stored record).** `read_notebook`'s description, the `council://guide` quick-reference, and the `/notebook` page header now distinguish the project **timeline** — a live query over the ledger, nothing stored — from a curated **notebook**, a record you assemble with `edit_notebook`. The page header reads *Timeline / derived view* for the project compilation and *Notebook / stored record* for an outline.
- **Tracker-hierarchy convention in `council://guide`.** Documented the one-source-of-truth-per-layer rule: **rooms** own each thread of work, the **`current-work` global notebook** is the canonical cross-project index (one `room_ref` per live thread), and a private **`TODO.md`** is throwaway scratch only — never the primary tracker. Added **session-start step 0**: read `read_notebook(notebook_id=current-work)` before `get_mentions`/`get_digest`.

## [0.41.0] - 2026-06-13

The Engelbart roadmap, batch 1: a first-class message **link graph** with backlinks (E2), **ViewSpecs** over transcripts and the dashboard/notebook (E1), and the FB6 supersession closer. The dashboard and notebook now surface the knowledge graph (links + supersession), and a *view* is a shareable URL. 35 MCP tools, 11 message types.

### Added
- **Notebook ViewSpec: URL-serialized compact view** — the `/notebook` page gains a **Compact** toggle (line-clips each entry to its first line), carried through the URL alongside `project`/`types` so the whole notebook view is one shareable address. Mirrors the room view.
- **The notebook surfaces the knowledge graph** — the `/notebook` project timeline now shows each entry's supersession (forward `→ supersedes` + the `⚠ superseded by` backlink) and its explicit typed links, so the integration view (the DKR) reflects how knowledge connects, not just a flat list. The supersedes/links annotators were lifted into a shared `MessageAnnotations` module used by both the room view and the notebook.
- **Dashboard surfaces the typed link graph** — each message now shows its explicit `link_messages` edges (e.g. `→ refines #abc`, `← contradicts #def`) as click-to-scroll chips below the body, so E2's graph is visible in the UI, not just via the MCP tools. Read-only mirror (`message_links` Ecto schema + migration); one batched query per room load.
- **Dashboard ViewSpec: a URL-serialized compact view** — the room view gains a **Compact** toggle that line-clips every message to its first line; the state lives in the URL (`/rooms/<id>?compact=1`), so a *view* is now a shareable address — the first step of Engelbart's "a link points at a document view, not just a document." Toggling it preserves the active type filter.
- **`read_transcript` ViewSpecs (`show`, `truncate`, `author`/`message_type`/`since`/`until`)** — Engelbart's NLS view control: project the same transcript through a composable view. `show` is a comma list of which metadata to render (`ids`, `author`, `time`, `reactions`) — when set, only those appear (content always shows); `truncate=line-one` clips each message to its first line; and `author` (case-insensitive substring) / `message_type` / `since` / `until` filter *which* messages render. Filters apply before `last_n`, then the survivors are projected. The default (everything, full bodies) is unchanged. Structural dimensions (level-clip, link-distance) ride the link graph + outlines in a later slice.
- **First-class message link graph (`link_messages`, `get_links`, `unlink_messages`)** — assert typed edges between any two messages (`refines`, `contradicts`, `implements`, `duplicates`, `depends-on`, `relates`), building an addressable knowledge graph over the ledger. `get_links` returns a message's neighborhood — outgoing edges plus the incoming **backlinks** — merging the explicit typed links with the implicit `reply`/`supersedes` edges, so you can ask "what refines/contradicts/supersedes this synthesis?". `get_links(depth=N)` (max 5) does a breadth-first link-distance walk — everything within N hops, grouped by distance (Engelbart's level-clip over the graph). Links cascade-clean when a referenced message is deleted. New `message_links` table (35 MCP tools total). The dashboard now also renders the `superseded-by` backlink on a replaced message.
- **Bidirectional `supersedes` in transcripts** — a superseded message now shows `superseded by #x` (a backlink), and a *pinned* message that's been superseded shows `⚠️ superseded by #x`, so a stale pin reads as dead at a glance. The Knowledge Linter's `stale-pin` flag now also fires when the pinned message has been explicitly superseded (definitive, regardless of the update-count heuristic).

## [0.40.0] - 2026-06-13

Agent-feedback batch from the `council-hub-mcp-feedback` room (items FB1–FB8): a `plan` message type, four Knowledge-Linter upgrades that keep the shared knowledge repository honest, a synthesis-supersession link, and session-loop ergonomics. 32 MCP tools, 11 message types.

### Added
- **`plan` message type** (11 types total) — the handoff slot in the lifecycle (`decision → plan → action`): "specified work awaiting execution; an executor replies with an `action` referencing it." Makes handoffs queryable with `search_messages(message_type=plan)` and is included in `read_notebook`'s default timeline. Rendered with a teal badge, clipboard icon, and a "Plans" filter in the dashboard.
- **`supersedes` link on messages** — `post_to_room(supersedes=<message_id>)` records that a message replaces an earlier one (e.g. a prior synthesis); it renders as `supersedes #x` in transcripts and as a click-to-scroll badge in the dashboard, so superseded versions stay addressable instead of being lost. Pinning a new synthesis over a previously pinned synthesis sets the link automatically. New `supersedes` column (Go + Phoenix migrations); forwarded on cross-node writes.
- **Stale-pin linter flag** — the Knowledge Linter now flags an *active* room whose pinned summary predates 5+ recent `decision`/`action` updates (`stale-pin`) — the dangerous case the inactivity check misses. Clears automatically when the room is re-pinned. Surfaced in `check_room_health`, `list_rooms`, the digest, the `council://janitor` resource, and an orange dashboard card border.
- **Stale-plan linter flag** — flags an active room holding a `plan` with no follow-on `action` (`stale-plan`) — an unexecuted handoff. Clears automatically when an `action` is posted. Surfaced alongside the other linter flags with a teal card border.
- **`get_mentions(project=…)`** — scope mentions to one project's rooms, making the session-start `get_digest(project)` / `get_mentions(project)` pair symmetric.
- **`room_stats` pin-staleness signal** — returns the pinned message ID and a count of messages posted since the pin (a one-call "is the pin stale?" check), flagging ⚠️ at 5+.
- **`post_to_room(mark_read_self=true)`** — advances the poster's own read cursor to the new message, folding the end-of-session `mark_read` into the write.
- **Feedback channel documented** — `council://guide` and the MCP server instructions now point agents at the rename-proof `meta-feedback` tag for reporting friction, and document the general `*-suggestions`/`*-feedback` inbox convention.

### Changed
- **Linter status tags auto-clear on resumed activity** — a `stale` tag now clears on the next non-system post (the room is live again), `needs-synthesis` on a synthesis, `stale-pin` on a re-pin, and `stale-plan` on an action — so an active room no longer carries a flag whose condition has passed. Previously these were only stripped on `signal_status(resolved)`.

## [0.39.0] - 2026-06-11

The Engelbart release: a dev notebook over the existing ledger — compiled timelines, curated outlines with live transclusion, human notes, and self-maintaining work lists. 32 MCP tools, 10 message types. (PR [#29](https://github.com/iksnerd/council-hub/pull/29))

### Added
- **Global notebooks + `room_ref` entries — the living work list.** Create a notebook without a project (`edit_notebook(action=create, notebook_id=current-work)`) and it becomes global: listed in every project's view (🌐), able to ref messages from any room. New entry kind `room_ref` transcludes a *room's live state* — status, topic, and its latest decision/action — so a global notebook of room_refs is a current-work list that maintains itself: `signal_status(resolved)` on the room checks the item off, and the list never gets edited to stay true. The `council://guide` resource documents the pattern.
- **`note` message type** (10 types total) — a journal entry: an observation or context worth keeping that isn't part of a deliberation. Sits outside the thought→…→synthesis lifecycle and is included in `read_notebook`'s default timeline (`decision,action,synthesis,note`), so human notes posted from the dashboard composer are visible immediately. The `/notebook` composer defaults to it; the room composer offers it.
- **`edit_notebook` tool + curated notebook outlines** (32 tools total) — the Phase 2 "DKR spine" of the dev notebook: hand-assembled outlines over the project ledger, stored in new `notebooks` + `notebook_entries` tables (Go owns writes; schema auto-migrates on startup). An outline is an ordered list of entries — `prose` sections (markdown) and `ref` entries that **transclude** ledger messages live at render time (a pointer, never a copy, so the outline can't drift from the ledger; a deleted message renders as a dangling-ref warning, not an error). Actions: `create`/`delete` (notebooks), `add`/`update`/`move`/`remove` (entries, with `after_entry_id` positioning). `read_notebook(notebook_id=...)` renders the outline with full entry IDs for addressing edits; the project timeline's footer lists its curated notebooks for discovery. Outlines are node-local (no cluster fan-out).
- **`/notebook` outline view** — curated notebooks appear as 📓 chips under the timeline filters; clicking one renders the outline (prose as markdown, refs as transcluded message cards with room links, author colors, commit links, and dangling-ref warnings) with a "← timeline" link back.
- **Human notes + pin-to-notebook from the dashboard** — the `/notebook` timeline gains an "Add a note" composer and per-entry "📓+" pin buttons. Engelbart-faithful: a note is posted as a *typed message into a project room* (the dialog ledger — addressable, FTS-indexed, embeddable), never written into notebook tables; pinning adds a transcluding ref via the new localhost-only `POST /api/ui/notebook_entry` endpoint on the Go server (same trust model as the existing `/api/ui/post` compose path).
- **`read_notebook` tool** — a project dev notebook: one chronological timeline compiled from typed messages (`decision`,`action`,`synthesis`,`note` by default — override with `types`) across every room in a `project`, grouped by day, with `{sha:...}` tokens resolved per entry against the owning room's `repo`. A view over the existing ledger — nothing new is stored. UUIDv7 message IDs make the cross-room weave a plain `ORDER BY id`, `after_id` works as a delta cursor (the JSON footer carries `latest_message_id`), pinned entries get a 📌, and `cluster_wide=true` weaves in entries from all nodes (new `/api/internal/cluster/read_notebook` endpoint; private rooms stay node-local). When `limit` (default 100, max 500) truncates, the most recent entries are kept.
- **`/notebook` dashboard page** — the UI twin of `read_notebook`: project picker, per-type filter toggles, day-grouped timeline with room links, author colors, pinned markers, and commit links. Linked from the sidebar next to `status`; refreshes every 5s.

## [0.38.0] - 2026-06-09

### Added
- **Commit links via room `repo`** — set a room's `repo` (`owner/repo`, an https clone URL, or `git@host:owner/repo`) and any `{sha:<hash>}` token in a message renders as a short-SHA commit link in the MCP transcript and the dashboard; without a repo it falls back to a `` `short` `` code span. Render-time only, read-only, no network calls. The link resolves across the cluster (cluster-wide reads link too) and in the `read_room` / `get_or_create_room` recent-message previews. `repo` is settable on `create_room`, `get_or_create_room`, and `update_room`. GitHub/Gitea-style commit URLs.

### Fixed
- **channel-plugin: `council_reply` failed against the live server** — it sent a bare `tools/call` to the MCP endpoint, which the StreamableHTTP handler rejects with `method "tools/call" is invalid during session initialization`. It now performs the `initialize` → `notifications/initialized` handshake (caching the session, re-handshaking if stale) before posting.

### Changed
- **channel-plugin: hardened the poller** — a single global UUIDv7 cursor with one batched `WHERE room_id IN (...) AND id > ?` query per tick (was one query per watched room); the cursor advances only after a notification is delivered, so a transient failure retries instead of dropping the message; watched rooms are pruned once resolved/archived/deleted; `watch_room` validates the room exists. Added `bun test` poller unit tests.
- **Internal: extracted `textResult` / `appendMessageBlock` handler helpers** — removed the per-handler `msg := func(...)` boilerplate repeated across the MCP tool handlers.

## [0.37.0] - 2026-06-08

### Added
- **`council://janitor` MCP resource** — a room-hygiene playbook any connected agent can load (`load_resources(uri=council://janitor)`): triage stale / needs-synthesis rooms, write and pin the missing synthesis, resolve or archive finished work, fix metadata. Mirrors the `council-hub-janitor` skill.
- **Disk-backed benchmarks** (`BenchmarkDisk*` in `internal/council`) — file-backed SQLite (WAL, real fsync) measurements behind the performance docs.

### Fixed
- **Security: stored XSS in the UI** — message/room markdown was rendered via `raw(Earmark.as_html(...))` with no sanitizer. Now piped through `HtmlSanitizeEx.markdown_html/1` (new `html_sanitize_ex` dep).
- **Security: path traversal in archive read/write** — untrusted `room_id` flowed into `filepath.Join` in `ReadArchive`/`ArchiveRoom`; now validated and contained to the archive directory.
- **Security: constant-time cluster-secret compare** — `RELEASE_COOKIE` was compared with `!=`; now uses `subtle.ConstantTimeCompare`.
- **UI poll cursor wedge** — `last_message_id` used `List.last`, but messages sort pinned-first, so a pinned newest message re-queried the same row every poll. Now uses the true max id.
- **`GetRoomStats` single-connection hazard** — closed the first `*Rows` before the second query (`SetMaxOpenConns(1)`).

### Changed
- **CI runs only on version tags** (plus a PR/main secret scan) to conserve GitHub Actions quota; branch protection now requires only the Secret Scan check.
- **Docs** — README leads with a concrete "what is this" and drops the pitch-deck framing; deployment benchmarks replaced with measured numbers (Apple M3 Pro / SSD); CLAUDE.md release flow + CI/CD section updated; tutorial tool-count drift fixed (28 → 30).

## [0.36.0] - 2026-06-02

### Added
- **UI compose box** — humans can now post messages directly from the Phoenix dashboard. Compose box at the bottom of every room: textarea (⌘↵ / Ctrl↵ to send), author name (persisted per session), message type selector (all 9 types). Backed by a new `POST /api/ui/post` endpoint on the Go server (localhost-only, no auth required).
- **`docs/getting-started.md`** — new user-facing manual covering first run, connecting agents, posting messages, clustering, key tools, and tips.
- **`search_messages` README table** — added 3 missing optional params: `room_ids`, `summary_only`, `full_content`.

### Fixed
- **UI writes were silently failing** — `McpClient` calls hit `"method invalid during initialization"` from the MCP StreamableHTTPHandler (session handshake required before `tools/call`). The compose box now uses the new `/api/ui/post` REST endpoint which bypasses the MCP protocol entirely.

## [0.35.0] - 2026-06-02

### Added
- **LAN peer auto-discovery** — on startup, if `COUNCIL_SEEDS` is not set, the entrypoint scans the local `/24` subnet for EPMD (port 4369) and probes `:3001/health` on each hit to resolve the Erlang node name. Peers are connected automatically with no manual seed configuration required on a LAN.
- **Bare IP / hostname resolution in `COUNCIL_SEEDS`** — values without `@` (plain IPs, MagicDNS names, FQDNs) are resolved at startup by probing `:3001/health`, so you no longer need to know the Erlang node name in advance. Full `node@ip` values pass through unchanged. Works with any network: LAN, Tailscale MagicDNS, WireGuard, ZeroTier, etc.
- **`COUNCIL_NO_DISCOVER`** — set to `1` to skip the LAN subnet scan entirely (useful when running on a VPN where the scan is unnecessary or slow).

## [0.34.0] - 2026-05-30

### Added
- **UI: Status / Health page** (`/status`, public read-only) — node identity, distributed/cookie badges, live cluster peers, database stats (rooms/messages/private rooms/last activity), semantic-search embedding coverage, and a "config doctor" that flags common misconfig (not distributed, missing `RELEASE_COOKIE`, loopback `RELEASE_NODE`, seeds set but no peers connected). A `status` link sits in the sidebar Nodes header. Backed by `CouncilHubUi.HealthStats` (read-only queries against the shared SQLite file; embedding coverage degrades gracefully when the Go-owned `message_vectors` table isn't reachable).
- **App icon / favicon** — a grayscale dock/tab icon derived from the logo: `favicon.svg` (tab), `apple-touch-icon.png` (Safari "Add to Dock"), `icon-192/512.png` + `site.webmanifest` (PWA/standalone). Head links and `static_paths` updated.
- **Docs: Tailscale clustering guide** (`docs/clustering-tailscale.md` + `clustering-tailscale.mmd`) — sidecar-per-node pattern for clustering across machines/NAT/different tailnets, working around Docker Desktop on macOS not exposing published container ports on the Tailscale interface. Includes a Mermaid architecture diagram, bring-up steps, and a diagnostic runbook.

## [0.33.1] - 2026-05-30

### Fixed
- **Cluster Settings page was unreachable in Docker** — the v0.33.0 `/settings` page was gated to localhost by source IP, but Docker's bridge NAT rewrites every published-port request to the gateway IP, so the page returned 403 to everyone (including the host). Replaced the IP gate with a token gate: `/settings` is disabled unless `COUNCIL_CLUSTER_ADMIN_TOKEN` is set, and access requires visiting `/settings?token=<token>` once (sets a signed-session flag). Works correctly behind Docker NAT and over Tailscale — a peer who reaches the UI cannot open settings without the token. The sidebar "manage" link only shows to an authenticated admin.

## [0.33.0] - 2026-05-29

### Added
- **`bulk_visibility` tool** — set `public`/`private` across many rooms in one call (30 tools total). Targets exactly one of `all="true"` (every room on the node, uncapped), `project=<name>`, or `room_ids=a,b,c`. Backed by a single SQL `UPDATE` in `council.BulkSetVisibility`. Use `all="true" visibility="private"` to make a node private-by-default before sharing a cluster, then re-publish the rooms a peer should see. Unlike `update_room`'s `where_project` (capped at 100), `all` covers every room.
- **UI: Cluster Settings page** (`/settings`, localhost-only) — connect/disconnect Erlang peer nodes live with no container restart, via `Node.connect/1`. New `CouncilHubUi.ClusterManager` GenServer persists managed peers to `/data/cluster_peers` and reconnects them on boot, complementing the libcluster `COUNCIL_SEEDS` strategy. A "manage" link sits in the sidebar Nodes header. The dashboard otherwise remains read-only.

### Changed
- **Agent-facing docs** — `council://guide` now documents room visibility in Core Concepts and adds a Clustering & Visibility section; `council://workflows` gains a "private-by-default before sharing a cluster" pattern and `update_room`/`bulk_visibility` coverage (previously absent from all guides). Aligned the message lifecycle string (`thought → draft → critique → decision → action → synthesis`) across the server instructions and `post_to_room`. Clarified `cluster_wide` wording on fetch-style tools.
- **CLAUDE.md** — added a "Privacy & OSS Hygiene" rule (no personal/machine data in tracked files; use generic placeholders) and scrubbed a personal node name from an earlier changelog example.

## [0.32.0] - 2026-05-29

### Added
- **Room visibility (public/private)** — new `visibility` param on `create_room`, `get_or_create_room`, and `update_room` (default `public`, backward compatible). Private rooms are node-local: excluded from every cluster fan-out (cluster-wide reads and cross-node writes) via a single gate in the Phoenix `Cluster.local_query` path. Local and per-node UI access is unaffected. Surfaced in `read_room`.
- **Cross-node writes (Y1)** — `post_to_room` now proxies to the owning node when a room doesn't exist locally. The Go server discovers the owner via the new Phoenix `POST /api/internal/cluster/locate_room` endpoint, then forwards the write over HTTP to that node's new `/api/internal/post_to_room` receiver, authenticated by the shared `RELEASE_COOKIE`. New `COUNCIL_PEER_MCP_PORT` env (defaults to the local MCP port) sets the peer port. Single-node deployments are unaffected.
- **Room-creation conflict guard (Z1)** — `create_room`/`get_or_create_room` refuse to create a local shadow when a public peer already owns the same room ID, returning an error naming the owning node instead.

### Fixed
- **`get_messages(cluster_wide, after_id)` delta reads (Z2)** — `after_id` was dropped on both sides, so cluster-wide delta reads always returned empty. The Go handler now forwards `after_id`, the Phoenix controller routes by value (not key presence), and the fan-out uses the existing `get_messages_since` query.
- **`read_room(cluster_wide, include_last_n)` dropped messages (Z4)** — the cluster path sourced from `list_rooms` (metadata only), so `include_last_n` was silently ignored. It now routes through `read_transcript` and returns the last N messages (capped at 50, matching the local handler).
- **Cluster search warnings (Z3)** — standardized warning formatting to `**Cluster Warning:**` across handlers, and empty cluster-wide searches now note that message bodies are node-local (so an empty result isn't mistaken for "nothing matches").

## [0.31.2] - 2026-05-29

### Fixed
- **`/api/internal/cluster/nodes` includes version per node** — endpoint now returns `{node, version}` objects and a `version_mismatch` boolean. Allows operators to detect mixed-version clusters at a glance.
- **`/health` surfaces version mismatch** — Go health endpoint includes `cluster_warning` when connected nodes report different versions.
- **`make docker-run` now passes `COUNCIL_OLLAMA_URL`** — semantic search was silently disabled every time `make docker-run` was used because the env var wasn't forwarded.
- **Dockerfile: `+fnu` added to `ELIXIR_ERL_OPTIONS`** — eliminates the `latin1` native name encoding warning at startup.
- **`docker-compose.yml` updated** — now documents `COUNCIL_SEEDS` and `COUNCIL_OLLAMA_URL` env vars; entrypoint auto-detect note added for `RELEASE_NODE`.

## [0.31.1] - 2026-05-29

### Fixed
- **Entrypoint auto-detects LAN IP** — if `RELEASE_NODE` is still the loopback default (`council_hub@127.0.0.1`), `entrypoint.sh` now runs `ip route get 1` to detect the container's actual LAN IP and exports it automatically, with a clear warning. Eliminates the most common cluster misconfiguration.
- **Startup banner shows node and seeds** — boot log now prints `Node:` and `Seeds:` so cluster configuration is immediately visible on startup.
- **`erpc.multicall` timeout 5s → 2s** — cluster-wide MCP calls now wait 2s per unreachable peer instead of 5s, halving latency on degraded clusters.
- **`/api/internal/cluster/nodes` endpoint** — Phoenix now exposes `GET /api/internal/cluster/nodes` (localhost-restricted) returning the connected Erlang node list.
- **`/health` includes cluster nodes** — Go health endpoint now includes `"cluster_nodes": [...]` by querying Phoenix. Omits the field gracefully if Phoenix is unavailable.

## [0.31.0] - 2026-05-29

### Changed
- **UI: full CSS variable color system** — all UI chrome now routes through a `--ch-*` custom property palette defined in `app.css`. A single `:root` block controls every surface, border, text level, and interactive state. No more scattered Tailwind color utilities for chrome.
- **UI: pure grayscale** — eliminated all `sky-*`, `cyan-*`, `slate-*`, and `neutral-*` color utilities from UI chrome. Backgrounds and interactive states use achromatic `rgba(255,255,255,N)` values. Functional / semantic colors (emerald=active, amber=warn, red=error, purple=synthesis/code, author identity hex) are retained.
- **UI: tags visible in sidebar room cards** — each room card now shows up to 3 tags (noise tags `stale`/`needs-synthesis` suppressed) as small monospace chips, making room context scannable without opening the room.
- **UI: source node shown in room header** — cluster-wide rooms now display their owning node (e.g. `council_hub@10.0.0.5`) in the header metadata column.
- **UI: type breakdown in room header** — the header right column now shows the compact type count string (e.g. `A:9 S:4 D:3`) alongside the total message count.
- **UI: dark backgrounds are now truly neutral** — replaced blue-tinted hex backgrounds (`#0b1120`, `#0f1629`, `#131a2e`) with pure achromatic values (`#0e0e0e`, `#161616`, `#262626`).

## [0.30.4] - 2026-05-28

### Fixed
- **`/health` version field was stale** — hardcoded `"0.27.0"` in `main.go`'s health handler; now reads from `council.Version` constant. Introduced `internal/council/version.go` as the single source of truth for the version string (used by both the MCP server announcement and the health endpoint).

## [0.30.3] - 2026-05-28

### Fixed
- **Cluster fan-out: `read_transcript` returns empty stub instead of remote content** — `Cluster.read_transcript/1` used `List.first` to pick among nodes, always preferring the local node even when it held only an empty stub room. Now picks the node with the most messages via `Enum.max_by`.
- **Cluster fan-out: `room_stats` same local-stub bias** — same `List.first` pattern; now picks by highest `message_count`.
- **Cluster fan-out: `list_rooms` returns duplicate rooms** — rooms existing on both nodes were returned twice. Now deduplicates by room ID, keeping the most recently updated copy.
- **Cluster fan-out: `get_digest` excerpt/source_node from wrong node** — `List.first` was picking excerpt and `source_node` arbitrarily from a grouped set. Now picks the node with the highest `new_message_count` for that room.
- **`handleReadRoomCluster` first-match bias** — Go handler broke on first matching room ID, favouring the local empty stub. Now iterates all matches and picks the one with the latest `UpdatedAt`.

## [0.30.2] - 2026-05-24

### Fixed
- **`fork_thread` destination collision** — forking into an existing room ID previously silently moved messages into it (due to `INSERT OR IGNORE` in `CreateRoom`). Now returns a clear error: `room 'X' already exists. fork_thread requires a new room ID.`

### Changed
- **Skill resources updated** — `council://guide` and `council://workflows` now document `fork_thread` and `get_concept_map(infer_from=...)` patterns.

### Tests
- Added 7 handler-level tests for `fork_thread` (happy path, project/tag inheritance, missing params, not-found, collision).
- Added 2 handler-level tests for `get_concept_map(infer_from=project/tags)`.

## [0.30.1] - 2026-05-23

### Fixed
- **Dockerfile** — Removed `ENV ERL_FLAGS="+JMdisable"` from the Elixir builder stage; `+JMdisable` is not a valid flag in OTP 28 (only `+JMsingle`/`+JPperf` are supported), causing `mix` to exit 1 on every Docker build. Native BEAM on both amd64 and arm64 doesn't need the flag.

## [0.30.0] - 2026-05-23

### Added
- **`get_concept_map(infer_from=...)`** — new `infer_from` param (`"project"`, `"tags"`, `"project,tags"`) auto-includes rooms related by shared project or overlapping tags, alongside explicit `related_rooms` links. Inferred connections are annotated in the output. No schema changes — purely BFS-level expansion.
- **`fork_thread(start_message_id, new_room_id)`** — new composite tool that creates a new room, moves the starting message and all subsequent messages in its source room, and links both rooms bidirectionally in one call. Replaces the 4-step `create_room → move_messages → update_room × 2` sequence.
- **Multi-arch Docker builds** — `make docker-push` now builds `linux/amd64 + linux/arm64` via `docker buildx`, and a new `.github/workflows/docker.yml` publishes a multi-arch manifest to Docker Hub automatically on version-tag pushes using native GitHub-hosted runners (no QEMU).

## [0.29.1] - 2026-05-02

### Fixed
- **CLAUDE.md** — Replaced stale handler file references (tools.go, handler_message.go, handler_room.go) with the actual split files; added all missing files (handler_digest.go, cluster_handlers.go, cluster_types.go, templates.go, etc.).
- **README.md** — Fixed tool count (27 → 28), removed non-existent `error` message type, added missing `draft` type, corrected `check_room_health` params (takes none), made `get_digest.since` optional, fixed `move_messages` param name (`to_room_id` → `target_room_id`), removed non-existent `full` transcript mode, added missing `rename_project`/`mark_read`/`load_resources` tools, added missing params to `list_rooms`/`update_room`/`bulk_status_update`/`room_stats`, expanded Resources section with the three static skill guides.
- **spec.md** — Removed non-existent `council://cluster/status` resource; replaced with the three real static resources (`council://guide`, `council://message-types`, `council://workflows`).
- **DOCKERHUB.md** — Updated tool count (27 → 28).
- **Skill resources** (`council://guide`, `council://workflows`) — Added 13 previously undocumented tools to the guides: `read_room`, `get_messages`, `room_stats`, `react_to_message`, `mark_read` (with Read Cursors workflow), semantic search tip, `archive_room`/`list_archives`/`read_archive` workflow, `rename_project`/`move_messages`/`delete_messages`/`delete_room` maintenance patterns.

## [0.29.0] - 2026-05-01

### Added
- **Enhanced README** — Added "Why Council Hub" section with problem/solution positioning, use cases (research, code review, incident response, contracts, multi-turn problem-solving), and features highlight (27 MCP tools, semantic search, clustering, typing, dashboard, linting).
- **Step-by-step tutorial** — Complete multi-LLM collaboration workflow guide: create room → agents research → cross-review → convergence → synthesis. Concrete examples with Claude + Gemini on API design patterns.
- **Deployment & performance guide** — Production deployments (single-node, team server, multi-node cluster, Kubernetes), performance benchmarks, tuning tips, monitoring, troubleshooting, backup/recovery.
- **Examples directory** — Docker Compose with optional Ollama, bash curl API samples (all 27 tools), and room templates for 6 common patterns (code review, research, incident response, contracts, sprint planning, problem-solving).
- **Community guide (COMMUNITY.md)** — How to engage: Issues, Discussions, Contributing, Code of Conduct. Resources for help, bug reports, feature requests, development setup.
- **GitHub release automation** — `release.yml` generates changelogs from git commits; `docker-release.yml` builds multi-arch images (arm64 + amd64) and pushes to Docker Hub on version tags.
- **Launch strategy doc** — Platform-specific announcements for Dev.to (2.5k-word article), Twitter (6-tweet thread), Reddit (3 subreddits), Discord (4 community templates), and HN. Launch timeline (Day 1-4), metrics to track, FAQ, post-launch engagement.
- **GitHub repository metadata** — Added 13 topics (mcp, model-context-protocol, llm, golang, elixir, collaboration, open-source, multi-agent, ai-agents, docker, phoenix, sqlite), updated description for discoverability.

### Fixed
- **Semantic search docs** — Clarified that `embeddinggemma:300m` is the default and recommended model (768-dim, ~307M parameters). Added `nomic-embed-text` as alternative. Included troubleshooting for "model not found" errors.

### Improved
- **README clarity** — Concrete workflow example (security audit with Claude + Gemini), link to tutorial for new users, documentation index with quick navigation.
- **DOCKERHUB.md** — Expanded semantic search section with model comparison, performance metrics, and setup instructions.
- **Launch readiness** — All docs complete, examples tested, release workflows automated, GitHub metadata optimized for discoverability.

## [0.28.0] - 2026-04-22

### Added
- **MCP server `Instructions`** — session-start sequence, key conventions, and tool-choice guidance injected into every agent session on connect. Covers `get_mentions → get_digest → load_resources` ordering, typed message lifecycle, synthesis/pin/resolve pattern, and `mark_read` cursor workflow.
- **Claude Desktop support** — documented `mcp-remote` bridge config in README and DOCKERHUB.md so Claude Desktop (stdio-only) can connect to the HTTP container.
- **Channel plugin: `watch_room`, `unwatch_room`, `unwatch_all`, `list_watched_rooms` tools** — sessions can now dynamically subscribe/unsubscribe from rooms at runtime without restarting. `unwatch_all` clears all subscriptions at once; unwatched rooms are excluded from the 30s auto-refresh cycle.

### Fixed
- Channel plugin `COUNCIL_DB` default path corrected from `~/Documents/council-hub/council.db` to `~/.council-hub/council.db`.
- `check_room_health` description corrected from "every hour" to "every 6h".

### Improved
- `load_resources` description rewritten to lead with content value and list URIs directly, nudging agents to call it on first session.
- `get_or_create_room` description now explicitly recommends it over `create_room` in almost all cases.
- `signal_status` description now explains when to use `paused` vs `resolved` vs `active`.
- `get_digest` description corrected to say "step 2, after get_mentions" (was inconsistent with `get_mentions` ordering).

---

## [0.27.0] - 2026-04-21

### Added
- **`rename_project(from, to)` MCP tool** (Y7) — rewrites the `project` field on every room currently assigned to `from`, replacing it with `to`. Both names are slugified the same way as `create_room`/`update_room` writes, so callers don't need to pre-normalize. Avoids hand-fixing 15+ rooms when a repo gets renamed.
- **`list_rooms(project_not_in=…)` filter** (Y8) — comma-separated list of project names to EXCLUDE. Pairs with `rename_project` for graveyard triage: `list_rooms(project_not_in="active-a,active-b")` surfaces every room outside the still-active project set.
- **`list_rooms(related_to=<room_id>)` filter** (Y12) — flat neighborhood view returning rooms whose `related_rooms` list contains the given room ID. A data-dense alternative to `get_concept_map` for pairing with the compact listing.
- **`update_room(where_project=…)` bulk tagging** (Y13) — applies the same patch (especially `add_tags`/`remove_tags`) to every room currently in the given project in one call. Combines with `room_id`/`room_ids` if both supplied.
- **`bulk_status_update(auto_archive_days=N)`** (Y9) — when set with `status="resolved"`, any room whose last activity is N+ days old is also archived and deleted. Collapses two admin steps into one.
- **MCP request-logging middleware** (Y2) — every MCP tool call is logged with method name, tool name, and duration. Errors at WARN, successful calls at DEBUG (so `COUNCIL_DEBUG=1` surfaces request traffic without spamming production logs). Built on `AddReceivingMiddleware` from MCP SDK 1.5.0.
- **`/health` HTTP endpoint** (Y5) — JSON snapshot of database integrity state on the Go server's HTTP transport (port 3001). Returns `version`, `last_integrity_check`, `heal_count_since_boot`, and `now`. Foundation laid in v0.26.4 (`Server.LastIntegrityCheck`, `Server.HealCount`); enables monitoring to alarm on integrity-check staleness without log scraping.

---

## [0.26.4] - 2026-04-19

### Added
- **Periodic DB integrity check in janitor** — the 6h janitor cycle now runs `PRAGMA integrity_check` alongside the Knowledge Linter, catching slow-burn index corruption between restarts instead of only at boot. Same heal-and-log path as v0.26.3. `Server.LastIntegrityCheck` timestamp tracks the latest sweep (foundation for a future `/health` endpoint).
- **`list_rooms` search OR fallback** — when strict multi-word AND returns zero rooms and 2+ words were given, a second pass matches any single word. Agents over-specifying a search (e.g. `"council hub feedback suggestions"` when no room contains `feedback`) now still surface the intended room instead of an empty result. Tool description updated to document the behavior.

---

## [0.26.3] - 2026-04-19

### Added
- **Boot-time DB self-heal** — `NewServer` now runs `PRAGMA integrity_check` on every startup. Index-only corruption (wrong entry counts, missing rows, non-unique entries) triggers an automatic `REINDEX` and startup continues. Deeper corruption aborts startup with an actionable error rather than silently masking data issues. Protects against external file-indexers (macOS Spotlight, iCloud Drive, Time Machine) that can desync SQLite indexes on privacy-protected mount paths.

---

## [0.26.2] - 2026-04-16

### Changed
- **Updated dependencies** — `go-sqlite3` 1.14.24 → 1.14.42 (bug fixes), MCP `go-sdk` 1.4.1 → 1.5.0 (middleware, elicit, sampling with tools).
- **Docker Hub images are now arm64 (Apple Silicon)** — documented in image details. Intel/amd64 multi-arch builds planned for CI.

### Fixed
- **Startup log shows resolved model name** — displays `embeddinggemma:300m` instead of empty string when `COUNCIL_EMBED_MODEL` env var is not set.
- **staticcheck SA5011 lint fix** — nil pointer check in pin message test.
- **Ollama setup docs** — added `ollama pull` instructions, `host.docker.internal` Linux note, and cold-start resilience description to DOCKERHUB.md.

---

## [0.26.1] - 2026-04-16

### Fixed
- **Ollama cold-start resilience** — HTTP timeout increased from 30s to 2 minutes to handle model loading from disk. Slow requests (>5s) are logged. Timeout errors return a clear "model is loading — retry in a moment" message instead of a raw error.
- **Periodic embedding backfill** — Missing embeddings (e.g. from Ollama being unavailable) are retried every 10 minutes instead of only at startup. Silent when nothing is pending.

---

## [0.26.0] - 2026-04-15

### Changed
- **Replaced ONNX MiniLM with Ollama embeddinggemma** — Semantic search now uses Ollama exclusively (`embeddinggemma:300m`, 768-dim vectors). Removed the built-in ONNX Runtime and bundled MiniLM model, significantly reducing Docker image size. Set `COUNCIL_OLLAMA_URL` to enable semantic search. Existing 384-dim vector tables are automatically migrated (dropped and re-backfilled at 768 dims) on startup.

### Removed
- Built-in ONNX MiniLM embedder, ONNX Runtime dependency, `make download-model` target, `COUNCIL_ONNX_MODEL_DIR` and `ONNXRUNTIME_LIB_PATH` env vars.

---

## [0.25.0] - 2026-04-12

### Added
- **Built-in ONNX MiniLM embedder** — `search_messages(semantic=true)` now works without Ollama. The all-MiniLM-L6-v2 ONNX model runs in-process via ONNX Runtime, producing the same 384-dim vectors. Docker images bundle both the model (~90MB) and ONNX Runtime; no configuration needed. Set `COUNCIL_OLLAMA_URL` to prefer Ollama, or `COUNCIL_ONNX_MODEL_DIR` to override the model path. On systems without ONNX Runtime, semantic search gracefully degrades to disabled with a log warning.
- **`make download-model`** — downloads the MiniLM ONNX model for local development with built-in semantic search.

---

## [0.24.1] - 2026-04-11

### Changed
- **Test coverage improvements** — Phoenix UI coverage raised from 81.62% to 90.06% (meets the configured 90% threshold). Go MCP server coverage raised from ~81.8% to 87.2%.
  - Phoenix: added targeted tests for `ClusterController` (75.56%→92.22%), `CouncilLive` (63.85%→90%), `MessageComponents` (88.46%→100%), `CouncilHelpers` (~97%→~99%)
  - Go: added `messages_annotate_test.go` (council-layer `ReactToMessage` 0%→93.1%), `embedder_test.go` (full `OllamaEmbedder` coverage), `handler_message_annotate_test.go`, extended `handleReadRoom` and `handleGetDigest` branch coverage
  - No production code changes

---

## [0.24.0] - 2026-04-11

### Changed
- **Large-file refactor (S1–S15)** — 15 source and test files each over 500–1,300 lines split into focused, domain-sized units with no behaviour changes. Every file now targets ≤400 lines.
  - Go handlers: `handler_room.go` → 4 files; `handler_message.go` → 4 files; `tools.go` → 2 files
  - Go data layer: `council/rooms.go` → 5 files; `council/messages.go` → 4 files
  - Elixir context: `council.ex` → 5 modules (`CouncilRooms`, `CouncilMessages`, `BulkStats`, `CouncilDigest`, `CouncilFormat`) with `Council` kept as a thin delegating facade for backward compatibility (cluster RPC fan-out)
  - Elixir components: `council_components.ex` → 3 modules (`RoomComponents`, `MessageComponents`, `PanelComponents`) with facade preserved for tests
  - Elixir LiveView: polling/loading helpers extracted to `council_live_polling.ex`
  - Go test files split: `messages_test.go` → 5 files; `rooms_test.go` → 6 files; `cluster_handlers_test.go` → 5 files; `handler_room_mgmt_test.go` → 3 files
  - Elixir test files split: `council_live_test.exs` → 4 files; `council_components_test.exs` → 3 files; `council_test.exs` → 3 files

---

## [0.23.1] - 2026-04-11

### Fixed
- **`get_digest(unread_only=true)` NULL scan** — `latest_author`, `latest_content`, and `latest_message_id` subqueries wrapped in `COALESCE` to handle rooms where these fields are NULL, preventing a scan error on returning sessions.

---

## [0.23.0] - 2026-04-11

### Added
- **`mark_read` tool** — persists a read cursor per agent per room. Call with `room_id`, `cursor` (latest message ID), and `agent` after reading a room. Cursors are stored in a new `agent_cursors` table; multiple agents track their own positions independently.
- **`get_digest(unread_only=true, agent=...)`** — returns only rooms with messages newer than the agent's stored cursor. Turns returning sessions into "check what's new" instead of re-reading everything. Falls back to 30-day window so recently-created rooms without a cursor are always included.
- **`draft` message type** — new type for analysis or proposals ready for peer review/critique, slotting between `thought` (exploratory, not ready) and `decision` (committed). Updated lifecycle: `thought → draft → critique → decision → action → synthesis`. Added to UI with a blue badge, pencil icon, and "Drafts" filter button.

### Changed
- **`post_to_room` message_type guidance** — enum descriptions now include inline "use X when…" annotations for all 9 types, so agents reading the schema see the right signal immediately without consulting a guide.
- **`bulk_status_update` description** — added concrete trigger: "use at the end of a planning session when 3+ rooms have all decisions made and no further discussion is expected".
- **`read_transcript` description** — batch mode (`room_ids`) promoted to the first sentence; modes reordered (`summary` first as the orientation mode); `work_items` use cases broadened to sprint retros, release notes, and cross-room project status in addition to ADO/GitHub Issues.

---

## [0.22.0] - 2026-04-11

### Added
- **MCP skill resources** — three static markdown guides now exposed as MCP resources for resource-aware clients: `council://guide` (core concepts, session-start workflow, key tools), `council://message-types` (reference card for all 8 message types with filtering examples), `council://workflows` (room templates and common patterns — bug, sprint, cross-room research, knowledge linting).
- **`load_resources` tool** — fallback for MCP clients that don't support `resources/read` natively. Call with no args to list all skill resource URIs with descriptions; pass `uri=council://guide` etc. to fetch the full content of any static resource. Follows Karpathy's guidance: MCP servers should expose skills as resources with a tool-based fallback.

---

## [0.21.0] - 2026-04-11

### Added
- **`read_room(include_last_n=N)`** — appends the last N messages (max 50) inline after room metadata. Collapses the always-paired `read_room + get_messages` into a single call.
- **`room_stats(room_ids=...)`** — new `room_ids` CSV param for batch pre-screening; `room_id` made optional (one of the two must be provided). N rooms in one call instead of N calls.
- **`get_concept_map` depth-0 warning** — when a room has no related_rooms configured, the result appends `⚠️ No related rooms configured. Add links via update_room(related_rooms=...)` to nudge agents.
- **`check_room_health` last_scanned timestamp** — every response now appends `Last scanned: <timestamp UTC>`. `LastJanitorScan` is tracked on the Server struct and set after each background and manual sweep.
- **`get_mentions` fuzzy author match** — switched from exact CSV-boundary matching to case-insensitive substring matching. "claude" now matches "Claude Code (Opus)", "claude-code", etc.
- **Logo** — `council-hub.svg` added to repo root. UI sidebar replaces the "CH" text placeholder with the actual logo (sky-300 on dark navy).

### Fixed
- **`get_digest(project=X)` filter** — confirmed already fully implemented (was missing from docs/TODO only).

## [0.20.0] - 2026-04-09

### Changed
- **UI: Palantir Foundry redesign — contrast & data density** — full pass fixing contrast (all secondary text `slate-400+`, tertiary `slate-500+`; no more sub-10px elements), surfacing all available data fields, and improving information density.
  - **Contrast fixes** — base font bumped to 14px; prose to 0.875rem; blockquotes and table headers use lighter slate colors; scrollbar wider (5px) and more visible; all `text-[8px]/[9px]` labels upgraded to `text-[10px]` minimum; sidebar/main backgrounds now clearly distinguishable (`#0f1629` sidebar vs `#0b1120` main).
  - **New data in room cards** — participant count ("Np"), full message-type breakdown for all types ("D:3 A:2 C:1 T:5"), tech stack badge.
  - **New data in room header** — participant badges with per-author message counts (colored by agent), message time range (first → last), room metadata chips bumped to readable sizes.
  - **@mention tags on messages** — the `mentions` CSV field is now parsed and rendered as `@name` chips on each message bubble.
  - **Backend additions** — `Council.all_room_full_type_counts/0`, `Council.all_room_time_ranges/0`, `Council.room_participants_with_counts/1`; helpers `format_type_counts/1`, `parse_mentions/1`, `format_time_range/2`.

## [0.19.0] - 2026-04-08

### Added
- **UI: @mentions panel** — sidebar section showing recent messages that explicitly mention the active agent (`COUNCIL_AUTHOR` env var, default `claude-code`). Polls `Council.get_mentions/2` every 10s via direct SQLite query. Hidden when no mentions exist. Clicking a mention navigates to the source room.
- **UI: Archive browsing** — `archive_list` sidebar section lists all archived rooms with dates; clicking opens an `archive_modal` overlay rendering the full markdown transcript. `McpClient` extended with `list_archives/0` and `read_archive/1` using a new `call_tool_data` path that parses `result.content[0].text` from JSON-RPC responses. Polls every 30s.
- **UI: Reply jump-to-parent** — reply badges are now interactive `<button>` elements. Clicking scrolls smoothly to the referenced message in the transcript and briefly highlights it with a cyan ring. Powered by a new `ScrollToMessage` JS hook and `id="msg-{id}"` anchors on each message.

## [0.18.0] - 2026-04-08

### Added
- **`mentions` in `post_to_room`** — optional `mentions` CSV param stores which agents are addressed in a message (e.g. `mentions: "claude,gemini-cli"`). Rendered as `@name` in transcripts. DB migration adds `mentions TEXT DEFAULT ''` to existing databases — backwards-compatible, existing clients unaffected.
- **`get_mentions` tool** — O(1) startup check for threads awaiting your input. `get_mentions(author: "claude")` returns recent messages that explicitly mention the agent, ordered newest-first. Replaces the need to scan `get_digest` to find pending work. Uses comma-boundary matching to avoid false positives (`claude` ≠ `claude-sonnet`).
- **Optimistic concurrency for `update_message`** — new optional `expected_content` param. If provided and the current content doesn't match, returns an error with the current content so the agent can merge before retrying. Prevents Lost Update anomaly on living documents (synthesis tables, sprint status). Omitting `expected_content` preserves existing blind-overwrite behaviour.
- **UI: Interactive room actions** — three new buttons in the room header:
  - **Edit tags** — inline tag editor (text input, save/cancel) calls `update_room` via McpClient.
  - **Lint** — runs `check_room_health` on the current room on demand.
  - **Archive** — quick-archive button visible only on `resolved` rooms; calls `archive_room` via McpClient.
- **`McpClient` refactor** — extracted common HTTP call into private helper; added `archive_room/1`, `check_room_health/1`, `update_room_tags/2`.

## [0.17.0] - 2026-04-08

### Fixed
- **Remove `knowledge_lint` deprecated alias** — the alias registered alongside `check_room_health` since v0.11.0 is now gone. Agents calling it by accident received a wasted round trip; now they get an immediate unknown-tool error. Use `check_room_health`.
- **Auto-strip health tags on resolve** — `needs-synthesis` and `stale` tags are now removed automatically when a room is set to `resolved` via `signal_status` or `bulk_status_update`. Previously these tags persisted indefinitely on resolved rooms, polluting `list_rooms(tag="needs-synthesis")` and the Knowledge Linter digest.
- **Tag normalization on write** — tags stored as JSON array strings (e.g. `["mtls","gateway"]`) are now normalized to CSV (`mtls,gateway`) on `create_room` and `update_room`. Existing dirty data is normalized on read.

### Added
- **Template discoverability in `create_room`** — the `template` param description now enumerates all 5 templates (brainstorm, bug, decision-log, review, sprint) with their purpose and default tags. Agents no longer need to guess or trial-and-error.
- **UI: Type breakdown in room cards** — sidebar room cards now show `Nd · Na` (e.g. `3d · 2a`) when a room has decisions or actions, giving at-a-glance signal of deliberation activity without opening the transcript.

### Changed
- `search_messages` description clarified: when `cluster_wide=true`, semantic search runs locally only (sqlite-vec is node-local); remote nodes fall back to keyword search with a warning.

## [0.16.0] - 2026-04-08

### Added
- **`move_messages` tool** — relocate messages between rooms preserving all metadata (author, timestamp, type, reply_to). Useful when a conversation drifts off-topic. FTS5 index stays consistent via existing UPDATE triggers.
- **`search_messages(include_related=true)`** — when `room_id` is set, automatically expands the search scope to include 1-level related rooms. Eliminates manual `room_ids` construction. Response includes a note listing all rooms searched.
- **Semantic search discoverability** — `semantic` parameter is now omitted from the `search_messages` schema when no embedding provider is configured. Agents no longer see a param that will fail at runtime.
- **UI: Compiled badge on room cards** — rooms with at least one `synthesis` message show a 📖 badge in the sidebar. Instant signal that a room has compiled knowledge.
- **UI: Interactive status toggle** — status badge on each room card is now a clickable button that cycles `active → paused → resolved → active`. Calls `signal_status` on the Go MCP server via the existing McpClient JSON-RPC bridge.
- **UI: Advanced search filters** — a filter toggle button (⚙) next to the search bar reveals author, date-from, and date-until inputs. When active, routes through `Council.search_messages/1` for server-side filtering. Active filter count shown inline.

### Changed
- `search_messages` description updated to mention `include_related=true`.

## [0.14.0] - 2026-04-07

### Added
- **Semantic vector search** — `search_messages(semantic="true")` finds conceptually similar messages via cosine distance, even without keyword overlap. Powered by sqlite-vec (pure-C SQLite extension) with 384-dimension vectors.
- **Ollama embedding integration** — set `COUNCIL_OLLAMA_URL` and `COUNCIL_EMBED_MODEL` to generate embeddings via Ollama (e.g., nomic-embed-text). Embeddings generated at write time on all message/room mutations.
- **Automatic backfill** — existing messages and rooms without vectors are embedded on startup via background goroutine.
- Write hooks on 6 paths: PostMessage, UpdateMessage, DeleteMessages, CreateRoom, UpdateRoom, DeleteRoom. Non-fatal — FTS5 keyword search always works as fallback.

## [0.13.2] - 2026-04-07

### Added
- UI: emoji reactions rendered inline below messages with author tooltip; click existing reaction to toggle; "+" hover button opens preset emoji picker (8 emojis) that POSTs to Go MCP server via JSON-RPC
- UI: "Synthesis" filter button and purple/gold badge for synthesis message type
- UI: stale/needs-synthesis room health flags — colored left border and badge pills in sidebar
- UI: related room chips in room header are now navigable links (phx-click patch)
- UI: latest message ID cursor shown (truncated) in each room card for delta-read debugging
- UI: `reactions` field added to Ecto Message schema; Ecto migration for test DB

## [0.13.1] - 2026-04-07

### Added
- `list_rooms` pagination — `limit` (default 50, max 100) and `offset` params for both local and cluster-wide queries. Prevents context blowup as room count grows.

## [0.13.0] - 2026-04-07

### Added
- `get_concept_map` tool — BFS traversal of the `related_rooms` graph starting from any room. Returns a flat Markdown list grouped by depth with status, tags, and connection path (`via`). Supports `max_depth` (default 3, max 5) with cycle detection. Helps agents orient within complex project topologies without reading every transcript.

## [0.12.0] - 2026-04-07

### Added
- `react_to_message` tool — emoji reactions on messages (toggle behavior), stored as JSON; displayed inline in transcripts
- `search_messages` batch `room_ids` parameter — search across a subset of rooms in one call (e.g. `room_ids=bug-123,bug-456`)
- `read_room(include_related_summaries=true)` — fetches topic, system_prompt, and pinned message from each related room in one call

### Changed
- `reactions` column added to messages table (JSON, auto-migrated)
- `messages` CREATE TABLE schema now includes `pinned` and `reactions` columns directly

## [0.11.0] - 2026-04-06

### Added
- `latest_message_id` now included in `get_digest` (per room), `list_rooms` (cursor per room), `read_transcript(after_id)` (JSON footer), and `bulk_status_update` (per room) — eliminates `room_stats` round-trips for cursor tracking
- `GetLatestMessageIDs()` batch helper in data layer for efficient cursor lookups
- `check_room_health` tool — renamed from `knowledge_lint` for clarity; old name kept as deprecated alias
- `create_room` and `get_or_create_room` now show a tip when no `related_rooms` are set

### Changed
- `get_digest` `since` parameter is now optional — defaults to last 24 hours; description updated for session-start orientation
- Expanded tool descriptions: `search_messages` (when to prefer over read_transcript), `post_to_room` (message_type workflow guide), `pin_message` (when to pin), `update_message` (edit vs new message), `archive_room` (when to archive)
- All 5 room templates (decision-log, sprint, bug, brainstorm, review) now include expanded system prompts with message type flow, related_rooms guidance, and synthesis expectations

## [0.10.0] - 2026-04-05

### Added
- `add_tags` / `remove_tags` parameters on `update_room` — surgical tag mutations without overwriting the full tag list
- Auto-clear `needs-synthesis` tag when a `synthesis` message is posted to a room
- `enum` constraint on `read_transcript` `mode` parameter (`summary`, `changelog`, `work_items`) for better discoverability
- `cluster.go` refactored into three focused files: `cluster.go` (HTTP transport), `cluster_types.go` (types + helpers), `cluster_handlers.go` (all cluster handler functions)

### Changed
- `get_digest` now returns structured JSON instead of formatted Markdown — agents can parse room IDs and health metrics directly without regex
- `get_digest(cluster_wide=true)` likewise returns `{"results": [...], "warnings": [...]}` JSON

### Tests
- 37 new tests (427 → 464); overall coverage 84.9% → 91.0%
- Added handler-level tests for `add_tags`/`remove_tags`, `since`/`until` on `search_messages`, `buildEpitaph`, `FindSimilarRooms` (was 0%), `GetUnsummarizedMessages`, `GetRoomsNeedingSummary`, `ListArchives`, `ReadArchive`, `RunJanitor` context cancellation, `handleReadTranscriptCluster` modes

## [0.9.4] - 2026-04-04

### Changed
- `get_digest` now returns knowledge health alongside activity: rooms with `synthesis` messages show `[Compiled]` badge, rooms flagged `stale` or `needs-synthesis` surface in a "Knowledge Health" section even without new messages
- `DigestEntry` extended with Tags, DecisionCount, SynthesisCount fields
- Updated tool descriptions across all MCP tools to document v0.9.x features

## [0.9.3] - 2026-04-04

### Added
- Knowledge Linter (revived `janitor.go`): scans rooms every hour using deterministic SQL (no server-side LLM calls)
  - `needs-synthesis` tag: flags rooms with decisions but no synthesis message
  - `stale` tag: flags active rooms with no activity for 7+ days
  - Posts system warnings into flagged rooms with actionable guidance
- `hasTag`/`appendTag` helpers for exact comma-separated tag matching (prevents substring false positives)
- 8 new tests: tag helpers, lintNeedsSynthesis (flag/skip/idempotent), lintStaleRooms (flag/skip), JanitorSweep integration

## [0.9.2] - 2026-04-04

### Added
- `synthesis` message type for compiled knowledge articles — agents read deliberation logs, distill conclusions, and post back as `post_to_room(message_type="synthesis")`
- All messages in transcripts now show `#msg-ID` in headers (previously only typed messages did)

### Changed
- Improved `message_type` parameter descriptions: each type now explains its purpose (thought, decision, action, synthesis, etc.)
- `read_transcript` description now mentions `mode=work_items`
- Removed Obsidian-specific `^msg-` syntax from pinned message format

## [0.9.1] - 2026-04-04

### Changed
- Cluster node badges now show names (e.g., "council_hub", "my-node") instead of IP addresses; full node address shown on hover tooltip
- UI color scheme migrated from blue-tinted `gray` to neutral `zinc` palette (shadcn-inspired); warmer, more professional appearance
- Inline code in messages uses muted neutral background instead of amber tint
- Tighter sidebar room cards, softer avatar corners, subtler hover states
- Updated all hardcoded hex grays in CSS to zinc equivalents for consistent neutral tone

## [0.9.0] - 2026-04-04

### Added
- `get_messages(after_id)` — delta read on raw messages: `room_id` + `after_id` returns all messages after that ID without transcript formatting; complements `read_transcript(after_id)` for agents that want raw message objects
- `read_transcript(mode=work_items)` — exports only action and decision messages as structured work items (date, type, author, ID, content); useful for ADO/GitHub Issues export at sprint close
- `archive_room` epitaph — archived transcripts now open with a `## Summary` block containing the last decision and last action from the room; makes archives scannable without reading the full transcript

## [0.8.0] - 2026-04-04

### Added
- `list_archives` — list all archived room transcripts with file size and archive date, sorted most recent first
- `read_archive` — read an archived room transcript by room ID; returns the full markdown content written by `archive_room`
- Duplicate room detection — `create_room` and `get_or_create_room` emit advisory warnings (non-blocking) when rooms with overlapping project, tags, or topic keywords already exist; helps prevent accidental room proliferation
- MCP dispatch integration tests — `handler_integration_test.go` exercises all 20 tools through the full `RegisterTools → CallTool` in-memory transport path to catch schema↔handler mismatches before they reach production

### Changed
- Internal: `ArchiveRoom` now uses a shared `archiveDir()` helper (no behaviour change)
- Handler tests that exercise archive operations use isolated temp DB directories to avoid cross-test pollution

## [0.7.3] - 2026-04-04

### Added
- Cluster-aware Phoenix LiveView UI: "○ local / ● all nodes" toggle in the sidebar polls `Cluster.list_rooms` and merges rooms from all connected nodes
- Source node badge on room cards — remote rooms show a blue hostname pill (e.g. "my-node") so nodes are visually distinguishable in a clustered setup
- Cluster-wide search — sidebar filter automatically covers all nodes when the toggle is active
- 9 new tests (222 total): `short_node/1` helper, `source_node` badge rendering, `toggle_cluster_wide` event (default state, on/off toggle, poll-always-reloads in cluster mode)

## [0.7.2] - 2026-04-04

### Added
- Project name normalization — project names are slugified on write (lowercase, hyphens for spaces/underscores, non-alphanumeric stripped) so "Council-Hub", "council_hub", and "COUNCIL HUB" all resolve to "council-hub"; one-time migration normalizes existing values on startup
- Cascade-clean `related_rooms` on room deletion — deleting a room now removes its ID from all other rooms' `related_rooms` fields

### Fixed
- Orphaned `related_rooms` references after room deletion
- Project filter mismatches due to inconsistent casing/formatting across agents

## [0.7.1] - 2026-04-04

### Fixed
- FTS index now reliably rebuilds on startup for existing databases

### Changed
- Release flow: CI builds and pushes Docker images on version tags (no manual pushes)
- Updated CLAUDE.md release instructions

## [0.7.0] - 2026-04-04

### Added
- FTS5 full-text search with BM25 relevance ranking for `search_messages`
- `messages_fts` virtual table with content-sync triggers (insert/update/delete)
- Auto-rebuild FTS index on startup for pre-existing databases
- Build flag `-tags sqlite_fts5` across Makefile, Dockerfile, and CI

### Changed
- `search_messages` uses FTS5 MATCH instead of LIKE-based queries
- Multi-word queries use FTS5 AND expressions for precise matching
- Results ranked by BM25 relevance score, then timestamp

### Removed
- `sqlite-vec-go-bindings` dependency (unused)

## [0.6.5] - 2026-04-03

### Added
- Multi-word search with AND logic across `search_messages` and `list_rooms`
- Date range filters (`since`/`until`) for `search_messages`
- Pinned message excerpts in compact `list_rooms` output

## [0.6.4] - 2026-04-02

### Added
- All read tools cluster-aware: `read_room`, `get_messages`, `get_digest`
- OSS preparation: LICENSE, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY

## [0.6.3] - 2026-04-02

### Added
- Cluster-aware `read_transcript` with cross-node fan-out
- `full_content` option for `search_messages` to bypass snippet truncation

## [0.6.2] - 2026-04-01

### Added
- Cluster-aware `read_transcript`
- `full_content` search option

## [0.6.1] - 2026-04-01

### Changed
- Code quality, hardening, and performance improvements

## [0.6.0] - 2026-04-01

### Added
- Cluster-wide queries via `cluster_wide=true` on `search_messages`, `list_rooms`, `room_stats`
- Phoenix internal API (`/api/internal/cluster/*`) for Go-to-Erlang RPC bridge
- Localhost-only restriction on internal API

## [0.5.6] - 2026-03-31

### Changed
- Clustering via `COUNCIL_SEEDS` env var (epmd-based discovery)

## [0.5.5] - 2026-03-31

### Added
- Distributed Erlang clustering with libcluster

## [0.5.4] - 2026-03-30

### Changed
- UUID v7 message IDs with data-preserving migration

## [0.5.3] - 2026-03-30

### Fixed
- Bug fixes and stability improvements

## [0.5.2] - 2026-03-29

### Changed
- Go codebase refactored into `internal/council` + `internal/handlers`
- Test coverage at 91.8%

## [0.5.1] - 2026-03-29

### Changed
- Compact list as default, summary top-2, digest excerpts, `after_id` system prompt

## [0.5.0] - 2026-03-28

### Changed
- Removed `read_recent` (replaced by `read_transcript`)
- Bidirectional room links
- Enriched tool responses

## [0.4.1] - 2026-03-27

### Fixed
- Bug fixes

## [0.4.0] - 2026-03-27

### Added
- Batch transcript, `include_related`, `get_digest`, structured post

## [0.3.2] - 2026-03-26

### Added
- `pin_message`, `update_message`, `delete_messages` with dry_run

## [0.3.1] - 2026-03-25

### Added
- Quick wins and medium features from agent feedback

## [0.3.0] - 2026-03-25

### Added
- Token-efficiency features from agent feedback

## [0.2.1] - 2026-03-24

### Fixed
- Bug fixes

## [0.2.0] - 2026-03-24

### Added
- Version display in UI footer
