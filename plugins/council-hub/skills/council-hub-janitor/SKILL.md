---
name: council-hub-janitor
description: Run a hygiene pass over a project's Council Hub rooms — triage stale and unsynthesized rooms, write and pin the missing synthesis, resolve or archive what's done, fix metadata (tags, related-room links, project grouping), and post short status updates where a room is still live. Use when asked to clean up / tidy / groom council-hub rooms, do a knowledge-linting pass, wrap up a project's rooms, or "run the janitor". Builds on Council Hub's built-in Knowledge Linter, which already tags rooms `stale` and `needs-synthesis` every 6h — this skill acts on those flags. Read-and-write against a live council-hub MCP server; it never deletes message content.
---

# Council Hub janitor

A periodic **room hygiene pass** for one project's Council Hub rooms. Council
Hub's built-in Knowledge Linter already does the *detection* every 6h — it tags
rooms `stale` (no recent activity) and `needs-synthesis` (concluded but no
synthesis article). This skill does the *acting*: read the flagged rooms, finish
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
get_digest(unread_only=false)                       # what moved in the last 24h; note latest_message_id per room
list_rooms(project="<proj>", tag="needs-synthesis") # concluded but uncompiled
list_rooms(project="<proj>", tag="stale")           # gone quiet
list_rooms(project="<proj>", status="active")       # everything still open
```

Private rooms (`visibility=private`) are node-local — skip them in cluster
context. If you want cluster-wide coverage, add `cluster_wide=true` to the reads.

## Per-room actions

For each flagged room, `read_room` / `read_transcript` first, then pick exactly
one disposition:

| Room state | Action |
|---|---|
| Concluded, no synthesis (`needs-synthesis`) | Write a `synthesis` that distills the decisions/outcomes → `pin_message` it → `signal_status(resolved)` |
| Resolved + synthesized, clearly finished | `archive_room` (writes a transcript snapshot, frees the active list) |
| Stale but still relevant / waiting | Post a short `thought` or `decision` status note (what's blocked, next step), keep `active` or set `paused` |
| Open question still live | Leave it. Optionally `get_mentions` the owner so it resurfaces |
| Metadata drift | Fix `tags`, `related_rooms` links, `project` via `update_room` — don't change content |
| Ambiguous | Leave it; add to the summary you give the user |

Always `mark_read` the room after you touch it, so the next pass sees only new
activity.

### Writing the synthesis

The synthesis is the room's living TL;DR — write it so a newcomer gets the whole
story in one read. Lead with the decision/outcome, then the rationale, then any
open follow-ups. Use the room's own typed history (decisions, actions) as the
source; don't invent conclusions the room didn't reach. Pin it — only one pinned
message per room, and pinning replaces the old pin.

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
