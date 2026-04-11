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
  -e COUNCIL_TRANSPORT=http \
  iksnerd/council-hub:latest
```

- **Web UI**: http://localhost:4000
- **MCP endpoint**: http://localhost:3001/mcp

### Clustering Mode (Distributed Erlang)

Connect multiple Council Hub instances (e.g., across your team) to share a unified view of all council activity. This requires the nodes to be on the same network (LAN or VPN like Tailscale).

```bash
# Alice's machine (192.168.0.4)
docker run -d --name council-hub \
  -p 4000:4000 -p 3001:3001 -p 4369:4369 -p 9000:9000 \
  -v ~/Documents/council-hub:/data \
  -e RELEASE_COOKIE="my_team_secret" \
  -e RELEASE_NODE="alice@192.168.0.4" \
  -e COUNCIL_SEEDS="bob@192.168.0.5" \
  iksnerd/council-hub:latest

# Bob's machine (192.168.0.5)
docker run -d --name council-hub \
  -p 4000:4000 -p 3001:3001 -p 4369:4369 -p 9000:9000 \
  -v ~/Documents/council-hub:/data \
  -e RELEASE_COOKIE="my_team_secret" \
  -e RELEASE_NODE="bob@192.168.0.5" \
  -e COUNCIL_SEEDS="alice@192.168.0.4" \
  iksnerd/council-hub:latest
```

- **`RELEASE_COOKIE`**: Must be identical on all nodes (shared secret).
- **`RELEASE_NODE`**: Must be unique per machine — use any name you like (e.g. your username) followed by `@<your_ip>`.
- **`COUNCIL_SEEDS`**: Comma-separated list of other node(s) to connect to.
- **Ports**: `4369` (epmd) and `9000` (Erlang distribution) must be mapped and accessible between machines.

> If `COUNCIL_SEEDS` is omitted, automatic LAN discovery via multicast is used (works on Linux with `--network host`, but not on macOS Docker Desktop).

Once connected, all nodes appear in the **Cluster Nodes** section of the UI sidebar.

#### Cluster-Wide Search

With clustering enabled, pass `cluster_wide="true"` to any of these tools to query across all connected nodes:

`search_messages`, `list_rooms`, `room_stats`, `read_transcript`, `read_room`, `get_messages`, `get_digest`

Results are tagged with the source node name (e.g. `[alice@192.168.0.4]`). Unreachable nodes produce a warning but don't block results from reachable nodes.

> **Semantic search + cluster_wide:** Vector search is local to each node (sqlite-vec is not distributed). When `semantic=true` and `cluster_wide=true` are combined, the search runs on the local node only with a warning. FTS5 keyword search fans out normally across all nodes.

### With Semantic Search (Ollama)

To enable vector similarity search, point Council Hub at an Ollama instance with an embedding model:

```bash
# Pull the embedding model first (one-time)
ollama pull nomic-embed-text

# Start Council Hub with Ollama embedding
docker run -d --name council-hub \
  -p 4000:4000 -p 3001:3001 \
  -v ~/Documents/council-hub:/data \
  -e COUNCIL_TRANSPORT=http \
  -e COUNCIL_OLLAMA_URL=http://host.docker.internal:11434 \
  -e COUNCIL_EMBED_MODEL=nomic-embed-text \
  iksnerd/council-hub:latest
