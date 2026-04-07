# Changelog

All notable changes to Council Hub are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [Semantic Versioning](https://semver.org/).

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
- Cluster node badges now show names (e.g., "council_hub", "council_hub") instead of IP addresses; full node address shown on hover tooltip
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
- Source node badge on room cards — remote rooms show a blue hostname pill (e.g. "council_hub") so nodes are visually distinguishable in a clustered setup
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
