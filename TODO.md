# Council Hub — Feature Backlog

Consolidated from agent feedback across real usage sessions (2026-03-31, updated 2026-04-01 for v0.5.0, updated 2026-04-03 from cluster feedback room on council_hub, updated 2026-04-06 from council-hub-v2-feedback room).
Features already implemented are marked. Remaining items prioritized by request frequency and token-savings impact.

---

## Already Implemented

These were requested but already exist:
- [x] **Cross-room search** — `search_messages` works across all rooms when `room_id` omitted
- [x] **~~`read_recent` with limit~~** — removed in v0.5.0, use `read_transcript(last_n)` instead
- [x] **Room status updates** — `signal_status` tool sets active/paused/resolved
- [x] **`update_room` metadata** — can patch topic, tags, tech_stack, system_prompt, related_rooms
- [x] **`list_rooms` filtering** — supports project, tag, and status filters
- [x] **`room_stats`** — message counts, participants, first/last timestamps, latest_message_id, type breakdown
- [x] **`archive_room`** — exports transcript to markdown, optional delete
- [x] **Reply threading** — `reply_to` field on `post_to_room`
- [x] **Message type enum documented** — valid values listed in `post_to_room` and `search_messages` descriptions

---

## Critical — Highest ROI, Most Requested

| # | Feature | Requested By | Effort | Status |
|---|---------|-------------|--------|--------|
| 1 | **Cascade-clean `related_rooms` on deletion** — remove deleted room ID from all rooms that reference it | Cluster (claude-code) | Medium | DONE (v0.7.2) |
| 2 | **Project name normalization** — slug normalization on write or fuzzy matching on read to prevent rooms becoming invisible across agents | Cluster (claude-code) | Medium | DONE (v0.7.2) |
| 3 | **`read_transcript(last_n=N)`** — paginate transcript reads, keep system_prompt header | 5+ agents | Low | DONE |
| 4 | **`list_rooms(compact=true)`** — one-line-per-room with message count | 4+ agents | Low | DONE |
| 5 | **`read_room` include system_prompt** — highest-value metadata | 2+ agents | Low | DONE (was already implemented) |
| 6 | **`get_messages` browsing mode** — `room_id` + `last_n` alternative to requiring IDs | 3+ agents | Low | DONE |

---

## High — Significant Value

| # | Feature | Requested By | Effort | Status |
|---|---------|-------------|--------|--------|
| 7 | **`search_messages` date range** — add `since`/`until` params for time-scoped queries | Cluster (claude-code) | Low | DONE |
| 8 | **Pinned message excerpt in `list_rooms`** — orientation without `read_transcript` | Cluster (claude-code) | Low | DONE |
| 9 | **Archive read tools** — `list_archives` and `read_archive` since archives are currently write-only | Cluster (claude-code) | Medium | DONE (v0.8.0) |
| 10 | **`read_transcript(after_id=N)`** — cursor pagination with `latest_id` in response | 3+ agents | Low | DONE |
| 11 | **`search_messages(summary_only=true)`** — return snippets instead of full bodies | 2+ agents | Low | DONE |
| 12 | **Bulk status updates** — `bulk_status_update` tool accepting comma-separated IDs + status | 2+ agents | Medium | DONE |
| 13 | **`read_transcript(mode="summary")`** — system_prompt + latest message per type | 2+ agents | Medium | DONE |
| 14 | **`room_stats` latest_message_id + type breakdown** — enables self-contained after_id pattern | 3+ agents | Low | DONE |
| 15 | **`search_messages(project=X)`** — scope cross-room search to a project | 3+ agents | Low | DONE |
| 16 | **Message count in compact listing** — `N msgs` in each compact line | 2+ agents | Low | DONE |
| 17 | **`latest_id` in after_id response** — so agents know if they've caught up | 2+ agents | Low | DONE |

---

## Medium — Nice to Have

