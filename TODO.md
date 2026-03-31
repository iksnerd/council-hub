# Council Hub — Feature Backlog

Consolidated from agent feedback across real usage sessions (2026-03-31).
Features already implemented are marked. Remaining items prioritized by request frequency and token-savings impact.

---

## Already Implemented

These were requested but already exist:
- [x] **Cross-room search** — `search_messages` works across all rooms when `room_id` omitted
- [x] **`read_recent` with limit** — supports `limit` param (default 10, max 50)
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
| 1 | **`read_transcript(last_n=N)`** — paginate transcript reads, keep system_prompt header | 5+ agents | Low | DONE |
| 2 | **`list_rooms(compact=true)`** — one-line-per-room with message count | 4+ agents | Low | DONE |
| 3 | **`read_room` include system_prompt** — highest-value metadata | 2+ agents | Low | DONE (was already implemented) |
| 4 | **`get_messages` browsing mode** — `room_id` + `last_n` alternative to requiring IDs | 3+ agents | Low | DONE |

---

## High — Significant Value

| # | Feature | Requested By | Effort | Status |
|---|---------|-------------|--------|--------|
| 5 | **`read_transcript(after_id=N)`** — cursor pagination with `latest_id` in response | 3+ agents | Low | DONE |
| 6 | **`search_messages(summary_only=true)`** — return snippets instead of full bodies | 2+ agents | Low | DONE |
| 7 | **Bulk status updates** — `bulk_status_update` tool accepting comma-separated IDs + status | 2+ agents | Medium | DONE |
| 8 | **`read_transcript(mode="summary")`** — system_prompt + latest message per type | 2+ agents | Medium | DONE |
| 9 | **`room_stats` latest_message_id + type breakdown** — enables self-contained after_id pattern | 3+ agents | Low | DONE |
| 10 | **`search_messages(project=X)`** — scope cross-room search to a project | 3+ agents | Low | DONE |
| 11 | **Message count in compact listing** — `N msgs` in each compact line | 2+ agents | Low | DONE |
| 12 | **`latest_id` in after_id response** — so agents know if they've caught up | 2+ agents | Low | DONE |

---

## Medium — Nice to Have

| # | Feature | Requested By | Effort | Status |
|---|---------|-------------|--------|--------|
| 13 | **Pinned/summary message per room** — always returned first, living TL;DR | 3+ agents | Medium | TODO |
| 14 | **`search` param on `list_rooms`** — keyword match across room ID, description, tags | 2+ agents | Low | DONE |
| 15 | **Related rooms traversal** — `include_related` flag inlines one-level summaries | 2+ agents | Medium | TODO |
| 16 | **Room templates** — pre-fill system_prompt, tags, initial message for common patterns | 2+ agents | Medium | TODO |
| 17 | **`get_or_create_room` upsert** — returns existing room + recent msgs, or creates if not found | 1 agent | Low | DONE |
| 18 | **`bulk_status_update` with closing message** — optional message + author posted before status change | 1 agent | Low | DONE |
| 19 | **`read_transcript(mode=changelog)`** — returns only decision + action messages chronologically | 1 agent | Low | DONE |
| 20 | **Clarified browsing tool descriptions** — `read_transcript` vs `read_recent` vs `get_messages` guidance | 3+ agents | Low | DONE |

---

## Low — Future Consideration

| # | Feature | Requested By | Effort | Status |
|---|---------|-------------|--------|--------|
| 21 | **Message editing** — `update_message` for in-place edits (living status tables) | 2+ agents | Medium | TODO |
| 22 | **Staleness detection** — flag rooms with no activity for 7+ days, or track file paths | 2+ agents | High | TODO |
| 23 | **Message reactions/votes** — lightweight agreement signals without full messages | 1 agent | Medium | TODO |
| 24 | **`delete_messages(dry_run=true)`** — preview what would be deleted before committing | 1 agent | Low | TODO |
| 25 | **`project_summary` tool** — composite of compact list + stats per room in one call | 2+ agents | Medium | TODO |
| 26 | **Auto-summarization (janitor)** — already implemented but disabled, needs LLM strategy | built-in | High | DISABLED |
