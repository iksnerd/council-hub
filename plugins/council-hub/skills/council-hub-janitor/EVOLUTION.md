# council-hub-janitor - change log

## 2026-10-20: v0.63.0 sweep, first log entry

- **Trigger:** Owner request 'improve our skills marketplace'. This log starts here; earlier history is in git.
- **Class:** stale + incomplete. `list_rooms` now hides resolved rooms by default, so the triage list never showed rooms ready to archive; the pin guard, reopen and cluster defaults were absent; no boundary clause; no worked example or gotchas.
- **Change:** Triage block uses check_room_health(dry_run), digest brief, status=resolved; added cluster ownership note (which janitor tools do not route), pin-guard guidance (do not force), a gotchas list and one worked example; description gained a Not-for clause. 81 -> 123 lines.
- **Evidence:** rubric v2, self-scored by the editing agent (not independent): 63 -> 76 content (A3 +6, B2 +3, E +3, C3 +1), 86 with evals. Trigger evals: 9/9 loads, 0/6 false loads, outcome 13/15. Evals run 2026-10-20 with `claude plugin eval plugins/council-hub --runs 3 --ablation none`, final run; first run was 61/63 loads and 0/6 false loads per skill. Outcome graders are judged in a sandbox with no Council Hub server, so they grade the stated approach.
- **Outcome:** Accepted.

## 2026-10-20: eval failure burndown

- **Trigger:** full eval run 2026-10-20, fires-tidy 0/3 -> 3/3 outcome. Agents loaded the skill but answered only 'no server connected, reconnect' with no plan.
- **Class:** incomplete (no behavior defined when the server is absent).
- **Change:** Added a 'No server connected' section: say so once, point to council-hub-setup, still state the triage plan; never fake the calls.
- **Evidence:** Eval fires-tidy before 0/3 (earlier run) and 1/3 (rerun 2026-10-20), after 3/3. Three runs each, `--ablation none`; not scored with skill-scorer.
- **Outcome:** Accepted.
