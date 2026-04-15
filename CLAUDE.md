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
6. **Build & push to Docker Hub**: `make docker-build && make docker-push VERSION=vX.Y.Z`

**Important:** Never move tags. If a fix is needed after tagging, bump to vX.Y.Z+1.

### Channel Plugin (Claude Code notifications)
```bash
cd channel-plugin
bun install          # install dependencies (first time)
bun run src/index.ts # run locally (for testing)
```

The channel plugin is a Claude Code MCP channel that watches for new messages in council-hub rooms and pushes them as `<channel>` notifications into the active Claude Code session. It polls the SQLite database directly (read-only, WAL mode — same pattern as Phoenix UI).

**Configuration** (env vars):
- `COUNCIL_DB` — SQLite path (default: `~/Documents/council-hub/council.db`)
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

## Architecture

### MCP Server (Go)

Code is organized into `internal/council` (data layer) and `internal/handlers` (MCP tool handlers):

- `main.go` — Entry point. Selects transport based on `COUNCIL_TRANSPORT` env var (`stdio` default, `http` for persistent service). In http mode, serves MCP over SSE at `:3001/mcp`. Initializes HTTP client for cluster queries via `COUNCIL_PHOENIX_URL`.
- `internal/council/db.go` — `Server` struct holds `*sql.DB` + `sync.RWMutex`. Schema: `rooms` and `messages` tables with indexes on `messages(room_id)`, `messages(room_id, id)`, `messages(room_id, timestamp)`, `messages(room_id, is_summary)`, `rooms(project)`, `rooms(status)`. FTS5 virtual table `messages_fts` with triggers for insert/update/delete sync and auto-rebuild on startup. WAL mode with 5s busy timeout. UUID v7 migration for message IDs.
- `internal/council/rooms.go` — Room CRUD: `CreateRoom`, `GetRoom`, `UpdateRoom`, `DeleteRoom`, `ListRooms`, `UpdateStatus`, bidirectional `syncReverseLinks`.
- `internal/council/messages.go` — Message CRUD: `PostMessage`, `SearchMessages` (FTS5 full-text search with BM25 ranking, multi-word AND queries, since/until date filters), `GetRecentMessages`, `GetMessagesAfterID`, `GetLatestPerType`, `PinMessage`, `DeleteMessages`.
- `internal/council/stats.go` — `GetRoomStats`, `GetDigest`, `GetMessageCounts`, `GetPinnedExcerpts`, `GetRoomsNeedingSummary`.
- `internal/council/summary.go` — `GetTranscript`, `GetUnsummarizedMessages`, `InsertSummary`, `ArchiveRoom`.
- `internal/council/transcript.go` — Transcript formatting helpers.
- `internal/handlers/tools.go` — `Registry` struct (holds Server + HTTPClient + PhoenixURL), MCP tool registration, schema/prop helpers.
- `internal/handlers/cluster.go` — Cluster-wide query support: `clusterCall` (HTTP POST to Phoenix internal API), `handleSearchMessagesCluster`, `handleListRoomsCluster`, `handleRoomStatsCluster`. Formats results with `[node-name]` prefix and appends warnings for unreachable nodes.
- `internal/handlers/handler_message.go` — Message tool handlers. `search_messages` branches on `cluster_wide=true`. Supports `since`/`until` date range filters and multi-word query tokenization (AND logic).
- `internal/handlers/handler_room.go` — Room tool handlers. `list_rooms` and `room_stats` branch on `cluster_wide=true`. Compact listing includes pinned message excerpts (📌). Multi-word search splits on whitespace (AND logic across id/description/tags).
- `internal/handlers/handler_transcript.go` — `read_transcript` with modes (summary, changelog), `archive_room`.
- `internal/handlers/resources.go` — Serves `council://room/{id}/transcript` as prompt-optimized markdown.
- `internal/council/janitor.go` — Background summarization goroutine (currently disabled, pending refinement).

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
- `COUNCIL_DB_PATH` — Phoenix read-only DB path
- `RELEASE_COOKIE` — Shared secret for distributed Erlang clustering
- `RELEASE_NODE` — Unique node name with reachable IP (e.g. `council_hub@10.0.0.5`)
- `COUNCIL_SEEDS` — Comma-separated node names to connect to for clustering
- `COUNCIL_OLLAMA_URL` — Ollama API endpoint for semantic search (e.g. `http://localhost:11434`)
- `COUNCIL_EMBED_MODEL` — Ollama embedding model name (default: `embeddinggemma:300m`)
- Data volume mounts to `/data` in Docker