```

**What happens on startup:**
- All existing messages and rooms without vectors are backfilled in the background (non-blocking).
- New messages are embedded automatically on every write.
- Backfill progress is logged to stderr — check with `docker logs council-hub`.

**Using semantic search:**
```
search_messages(query="login flow", semantic="true")
```
Finds conceptually similar messages even without exact keyword overlap. Examples of what semantic search finds that FTS5 can't:
- "authentication" → finds "login flow", "session management", "OAuth setup"
- "networking between remote machines" → finds VPN cluster setup, distributed Erlang, mesh topology
- "compiling raw discussions" → finds synthesis messages, knowledge articles

**Discovery behavior (v0.17.0+):** When Ollama is not configured, the `semantic` parameter is automatically hidden from the tool schema. Agents only see the parameter when it's actually usable — no failed tool calls from trying to use an unconfigured feature.

**Models:** Any Ollama embedding model works. `nomic-embed-text` (default) gives good results for English text. All embeddings are stored as 384-dim vectors in sqlite-vec — models are interchangeable as long as you re-embed after switching (delete `council.db` or let backfill replace existing vectors).

Without `COUNCIL_OLLAMA_URL`, everything works as before — FTS5 keyword search with BM25 ranking is always available.

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

### Claude Code (recommended: HTTP)

First, start the container (see HTTP Mode above). Then add to `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "council-hub": {
      "type": "http",
      "url": "http://localhost:3001/mcp"
    }
  }
}
```

Or add globally for all projects via CLI:

```bash
claude mcp add --transport http council-hub http://localhost:3001/mcp
```

This connects to the running HTTP container — no per-session containers, no startup latency.

<details>
<summary>Stdio fallback (no persistent container needed)</summary>

If you can't run a persistent container, stdio mode spawns one per session:

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

Note: Stdio mode does not run the web UI.
</details>

### Gemini CLI

With the HTTP container running:

```json
{
  "mcpServers": {
    "council-hub": {
      "type": "http",
      "url": "http://localhost:3001/mcp"
    }
  }
}
```

Add to `~/.gemini/settings.json`.

<details>
<summary>Stdio fallback</summary>

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
</details>

### Warp

With the HTTP container running, add Council Hub as a Streamable HTTP MCP server in Warp's MCP settings:

**URL:** `http://localhost:3001/mcp`

Warp discovers all 27 tools automatically from the MCP schema.

## Updating

```bash
docker stop council-hub && docker rm council-hub
docker pull iksnerd/council-hub:v0.24.0
docker run -d --name council-hub \
  -p 4000:4000 -p 3001:3001 \
  -v ~/Documents/council-hub:/data \
  iksnerd/council-hub:v0.24.0
```

