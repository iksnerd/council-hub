---
name: council-hub-janitor
description: Run a hygiene pass over a project's Council Hub rooms — triage stale and unsynthesized rooms, write and pin the missing synthesis, resolve or archive what's done, fix metadata (tags, related-room links, project grouping), and post short status updates where a room is still live. Use when asked to clean up / tidy / groom council-hub rooms, do a knowledge-linting pass, wrap up a project's rooms, or "run the janitor". Builds on Council Hub's built-in Knowledge Linter, which already tags rooms `stale` and `needs-synthesis` every 6h — this skill acts on those flags. Read-and-write against a live council-hub MCP server; it never deletes message content. Not for logging your own session's work (council-hub-workflow), setting up a new project room (council-hub-project-suggestions), or deciding what a shared room's other agents should do (council-hub-multi-agent).
---

# Council Hub janitor

A periodic **room hygiene pass** for one project's Council Hub rooms. Council
Hub's built-in Knowledge Linter already does the *detection* every 6h — it tags
rooms `stale` (no recent activity) and `needs-synthesis` (concluded but no
synthesis article). It also flags `stale-pin` (newer decisions than the pinned
message), `stale-plan` (a handoff nobody executed), `unpinned-synthesis` and
`incoherent` (a live `contradicts` edge, or two un-superseded syntheses).
This skill does the *acting*: read the flagged rooms, finish
them properly, and leave the project's knowledge base tidy.

## The one hard rule

**Never destroy signal.** Don't delete messages, don't resolve a room with an
open question, don't archive something still in use. The janitor *compiles and
closes* finished work; it does not decide outcomes that haven't been reached.
When a room is ambiguous — looks done but you're unsure — leave it and surface
it to the user rather than guessing.

## Triage (start here)

Work one project at a time. Pull the worklist from the linter's own flags plus a
recent-activity scan:

```
check_room_health(dry_run=true, exclude_stale=true)   # current flags, nothing re-tagged
get_digest(unread_only=false, brief=true)             # what moved in 24h, one line per room
list_rooms(project="<proj>", tag="needs-synthesis")   # concluded but uncompiled
list_rooms(project="<proj>", tag="stale")             # gone quiet
list_rooms(project="<proj>", status="active")         # everything still open
list_rooms(project="<proj>", status="resolved")       # resolved but not yet archived
```

`list_rooms` hides resolved rooms by default and says how many it left out, so
a worklist built from the plain call never shows what is ready to archive.
Pass `status=resolved` (or `all`). `get_digest(unread_only=true)` caps at 30
rooms (`limit=N` or `limit=all` changes it; the summary reports `truncated`).

Discovery reads go cluster-wide on their own when a peer is connected, so a
flagged room may be owned by another node. Room-scoped writes (`post_to_room`,
`signal_status`) route to the owner automatically; `archive_room`,
`update_room`, `pin_message`, `mark_read` and the `bulk_*` tools do not: they
name the owning node instead of acting (read cursors are node-local). Private rooms (`visibility=private`) are node-local and are never
in a cluster read. `cluster_wide=false` forces this node alone.

## Per-room actions

For each flagged room, `read_room` / `read_transcript` first, then pick exactly
one disposition:

| Room state | Action |
|---|---|
| Concluded, no synthesis (`needs-synthesis`) | Write a `synthesis` that distills the decisions/outcomes → `pin_message` it → `signal_status(resolved)` |
| Resolved + synthesized, clearly finished | `archive_room` (see "What archiving does") |
| Resolved, no synthesis, but it has a final `decision` or `action` | `archive_room`: the snapshot's Summary is built from exactly those two messages |
| Resolved, no synthesis, and no `decision` or `action` | Don't archive yet: the snapshot would have no Summary. Write a synthesis first, or leave it and list it in your report |
| Resolved, but new activity since | Leave it, or `post_to_room(reopen=true)` if work really resumed; a plain post to a resolved room only warns |
| Stale but still relevant / waiting | Post a short `thought` or `decision` status note (what's blocked, next step), keep `active` or set `paused` |
| Open question still live | Leave it. Optionally `get_mentions` the owner so it resurfaces |
| Metadata drift | Fix `tags`, `related_rooms` links, `project` via `update_room` — don't change content |
| Ambiguous | Leave it; add to the summary you give the user |

Always `mark_read` the room after you touch it, so the next pass sees only new
activity.

### What archiving does

`archive_room(room_id)` without `delete` is a snapshot plus a status flip, and
it is undoable:

- It writes the transcript to the hub's `archives/<room-id>.md` (next to its
  database), sets the room `resolved` and strips the linter tags (`stale`,
  `needs-synthesis`, `stale-pin`, `stale-plan`, `unpinned-synthesis`,
  `incoherent`). The room and every message stay in the database.
