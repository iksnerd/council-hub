# council-hub-workflow — change log

## 2026-10-20: stale cluster section, boundaries, pin guard

- **Trigger:** Owner request 'improve our skills marketplace'; CHANGELOG [0.63.0]; mcp-server cluster_writes.go.
- **Class:** wrong + incomplete. The 'Cluster nodes' section still said signal_status does NOT proxy; it has proxied to the owner node for several releases. Description had no 'Not for' clause (A3).
- **Change:** Replaced that section with a short corrected map that points to council-hub-cluster; added pin guard (12h, force), idempotency_key and reopen to Wrapping up; pointer to council-hub-notebook; description now names siblings. Edits kept local to those spots because another branch edits a different paragraph.
- **Evidence:** rubric v2, self-scored by the editing agent (not independent): 71 -> 75 content (A3 +4, B1 +2 corrected fact, B3 -2 net, body 185 -> 187 lines), 85 with evals. Trigger evals: 9/9 loads, 0/6 false loads, outcome 12/15. Evals run 2026-10-20 with `claude plugin eval plugins/council-hub --runs 3 --ablation none`, final run; first run was 61/63 loads and 0/6 false loads per skill. Outcome graders are judged in a sandbox with no Council Hub server, so they grade the stated approach.
- **Outcome:** Accepted.

## 2026-10-03 — 1.3.0: pinning in a shared room replaced another session's live pin

- **Trigger:** twice in `contra-bootstrap`, a room five sessions share. 2026-10-02: a `decision` posted with `pin=true` replaced a peer session's live "v0.2.0 scope agreed; main is frozen" decision, minutes after it was posted. 2026-10-03: an ADR 0023 `synthesis` with `pin=true` replaced another peer's review synthesis. Both restored by hand with `pin_message`. A meta-feedback post after the first (`council-hub-mcp-feedback`, 2026-10-02) did not stop the second.
- **Class:** incomplete. "Wrapping up" said only "Post a `synthesis` … and `pin=true` it … so the next reader gets the TL;DR first": right for a room one session owns, silent on a shared room, where a room's single pin may be someone else's live notice.
- **Change:** that bullet now says to read the current pin first in a shared room; if it is another author's and live, post unpinned and `reply_to` it; and treat the post's "replaced #…" line as the last chance to `pin_message` it back. Both incidents dated in the text.
- **Evidence:** rubric v2 (self-scored): ~81 → ~83 (B1 +1: a non-obvious rule, since the replacement is silent; B2 +1: two dated incidents). Body 164 → 171 lines. Validator clean before and after. The description already covers wrapping up with a pinned synthesis, so A is unchanged. The tool-side ask (refuse or warn when replacing another author's recent pin) stays in `council-hub-mcp-feedback`.
- **Outcome:** Accepted.

## 2026-09-27 — 1.2.0: the cheap-read advice was incomplete, and didn't cover overflow at all

- **Trigger:** council-hub-mcp-feedback `#01a0e214-1069`, filed by a session whose session-start ritual overflowed on both `get_digest` and `read_notebook` and had no better signal than the harness's generic "read the saved file" error — forking two subagents to read 288K/337K-token saved files (~625K tokens total) to answer "is there anything urgent."
- **Class:** stale + missing. The skill recommended `read_notebook(..., level=1)` alone, which is the weaker of the two shipped remedies (~81% reduction vs ~92% with `status=open` added, per v0.60.0's own measurement) — and said nothing about what to do when the call overflows anyway, which is exactly the failure this session hit.
- **Change:** the cheap-read line now says `level=1, status=open` together, with both measured percentages; added a new paragraph naming the harness's generic oversized-result error as the actual failure mode and its fix (retry with the size params, not fork a subagent onto the saved file), with the token cost this session paid for not knowing that.
- **Also this cycle:** the tool descriptions themselves (`get_digest`, `read_notebook`) were fixed in the same release (v0.62.0) to state this remedy up front — this skill edit is the second surface for the same fix, not a duplicate of it; a caller reading the tool description before the call and a caller reading this skill before a session are different audiences hitting the same gap.
- **Evidence:** rubric v2 (self-scored): ~79 → ~81 (B1 +1: corrects an incomplete recommendation and adds a previously-undocumented failure mode; B2 +1: one dated incident with a measured token cost). Body 161 → 169 lines. Validator clean before and after.
- **Outcome:** Accepted.

## 2026-09-21 — 1.1.0: `get_or_create_room` is node-local

- **Trigger:** current-work task `01a0b10b`. Hit 2026-09-17 in `iksnerd/adeloc`: a session created `adeloc-use-case-fit` while the real room, `adeloc-real-world-fit`, lived on a peer node. Two rooms, same subject, neither visible to the other.
- **Class:** wrong. The skill said `get_or_create_room` "returns existing content if a room already exists under that name, so it's safe to call even when unsure." That reassurance is true locally and false across a cluster — the call does not fan out, so it matches nothing on a peer-owned room and creates a local shadow. The sentence actively encouraged the failure.
- **Change:** dropped the "safe to call even when unsure" clause. Added the node-local limit with the adeloc incident, `list_rooms(project=…, cluster_wide=true)` as the pre-check, and a second paragraph on why a clean cluster-wide result is not proof: fan-out reaches *connected* nodes only, so a peer that has dropped out is absent rather than unreachable, produces no warning, and the reply looks complete. Observed live the same day while writing this — `/health` showed one `cluster_nodes` entry during a deliberate cookie-rotation split, and `list_rooms(cluster_wide=true)` returned 30 rooms all tagged `[local]`.
- **Evidence:** rubric v2 (self-scored): ~76 → ~79 (B1 +2: removes a false guarantee and documents a silent-degradation path the tool's own docs imply is warned about; B2 +1: two dated incidents, one first-hand). Body 133 → 158 lines, which costs a little on D1. Validator clean before and after.
- **Outcome:** Accepted.

*Skill moved into `iksnerd/council-hub` from `iksnerd/skills` in {sha:0cb8f8b}; this log starts there.*

## 2026-10-20: session-start calls use the size-safe parameters

- **Trigger:** council-hub-mcp-feedback, six independent reports (#01a0b11a, #01a0c4e6, #01a0ff51, #01a102e2, #01a14ee8, #01a1508d): the three-call ritual overflowed the client's result limit on a long-lived hub, and `get_digest` rejected the `limit` it advertised.
- **Class:** incomplete. The ritual named the calls but not the parameters that keep them readable.
- **Change:** the ritual block now passes `limit=15` to `get_digest` and `status=open, level=1` to `read_notebook`, with a short paragraph on `brief`, `limit=all`, `project` and `entry_id`. Needs server v0.63.0 (digest `limit`/`brief`, notebook `project`/`entry_id`).
- **Evidence:** not scored with skill-scorer in this session.
- **Outcome:** Accepted.

## 2026-10-20: eval failure burndown

- **Trigger:** full eval run 2026-10-20, fires-wrapup outcome 0/3 earlier, 2/3 on rerun; failing runs stopped at 'reconnect' without the wrap-up sequence.
- **Class:** incomplete (no behavior defined when the server is absent).
- **Change:** Added a 'No server connected' section naming the wrap-up (action with SHA, pinned synthesis after reading the existing pin, signal_status, mark_read) and start/state reads.
- **Evidence:** Eval fires-wrapup before 2/3, after 3/3. Three runs each, `--ablation none`; not scored with skill-scorer.
- **Outcome:** Accepted.
