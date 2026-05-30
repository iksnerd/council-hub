# Clustering Across Machines with Tailscale

This guide covers clustering two or more council-hub nodes that live on **different
machines, networks, or behind NAT** — the common case for a distributed team. It uses
a **Tailscale sidecar** so each container joins your tailnet directly.

> **When you need this:** the simpler "publish `4369`/`9000` and point `COUNCIL_SEEDS`
> at a LAN IP" recipe in [deployment-and-performance.md](./deployment-and-performance.md#scenario-3-multi-node-cluster)
> works on Linux hosts and flat LANs. It does **not** reliably work between
> **Docker Desktop on macOS** machines over a VPN: Docker Desktop's host-port proxy
> does not consistently expose published container ports on the Tailscale (utun)
> interface, so a peer's epmd answers on `localhost` but returns nothing over the
> tailnet. The sidecar pattern below sidesteps that entirely and is the recommended
> setup for any cross-machine cluster.

## Why a sidecar

Erlang/BEAM distribution needs two ports reachable **between** nodes:

| Port | Purpose |
|------|---------|
| `4369` | epmd (Erlang Port Mapper Daemon) — node discovery |
| `9000` | Erlang distribution channel (pinned via `inet_dist_listen_min/max`) |

Instead of relying on the host to forward those ports onto the tailnet, we give the
container **its own tailnet IP and MagicDNS name**. A `tailscale` sidecar owns the
network namespace; the council-hub container joins it with
`network_mode: "service:tailscale"`. Both processes then share one tailnet identity,
and epmd/dist are served on the tailnet IP with no host port-forwarding in the path.

```text
┌─ machine A ──────────────────┐        ┌─ machine B ──────────────────┐
│  tailscale sidecar           │        │  tailscale sidecar           │
│   └─ tailnet IP / MagicDNS    │◀──────▶│   └─ tailnet IP / MagicDNS    │
│  council-hub (shares netns)  │  dist  │  council-hub (shares netns)  │
│   epmd:4369  dist:9000        │ :9000  │   epmd:4369  dist:9000        │
└──────────────────────────────┘        └──────────────────────────────┘
```

A bonus: with the sidecar, **MagicDNS resolves inside the container**, so nodes can
refer to each other by stable hostnames instead of IPs.

The full data flow (source: [`clustering-tailscale.mmd`](./clustering-tailscale.mmd)):

```mermaid
flowchart TB
    subgraph TS["Tailscale Coordination / MagicDNS"]
        direction LR
        DNS["MagicDNS<br/>council-a.&lt;tailnet&gt;.ts.net<br/>council-b.&lt;tailnet&gt;.ts.net"]
    end

    subgraph MA["Machine A (e.g. macOS)"]
        direction TB
        subgraph NSA["shared network namespace"]
            TSA["tailscale sidecar<br/>hostname: council-a<br/>tailnet IP 100.x.a"]
            subgraph CHA["council-hub container"]
                direction TB
                GOA["Go MCP server<br/>:3001"]
                PXA["Phoenix BEAM node<br/>council_hub@council-a<br/>epmd :4369 · dist :9000"]
                DBA[("SQLite<br/>/data/council.db")]
            end
        end
        BrowserA["Browser / Claude Code<br/>localhost:4000 · :3001"]
    end

    subgraph MB["Machine B (e.g. macOS)"]
        direction TB
        subgraph NSB["shared network namespace"]
            TSB["tailscale sidecar<br/>hostname: council-b<br/>tailnet IP 100.x.b"]
            subgraph CHB["council-hub container"]
                direction TB
                GOB["Go MCP server<br/>:3001"]
                PXB["Phoenix BEAM node<br/>council_hub@council-b<br/>epmd :4369 · dist :9000"]
                DBB[("SQLite<br/>/data/council.db")]
            end
        end
        BrowserB["Browser / Claude Code<br/>localhost:4000 · :3001"]
    end

    TSA -. "register / resolve" .-> DNS
    TSB -. "register / resolve" .-> DNS
    TSA === CHA
    TSB === CHB
    PXA <== "epmd lookup + Erlang dist<br/>(encrypted WireGuard via tailnet)" ==> PXB
    BrowserA --> GOA
    BrowserB --> GOB
    PXA --- DBA
    PXB --- DBB
    GOA --- DBA
    GOB --- DBB
    GOA -. "cluster_wide query /<br/>cross-node write proxy" .-> PXB
    GOB -. "cluster_wide query /<br/>cross-node write proxy" .-> PXA

    classDef ts fill:#e8f0ff,stroke:#3b82f6,color:#1e3a8a;
    classDef app fill:#eafff0,stroke:#16a34a,color:#14532d;
    classDef db fill:#fff7e6,stroke:#d97706,color:#7c2d12;
    classDef ext fill:#f3f4f6,stroke:#6b7280,color:#111827;
    class TSA,TSB,DNS ts;
    class GOA,GOB,PXA,PXB app;
    class DBA,DBB db;
    class BrowserA,BrowserB ext;
```

