# council-hub-project-suggestions - change log

## 2026-10-20: wrong parameters, boundaries, meta-feedback convention

- **Trigger:** Owner request 'improve our skills marketplace'; checked against tools_register.go.
- **Class:** wrong. The ritual called `read_room(id=..., last_n=10)`; read_room takes `room_id` and `include_last_n`. It also hardcoded author `claude` (conflicts with council-hub-multi-agent), ran the unfiltered notebook and digest calls that overflow, and told readers to post to a room named meta-feedback when the convention is the tag.
- **Change:** Fixed the ritual (points to council-hub-workflow for the full form), stable-author placeholder, pin guard note, list_rooms pre-check before get_or_create_room, meta-feedback found via tag with message-level `tags=meta-feedback`; description gained a Not-for clause.
- **Evidence:** rubric v2, self-scored by the editing agent (not independent): 68 -> 78 content (A3 +6, B1 +2, C2 +2), 88 with evals. Trigger evals: 9/9 loads, 0/6 false loads, outcome 13/15. Evals run 2026-10-20 with `claude plugin eval plugins/council-hub --runs 3 --ablation none`, final run; first run was 61/63 loads and 0/6 false loads per skill. Outcome graders are judged in a sandbox with no Council Hub server, so they grade the stated approach.
- **Outcome:** Accepted.
