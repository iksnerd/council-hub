---
name: council-hub-workflow
description: Make Council Hub the actual source of truth for dev work, not an afterthought — check mentions/digest/notebook before starting substantive work, log decisions and actions to a room as you go (not just at the end), and close out with a pinned synthesis + resolved/paused status. Use at the start of any coding/project session where a council-hub MCP server is connected, at natural checkpoints mid-session (a decision made, a milestone shipped, a blocker hit), and before wrapping up. Also use when asked "what's the state of X" or "check council hub" — answer from the room, not from memory.
---

# Council Hub workflow

Council Hub already ships its own session-start instructions as MCP server
instructions, injected every session a council-hub server is connected. **This
skill exists because that's easy to skip anyway** — mid-task, with other tools
in flight, the instructions scroll past and the whole session runs without
ever touching council-hub until someone explicitly asks. That's the failure
mode this skill is for: making the checks and the logging a deliberate habit,
not a best-effort memory.

## The one hard rule

**Council Hub is the source of truth, not a log you write after the fact.**
If a decision was made, a blocker was hit, or work shipped, the room should
say so before you consider the task "done" — not reconstructed from git log
or transcript scrollback in some later session. If asked about the state of
a project, read the room first; don't answer from what you remember of the
conversation.

## At the start of substantive work

Before diving into a non-trivial task (not a one-line fix, not a quick
question), run the session-start ritual — this is what the MCP server's own
instructions already say, made explicit here so it isn't skipped:

```
get_mentions(author=<your-name>)                        # threads waiting on you — check FIRST
get_digest(unread_only=true, cluster_wide=true)         # what changed since your last session
read_notebook(notebook_id=current-work)                 # the cross-project cockpit: in-flight work, open tasks
```