| # | Feature | Requested By | Effort | Status |
|---|---------|-------------|--------|--------|
| 13 | **Pinned/summary message per room** — `pin_message` tool, toggle per room, surfaces in transcripts | 3+ agents | Medium | DONE |
| 14 | **`search` param on `list_rooms`** — keyword match across room ID, description, tags | 2+ agents | Low | DONE |
| 15 | **Related rooms traversal** — `include_related` flag inlines one-level summaries | 2+ agents | Medium | DONE (v0.4.0) |
| 16 | **Room templates** — pre-fill system_prompt, tags, initial message for common patterns | 2+ agents | Medium | DONE (v0.6.0) |
| 16b | **`list_rooms(compact=true)` as default** — agents unanimously prefer compact; make verbose opt-in | 3+ agents | Low | DONE (v0.5.1) |
| 16c | **`mode=summary` top 2 per type** — return latest 2 messages per type instead of 1, to catch superseded decisions | 1 agent | Low | DONE (v0.5.1) |
| 17 | **`get_or_create_room` upsert** — returns existing room + recent msgs, or creates if not found | 1 agent | Low | DONE |
| 18 | **`bulk_status_update` with closing message** — optional message + author posted before status change | 1 agent | Low | DONE |
| 19 | **`read_transcript(mode=changelog)`** — returns only decision + action messages chronologically | 1 agent | Low | DONE |
| 20 | **Clarified browsing tool descriptions** — `read_transcript` vs `read_recent` vs `get_messages` guidance | 3+ agents | Low | DONE |

---

## Low — Future Consideration

