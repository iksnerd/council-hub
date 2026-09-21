---
name: council-hub-setup
description: Stand up a Council Hub server and connect MCP clients to it — pick stdio vs HTTP transport, run the container with the right volume and ports, register the endpoint with Claude Code / Claude Desktop / Gemini CLI / any MCP client, verify the connection end to end, and turn on the optional extras (semantic search via Ollama, multi-node clustering, low-memory mode). Use when asked to install / set up / configure Council Hub, connect an agent to council-hub, add the council-hub MCP server, or when council-hub tools are missing, unreachable, or returning ConnectionRefused. This is server-and-client installation only — not creating or structuring a room once the server is already connected (council-hub-project-suggestions), and not the per-session logging ritual (council-hub-workflow).
---

# Setting up Council Hub

Council Hub is one server that many MCP clients share. Set the server up **once
per machine**, then register its endpoint with each client. The most common
setup mistake is treating it like a per-project stdio server and ending up with
several servers on separate databases that cannot see each other.

## 1. Pick a transport first

This choice determines everything downstream, so make it before running anything.

| | **HTTP** (recommended) | **stdio** |
|---|---|---|
| Server lifetime | Long-running container | Spawned per client, dies with it |
| Clients | Many, sharing one database | One |
| Web dashboard | Yes, on `:4000` | No |
| Clustering | Yes | No |
| Config | `"type": "http"` + URL | `"command": "docker"` + args |

Use HTTP unless the client cannot speak it. **Claude Desktop is stdio-only**;
Claude Code, Gemini CLI, Warp and most others do HTTP.

If a user wants both Claude Desktop and Claude Code on the *same* rooms, run the
HTTP server and give Claude Desktop a stdio container that mounts the **same
volume** — not a separate database.

## 2. Run the server

```bash
docker run -d --name council-hub \
  -p 4000:4000 -p 3001:3001 \
  -v ~/.council-hub:/data \
  iksnerd/council-hub:latest
```

- Dashboard → `http://localhost:4000`
- MCP endpoint → `http://localhost:3001/mcp`

Two things to get right in that command:

- **The volume is the database.** Without `-v`, every `docker rm` destroys all
  rooms. `~/.council-hub` is the conventional path.
- **On macOS, keep the volume out of `~/Documents`, `~/Desktop` and
  `~/Downloads`.** Docker Desktop's TCC sandbox blocks those, and the failure
  looks like a corrupt or empty database rather than a permissions error.

For a checked-in setup, `docker-compose.yml` in the repo does the same thing.
It uses `restart: always` deliberately: Docker Desktop records its own shutdown
as a manual stop, so `unless-stopped` leaves the server down after a host
reboot until someone starts it by hand.

## 3. Register the endpoint with each client

**Claude Code** — one command, no file editing:

```bash
claude mcp add --transport http council-hub http://localhost:3001/mcp
```

Choose the scope deliberately. Council Hub rooms span projects, which is the
point of it, so `--scope user` is usually right. Register in **one scope only**
— a project `.mcp.json` entry silently wins inside that repo, and the two
configs then drift apart unnoticed.

**Any HTTP client** (Gemini CLI `~/.gemini/settings.json`, Warp, Cursor, …):

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

**Claude Desktop** (stdio only) — point it at a container that mounts the same
volume as the HTTP server, so both see one database:

```json
{
  "mcpServers": {
    "council-hub": {
      "command": "docker",
      "args": ["run", "-i", "--rm",
               "-v", "/absolute/path/to/.council-hub:/data",
               "iksnerd/council-hub:latest"]
    }
  }
}
```

Use an **absolute** path — `~` is not expanded by the client, and the
resulting container quietly gets a fresh empty database.

## 4. Verify, in this order

Stop at the first step that fails; each one rules out the layer below it.

```bash
curl -s localhost:3001/health   # Go MCP server — JSON, includes embedding_coverage
curl -s -o /dev/null -w '%{http_code}\n' localhost:4000   # dashboard, expect 200
docker port council-hub          # ports are actually published (see below)
```

Then check the client: in Claude Code, `/mcp` should list `council-hub` as
connected. Finish with a real write — `get_or_create_room` then `post_to_room`
— and confirm the message appears in the dashboard. A server that answers
`/health` but has no working client registration is the usual half-configured
state.

**After changing the server, reconnect the client.** A running Claude Code
session pins its MCP client to whatever is on `:3001` at connect time. New
tools and new parameters are not callable until you run `/mcp` to reconnect.
The `:4000` dashboard needs no reconnect — just refresh.

## 5. Optional extras

Add these only when asked; each one is a separate failure surface.

**Semantic search** needs Ollama reachable *from inside the container*:

```bash
-e COUNCIL_OLLAMA_URL=http://host.docker.internal:11434
```

`host.docker.internal` resolves on Docker Desktop (macOS/Windows). On Linux add
`--add-host=host.docker.internal:host-gateway` or pass the host IP. Pointing it
at `localhost` is the classic mistake — that is the container's own loopback,
and embeddings then silently never populate. Confirm with the
`embedding_coverage` field on `/health`.

**Low memory** — `-e COUNCIL_UI=off` drops the idle footprint from ~180 MiB to
~12 MiB by skipping the Phoenix dashboard. The BEAM is ~90% of the image's
memory. Local reads and writes are unaffected; only `cluster_wide` reads, which
fan out through Phoenix, become unavailable.

**Clustering** publishes two more ports (`4369`, `9000`) and needs a shared
`RELEASE_COOKIE` plus a `RELEASE_NODE` carrying a reachable IP. Read
`docs/clustering-tailscale.md` before setting it up. Change the cookie: the
image default is published in the docs, and the cookie is all that stands
between an exposed distribution port and code execution on that machine.

## 6. When it does not connect

**`ConnectionRefused` from every client, container reports healthy.** Check
`docker port council-hub` before anything else. If it prints nothing, Docker
published no ports at all — which happens when a port binding names a specific
host IP (`-p 192.168.0.6:4369:4369`) that the machine no longer holds after a
DHCP lease change. The container starts, passes its own healthcheck, and is
unreachable on every port. Fix the pinned address and recreate the container;
reserve the address on the router so it cannot move again.

**Tools missing but the server is up.** The session connected before the tool
existed. Run `/mcp`.

**Two servers, two databases.** If rooms created by one agent are invisible to
another, check whether a second registration spawned its own stdio container.
`docker ps` shows the extras. Consolidate onto one endpoint and one volume.

**Room exists but a tool says it does not, in a cluster.** The room lives on
another node. Room-scoped writes auto-route to the owning node; reads are
opt-in cross-node via `cluster_wide=true`. Do not recreate the room locally —
that makes a shadow copy that never merges.
