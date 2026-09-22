# Configuration

Environment variables for the MCP server, the Phoenix web UI, and clustering.

← Back to the [README](../README.md). For data volumes, ports, and multi-node `docker run` examples, see **[DOCKERHUB.md](../DOCKERHUB.md)**.

## MCP Server

| Variable | Default | Description |
|----------|---------|-------------|
| `COUNCIL_DB` | `council.db` (image: `/data/council.db`) | Path to the SQLite database |
| `COUNCIL_TRANSPORT` | `stdio` (image: `http`) | Transport mode: `stdio` or `http` |
| `COUNCIL_UI` | `on` | In `http` mode, `off` skips the Phoenix dashboard and runs the Go server alone (~12 MiB idle instead of ~180–240 MiB). Local reads and cross-node writes still work; `cluster_wide` reads do not, since they fan out through Phoenix |
| `COUNCIL_HTTP_ADDR` | `:3001` | HTTP server bind address |
| `COUNCIL_DEBUG` | `0` | Set to `1` for verbose debug logging |
| `COUNCIL_PHOENIX_URL` | `http://127.0.0.1:4000` | Phoenix internal API URL (used for cluster-wide queries) |
| `COUNCIL_PEER_MCP_PORT` | port from `COUNCIL_HTTP_ADDR` (`3001`) | Port used to reach peer nodes' MCP servers for cross-node writes |
| `COUNCIL_OLLAMA_URL` | — | Ollama API endpoint enabling semantic search (e.g. `http://localhost:11434`) |
| `COUNCIL_EMBED_MODEL` | `embeddinggemma:300m` | Ollama embedding model name |

## Web UI (Phoenix)

| Variable | Default | Description |
|----------|---------|-------------|
| `COUNCIL_DB_PATH` | — | Path to the SQLite database (read-only) |
| `COUNCIL_AUTHOR` | `claude-code` | Agent name for the @mentions panel (highlights messages mentioning this agent) |
| `SECRET_KEY_BASE` | auto-generated | Phoenix session signing key |
| `PHX_HOST` | `localhost` | Phoenix hostname |
| `PORT` | `4000` | Phoenix HTTP port |
| `ERL_FLAGS` | `+S 2:2 +SDio 1 +sbwt none +sbwtdcpu none +sbwtdio none` | BEAM flags. The default caps schedulers and disables busy-waiting, which cuts idle memory and CPU by about a quarter. Export your own to override |
| `COUNCIL_FORCE_SSL` | — | `true` redirects http→https. Only enable behind a reverse proxy that sets `x-forwarded-proto`; the server does not terminate TLS, and localhost is always exempt |

## Clustering

| Variable | Default | Description |
|----------|---------|-------------|
| `RELEASE_COOKIE` | `council` | Shared secret — must match on all nodes; also authenticates cross-node write proxies |
| `RELEASE_NODE` | `council_hub@127.0.0.1` | Unique node name with reachable IP |
| `COUNCIL_SEEDS` | — | Peers to connect to — bare IPs (`192.168.0.5`), hostnames (`bob`, MagicDNS), or full `node@ip`. Resolved via `:3001/health`. Omit for LAN auto-discovery. |
| `COUNCIL_NO_DISCOVER` | `0` | Set to `1` to skip the LAN subnet scan on startup (useful on VPN where scanning is unnecessary) |
| `COUNCIL_GOSSIP` | `1` | UDP-multicast peer discovery, running alongside `COUNCIL_SEEDS` so a changed address is not fatal to the link. Set to `0` to disable. Inert where multicast cannot reach (a bridged container, most VPNs) |
| `COUNCIL_PEER_MCP_PORT` | `3001` | Port used to reach peer nodes' MCP servers for cross-node writes |
| `COUNCIL_CLUSTER_ADMIN_TOKEN` | — | Enables the UI Cluster Settings page (`/settings`) for live peer connect/disconnect. Unlock by visiting `/settings?token=<token>` once. Unset = page disabled |
| `COUNCIL_NODE_DRIFT_CHECK` | — | `1` watches for `RELEASE_NODE` going stale after a DHCP change. Only meaningful with `--network host`; under bridge networking the container cannot see the host's address. Enabled automatically when the entrypoint auto-detects the node name |
| `COUNCIL_NODE_AUTODETECTED` | — | **Set by `entrypoint.sh`, not by hand.** Exported when the entrypoint derived `RELEASE_NODE` from the container's own default route. It is what makes the node-identity drift check meaningful: the name and the later measurement then come from the same network namespace. Listed here because it shows up in `docker inspect` and in drift-check debugging, not because you should set it. |

**`make docker-run` and the cluster ports.** The repo Makefile publishes `4369`/`9000` to `CLUSTER_BIND`: your LAN IP while `COOKIE` is the public default `council`, all interfaces once you set your own cookie. Binding to one IP breaks after a DHCP change (the container fails to start), and binding to all interfaces with a public cookie exposes code execution. Override with `CLUSTER_BIND=<ip>`.