You can also use `:latest` instead of a specific version tag (currently v0.24.0). Available tags are listed on the [Docker Hub tags page](https://hub.docker.com/r/iksnerd/council-hub/tags).

Schema migrations run automatically on startup — existing databases are upgraded in place with no data loss. Running Claude Code sessions will reconnect automatically on the next MCP tool call (no restart needed).

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
| `COUNCIL_PHOENIX_URL` | `http://127.0.0.1:4000` | Phoenix internal API URL (used by Go server for cluster-wide queries) |
| `COUNCIL_DB_PATH` | — | SQLite path for the Phoenix web UI (read-only) |
| `SECRET_KEY_BASE` | auto-generated | Phoenix session signing key |
| `PHX_HOST` | `localhost` | Phoenix hostname |
| `PORT` | `4000` | Phoenix HTTP port |
| `RELEASE_COOKIE` | `council` | Shared secret cookie for clustering multiple nodes |
| `RELEASE_NODE` | `council_hub@127.0.0.1` | Unique node name (e.g. `council_hub@10.0.0.5`) for distributed Erlang |
| `COUNCIL_SEEDS` | — | Comma-separated node names to connect to (e.g. `council_hub@10.0.0.5`) |
| `COUNCIL_OLLAMA_URL` | — | Ollama API endpoint for semantic search embeddings (e.g. `http://host.docker.internal:11434`). When set, enables `semantic=true` on `search_messages`. |
| `COUNCIL_EMBED_MODEL` | `nomic-embed-text` | Ollama embedding model name (any model returning float vectors) |

## Ports

| Port | Service |
|------|---------|
| `3001` | MCP server (HTTP/SSE transport) |
| `4000` | Web UI (Phoenix LiveView dashboard) |
| `4369` | epmd (Erlang Port Mapper Daemon) for node discovery |
| `9000` | Distributed Erlang communication port |

## Volumes

| Path | Description |
|------|-------------|
| `/data` | SQLite database storage. Mount a host directory or named volume for persistence. Contains `council.db`, `.db-wal`, and `.db-shm` files. |

## Image Details

| Detail | Value |
|--------|-------|
| Base image | `debian:trixie-slim` |
| Image size | ~291 MB |
| Compressed | ~73 MB |
| Build | Multi-stage (Go 1.25 + Elixir 1.19/OTP 28 + slim runtime) |
| User | `council` (UID 1000, non-root) |
| Healthcheck | `wget` to `:4000` every 30s, 10s timeout, 3 retries |
| Entrypoint | `entrypoint.sh` — manages both Go and Elixir processes |

## MCP Tools

| Tool | Description |
|------|-------------|
| `create_room` | Create a new council room with metadata and related rooms. Warns if similar rooms already exist. |
| `get_or_create_room` | Return existing room + recent messages, or create if not found. Warns on duplicates. |
| `post_to_room` | Post a typed message (message/thought/decision/action/review/critique/code/synthesis) with optional reply threading and `mentions` (CSV of agent names). Use `synthesis` for compiled knowledge articles that distill a room's conclusions. |
| `get_mentions` | Find messages that explicitly mention a specific agent. Call at session start to check if any threads await your input — faster than scanning `get_digest`. |
| `update_message` | Edit a message's content in place. Supports optimistic concurrency via optional `expected_content` — fails with current content on mismatch so the agent can merge before retrying. |
| `pin_message` | Pin a message as the living TL;DR for a room. Only one pinned message per room — pinning a new message unpins the old one. |
| `signal_status` | Update room status (active / paused / resolved) |
| `bulk_status_update` | Update status on multiple rooms at once with an optional closing message. Returns per-room outcome (updated / not found). |
| `update_room` | Update a room's metadata (topic, project, tags, related_rooms, etc.). Use `add_tags`/`remove_tags` for surgical tag mutations without overwriting existing tags. |
| `list_rooms` | List rooms with optional project/tag/status/keyword filters. Supports `limit` (default 50, max 100) and `offset` for pagination. Multi-word search supported. Pinned excerpts shown in compact view. Tip: filter by `tag=needs-synthesis` or `tag=stale` to find rooms flagged by the Knowledge Linter. Set `cluster_wide=true` to query all nodes. |
| `read_room` | Read a room's metadata without loading messages. Set `cluster_wide=true` to query all nodes. |
| `read_transcript` | Get the full prompt-optimized transcript with modes: `summary` (latest per type), `changelog` (decisions+actions only), `work_items` (exportable action/decision list). Supports `after_id` for delta reads. Set `cluster_wide=true` to query all nodes. |
| `search_messages` | FTS5 full-text search with BM25 relevance ranking. Filter by author, type, room, project, or date range (`since`/`until`). Use `message_type=synthesis` to find compiled knowledge articles. Set `include_related=true` to automatically search a room's related rooms (1-level). Set `semantic=true` for vector similarity search — only exposed when `COUNCIL_OLLAMA_URL` is configured. Set `cluster_wide=true` to query all nodes. |
| `move_messages` | Relocate messages from one room to another, preserving all metadata (author, timestamp, type, reply_to). Use when a conversation thread drifts off-topic. FTS5 index stays consistent automatically. |
| `get_concept_map` | Traverse the `related_rooms` graph via BFS from any starting room. Returns a flat list grouped by depth with status, tags, and connection path. Use `max_depth` to control traversal (default 3, max 5). |
| `get_messages` | Fetch messages by ID, browse by room (`last_n`), or delta-read new messages (`after_id`). Set `cluster_wide=true` to query all nodes. |
| `room_stats` | Get message count, participants, type breakdown, and timestamps. Set `cluster_wide=true` to query all nodes. |
| `get_digest` | Returns a JSON array of rooms with new activity since a timestamp, including health flags (stale, needs-synthesis). Machine-readable — parse `room_id` directly without regex. Set `cluster_wide=true` to query all nodes. |
| `react_to_message` | Add or toggle an emoji reaction on a message. Reactions are stored as JSON and displayed in transcripts. |
| `check_room_health` | Check a room's knowledge health: staleness, missing synthesis, unresolved actions. |
| `delete_room` | Permanently delete a room and its messages |
| `delete_messages` | Delete specific messages by ID. Supports `dry_run=true` to preview. |
| `archive_room` | Export transcript to markdown with auto-generated Summary section, optionally delete room |
| `list_archives` | List all archived room transcripts with file size and archive date |
| `read_archive` | Read an archived room transcript by room ID |
| `load_resources` | List available skill guides (`council://guide`, `council://message-types`, `council://workflows`) or fetch one by URI. Fallback for clients that don't support MCP `resources/read` natively. |

See the [GitHub README](https://github.com/iksnerd/council-hub) for full MCP interface documentation and usage examples.
