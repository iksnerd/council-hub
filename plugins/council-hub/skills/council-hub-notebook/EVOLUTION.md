# council-hub-notebook - change log

## 2026-10-20: 1.0.0 created

- **Trigger:** Owner asked for a notebook/current-work skill if papercut evidence justified it. Room council-hub-mcp-feedback holds at least eight reports: whole-notebook delete (#01a14ee7), closing a task by appending DONE (#01a0c654), read_notebook overflow (#01a0b6c1, #01a0c4cd, #01a0e214, #01a0ff51, #01a102e2), room_ref parameter spelling ref_id (#01a0abce), no dedupe on add (#019eda78).
- **Class:** missing. Notebook conventions were scattered across a workflow paragraph and tool descriptions.
- **Change:** New skill: room_ref vs task table, check not add DONE, remove vs delete with confirm/restore/purge, read parameters (status, level, project, limit, entry_id) with the overflow remedy. Every parameter checked against edit_notebook / read_notebook schemas and handler_notebook_outline.go.
- **Evidence:** rubric v2, self-scored by the editing agent (not independent): Self-scored ~80 content, 90 with evals. Trigger evals: 9/9 loads, 0/6 false loads, outcome 15/15. Evals run 2026-10-20 with `claude plugin eval plugins/council-hub --runs 3 --ablation none`, final run; first run was 61/63 loads and 0/6 false loads per skill. Outcome graders are judged in a sandbox with no Council Hub server, so they grade the stated approach.
- **Outcome:** Accepted.
