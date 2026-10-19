# council-hub-multi-agent — change log

## 2026-10-20: v0.63.0 coordination features

- **Trigger:** CHANGELOG [0.63.0]: room-scoped prefixes, links, idempotency_key, 12h pin guard, reopen, mark_read neighbour warning, cluster-wide get_mentions.
- **Class:** incomplete. 'Cross-node' section said reads are opt-in and called get_or_create_room's shadow risk without the new defaults; no mention of the pin guard or room#id.
- **Change:** Added the mark_read substring warning, room-scoped prefixes and room#id, `links=` and `idempotency_key` on a post, the 12h pin guard with force, reopen, and rewrote the cross-node paragraph around the new defaults with a pointer to council-hub-cluster. Mention matching is unchanged in code (LOWER(mentions) LIKE) and the existing text stays correct.
- **Evidence:** rubric v2, self-scored by the editing agent (not independent): 81 -> 83 content (B1 +1, C2 +1), 93 with evals. Trigger evals: 8/9 loads (one miss on 'get_mentions overmatch' in run 2, 3/3 in run 1), 0/6 false loads, outcome 14/15. Evals run 2026-10-20 with `claude plugin eval plugins/council-hub --runs 3 --ablation none`, final run; first run was 61/63 loads and 0/6 false loads per skill. Outcome graders are judged in a sandbox with no Council Hub server, so they grade the stated approach.
- **Outcome:** Accepted.

## 2026-09-21 — 1.0.0: created

- **Trigger:** asked whether a peer-collaboration skill belonged in the council-hub plugin. It did not — that one is about agent sessions sharing a machine and a git checkout — but the question surfaced a real gap: nothing covered several agents in one *room*. `council-hub-workflow` is one agent logging its own work, `council-hub-project-suggestions` is room setup, `council-hub-janitor` is cleanup afterwards.
- **Grounded in the live ledger rather than intuition.** 169 of 263 rooms with messages have more than one author, so this is the normal case, not an edge case — yet the coordination features are barely touched: 37 mentions and 172 `reply_to` across 2,286 messages, 41 `plan` handoffs.
- **The load-bearing finding is author identity**, which was verified against the code rather than assumed. `GetMentions` matches `LOWER(mentions) LIKE '%<author>%'` — a case-insensitive *substring*, not an exact match. The database holds the same few tools under a dozen names (`claude` 875, `claude-code` 520, `Claude Code (Opus 4.8)` 28, `Gemini CLI` 90 vs `gemini-cli` 87). So `get_mentions("claude")` over-matches every claude-prefixed agent, while `get_mentions("gemini-cli")` fails to match `Gemini CLI` because a hyphen is not a space. Read cursors are keyed by the same string — 16 cursor agents for about five tools — so a renamed agent replays its whole backlog. An initial draft asserted exact matching; reading `messages_sync.go` corrected it, and the substring behaviour turned out to be the more interesting failure.
- **Also covers:** mentions as an inbox that must be answered or declined, `reply_to` threading, `plan` for handoff and a claiming post before starting, `critique` rather than overwriting, `supersedes` so a room never holds two live syntheses (and the Knowledge Linter's `incoherent` flag that catches it), and not resolving or archiving a room another agent is still working in.
- **Evidence:** validator clean, 0 errors 0 warnings; plugin registry 5/5 valid. Rubric v2 (self-scored) ~81, **unverified** (no trigger evals — capped at 89). Every statistic in the body is a query against the live database, and the matching semantics are read from source.
- **Outcome:** Accepted.

## 2026-10-20: eval failure burndown

- **Trigger:** full eval run 2026-10-20, fires-get-mentions-overmatch loaded the skill in 1 of 3 runs; the body already explains the substring match.
- **Class:** trigger gap.
- **Change:** Description 'Use when' now names the symptom: get_mentions returning threads meant for a different or similarly named agent.
- **Evidence:** Eval fires-get-mentions-overmatch load before 1/3, after 3/3; outcome 2/3 (one judge FAIL FAIL FAIL on a correct answer, not fixed). Three runs each, `--ablation none`; not scored with skill-scorer.
- **Outcome:** Accepted.
