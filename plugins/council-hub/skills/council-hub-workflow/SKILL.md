---
name: council-hub-workflow
description: Make Council Hub the actual source of truth for dev work, not an afterthought — check mentions/digest/notebook before starting substantive work, log decisions and actions to a room as you go (not just at the end), and close out with a pinned synthesis + resolved/paused status. Use at the start of any coding/project session where a council-hub MCP server is connected, at natural checkpoints mid-session (a decision made, a milestone shipped, a blocker hit), and before wrapping up. Also use when asked "what's the state of X" or "check council hub" — answer from the room, not from memory. Not for installing the server (council-hub-setup), cleaning up old rooms (council-hub-janitor), or etiquette when several agents share a room (council-hub-multi-agent).
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
get_digest(unread_only=true, limit=15)                  # what changed since your last session (unread cursors are node-local)
read_notebook(notebook_id=current-work, status=open, level=1)   # the cross-project cockpit: in-flight work, open tasks
```

On a long-lived hub the unfiltered versions overflow the client's result
limit. `get_digest` takes `limit` (`limit=all` lifts it; `unread_only`
defaults to 30 rooms) and `brief=true` for one line per room.
`read_notebook` takes `status=open`, `level=1`, `limit`, and `project` to keep
only one project's work items; `entry_id=<id>` reads a single entry in full.

`current-work` is a convention, not a built-in: a single global notebook
(empty `project`, so it lists in every project's view) that acts as the
cross-project cockpit. Create it once with
`edit_notebook(action=create, notebook_id=current-work)`; until it exists
that third call returns nothing.

`get_digest`, `list_rooms`, `search_messages`, `get_mentions`, `get_messages`
(by id) and `read_room` go cluster-wide on their own when a peer is connected
(the reply says so); pass `cluster_wide=false` only when you want this node
alone. Still pass `cluster_wide=true` on `room_stats`, `read_transcript` and
`read_notebook` timelines. Before this default, reads were local, so a room
another node owned was invisible: the call returned a clean "nothing new"
instead of an error, and you reported no feedback when there was some. An
operator can restore that with `COUNCIL_CLUSTER_READS=local`; if a reply lacks
the "cluster-wide by default" line on a multi-node project, assume it did not
cover the cluster. Seen 2026-09-17: a local-only `search_messages` for a project
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
list_rooms(project=<project>)        # cluster-wide by default when peers are connected
```

**And do not read a clean result as proof.** A cluster-wide read (default or explicit) fans out to
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

Notebook conventions (tasks vs room_refs, `check` rather than appending
"DONE", soft delete) are in `council-hub-notebook`.

For work that's real but will never grow into its own room — a config
change, a tooling tweak, a one-off decision — use a `task` instead
(`edit_notebook(action=add, kind=task, prose=...)`, then `check` it once
done). Don't leave it untracked just because it doesn't warrant a room; a
task is the lightweight option for exactly this.

Write `{sha:abc1234}` for a commit and `{wi:13021}` for an issue-tracker item in
messages. Transcripts render them as links when the room has `repo` /
`work_items` set (a URL template containing `{id}`, e.g.
`https://tracker.example.com/items/{id}`); otherwise they show as plain code.

## Wrapping up

- Post a `synthesis` that distills the room's conclusion, and `pin=true` it
  (or `pin_message` separately) so the next reader gets the TL;DR first.
  **In a room other sessions share, read the current pin first.** A room has
  one pin and pinning replaces it without asking: if the pin is another
  author's and still live (a freeze notice, their synthesis), post unpinned
  and `reply_to` their pin instead. The post's "replaced #…" line is the last
  chance: if it names someone else's message, `pin_message` it back at once.
  Since v0.63.0 the server enforces this for the common case: `pin=true` over
  another author's pin under 12 hours old is withheld, the message still posts,
  and `force=true` overrides. Do not reach for `force` to get past it.
  Hit twice in one shared room (2026-10-02: a decision replaced a live "main is
  frozen" notice; 2026-10-03: a synthesis replaced a peer's review synthesis).
- Retrying a post after a timeout? Pass `idempotency_key` so a repeat returns
  the first message. Posting into a resolved room only warns; `reopen=true`
  flips it back to active.
- `signal_status`: `resolved` when the goal is complete, `paused` when
  blocked or waiting (not done, just on hold) — don't leave a finished room
  `active` or a blocked one looking abandoned.
- `mark_read` so your own cursor advances — next session's `get_digest`
  won't re-surface what you already saw.

## No server connected

If the `council-hub` tools are not available in this session, say so once and
point to `council-hub-setup`, then still give the user the plan: for a wrap-up, the `action` with the commit SHA, the pinned `synthesis` (read the existing pin first in a shared room), `signal_status` `resolved` or `paused`, and `mark_read`; for a start or state question, the mentions, digest and notebook reads.
Do not pretend the calls ran, and do not stop at "reconnect and ask again".

## Cluster nodes: writes route, some reads stay local

`post_to_room`, `signal_status` and `read_transcript`/`get_messages` by id work
on a room a peer node owns; the reply names `owner_node`. (`signal_status` used
to 404 on peer-owned rooms; that was fixed, so a remote room can be closed from
anywhere.) `read_notebook` timelines, `room_stats` and `read_transcript` still
need `cluster_wide=true`, notebook outlines never leave the node, and
`link_messages` across nodes is refused. The full map is in
`council-hub-cluster`.

## Answering "what's the state of X"

Don't answer from conversation memory or from what you assume is still true.
`search_messages` or `read_transcript(room_id=X, mode=summary)` first — a
pinned synthesis is stale the moment newer `decision`/`action` messages land
without a fresh one, so also check `list_rooms(tag="stale-pin")` if the
answer matters. If nothing in council-hub covers it, say so explicitly rather
than filling the gap with an assumption.

For a cheap first pass over a lot of ground — "what's still open across
everything" — `read_notebook(notebook_id=current-work, level=1, status=open)`
collapses prose entries to their headings and drops done tasks/room_refs, so
it's a fraction of the token cost of the unclipped read (level alone cut a
237-entry `current-work` by ~81%; level+status together by over 90%). **If
`get_digest`/`read_notebook` overflow the tool-call limit anyway** (a mature
hub can exceed it even with these params), that generic oversized-result
error means retry with `level=1,status=open` / `limit=N`, not fork a subagent
to read the saved file — the harness's own remediation text doesn't say this,
so it's easy to miss. Cost a full session ~625K tokens across two forked
subagents before this was written down (2026-09-27).

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
