---
name: council-hub-notebook
description: Drive dev work from Council Hub's notebooks, above all the global `current-work` cockpit. Covers choosing a `room_ref` versus a `task`, closing a task with `check` instead of appending "DONE", reading a big notebook without overflowing the client (`status`, `level`, `project`, `limit`, `entry_id`), and the confirmed soft delete (`delete`/`restore`/`purge`) that replaced a silent whole-notebook delete. Use when adding, finishing, reordering or removing work items in a notebook, when a `read_notebook` result is too large or unreadable, when `edit_notebook` returns an error, or when asked to "track this in current-work", "mark that done", "delete that task" or "what's open across projects". Not for the session-start ritual as a whole (council-hub-workflow), room synthesis and cleanup (council-hub-janitor), or the timeline of decisions in one project, which is plain `read_notebook(project=...)` with no `notebook_id`.
---

# Council Hub notebooks and the `current-work` cockpit

A notebook is a hand-curated outline over the ledger. Rooms are the journal;
notebooks are the index you steer by. Notebooks are **node-local** and never
fan out across a cluster, so the cockpit lives on one node.

`current-work` is a convention, not a built-in: one global notebook (no
`project`, so every project lists it). Create it once with
`edit_notebook(action=create, notebook_id=current-work)`.

## Room, or task?

| Work is... | Add as | Closes by |
|---|---|---|
| A thread with its own discussion | `kind=room_ref`, `ref_id=<room_id>` | `signal_status(resolved)` on the room; the entry moves from In flight to Done by itself |
| A checklist item too small for a room | `kind=task`, `prose=<label>` | `start`, then `check` |
| A fact to keep in view | `kind=ref` (message) or `query_ref` (`room:type`, resolved live) | stays live; never goes stale |

Never hand-edit a `room_ref` to reflect a status change, and never hand-sort
tasks: both self-sort into In progress / Open / Done.

## Close a task with `check`

```
edit_notebook(action=check, entry_id=<full entry id>)
```

Do **not** call `add` with `prose="DONE 2026-09-22: shipped X"`. That makes a
new open task whose label happens to say DONE, so the cockpit still counts it
as open (reported 2026-09-21). Since v0.63.0 an `add` whose label starts with
DONE, a check mark or `[x]` returns a nudge, but it still adds the entry.
`uncheck` reopens; `update` rewrites a label without changing state.

Entry ids are the `*(entry #...)*` markers in `read_notebook(notebook_id=...)`.
Pass the **full** id: UUIDv7 prefixes collide within a millisecond.
`ref_id` is the room-id parameter for a `room_ref` (not `room_id`). Re-adding
a target that is already referenced returns the existing entry, so repeating
an `add` across sessions does not create duplicates. `ref_ids=[...]` adds a
batch and keeps the given order.

## Remove one entry vs delete a notebook

- `remove` + `entry_id` drops one entry.
- `delete` acts on the **whole** notebook and needs `notebook_id` plus
  `confirm` set to the notebook id or its entry count. It is a soft delete:
  a markdown snapshot goes under `archives/notebooks/`, `restore` undoes it,
  `purge` (also confirmed) is permanent. It rejects `entry_id`, because
  before v0.63.0 `delete` with an `entry_id` silently removed an entire
  419-entry `current-work` (2026-10-18).
- Every write reports entry counts before and after. If the count dropped by
  more than you meant, `restore` at once.
- Never pass `confirm` to make an error go away. Read the count it asks for.

## Reading a big notebook

An unfiltered `current-work` on a mature hub is 100-170K characters and
overflows the client. Ask for less, in this order:

```
read_notebook(notebook_id=current-work, status=open, level=1)       # what is open, headings only
read_notebook(notebook_id=current-work, status=open, project=atlas) # one project's open items
read_notebook(notebook_id=current-work, entry_id=<id>)              # one entry in full
read_notebook(notebook_id=current-work, status=open, limit=20)
```

`status` (`open|doing|done`) and `project` filter before entries resolve, so
they are cheap. `level=1` also clips task labels and room_ref topics to their
first sentence. If a call overflows anyway, the harness says "read the saved
file"; retry with these parameters instead of forking a subagent onto the file
(that cost one session about 625K tokens, 2026-09-27).

Without `notebook_id`, `read_notebook(project=...)` is the project **timeline**
of typed messages (decision, action, synthesis by default), a different thing.
Messages typed `message` never appear there; use `note` if nothing else fits.

## Gotchas

- The `current-work` entry you want may be done: `status=open` hides it. Drop
  the filter before concluding it does not exist.
- A `room_ref` is open until its room resolves. A paused room still reads as
  In flight.
- A soft-deleted notebook disappears from listings and from the janitor's
  "no notebook" nudge. `restore` is the only way back short of the archive file.
