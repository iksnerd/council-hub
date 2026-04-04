# Changelog

All notable changes to Council Hub are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [Semantic Versioning](https://semver.org/).

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