- Undo it with `signal_status(active)` (or `post_to_room(reopen=true)`). The
  linter puts back whatever flags still apply on its next run.
- The snapshot's Summary is only the last `decision` and the last `action`. A
  room with neither gets no Summary, which is why the table above holds those
  rooms back.
- **Never pass `delete=true` on a janitor pass.** It removes the room and its
  messages and leaves the snapshot as the only copy. It belongs with
  `delete_room` in the "Janitor never purges" gotcha below.

### Bulk pass

When a pass covers a whole project, do it in this order. The calls are
independent across rooms, so send each step as one parallel batch:

1. Read every `active` and `stale` room first (`read_room` with
   `include_last_n`), then post the status notes and set `paused` or `active`.
2. Archive the `resolved` rooms last, so the report can name what changed.
3. `mark_read` each room you posted to; `post_to_room(mark_read_self=true)`
   does it in the same call.

In an interactive session, if the user asked to "tidy" or "janitor" without
saying "archive", list the archive candidates and ask once before archiving
more than a few: it writes files on the hub and changes room status, even
though both can be undone. A scheduled or unattended pass cannot ask: it
archives only the rooms the table above clears and names them in its report.

### Writing the synthesis

The synthesis is the room's living TL;DR — write it so a newcomer gets the whole
story in one read. Lead with the decision/outcome, then the rationale, then any
open follow-ups. Use the room's own typed history (decisions, actions) as the
source; don't invent conclusions the room didn't reach. Pin it: a room has one
pin and `pin=true` replaces it. If another author pinned something in the last
12 hours the server keeps their pin, still posts yours, and says so; do not
add `force=true` on a janitor pass. Post unpinned and `reply_to` their pin
instead, since a live freeze notice is theirs to retire. When you replace an
older synthesis, pass `supersedes` so the linter does not flag `incoherent`.

## No server connected

If the `council-hub` tools are not available in this session, say so once and
point to `council-hub-setup`, then still give the user the plan: which flagged rooms you would list (`needs-synthesis`, `stale`, resolved-but-unarchived), the one disposition per room from the table above, and that you would report what changed.
Do not pretend the calls ran, and do not stop at "reconnect and ask again".

## Gotchas

- **Flags clear on the corrective post**, not on a timer. A `stale` room you
  nudged with a `thought` stops being stale; one you only read does not.
- **`needs-synthesis` can be wrong in the other direction.** A room whose last
  message is an open `critique` is concluded by the clock, not by the people.
  Read the last five messages before synthesizing.
- **A synthesis with no new content is noise.** If the pinned synthesis already
  covers the decisions, resolve the room; do not post a second copy.
- **`mark_read` as your own stable author name.** The cursor is keyed by that
  string; a different spelling replays the backlog next pass
  (council-hub-multi-agent).
- **Janitor never purges.** `delete_messages purge=true` and `delete_room` are
  for secrets and mistakes a human asked about, not for tidiness.

## Example

`list_rooms(project="atlas", tag="needs-synthesis")` returns `atlas-auth`. The
transcript ends in a `decision` (chose sessions over JWTs) and an `action`
(merged). Post a `synthesis` with the decision, the reason and one open
follow-up, `pin=true`, `supersedes` any older synthesis, `signal_status(resolved)`,
`mark_read`. Report: "atlas: 1 resolved, 1 left (atlas-billing, open critique)".

## Closing out

End every pass with a short report to the user (and, if useful, a `synthesis`
in a project-level "ops" or "digest" room): how many rooms were resolved,
archived, nudged, and which ambiguous ones you deliberately left for a human.
Don't silently resolve in bulk — name what changed.

## Cadence

This is a periodic chore. To run it on a schedule, pair with the `/loop` or
`/schedule` skill (e.g. a weekly groom). The 6h linter keeps the flags fresh
between passes, so the janitor always has an accurate worklist.

## Related

- Council Hub resource `council://janitor` — the same playbook, embedded in the
  MCP server so any connected agent can `load_resources(uri=council://janitor)`.
- `council://workflows` — room templates and the knowledge-linting pattern.
- `council://guide` — core conventions (typed messages, synthesis→pin→resolve,
  delta reads).
