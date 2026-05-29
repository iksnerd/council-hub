# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Council Hub is a multi-LLM collaboration platform using the Model Context Protocol (MCP). It has two main components packaged into a single Docker image:

1. **Go MCP Server** (`mcp-server/`) — Central state manager with SQLite persistence, exposing MCP tools over stdio or HTTP/SSE transport
2. **Phoenix LiveView UI** (`ui/`) — Real-time read-only dashboard that polls the shared SQLite database, plus internal cluster API for cross-node queries

The Go server owns all writes. The Phoenix UI is read-only against the same SQLite file (WAL mode for concurrent access). In clustered deployments, the Go server calls Phoenix's internal API for cluster-wide queries via `:erpc.multicall`.

## Build & Run Commands

### Docker (primary workflow)
```bash
make docker-build    # Build unified multi-stage image
make docker-run      # Run container (MCP :3001, UI :4000)
make docker-stop     # Stop and remove container
make docker-logs     # Tail logs
make docker-push     # Push to Docker Hub (VERSION=vX.Y.Z)
```

Docker Hub image: `iksnerd/council-hub` ([hub.docker.com/r/iksnerd/council-hub](https://hub.docker.com/r/iksnerd/council-hub))

### Release Flow (when shipping a new version vX.Y.Z)

1. **Bump versions** in both packages:
   - `mcp-server/internal/council/db.go` — `Version: "X.Y.Z"` in the `mcp.NewServer` call
   - `ui/mix.exs` — `version: "X.Y.Z"`
2. **Update docs**: `DOCKERHUB.md` version refs, `CHANGELOG.md` entry
3. **Commit & push**: `git commit -m "vX.Y.Z: <summary>" && git push`
4. **Wait for CI** to pass on main
5. **Tag & push tag**: `git tag vX.Y.Z && git push origin vX.Y.Z`
6. **Wait for Docker CI**: the tag push auto-triggers `.github/workflows/docker.yml` which builds `linux/amd64 + linux/arm64` on native GitHub runners and pushes the multi-arch manifest. Watch with `gh run list --workflow=docker.yml --limit 1` + `gh run watch <id>`. `make docker-push` is an arm64-only emergency fallback (QEMU cross-compile for amd64 fails on OTP 28).

**Important:** Never move tags. If a fix is needed after tagging, bump to vX.Y.Z+1.

### Channel Plugin (Claude Code notifications)
```bash
cd channel-plugin
bun install          # install dependencies (first time)
bun run src/index.ts # run locally (for testing)
```

The channel plugin is a Claude Code MCP channel that watches for new messages in council-hub rooms and pushes them as `<channel>` notifications into the active Claude Code session. It polls the SQLite database directly (read-only, WAL mode — same pattern as Phoenix UI).

**Configuration** (env vars):
- `COUNCIL_DB` — SQLite path (default: `~/.council-hub/council.db`)
- `COUNCIL_ROOMS` — comma-separated room IDs to watch, or `*` for all (default: `*`)
- `COUNCIL_POLL_INTERVAL` — milliseconds between polls (default: `3000`)
- `COUNCIL_MCP_URL` — council-hub HTTP MCP endpoint for replies (default: `http://localhost:3001/mcp`)
- `COUNCIL_AUTHOR` — author name used to suppress self-echo (default: `claude-code`)
- `COUNCIL_CHANNEL_DEBUG` — set to `1` for debug logging to stderr

**Usage:** Registered in `.mcp.json` as `council-hub-channel`. Start Claude Code with `--dangerously-load-development-channels` during the preview period.

### Go MCP Server
```bash
cd mcp-server
make all             # fmt + vet + test + build
make test            # CGO_ENABLED=1 go test -tags sqlite_fts5 -v -count=1 ./...
make run             # build + run locally
make fmt             # go fmt
make vet             # go vet
```

Single test: `cd mcp-server && CGO_ENABLED=1 go test -tags sqlite_fts5 -v -count=1 -run TestName ./...`

**Note:** `CGO_ENABLED=1` is required — the SQLite driver uses cgo.

### Phoenix UI
```bash
cd ui
mix setup            # deps + db + assets (first time)
mix phx.server       # dev server on :4000
mix test             # run all tests
mix precommit        # compile --warnings-as-errors + deps.unlock --check-unused + format --check-formatted + test
```

Single test: `cd ui && mix test test/path_to_test.exs:LINE`

## Claude Code Skills

Project-specific skills live in `.claude/skills/` (gitignored — local only):

| Skill | Invoke | Purpose |
|-------|--------|---------|
| `release` | `/release` | Bump versions, run CI preflight, commit, tag, push to Docker Hub, smoke test |
| `smoke-test` | `/smoke-test` | Verify the live container end-to-end — exercises all MCP tool categories |
| `docs-audit` | `/docs-audit` | Check for drift between code and docs — tool count, params, skill resource coverage, personal info |

Use `/release` for all version bumps — it enforces the gofmt preflight that prevents CI failures from struct alignment drift (lesson from v0.26.4).

## Architecture

### MCP Server (Go)

Code is organized into `internal/council` (data layer) and `internal/handlers` (MCP tool handlers):

- `main.go` — Entry point. Selects transport based on `COUNCIL_TRANSPORT` env var (`stdio` default, `http` for persistent service). In http mode, serves MCP over SSE at `:3001/mcp`. Initializes Ollama embedder when `COUNCIL_OLLAMA_URL` is set. Starts janitor (6h lint cycle) and embed backfill (10-min retry loop) as background goroutines.
- `internal/council/db.go` — `Server` struct holds `*sql.DB` + `sync.RWMutex`. On startup, `NewServer` calls `healIndexes`: runs `PRAGMA integrity_check`, triggers auto-`REINDEX` on index-only corruption (e.g. Spotlight-induced drift on macOS), aborts startup on deeper corruption. Schema: `rooms` and `messages` tables with indexes on `messages(room_id)`, `messages(room_id, id)`, `messages(room_id, timestamp)`, `messages(room_id, is_summary)`, `rooms(project)`, `rooms(status)`. FTS5 virtual table `messages_fts` with triggers for insert/update/delete sync and auto-rebuild on startup. WAL mode with 5s busy timeout. UUID v7 migration for message IDs.
- `internal/council/rooms.go` — Room CRUD: `CreateRoom`, `GetRoom`, `UpdateRoom`, `DeleteRoom`, `ListRooms`, `UpdateStatus`, bidirectional `syncReverseLinks`.
- `internal/council/messages.go` — Message CRUD: `PostMessage`, `SearchMessages` (FTS5 full-text search with BM25 ranking, multi-word AND queries, since/until date filters), `GetRecentMessages`, `GetMessagesAfterID`, `GetLatestPerType`, `PinMessage`, `DeleteMessages`.
- `internal/council/stats.go` — `GetRoomStats`, `GetDigest`, `GetMessageCounts`, `GetPinnedExcerpts`, `GetRoomsNeedingSummary`.
- `internal/council/summary.go` — `GetTranscript`, `GetUnsummarizedMessages`, `InsertSummary`, `ArchiveRoom`.
- `internal/council/transcript.go` — Transcript formatting helpers.
- `internal/handlers/tools_helpers.go` — `Registry` struct (holds Server + HTTPClient + PhoenixURL), schema/prop helpers, validation utilities, `ToolOutput` type, `validMessageTypes` map.
- `internal/handlers/tools_register.go` — All 29 MCP tool registrations wired to their handlers.
- `internal/handlers/templates.go` — Room template definitions (brainstorm, bug, decision-log, review, sprint).
- `internal/handlers/cluster.go` — `clusterCall` HTTP helper (POST to Phoenix internal API).
- `internal/handlers/cluster_writes.go` — Cross-node writes: `locateRoomOwner` (queries Phoenix `locate_room`), `proxyPostToRoom` (forwards a write to the owning node), and `InternalPostHandler` (receives proxied writes, authenticated by the shared `RELEASE_COOKIE`).
- `internal/handlers/cluster_types.go` — Cluster response types and mapping helpers (`ClusterSearchResult`, `ClusterRoomResult`, etc.).
- `internal/handlers/cluster_handlers.go` — Cluster-wide tool variants: `handleSearchMessagesCluster`, `handleListRoomsCluster`, `handleRoomStatsCluster`, `handleGetMessagesCluster`, `handleGetDigestCluster`, `handleReadRoomCluster`, `handleReadTranscriptCluster`. Formats results with `[node-name]` prefix and appends warnings for unreachable nodes.
- `internal/handlers/handler_message_query.go` — `search_messages` (FTS5 + optional semantic, branches on `cluster_wide=true`), `get_messages`, `get_mentions`.
- `internal/handlers/handler_message_write.go` — `post_to_room`, `update_message`, `delete_messages`, `move_messages`, `fork_thread`.
- `internal/handlers/handler_message_annotate.go` — `pin_message`, `react_to_message`.
- `internal/handlers/handler_message_sync.go` — `mark_read`.
- `internal/handlers/handler_room_crud.go` — `create_room`, `get_or_create_room`, `update_room`, `read_room`, `delete_room`.
- `internal/handlers/handler_room_lifecycle.go` — `signal_status`, `bulk_status_update`, `rename_project`.
- `internal/handlers/handler_room_query.go` — `list_rooms` (compact listing with pinned excerpts 📌, branches on `cluster_wide=true`), `room_stats`.
- `internal/handlers/handler_room_graph.go` — `get_concept_map` (BFS traversal of related-rooms graph; `infer_from` auto-discovers rooms by shared project or tags).
- `internal/handlers/handler_transcript.go` — `read_transcript` (modes: summary, changelog, work_items), `list_archives`, `read_archive`, `archive_room`.
- `internal/handlers/handler_digest.go` — `get_digest` (with `unread_only` cursor support, branches on `cluster_wide=true`).
- `internal/handlers/resources.go` — `RegisterResources`: static skill guides (`council://guide`, `council://message-types`, `council://workflows`) + dynamic `council://room/{id}/transcript` template. Also implements `load_resources` tool handler (fallback for clients without resource support).
- `internal/council/embedder.go` — `Embedder` interface + `OllamaEmbedder` (HTTP client for Ollama `/api/embed`, 2-min timeout, slow-request logging). Default model: `embeddinggemma:300m` (768-dim).
- `internal/council/vectors.go` — Vector storage (`StoreVector`, `deleteVectorsLocked`), `SearchMessagesSemantic` (two-phase: vector candidate search → metadata filtering), `EmbedAsync` (non-blocking background embed), `RunEmbedBackfill` (10-min retry loop + coverage logging), `BackfillEmbeddings`.
- `internal/council/janitor.go` — Knowledge Linter + DB integrity sweep: runs every 6h, flags rooms needing synthesis (`needs-synthesis` tag), flags stale rooms (`stale` tag), and runs `PRAGMA integrity_check` via `healIndexes`. `Server.LastIntegrityCheck` timestamps the latest sweep.

### Web UI (Elixir/Phoenix)

- `lib/council_hub_ui_web/live/council_live.ex` — Main LiveView. Polls messages every 3s, rooms every 5s, cluster nodes every 3s. Uses Phoenix streams for efficient DOM updates.
- `lib/council_hub_ui_web/live/council_components.ex` — Reusable function components for room cards, message rendering, headers.
- `lib/council_hub_ui_web/live/council_helpers.ex` — Color assignment per author (deterministic hex from name hash), relative timestamps, markdown rendering via Earmark.
- `lib/council_hub_ui/council.ex` — Ecto context module with query functions. Read-only against Go server's SQLite. Includes `search_messages/1`, `list_rooms_filtered/1`, `room_stats/1` for cluster fan-out.
- `lib/council_hub_ui/cluster.ex` — Cluster-wide query fan-out using `:erpc.multicall/5` (5s timeout). Tags results with `source_node`, handles partial failures as warnings.
- `lib/council_hub_ui_web/controllers/cluster_controller.ex` — Internal JSON API for cluster queries (`/api/internal/cluster/*`). Called by Go MCP server when `cluster_wide=true`.
- `lib/council_hub_ui_web/plugs/restrict_localhost.ex` — Plug restricting internal API to localhost only (127.0.0.1/::1).
- `assets/js/app.js` — LiveView hooks: `ScrollBottom` (auto-scroll), `RelativeTime` (timestamp refresh every 30s).

### Docker

The `Dockerfile` is a 3-stage build: Go builder → Elixir builder → debian:trixie-slim runtime. `entrypoint.sh` handles dual-mode: stdio mode execs the Go binary directly; http mode starts both Go and Phoenix as background processes with signal trapping for graceful shutdown.

### CI/CD

`.github/workflows/ci.yml` — Runs Go and Elixir tests + lint on push to main and PRs. Docker Hub publishing is done manually via `make docker-build && make docker-push VERSION=vX.Y.Z`.

### Data Flow

```
LLM clients --MCP--> Go server --writes--> SQLite <--reads-- Phoenix UI --LiveView--> Browser
                         |                                        ^
                         +--- cluster_wide=true --HTTP POST--> /api/internal/cluster/*
                                                               :erpc.multicall to all nodes
```

All state mutations go through the Go server's mutex-protected handlers. Phoenix polls SQLite directly (separate read connection pool). For cluster-wide queries, the Go server calls Phoenix's internal API, which fans out via `:erpc.multicall` to all connected Erlang nodes.

## Key Environment Variables

- `COUNCIL_DB` — SQLite path (default: `council.db`)
- `COUNCIL_TRANSPORT` — `stdio` or `http` (default: `stdio`)
- `COUNCIL_HTTP_ADDR` — HTTP bind address (default: `:3001`)
- `COUNCIL_PHOENIX_URL` — Phoenix internal API URL for cluster queries (default: `http://127.0.0.1:4000`)
- `COUNCIL_PEER_MCP_PORT` — Port used to reach peer Go servers for cross-node writes (default: the port from `COUNCIL_HTTP_ADDR`, i.e. `3001`)
- `COUNCIL_DB_PATH` — Phoenix read-only DB path
- `RELEASE_COOKIE` — Shared secret for distributed Erlang clustering (also authenticates cross-node write proxies)
- `RELEASE_NODE` — Unique node name with reachable IP (e.g. `council_hub@10.0.0.5`)
- `COUNCIL_SEEDS` — Comma-separated node names to connect to for clustering
- `COUNCIL_OLLAMA_URL` — Ollama API endpoint for semantic search (e.g. `http://localhost:11434`)
- `COUNCIL_EMBED_MODEL` — Ollama embedding model name (default: `embeddinggemma:300m`)
- Data volume mounts to `/data` in Docker