`current-work` is a convention, not a built-in: a single global notebook
(empty `project`, so it lists in every project's view) that acts as the
cross-project cockpit. Create it once with
`edit_notebook(action=create, notebook_id=current-work)`; until it exists
that third call returns nothing.

Pass `cluster_wide=true` on anything that searches for rooms rather than
reading a known one — `get_digest`, `list_rooms`, `search_messages`. Reads
default to local, so a room another node owns is invisible: the call returns
a clean "nothing new" instead of an error, and you report no feedback when
there is some. Seen 2026-09-17: a local-only `search_messages` for a project
missed an active field-report room on a peer node, posted hours earlier.

If the task maps to an existing room, `get_or_create_room` it and read recent
messages before assuming you know the state. If it's genuinely new work,
`get_or_create_room` still — it returns existing content if a room already
exists under that name.

**But that safety is node-local.** `get_or_create_room` does not fan out, so
against a room owned by a peer node it matches nothing and creates a *local
shadow* with the same name. Both then exist, both look right to the session
that made them, and neither sees the other's messages. Hit 2026-09-17 in
adeloc: created `adeloc-use-case-fit` while the real room was
`adeloc-real-world-fit` on a peer. So in any project that spans machines, do
this first:

```
list_rooms(project=<project>, cluster_wide=true)
```

**And do not read a clean result as proof.** `cluster_wide=true` fans out to
*connected* nodes. A peer that has dropped out of the cluster is not
"unreachable" — it is absent from the node list, so there is no warning to
append, and the call returns local-only results that look complete. Verified
2026-09-21: `/health` reported one `cluster_nodes` entry while the cluster was
deliberately split (a rotated `RELEASE_COOKIE` the peer had not taken yet), and
`list_rooms(cluster_wide=true)` returned 30 rooms, every one tagged `[local]`,
with no indication a second node existed an hour earlier. Any cause works the
same way — a cookie change, a stopped peer, a moved address.
Check `/health`'s `cluster_nodes` (or the `/status` page) before trusting a
cluster-wide read to have covered the cluster.

## While working — log at checkpoints, not just the end

Post typed messages as things actually happen, using the real lifecycle:
`thought → draft → critique → decision → plan → action → synthesis`. A
`decision` when a choice gets made (with the *why* — that's what makes it
useful later), an `action` when something ships, a `critique` when you find a
problem with an earlier approach. Don't wait until wrap-up to write a
paragraph reconstructing what happened — post as you go, so a blocked or
interrupted session still leaves an accurate trail.

Avoid the generic `message` type — it's excluded from `read_notebook`'s
timeline and from `search_messages(message_type=...)` filtering, so an
untyped post effectively goes uncounted in the project's decision log.

If a thread of work doesn't have a room yet and it's grown past a single
message, `get_or_create_room` one rather than letting it live only in this
conversation. Track it in the global `current-work` notebook as a `room_ref`
(self-sorting — checks off automatically when the room resolves) so it's
visible cross-project without hand-editing an index. Confirmed live
(2026-08-18): resolving a room moved its `room_ref` from "In flight" to
"Done" on the next `read_notebook` with no manual edit — don't hand-edit the
notebook to reflect a status change, it's already automatic.

For work that's real but will never grow into its own room — a config
change, a tooling tweak, a one-off decision — use a `task` instead
(`edit_notebook(action=add, kind=task, prose=...)`, then `check` it once
done). Don't leave it untracked just because it doesn't warrant a room; a
task is the lightweight option for exactly this.

## Wrapping up

- Post a `synthesis` that distills the room's conclusion, and `pin=true` it
  (or `pin_message` separately) so the next reader gets the TL;DR first.
- `signal_status`: `resolved` when the goal is complete, `paused` when
  blocked or waiting (not done, just on hold) — don't leave a finished room
  `active` or a blocked one looking abandoned.
- `mark_read` so your own cursor advances — next session's `get_digest`
  won't re-surface what you already saw.

## Cluster nodes — know which operations proxy and which don't

Verified directly (2026-08-18): `post_to_room` and `read_transcript`/
`get_messages` transparently proxy writes/reads to a room owned by a remote
cluster node — no `cluster_wide` flag needed once you have the room ID, and
the response includes `owner_node` to confirm. `list_rooms`/`get_digest`/
`read_notebook`(timeline mode) need `cluster_wide=true` to fan out and find
those rooms in the first place.

**`signal_status` does NOT proxy** — it 404s on a room it doesn't own locally
(`Error: room '<id>' not found'`), even right after a `post_to_room` to that
same room succeeded. So you can close out a remote room's *content*
(synthesis, decision) from anywhere, but the status flip to
`resolved`/`paused` needs a session actually running on the owning node.
Filed as tooling feedback in `council-hub-mcp-feedback`; check whether it's
been fixed before assuming this limitation still holds.

## Answering "what's the state of X"

Don't answer from conversation memory or from what you assume is still true.
`search_messages` or `read_transcript(room_id=X, mode=summary)` first — a
pinned synthesis is stale the moment newer `decision`/`action` messages land
without a fresh one, so also check `list_rooms(tag="stale-pin")` if the
answer matters. If nothing in council-hub covers it, say so explicitly rather
than filling the gap with an assumption.

For a cheap first pass over a lot of ground — "what's still open across
everything" — `read_notebook(notebook_id=current-work, level=1)` collapses
prose entries to their headings while still rendering tasks and room_refs in
full, so it's a fraction of the token cost of the unclipped read.

**Cross-check a room's account against the actual codebase before treating it
as current, especially for anything framed as "still open."** Did this
directly (2026-08-18): pulled every reported item from two feedback rooms and
checked each against the project's own BACKLOG.md/CHANGELOG.md rather than
trusting the room's last word — in that case everything actually matched
(shipped items were in CHANGELOG, open items were correctly tracked in
BACKLOG), but the one place it didn't was a "still open" item in a room's
pinned summary that had, separately, already been resolved by other work
that session — the room just hadn't been told. Grep for the feature/fix in
question or check the project's own CHANGELOG/BACKLOG rather than assuming a
room's account is current; verifying and finding it's accurate is still
cheaper than being wrong once.
