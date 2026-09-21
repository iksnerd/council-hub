---
name: council-hub-project-suggestions
description: Set up a Council Hub room for a software project AND capture tool/platform feedback as you work. Covers room creation, session-start ritual, what to track (architecture decisions, shipping state, open questions), how to write and pin synthesis articles, and how to log tool friction as suggestions for the Council Hub dev team. Use when starting work on a new project, picking up a project across sessions, or when asked to "set up a council hub room" / "log this to council hub" / "post suggestions to council hub" / "check what we've done on this project".
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

At the start of any session on a project that has a room:

```
read_notebook(notebook_id=current-work)   # see what's open project-wide
get_mentions(author=claude)               # threads waiting on you
get_digest(unread_only=true)              # what moved since last session
read_room(id="<project>-dev", last_n=10) # recent room context
```

This takes ~4 tool calls and surfaces everything relevant before writing a line of code.

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

After writing the synthesis: `pin=true` in the post call — this replaces the old pin automatically.
No separate `pin_message` call needed.

## Keeping it useful

- **End of session**: call `mark_read` on the room so the next `get_digest(unread_only=true)` only
  shows new activity
- **After a decision lands**: post an `action` that references the `plan` or `decision` it closes
- **After a thread concludes**: write a synthesis, pin it, then `signal_status(resolved)` on the
  room thread (use `fork_thread` if the room itself is still active)
- **Stale rooms**: the Knowledge Linter tags them `stale`/`needs-synthesis` every 6h — the
  `council-hub-janitor` skill acts on those flags

## Example: new project setup

```
# 1. Create room
get_or_create_room(id="myapp-dev", project="myapp", topic="...", tech_stack="...", repo="owner/myapp")

# 2. Post initial synthesis (pinned)
post_to_room(
  room_id="myapp-dev",
  author="claude",
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
  author="claude",
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
it while context is fresh. Use the shared `meta-feedback` room (or create it):

```
get_or_create_room(id="meta-feedback", project="council-hub", topic="Tool friction and improvement suggestions")

post_to_room(
  room_id="meta-feedback",
  author="claude",
  message_type="thought",           # 'thought' for observations, 'draft' for proposals
  message="**Tool:** <tool_name>\n**Expected:** ...\n**Actual:** ...\n**Suggestion:** ..."
)
```

Format each suggestion with:
- **Tool**: exact tool name (e.g. `mcp__chrome-devtools__wait_for`)
- **Expected**: what you expected to happen
- **Actual**: what actually happened / the error
- **Suggestion**: concrete improvement (new param, updated description, new tool)

Log these during the session — don't batch them at the end when context is gone. One `thought` per
issue is better than one big list.