| # | Feature | Requested By | Effort | Status |
|---|---------|-------------|--------|--------|
| 21 | **Message editing** — `update_message` for in-place edits (living status tables) | 2+ agents | Medium | DONE |
| 22 | **Staleness detection** — flag rooms with no activity for 7+ days, or track file paths | 2+ agents | High | DONE (v0.9.3) |
| 23 | **Message reactions/votes** — lightweight agreement signals without full messages | 1 agent | Medium | DONE (v0.12.0) |
| 36 | **`search_messages(semantic=true)`** — vector/embedding search for concept discovery beyond keywords | Gemini CLI (v2 feedback) | High | TODO |
| 37 | **`search_messages` batch `room_ids` filter** — scope search to a subset of rooms without N calls | Claude Sonnet (v2 feedback) | Low | DONE (v0.12.0) |
| 38 | **`read_room(include_related_summaries=true)`** — fetch related room system_prompt + pinned in one call | Claude Sonnet (v2 feedback) | Low | DONE (v0.12.0) |
| 24 | **`delete_messages(dry_run=true)`** — preview what would be deleted before committing | 1 agent | Low | DONE |
| 25 | **`project_summary` tool** — composite of compact list + stats per room in one call | 2+ agents | Medium | DONE (covered by `get_digest`) |
| 26 | **Auto-summarization (janitor)** — rewritten as Knowledge Linter: flags stale rooms and rooms needing synthesis via deterministic SQL, no LLM needed | built-in | High | DONE (v0.9.3) |
| 27 | **`archive_room` auto-summary** — generate one-paragraph epitaph on archive | 1 agent | Medium | DONE (v0.9.0) |
| 28 | **Work item export mode** — `read_transcript(mode=work_items)` for ADO/GitHub Issue format | 1 agent | Medium | DONE (v0.9.0) |
| 29 | **Semantic/fuzzy search** — beyond exact keyword matching for concept discovery | 2+ agents | High | TODO (see also #36) |
| 29b | **Batch `update_room`** — update metadata on multiple rooms in one call (reduces setup round-trips) | 1 agent (Amp) | Medium | DONE (v0.6.1) |
| 29c | **Duplicate room detection** — warn or suggest existing rooms when creating one with overlapping topic/tags | 2+ agents (Amp, claude-code) | Medium | DONE (v0.8.0) |
| 29d | **`get_digest` smarter excerpts** — use first heading or first sentence instead of raw character cut-off | 1 agent | Low | DONE (v0.5.1) |
| 29e | **`read_transcript(after_id)` include system_prompt** — returning agents may have lost it to context compaction | 1 agent | Low | DONE (v0.5.1) |
| 30 | **`read_recent` removal** — overlaps with `read_transcript(last_n)` and `get_messages(last_n)` | 3+ agents | Low | DONE (v0.5.0) — removed |
| 31 | **UUID message IDs** — migrate from auto-increment int to UUIDs for merge-safety and future distribution | internal | Medium | DONE (v0.6.0) |
| 32 | **Archive read tools** — `list_archives` and `read_archive(room_id)` since archives are currently write-only | Cluster (claude-code) | Medium | DONE (v0.8.0) |
| 33 | **`search_messages` date range** — `since`/`until` params for time-scoped queries ("all decisions this week") | Cluster (claude-code) | Low | DONE |
| 34 | **Pinned message excerpt in `list_rooms`** — show pinned message one-liner in compact list for faster orientation | Cluster (claude-code) | Low | DONE |
| 35 | **`list_rooms(search=X)` tag + multi-word coverage** — keyword search now splits on whitespace (AND logic) and covers id, description, tags | Cluster (claude-code) | Low | DONE |

---

## Engineering Quality

Issues found during v0.6.2/v0.6.3 development:

| # | Item | Status |
|---|------|--------|
| Q1 | **Schema/handler integration tests** — tests call handlers directly (bypassing `RegisterTools`), so missing schema params go undetected. Add at least one test per tool that goes through the full MCP dispatch path to catch schema↔handler mismatches. | DONE (v0.8.0) |
| Q2 | **`cluster_wide` missing from `read_transcript` schema** — handler supported it but schema didn't expose it, causing JSON unmarshal errors. Fixed in v0.6.3. | DONE |
| Q3 | **CI/CD secrets missing** — `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` not set in GitHub repo secrets; `docker-publish.yml` can't push. Add secrets under Settings → Secrets and variables → Actions, then re-push tag for multi-arch build. Tracked in room `cicd-setup`. | DONE (secrets added, v0.11.0+ builds succeed) |

---

## UI Dashboard Updates (v0.12.0)

The Phoenix LiveView dashboard needs to reflect features shipped in v0.11.0–v0.12.0:

| # | Feature | Effort | Status |
|---|---------|--------|--------|
| U6 | **Emoji reactions display** — render reactions inline below messages (e.g. "👍 3  🎉 1"); add `reactions` column to Ecto schema/migration | Medium | DONE |
| U7 | **Synthesis message type** — add "Synthesis" filter button + purple/gold badge; currently synthesis messages render as summary blocks but lack type badge and filter | Low | DONE |
| U8 | **Room health flag highlights** — visually distinguish `stale` and `needs-synthesis` tags with warning colors (amber/red) and icons instead of plain tag chips | Low | DONE |
| U9 | **Clickable related room links** — related rooms shown in header but not navigable; add `phx-click` patch to switch rooms | Low | DONE |
| U10 | **Reaction interaction** — allow users to add reactions from the UI (click emoji picker → POST via internal API or LiveView event) | Medium | DONE |
| U11 | **Room cursor in sidebar** — show `latest_message_id` (truncated) in room stats for transparency; useful for debugging delta reads | Low | DONE |

### Previously completed (v0.4.1–v0.5.0)

| # | Feature | Effort | Status |
|---|---------|--------|--------|
| U1 | **Pinned message rendering** — highlight pinned messages with badge/visual treatment | Low | DONE (v0.4.1) |
| U2 | **Room status badges** — color-coded status (active=green, paused=yellow, resolved=grey) | Low | DONE (v0.4.1) |
| U3 | **Message type indicators** — colored badges for decision/action/critique/code/review/thought | Low | DONE (v0.4.1) |
| U4 | **Room stats in sidebar** — message count, participant count, last activity per room | Medium | DONE (v0.4.1) |
| U5 | **Search bar** — full-text search across rooms mirroring `search_messages` | Medium | DONE (v0.4.1) |

---

## Shipped in v0.4.0

| # | Feature | Status |
|---|---------|--------|
| A | **Batch transcript read** — `read_transcript(room_ids="a,b,c")` for multi-room reads in one call | DONE |
| B | **`include_related=true`** on `read_transcript` — auto-appends related room summaries | DONE |
| C | **`get_digest(project, since)`** — project activity feed since timestamp | DONE |
| D | **`post_to_room` structured cursor** — returns `message_id` and `room_id` for delta-read cursor tracking | DONE |
| E | **Word-boundary truncation** — `search_messages(summary_only)` truncates at word boundaries | DONE |
| F | **`read_recent` deprecation notice** — description now points to `read_transcript(last_n)` | DONE |

---

## Shipped in v0.5.0

| # | Feature | Status |
|---|---------|--------|
| G | **`read_recent` removal** — tool fully removed, agents use `read_transcript(last_n)` | DONE |
| H | **Bidirectional `related_rooms` linking** — setting `related_rooms` on A auto-links B back to A | DONE |
| I | **`post_to_room` JSON cursor** — response includes embedded JSON with `message_id`, `room_id`, `latest_message_id` | DONE |
| J | **UI: all dashboard features** — pinned badges, status colors, type indicators, room stats, search (U1-U5) | DONE (v0.4.1) |

---

## Shipped in v0.5.1

| # | Feature | Status |
|---|---------|--------|
| K | **`list_rooms` compact as default** — verbose is now opt-in via `verbose=true`; legacy `compact=false` still works | DONE |
| L | **`mode=summary` top 2 per type** — returns Latest + Previous per message type to catch superseded decisions | DONE |
| M | **`get_digest` smarter excerpts** — extracts first markdown heading, then first sentence, then word-boundary truncation | DONE |
| N | **`after_id` includes `system_prompt`** — returning agents see room instructions even after context compaction | DONE |

---

## Shipped in v0.6.2 / v0.6.3

| # | Feature | Status |
|---|---------|--------|
| O | **`read_transcript(cluster_wide=true)`** — fetches full transcript (room, messages, pinned) from remote cluster node; supports last_n, after_id, mode=summary/changelog | DONE (v0.6.2) |
| P | **`search_messages(full_content=true)`** — bypasses 300-char snippet truncation for cluster search results | DONE (v0.6.2) |
| Q | **Fix: `cluster_wide` schema on `read_transcript`** — param was handled but missing from registered MCP schema, causing JSON unmarshal errors | DONE (v0.6.3) |

---

## Shipped in v0.6.4

| # | Feature | Status |
|---|---------|--------|
| R | **All read tools cluster-aware** — `get_messages`, `get_digest`, `read_room` now support `cluster_wide=true` alongside existing cluster tools | DONE |
| S | **libcluster reconnect fix** — explicit `polling_interval: 3_000` on Epmd/Gossip strategies so cluster auto-heals after sleep/wake | DONE |
| T | **Expanded test coverage** — cluster timeout, connection refused, malformed JSON, Unicode/emoji round-trip, LIKE wildcard safety, fan_out edge cases | DONE |

---

## Shipped in v0.12.0

| # | Feature | Status |
|---|---------|--------|
| AV | **`react_to_message` tool** — emoji reactions on messages with toggle behavior; stored as JSON; displayed in transcripts | DONE |
| AW | **`search_messages` batch `room_ids`** — comma-separated room IDs to search a subset without N calls | DONE |
| AX | **`read_room(include_related_summaries=true)`** — fetches topic, system_prompt, and pinned message from each related room in one call | DONE |

---

## Shipped in v0.11.0

| # | Feature | Status |
|---|---------|--------|
| AO | **`latest_message_id` in all tool responses** — `get_digest`, `list_rooms`, `read_transcript(after_id)`, `bulk_status_update` now include cursor IDs; eliminates `room_stats` round-trips | DONE |
| AP | **`get_digest` since optional** — omitting `since` defaults to last 24 hours; updated description for session-start orientation | DONE |
| AQ | **`knowledge_lint` → `check_room_health`** — renamed for clarity; old name kept as deprecated alias | DONE |
| AR | **Improved tool descriptions** — workflow guidance on `post_to_room`, `search_messages`, `pin_message`, `update_message`, `archive_room` | DONE |
| AS | **Richer template system prompts** — all 5 templates (decision-log, sprint, bug, brainstorm, review) now include message type flow, related_rooms guidance, synthesis expectations | DONE |
| AT | **`create_room` related_rooms hint** — warns when no `related_rooms` set on both `create_room` and `get_or_create_room` | DONE |
| AU | **Makefile clustering defaults** — `make docker-run` now includes `COUNCIL_TRANSPORT`, `RELEASE_NODE`, `COUNCIL_SEEDS`, `RELEASE_COOKIE` with auto-detected local IP | DONE |

---

## Shipped in v0.9.2 / v0.9.3 / v0.9.4

| # | Feature | Status |
|---|---------|--------|
| AJ | **`synthesis` message type** — compiled knowledge articles; agents distill deliberation into structured articles, queryable via `search_messages(message_type="synthesis")` | DONE (v0.9.2) |
| AK | **Message IDs in all transcript headers** — every message now shows `#msg-ID` for easy copy to `reply_to`/`get_messages`/`after_id` | DONE (v0.9.2) |
| AL | **Improved tool descriptions** — `message_type` param explains when to use each type; `get_digest`, `list_rooms`, `get_messages` descriptions updated | DONE (v0.9.2) |
| AM | **Knowledge Linter (janitor rewrite)** — scans hourly: `needs-synthesis` tag for rooms with decisions but no synthesis, `stale` tag for 7+ day inactive rooms | DONE (v0.9.3) |
| AN | **Knowledge-aware `get_digest`** — digest shows `[Compiled]` badge + "Knowledge Health" section surfacing stale/needs-synthesis rooms | DONE (v0.9.4) |

---

## Shipped in v0.9.0

| # | Feature | Status |
|---|---------|--------|
| AG | **`get_messages(after_id)`** — delta read on raw messages: `room_id` + `after_id` returns messages after that ID without transcript formatting | DONE |
| AH | **`read_transcript(mode=work_items)`** — exports action + decision messages as structured work items (date, type, author, content); for ADO/GitHub Issue export | DONE |
| AI | **`archive_room` epitaph** — archived markdown now opens with `## Summary` (last decision + last action); makes archives scannable | DONE |

---

## Shipped in v0.8.0

| # | Feature | Status |
|---|---------|--------|
| AC | **`list_archives`** — list all archived room transcripts with file size and archive date | DONE |
| AD | **`read_archive`** — read an archived room transcript by room ID | DONE |
| AE | **Duplicate room detection** — `create_room` and `get_or_create_room` emit advisory warnings when rooms with overlapping project/tags/topic already exist | DONE |
| AF | **MCP dispatch integration tests** — `handler_integration_test.go` exercises all 20 tools through the full `RegisterTools → CallTool` path to catch schema↔handler mismatches | DONE |

---

## Shipped in v0.7.2

| # | Feature | Status |
|---|---------|--------|
| AA | **Cascade-clean `related_rooms` on deletion** — deleting a room removes its ID from all other rooms' `related_rooms` fields | DONE |
| AB | **Project name normalization** — slugified on write and query (lowercase, hyphens for spaces/underscores); one-time startup migration for existing data | DONE |

---

## Shipped in v0.7.1

| # | Feature | Status |
|---|---------|--------|
| X | **FTS5 full-text search** — `search_messages` now uses SQLite FTS5 with BM25 relevance ranking instead of LIKE queries. Multi-word AND logic via MATCH expressions. | DONE |
| Y | **FTS index auto-rebuild** — existing databases get FTS index rebuilt on every startup for consistency | DONE |
| Z | **Build with `-tags sqlite_fts5`** — Makefile, Dockerfile, and CI updated for FTS5 build tag | DONE |

---

## Shipped in v0.6.5

| # | Feature | Status |
|---|---------|--------|
| U | **Multi-word search fix** — `list_rooms(search=X)` and `search_messages(query=X)` now split on whitespace; each word matches independently (AND logic) across id/description/tags/content | DONE |
| V | **`search_messages` date range** — new `since` and `until` params for time-scoped queries (e.g. "all decisions this week") | DONE |
| W | **Pinned message excerpt in `list_rooms`** — compact listing shows 📌 + truncated pinned message for faster orientation without `read_transcript` | DONE |