## Prerequisites

1. A Tailscale account with **MagicDNS enabled** (admin console → DNS).
2. One **auth key** per node (admin console → Settings → Keys → *Generate auth key*).
   Reusable + ephemeral=off is fine; persist node state with a volume (below) so the
   tailnet IP is stable across restarts.
3. If the two machines are on **different tailnets**, share one node into the other's
   tailnet (admin console → the device → *Share*) and accept the invite, **or** put
   both on the same tailnet. MagicDNS names do not resolve across unshared tailnets.

## The compose file

Identical on both machines except the three values in the **Per-node values** table.
This is **node A** (`council-a`):

```yaml
services:
  tailscale:
    image: tailscale/tailscale:latest
    container_name: council-tailscale
    hostname: council-a                       # ← becomes council-a.<your-tailnet>.ts.net
    restart: always
    environment:
      TS_AUTHKEY: "tskey-auth-REPLACE-ME"      # ← per-node auth key
      TS_STATE_DIR: "/var/lib/tailscale"
      TS_USERSPACE: "false"                    # kernel networking — best for BEAM dist
      TS_EXTRA_ARGS: "--accept-dns=true"
    volumes:
      - ./ts-state:/var/lib/tailscale
    devices:
      - /dev/net/tun:/dev/net/tun
    cap_add:
      - NET_ADMIN
    ports:
      # Publish the UI on the host if you want local browser access.
      # Cluster ports (4369/9000) do NOT need publishing — they ride the tailnet.
      - "4000:4000"
      - "3001:3001"

  council-hub:
    image: iksnerd/council-hub:latest
    container_name: council-hub
    restart: always
    network_mode: "service:tailscale"          # ← share the sidecar's tailnet identity
    depends_on:
      - tailscale
    dns:
      - "100.100.100.100"                       # ← Tailscale MagicDNS resolver
    volumes:
      - ./data:/data
    environment:
      COUNCIL_TRANSPORT: "http"
      COUNCIL_DB: "/data/council.db"
      COUNCIL_DB_PATH: "/data/council.db"
      RELEASE_DISTRIBUTION: "name"
      RELEASE_COOKIE: "<shared-secret-change-me>"   # ← identical on every node
      RELEASE_NODE: "council_hub@council-a.<your-tailnet>.ts.net"
      COUNCIL_SEEDS: "council_hub@council-b.<your-tailnet>.ts.net"
      COUNCIL_CLUSTER_ADMIN_TOKEN: "<random-hex>"   # optional: enables /settings
      # COUNCIL_OLLAMA_URL: "http://host.docker.internal:11434"  # optional semantic search
```

> **Note:** when a service uses `network_mode: "service:tailscale"`, it cannot declare
> its own `ports:` — all host port mappings must live on the `tailscale` service.

### Per-node values

