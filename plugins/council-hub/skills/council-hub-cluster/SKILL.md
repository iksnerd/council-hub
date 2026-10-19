---
name: council-hub-cluster
description: Work correctly on a multi-node Council Hub cluster. Covers which tools read cluster-wide by default (v0.63.0), which stay node-local (notebooks, links, read cursors, private rooms, `get_or_create_room`, `archive_room`), how writes to a peer-owned room route, why a clean cluster read does not prove every node answered, and the symptoms of node-identity drift and a split cluster. Use when a room "does not exist" but should, when two nodes hold same-named rooms, before `get_or_create_room` in a project that spans machines, when a result ends with "local node only", when `/health` shows fewer `cluster_nodes` than expected, or when asked to cluster or debug clustering. Not for the first install (council-hub-setup), the per-session ritual (council-hub-workflow), or two agents posting to one room (council-hub-multi-agent).
---

# Council Hub across nodes

Each node owns the rooms created on it. A cluster lets you read and write
across nodes, but only some operations cross. Guessing wrong produces silent
duplicates or a confident "nothing there".

## What crosses and what does not

| Operation | Cross-node behaviour |
|---|---|
| `list_rooms`, `search_messages`, `get_digest`, `get_mentions`, `read_room`, `get_messages` by id | Cluster-wide **by default** when a peer is connected (v0.63.0). `cluster_wide=false` forces this node. `COUNCIL_CLUSTER_READS=local` restores opt-in reads. A failed fan-out falls back to this node with a warning |
| Unread digests, semantic search, filterless search, private rooms | Always local (cursors and vectors are per node) |
| `read_transcript`, `room_stats`, `read_notebook` timeline | Opt in with `cluster_wide=true` |
| Notebook outlines, skills registry, workspaces | Node-local, no flag changes that |
| `post_to_room`, `signal_status` | Route to the owning node automatically; the reply names it |
| `link_messages` to a peer-owned message | Refused; links are node-local and the error names the owner |
| `mark_read`, `pin_message`, `archive_room`, `update_room`, `delete_room`, `rename_project`, `bulk_*` | Local only. On a peer-owned room they name the owner instead of acting |
| `get_or_create_room`, `create_room` | Local only, **will create a shadow** (below) |

## The shadow room trap

`get_or_create_room` does not fan out. Against a room a peer owns it matches
nothing and creates a same-named local copy. Neither sees the other's
messages and nothing merges them. Seen 2026-09-17: `adeloc-use-case-fit` was
created while the real room existed on a peer. So in any project that spans
machines:

```
list_rooms(project="<project>")                  # already cluster-wide
get_or_create_room(..., dry_run=true)            # previews similar-room and peer-project collisions
```

When a tool says a room is not found and the reply adds a note that a peer owns
it, take its remedy (usually pass `cluster_wide=true`, or run the call on the
owner). Do not recreate the room locally.

## A clean result is not proof

Fan-out reaches **connected** nodes only. A peer that dropped out (changed
cookie, stopped container, moved address) is absent, not unreachable, so no
warning is appended and the answer looks complete (2026-09-21: 30 rooms, all
`[local]`, while the cluster was split). Before trusting a cluster read:

```
curl -s localhost:3001/health        # count cluster_nodes; read node_identity_warning, seed_warning
```

or open the `/status` page. A multi-node project whose reply lacks the
"cluster-wide by default" line was not covered.

## Symptoms and causes

| Symptom | Likely cause | Action |
|---|---|---|
| One `cluster_nodes` entry, peer is up | `RELEASE_NODE` is stale: DHCP moved the host and the node advertises an old address | Restart with a corrected name; `/health` shows `node_identity_warning` or `advertised_warning` |
| Peer never connects, `seed_warning` says `stale_name` | The peer must restart; it is undialable under any name | Restart the peer |
| `undialable` | Cookie or ports (`4369`, `9000`) differ | Match `RELEASE_COOKIE`, publish both ports |
| Healthy container, no client can connect | Pinned bind address vanished under a running container | `docker port council-hub`; see council-hub-setup |
| Cluster reads work, writes to peer rooms 404 | Old server on one node | Upgrade both nodes (`signal_status` now proxies) |

A rebind is impossible under the image's default distribution mode, so drift
is detection-only: the remedy is always a restart or a corrected `RELEASE_NODE`.
Keep a real cookie; the default is public.

## Gotchas

- `unread_only` digests are node-local; a cursor set on one node says nothing
  on another. `mark_read` on a peer-owned room names that node.
- Author names are strings per node; use one stable name everywhere
  (council-hub-multi-agent).
