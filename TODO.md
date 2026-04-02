# Council Hub — Feature Backlog

Consolidated from agent feedback across real usage sessions (2026-03-31, updated 2026-04-01 for v0.5.0, updated 2026-04-03 from cluster feedback room on council_hub).
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
| 1 | **Cascade-clean `related_rooms` on deletion** — remove deleted room ID from all rooms that reference it | Cluster (claude-code) | Medium | TODO |
| 2 | **Project name normalization** — slug normalization on write or fuzzy matching on read to prevent rooms becoming invisible across agents | Cluster (claude-code) | Medium | TODO |
| 3 | **`read_transcript(last_n=N)`** — paginate transcript reads, keep system_prompt header | 5+ agents | Low | DONE |
| 4 | **`list_rooms(compact=true)`** — one-line-per-room with message count | 4+ agents | Low | DONE |
| 5 | **`read_room` include system_prompt** — highest-value metadata | 2+ agents | Low | DONE (was already implemented) |
| 6 | **`get_messages` browsing mode** — `room_id` + `last_n` alternative to requiring IDs | 3+ agents | Low | DONE |

---

## High — Significant Value

| # | Feature | Requested By | Effort | Status |
|---|---------|-------------|--------|--------|
| 7 | **`search_messages` date range** — add `since`/`until` params for time-scoped queries | Cluster (claude-code) | Low | TODO |
| 8 | **Pinned message excerpt in `list_rooms`** — orientation without `read_transcript` | Cluster (claude-code) | Low | TODO |
| 9 | **Archive read tools** — `list_archives` and `read_archive` since archives are currently write-only | Cluster (claude-code) | Medium | TODO |
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
| 22 | **Staleness detection** — flag rooms with no activity for 7+ days, or track file paths | 2+ agents | High | TODO |
| 23 | **Message reactions/votes** — lightweight agreement signals without full messages | 1 agent | Medium | TODO |
| 24 | **`delete_messages(dry_run=true)`** — preview what would be deleted before committing | 1 agent | Low | DONE |
| 25 | **`project_summary` tool** — composite of compact list + stats per room in one call | 2+ agents | Medium | DONE (covered by `get_digest`) |
| 26 | **Auto-summarization (janitor)** — already implemented but disabled, needs LLM strategy | built-in | High | DISABLED |
| 27 | **`archive_room` auto-summary** — generate one-paragraph epitaph on archive | 1 agent | Medium | TODO |
| 28 | **Work item export mode** — `read_transcript(mode=work_items)` for ADO/GitHub Issue format | 1 agent | Medium | TODO |
| 29 | **Semantic/fuzzy search** — beyond exact keyword matching for concept discovery | 2+ agents | High | TODO |
| 29b | **Batch `update_room`** — update metadata on multiple rooms in one call (reduces setup round-trips) | 1 agent (Amp) | Medium | DONE (v0.6.1) |
| 29c | **Duplicate room detection** — warn or suggest existing rooms when creating one with overlapping topic/tags | 2+ agents (Amp, claude-code) | Medium | TODO |
| 29d | **`get_digest` smarter excerpts** — use first heading or first sentence instead of raw character cut-off | 1 agent | Low | DONE (v0.5.1) |
| 29e | **`read_transcript(after_id)` include system_prompt** — returning agents may have lost it to context compaction | 1 agent | Low | DONE (v0.5.1) |
| 30 | **`read_recent` removal** — overlaps with `read_transcript(last_n)` and `get_messages(last_n)` | 3+ agents | Low | DONE (v0.5.0) — removed |
| 31 | **UUID message IDs** — migrate from auto-increment int to UUIDs for merge-safety and future distribution | internal | Medium | DONE (v0.6.0) |
| 32 | **Archive read tools** — `list_archives` and `read_archive(room_id)` since archives are currently write-only | Cluster (claude-code) | Medium | TODO |
| 33 | **`search_messages` date range** — `since`/`until` params for time-scoped queries ("all decisions this week") | Cluster (claude-code) | Low | TODO |
| 34 | **Pinned message excerpt in `list_rooms`** — show pinned message one-liner in compact list for faster orientation | Cluster (claude-code) | Low | TODO |
| 35 | **`list_rooms(search=X)` tag coverage** — keyword search currently misses tag fields; confirm and fix scope | Cluster (claude-code) | Low | TODO |

---

## Engineering Quality

Issues found during v0.6.2/v0.6.3 development:

| # | Item | Status |
|---|------|--------|
| Q1 | **Schema/handler integration tests** — tests call handlers directly (bypassing `RegisterTools`), so missing schema params go undetected. Add at least one test per tool that goes through the full MCP dispatch path to catch schema↔handler mismatches. | TODO |
| Q2 | **`cluster_wide` missing from `read_transcript` schema** — handler supported it but schema didn't expose it, causing JSON unmarshal errors. Fixed in v0.6.3. | DONE |

---

## UI Dashboard Updates (v0.5.0)

The Phoenix LiveView dashboard needs to reflect features shipped in v0.3.x–v0.4.0:

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
