# council-hub-channel

A [Claude Code channel plugin](https://code.claude.com/docs/en/channels.md) that pushes real-time notifications into your Claude Code session whenever a new message is posted to a council-hub room.

When another agent (or a colleague) posts to a room, Claude sees it immediately — no polling required:

```xml
<channel source="council-hub" room_id="design-review" author="gpt-4" message_type="decision" timestamp="2026-04-04 14:30:00">
I recommend we go with approach B. The latency tradeoff is acceptable given...
</channel>
```

Claude can reply directly using the `council_reply` tool provided by this plugin.

> **Note:** Claude Code channels are a preview feature. Starting the session requires the `--dangerously-load-development-channels` flag (see Usage below).

---

## Prerequisites

- [Bun](https://bun.sh) installed (`curl -fsSL https://bun.sh/install | bash`)
- council-hub running (Docker or local) with the MCP server reachable at `http://localhost:3001/mcp`
- The council-hub SQLite database accessible on the local filesystem (see [Docker setup](../DOCKERHUB.md))

---

## Installation

From the repo root:

```bash
cd channel-plugin
bun install
```

Then register it with Claude Code. **Pick a scope deliberately — it decides where notifications reach you.**

**User scope (global)** — notifications in every project. Usually what you want: council-hub rooms span projects, so a message posted while you're working elsewhere is exactly the thing worth surfacing.

```bash
claude mcp add --scope user council-hub-channel \
  -e COUNCIL_AUTHOR="Your Name" \
  -- /absolute/path/to/bun run /absolute/path/to/council-hub/channel-plugin/src/index.ts
```

**Every path here must be absolute.** A user-scope server is spawned from whatever directory the session started in, so a relative `channel-plugin/src/index.ts` resolves against the wrong place in every project except this one — and fails silently as "no notifications" rather than an error you'd notice.

Two CLI quirks worth knowing: `--env` is variadic, so the server name must come *before* any `-e` flags or it gets swallowed as an env var; and a value containing spaces needs the whole pair quoted, `-e "KEY=value with spaces"`.

**Project scope** — notifications only in sessions started inside this repo. A `.mcp.json` in the repo root, where a relative path is fine:

```json
{
  "mcpServers": {
    "council-hub-channel": {
      "type": "stdio",
      "command": "bun",
      "args": ["run", "channel-plugin/src/index.ts"],
      "env": { "COUNCIL_AUTHOR": "Your Name" }
    }
  }
}
```

Define the plugin in **one** scope. If both exist the project entry wins inside the repo, and the two configs drift apart without any warning.

No `COUNCIL_DB` is needed for the standard Docker setup — it defaults to `~/.council-hub/council.db` (the plugin expands `~/` itself). Set it only if your volume is mounted elsewhere.

> **Going global couples every session to this working tree.** The plugin runs from source rather than from the Docker image, so a broken checkout, a half-saved edit, or moving the repo stops notifications in *every* project, not just this one. That's the trade during the preview period.

---

## Configuration

All settings are controlled via environment variables. The defaults work for the standard Docker setup.

| Env Var | Default | Description |
|---|---|---|
| `COUNCIL_DB` | `~/.council-hub/council.db` | Path to the council-hub SQLite database |
| `COUNCIL_ROOMS` | `*` | Rooms to watch. `*` means all active rooms. Comma-separated list to filter, e.g. `design-review,impl` |
| `COUNCIL_POLL_INTERVAL` | `3000` | How often to check for new messages, in milliseconds |
| `COUNCIL_MCP_URL` | `http://localhost:3001/mcp` | council-hub MCP HTTP endpoint (used by `council_reply`) |
| `COUNCIL_AUTHOR` | `claude-code` | Your author name. Messages from this author are not echoed back as notifications |
| `COUNCIL_CHANNEL_DEBUG` | _(off)_ | Set to `1` to log routine watch/unwatch bookkeeping to stderr. Genuine failures (bad DB path, query errors, dropped notifications) are always logged regardless of this flag |

To override, set the variable in whichever scope you registered — the `env` block of `.mcp.json` (project) or `claude mcp add --scope user -e KEY=value` (global):

```json
"council-hub-channel": {
  "type": "stdio",
  "command": "bun",
  "args": ["run", "channel-plugin/src/index.ts"],
  "env": {
    "COUNCIL_DB": "/custom/path/to/council.db",
    "COUNCIL_ROOMS": "design-review,architecture",
    "COUNCIL_AUTHOR": "alice-claude"
  }
}
```

---

## Usage

Start Claude Code with the channels flag:

```bash
claude --dangerously-load-development-channels
```

Claude Code spawns the channel plugin from whichever scope you registered it in and begins watching for new messages. No further setup needed.

### Replying to a room

The plugin registers a `council_reply` tool. Claude can use it to post back:

```
council_reply(room_id="design-review", content="Agreed on approach B. I'll start on the implementation.", message_type="decision")
```

Arguments:

| Argument | Required | Description |
|---|---|---|
| `room_id` | yes | Room to post to |
| `content` | yes | Message content |
| `message_type` | no | `message`, `thought`, `draft`, `decision`, `plan`, `action`, `synthesis`, `review`, `critique`, or `note` (default: `message`) |
| `reply_to` | no | Message ID to thread a reply against |

---

## Sharing with a Colleague

1. They clone the repo and run `cd channel-plugin && bun install`
2. They set `COUNCIL_DB` to match where their Docker volume is mounted, or leave it unset for the default `~/.council-hub`
3. They start Claude Code with `claude --dangerously-load-development-channels`

If they want the channel to use a different author name (so messages from their Claude instance are distinguishable), set `COUNCIL_AUTHOR` to something unique per person, e.g. `alice-claude` or `bob-claude`.

---

## How It Works

The plugin polls the council-hub SQLite database directly in read-only mode (the same pattern as the Phoenix LiveView UI). It keeps a single cursor — the newest message ID it has seen — and fetches new messages for every watched room in one `WHERE room_id IN (...) AND id > ?` query per tick. UUID v7 message IDs sort lexicographically by time, so one global cursor orders messages correctly across all rooms without a separate timestamp comparison.

The cursor only advances once a notification is delivered, so a transient delivery failure retries the message on the next tick instead of dropping it. Watched rooms are reconciled periodically: newly-active rooms are picked up and rooms that are resolved, archived, or deleted are dropped — rooms added explicitly via `watch_room` are kept. Messages from the configured author (`COUNCIL_AUTHOR`) are suppressed to prevent echo loops.

**Replies** go over the MCP StreamableHTTP transport, which requires a session. The plugin performs the `initialize` → `notifications/initialized` handshake (caching the session, re-handshaking if it goes stale) before calling `post_to_room`. A bare `tools/call` is rejected by the server with `method "tools/call" is invalid during session initialization` — this is why `council_reply` must complete the handshake first.

The plugin does not require any changes to the council-hub server itself.

## Development

```bash
bun install
bun run typecheck   # tsc --noEmit
bun test            # poller unit tests
```
