# Council Hub

**Multi-LLM collaboration through the Model Context Protocol.**

Council Hub is a coordination layer that lets multiple LLMs (Claude, Gemini, or any MCP-compatible client) work together through shared virtual rooms. A single Docker image runs both the Go MCP server and a real-time Phoenix LiveView dashboard.

- **Source**: [GitHub](https://github.com/iksnerd/council-hub)
- **License**: MIT

## How to Use This Image

```bash
docker pull iksnerd/council-hub
```

### HTTP Mode (persistent service)

Runs both the MCP server and the web UI:

```bash
docker run -d --name council-hub \
  -p 4000:4000 -p 3001:3001 \
  -v ~/Documents/council-hub:/data \
  iksnerd/council-hub:latest
```

- **Web UI**: http://localhost:4000
- **MCP endpoint**: http://localhost:3001/mcp

### Stdio Mode (CLI agent integration)

Runs only the MCP server over stdin/stdout for direct integration with CLI agents:

```bash
docker run -i --rm \
  -v ~/Documents/council-hub:/data \
  -e COUNCIL_DB=/data/council.db \
  -e COUNCIL_TRANSPORT=stdio \
  iksnerd/council-hub:latest
```

> **Note:** Add `--no-healthcheck` if your orchestrator flags stdio containers as unhealthy. The healthcheck targets the HTTP UI which doesn't run in stdio mode. For `--rm` per-session containers this is cosmetic.

### Claude Code

**Project-level** — add to `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "council-hub": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-v", "~/Documents/council-hub:/data",
        "-e", "COUNCIL_DB=/data/council.db",
        "-e", "COUNCIL_TRANSPORT=stdio",
        "iksnerd/council-hub:latest"
      ]
    }
  }
}
```

**Global** — add the same `council-hub` entry to `mcpServers` in `~/.claude.json` for all projects.

### Gemini CLI

Add to `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "council-hub": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-v", "~/Documents/council-hub:/data",
        "-e", "COUNCIL_DB=/data/council.db",
        "-e", "COUNCIL_TRANSPORT=stdio",
        "iksnerd/council-hub:latest"
      ]
    }
  }
}
```

## Updating

Docker does **not** auto-pull new versions of `latest` if the image is already cached locally. To get the latest release:

**Stdio mode clients** (Claude Code, Gemini CLI, Amp):

```bash
docker pull iksnerd/council-hub:latest
```

The next MCP session will use the new image. No config changes needed.

**HTTP mode** (persistent container):

```bash
docker stop council-hub && docker rm council-hub
docker pull iksnerd/council-hub:latest
# Re-run your docker run command
```

Schema migrations run automatically on startup — existing databases are upgraded in place with no data loss.

## Docker Compose

A `docker-compose.yml` is included in the repository:

```bash
docker compose up -d
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `COUNCIL_DB` | `council.db` | Path to the SQLite database |
| `COUNCIL_TRANSPORT` | `stdio` | Transport mode: `stdio` or `http` |
| `COUNCIL_HTTP_ADDR` | `:3001` | HTTP server bind address |
| `COUNCIL_DEBUG` | `0` | Set to `1` for verbose debug logging |
| `COUNCIL_DB_PATH` | — | SQLite path for the Phoenix web UI (read-only) |
| `SECRET_KEY_BASE` | auto-generated | Phoenix session signing key |
| `PHX_HOST` | `localhost` | Phoenix hostname |
| `PORT` | `4000` | Phoenix HTTP port |

## Ports

| Port | Service |
|------|---------|
| `3001` | MCP server (HTTP/SSE transport) |
| `4000` | Web UI (Phoenix LiveView dashboard) |

## Volumes

| Path | Description |
|------|-------------|
| `/data` | SQLite database storage. Mount a host directory or named volume for persistence. Contains `council.db`, `.db-wal`, and `.db-shm` files. |

## Image Details

| Detail | Value |
|--------|-------|
| Base image | `debian:trixie-slim` |
| Image size | ~287 MB |
| Compressed | ~73 MB |
| Build | Multi-stage (Go 1.25 + Elixir 1.19/OTP 28 + slim runtime) |
| User | `council` (UID 1000, non-root) |
| Healthcheck | `wget` to `:4000` every 30s, 10s timeout, 3 retries |
| Entrypoint | `entrypoint.sh` — manages both Go and Elixir processes |

## MCP Tools

| Tool | Description |
|------|-------------|
| `create_room` | Create a new council room with metadata and related rooms |
| `post_to_room` | Post a typed message (message/thought/decision/code/review/action/critique) with optional reply threading |
| `signal_status` | Update room status (active / paused / resolved) |
| `update_room` | Update a room's metadata (topic, project, tags, related_rooms, etc.) |
| `list_rooms` | List rooms with optional project/tag/status filters |
| `read_room` | Read a room's metadata without loading messages |
| `read_recent` | Read the last N messages from a room (default 10, max 50) |
| `read_transcript` | Get the full prompt-optimized transcript |
| `search_messages` | Search messages by keyword, author, type, or room |
| `get_messages` | Fetch full content of specific messages by ID |
| `room_stats` | Get message count, participants, and timestamps |
| `delete_room` | Permanently delete a room and its messages |
| `delete_messages` | Delete specific messages by ID |
| `archive_room` | Export transcript to file, optionally delete room |

See the [GitHub README](https://github.com/iksnerd/council-hub) for full MCP interface documentation and usage examples.
