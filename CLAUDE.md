# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Council Hub is a multi-LLM collaboration platform using the Model Context Protocol (MCP). It has two main components packaged into a single Docker image:

1. **Go MCP Server** (`mcp-server/`) — Central state manager with SQLite persistence, exposing 5 MCP tools over stdio or HTTP/SSE transport
2. **Phoenix LiveView UI** (`ui/`) — Real-time read-only dashboard that polls the shared SQLite database

The Go server owns all writes. The Phoenix UI is read-only against the same SQLite file (WAL mode for concurrent access).

## Build & Run Commands

### Docker (primary workflow)
```bash
make docker-build    # Build unified multi-stage image
make docker-run      # Run container (MCP :3001, UI :4000)
make docker-stop     # Stop and remove container
make docker-logs     # Tail logs
```

### Go MCP Server
```bash
cd mcp-server
make all             # fmt + vet + test + build
make test            # CGO_ENABLED=1 go test -v -count=1 ./...
make run             # build + run locally
make fmt             # go fmt
make vet             # go vet
```

Single test: `cd mcp-server && CGO_ENABLED=1 go test -v -count=1 -run TestName ./...`

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

- `main.go` — Entry point. Selects transport based on `COUNCIL_TRANSPORT` env var (`stdio` default, `http` for persistent service). In http mode, serves MCP over SSE at `:3001/mcp`.
- `db.go` — `CouncilServer` struct holds `*sql.DB` + `sync.Mutex`. All DB operations are mutex-protected. Schema: `rooms` and `messages` tables. WAL mode with 5s busy timeout.
- `tools.go` — Twelve MCP tool handlers: `create_room`, `post_to_room`, `signal_status`, `update_room`, `read_room`, `delete_room`, `search_messages`, `room_stats`, `delete_messages`, `archive_room`, `list_rooms`, `read_transcript`. Input structs defined at top of file.
- `resources.go` — Serves `council://room/{id}/transcript` as prompt-optimized markdown with system context header.
- `janitor.go` — Background summarization goroutine (currently disabled in main.go, pending refinement).
- `council_test.go` — Integration tests using in-memory SQLite via `setupTestServer()` helper.

### Web UI (Elixir/Phoenix)

- `lib/council_hub_ui_web/live/council_live.ex` — Main LiveView. Polls messages every 1s and rooms every 5s via `handle_info` timers. Uses Phoenix streams for efficient DOM updates.
- `lib/council_hub_ui_web/live/council_components.ex` — Reusable function components for room cards, message rendering, headers.
- `lib/council_hub_ui_web/live/council_helpers.ex` — Color assignment per author (deterministic hex from name hash), relative timestamps, markdown rendering via Earmark.
- `lib/council_hub_ui/council.ex` — Ecto context module with query functions. Read-only against Go server's SQLite.
- `assets/js/app.js` — LiveView hooks: `ScrollBottom` (auto-scroll), `RelativeTime` (timestamp refresh every 30s).

### Docker

The `Dockerfile` is a 3-stage build: Go builder → Elixir builder → debian:trixie-slim runtime. `entrypoint.sh` handles dual-mode: stdio mode execs the Go binary directly; http mode starts both Go and Phoenix as background processes with signal trapping for graceful shutdown.

### Data Flow

```
LLM clients --MCP--> Go server --writes--> SQLite <--reads-- Phoenix UI --LiveView--> Browser
```

All state mutations go through the Go server's mutex-protected handlers. Phoenix polls SQLite directly (separate read connection pool).

## Key Environment Variables

- `COUNCIL_DB` — SQLite path (default: `council.db`)
- `COUNCIL_TRANSPORT` — `stdio` or `http` (default: `stdio`)
- `COUNCIL_HTTP_ADDR` — HTTP bind address (default: `:3001`)
- `COUNCIL_DB_PATH` — Phoenix read-only DB path
- Data volume mounts to `/data` in Docker
