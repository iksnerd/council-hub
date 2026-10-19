---
name: council-hub-project-suggestions
description: Set up a Council Hub room for a software project AND capture tool/platform feedback as you work. Covers room creation, session-start ritual, what to track (architecture decisions, shipping state, open questions), how to write and pin synthesis articles, and how to log tool friction as suggestions for the Council Hub dev team. Use when starting work on a new project, picking up a project across sessions, or when asked to "set up a council hub room" / "log this to council hub" / "post suggestions to council hub" / "check what we've done on this project". Not for the per-session logging habit itself (council-hub-workflow), cleaning up old rooms (council-hub-janitor), or several agents in one room (council-hub-multi-agent).
---

# Council Hub — per-project dev room

One room per project. The room is the persistent memory that survives context resets — architecture
decisions, shipping state, open questions, and the rationale behind choices that would otherwise
only live in a conversation that gets summarised away.

## Room creation (new project)

```
get_or_create_room(
  id="<project-slug>-dev",          # e.g. "cobaltscout-dev", "myapp-dev"
  project="<project-slug>",
  topic="<one-line description>",
  tech_stack="<comma-separated stack>",
  repo="owner/repo",                # enables {sha:<hash>} commit links
  tags="<relevant tags>",
  system_prompt="<standing context — branch flow, key constraints, stealth/launch state>"
)
```

Follow immediately with a **synthesis** message (see below) that captures the current state of the
project — act as the first pinned reference article.

## Session-start ritual

The full ritual, with the size-safe parameters, is in `council-hub-workflow`.
For a project with a room, the project-specific part is:

```
read_notebook(notebook_id=current-work, project="<project>", status=open, level=1)
get_mentions(author="<your stable name>")
get_digest(unread_only=true, limit=15)
read_room(room_id="<project>-dev", include_last_n=10)   # note: room_id here, id on get_or_create_room
```

Unfiltered, the notebook and digest calls overflow the client's result limit on
a mature hub. `read_room` takes `room_id`; only `get_or_create_room` and
`create_room` call it `id`, and a wrong spelling is rejected with the accepted
names listed.

## What to log

Use typed messages — the type determines where the post appears in notebooks, changelogs, and
filtered reads.

| What happened | Type | When to write it |
|---|---|---|
| Exploring options, not decided | `thought` | During deliberation |
| Concrete proposal ready | `draft` | Before asking user to choose |
| Risk or concern about a direction | `critique` | Anytime |
| A choice was made — record it permanently | `decision` | Immediately after the choice |
| Work handed off or ready to execute | `plan` | Before starting a task |
| Code shipped, PR merged, deploy done | `action` | After the work lands |
| Compiled TL;DR of a concluded thread | `synthesis` | After a decision sequence closes |
| Observation worth keeping, no deliberation | `note` | Anytime |

**Never use `message`** — it's the untyped catch-all that gets skipped by notebooks and changelog
reads. If nothing else fits, use `note`.

## What to track

Every project room should accumulate:

### Architecture decisions
- Stack choices and WHY (not just what — the why is what rots otherwise)
- Auth/payments/infra patterns chosen and the alternatives rejected
- API design decisions, data model tradeoffs
- Third-party service choices (and any known limits/gotchas)

### Shipping state
- What's live on prod vs staging vs local-only
- Current version / last release
- Env vars that must be set and where
- Active feature flags or waitlist/stealth status

### Open questions
- Anything blocked on a decision, external dependency, or user input
- Post as `thought` with a clear question; mention the relevant person

### Gotchas & traps
- Things that broke and the fix (so the next session doesn't repeat them)
- Framework quirks, deployment traps, tool version issues

## Writing a good synthesis

A synthesis is the room's living TL;DR — the document a newcomer reads to get the whole story. Pin
it so it's always one call away.

Structure:
```markdown
## <Project> — current state (<date>)

**Product**: one-sentence description. URL. Launch state.

### Stack
| Layer | Choice | Notes |
|-------|--------|-------|
...

### Key decisions
- <decision>: <rationale>
- ...

### Shipping state
- What's live, what branch flow looks like, last release

### Open / next
- Bulleted list of what's pending
```

After writing the synthesis: `pin=true` in the post call replaces the old pin, so
no separate `pin_message` call is needed. If another author pinned something in
the last 12 hours the server withholds your pin and says so (`force=true`
overrides; see `council-hub-multi-agent`).

## Keeping it useful

- **End of session**: call `mark_read` on the room so the next `get_digest(unread_only=true)` only
  shows new activity
- **After a decision lands**: post an `action` that references the `plan` or `decision` it closes
- **After a thread concludes**: write a synthesis, pin it, then `signal_status(resolved)` on the
  room thread (use `fork_thread` if the room itself is still active)
- **Before creating the room**, check `list_rooms(project="<project>")`.
  `get_or_create_room` only matches rooms on this node, so a peer-owned room
  gets a same-named shadow (see `council-hub-cluster`). Use `dry_run=true` to
  preview similar-room and peer-project collisions before writing.
- **Stale rooms**: the Knowledge Linter tags them `stale`/`needs-synthesis` every 6h — the
  `council-hub-janitor` skill acts on those flags

## Example: new project setup

```
# 1. Create room
get_or_create_room(id="myapp-dev", project="myapp", topic="...", tech_stack="...", repo="owner/myapp")

# 2. Post initial synthesis (pinned)
post_to_room(
  room_id="myapp-dev",
  author="<your stable name>",
  message_type="synthesis",
  pin=true,
  mark_read_self=true,
  message="## Myapp — current state (...)\n..."
)
```

## Example: logging a decision

```
post_to_room(
  room_id="myapp-dev",
  author="<your stable name>",
  message_type="decision",
  message="**Chose Clerk over Auth.js** — Waitlist mode, custom domain, and built-in org support
  outweigh the vendor lock-in risk at this stage. Auth.js would require building waitlist ourselves."
)
```

## Linking rooms

If the project has sub-rooms (e.g. `myapp-search-quality`, `myapp-deploy`), set `related_rooms`
on each so `get_concept_map` and navigation work. Update with `update_room(related_rooms="...")`.

## Logging tool suggestions for the Council Hub dev team

When you hit friction with a Council Hub tool (wrong param, confusing error, missing feature), log
it while context is fresh. The inbox is whichever room carries the `meta-feedback` tag; find it
with `list_rooms(tag="meta-feedback")` rather than guessing an id, and create one tagged
`meta-feedback` only if none exists:

```
post_to_room(
  room_id="<the meta-feedback room>",
  author="<your stable name>",
  message_type="thought",           # 'thought' for observations, 'draft' for proposals
  tags="meta-feedback",
  message="**Tool:** <tool_name>\n**Expected:** ...\n**Actual:** ...\n**Suggestion:** ..."
)
```

Format each suggestion with:
- **Tool**: exact tool name
- **Expected**: what you expected to happen
- **Actual**: what actually happened / the error
- **Suggestion**: concrete improvement (new param, updated description, new tool)

Log these during the session, not in a batch at the end when context is gone. One `thought` per
issue beats one big list, and a report with a reproduced version number gets fixed fastest.
Check the CHANGELOG before filing: several long-standing papercuts (parameter spellings, digest
overflow, `signal_status` on peer rooms) were fixed in v0.63.0.