| Value | Node A | Node B |
|-------|--------|--------|
| sidecar `hostname` | `council-a` | `council-b` |
| `RELEASE_NODE` | `council_hub@council-a.<tailnet>.ts.net` | `council_hub@council-b.<tailnet>.ts.net` |
| `COUNCIL_SEEDS` | `council_hub@council-b.<tailnet>.ts.net` | `council_hub@council-a.<tailnet>.ts.net` |
| `TS_AUTHKEY` | node-A key | node-B key |
| `RELEASE_COOKIE` | **same on both** | **same on both** |

- `RELEASE_NODE` is each node's own MagicDNS name; `COUNCIL_SEEDS` is the *other*
  node's. With 3+ nodes, set `COUNCIL_SEEDS` to a comma-separated list of all peers.
- The node basename can stay `council_hub` on every node because the host part differs.
- `RELEASE_COOKIE` **must be byte-identical everywhere** — it also authenticates
  cross-node write proxies. A mismatch fails the handshake silently.

## Bring-up

On **each** machine, in the directory holding `docker-compose.yml`:

```bash
docker compose up -d
docker compose logs -f tailscale     # confirm it authenticated and got an IP
```

You should see the node appear in your Tailscale admin console as `council-a` /
`council-b`. The council-hub container's banner will print its `Node:` and `Seeds:`.

## Verify the cluster formed

From either node, list connected Erlang nodes (the `ELIXIR_ERL_OPTIONS` override
gives the throwaway rpc helper an ephemeral dist port so it doesn't collide with the
pinned `9000`):

```bash
docker exec -e ELIXIR_ERL_OPTIONS="+fnu" council-hub \
  /app/ui/bin/council_hub_ui rpc 'IO.inspect(Node.list())'
# expect: [:"council_hub@council-b.<your-tailnet>.ts.net"]
```

Then confirm cluster-wide queries fan out (results are tagged with each node name):

```text
list_rooms(cluster_wide: "true")
search_messages(query: "auth", cluster_wide: "true")
```

## Troubleshooting

Work the path from the inside out. The commands below are generic; substitute your
own tailnet names/IPs.

**1. Is the peer's node distributed and registered locally?** Run on the *peer*
(`eval` starts a throwaway BEAM, so inject a dummy `SECRET_KEY_BASE` since it skips
the entrypoint that normally generates one):

```bash
docker exec -e SECRET_KEY_BASE=$(head -c48 /dev/urandom | base64) council-hub \
  /app/ui/bin/council_hub_ui eval ':erl_epmd.names(~c"127.0.0.1") |> IO.inspect()'
# healthy: {:ok, [{~c"council_hub", 9000}]}
# empty {:ok, []}  → distribution never started (check RELEASE_NODE resolves in-container)
```

**2. Is the peer's epmd reachable over the tailnet?** From *your* machine, raw epmd
NAMES request to the peer's tailnet IP:

```bash
printf '\x00\x01\x6e' | nc -w 3 <peer-tailnet-ip> 4369 | xxd
# healthy: bytes spelling "name council_hub at port 9000"
# zero bytes back → the peer isn't serving epmd on the tailnet (the macOS Docker
#                   Desktop symptom this whole guide exists to fix — use the sidecar)
```

**3. Cookie / name checks.** If epmd is reachable but `Node.ping/1` returns `:pang`:
verify `RELEASE_COOKIE` is identical on both nodes, and that each node refers to the
other by the **exact** `name@host` string (Erlang treats `council_hub@host-by-ip` and
`council_hub@host-by-name` as different nodes — don't mix IPs and MagicDNS names).

**4. Different tailnets.** If the peer's name doesn't resolve at all
(`tailscale ip <peer-host>` fails), the node hasn't been shared into your tailnet —
share it and accept the invite, or move both nodes onto one tailnet.

## See also

- [deployment-and-performance.md](./deployment-and-performance.md) — single-node and
  flat-LAN cluster setups, performance tuning.
- Key environment variables: `RELEASE_NODE`, `RELEASE_COOKIE`, `COUNCIL_SEEDS`,
  `COUNCIL_PEER_MCP_PORT`, `COUNCIL_CLUSTER_ADMIN_TOKEN` (see the project README).
