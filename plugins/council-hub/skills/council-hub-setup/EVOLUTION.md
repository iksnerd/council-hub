# council-hub-setup — change log

## 2026-09-21 — 0.1.0: created

New skill. Server install and client registration had no coverage anywhere: the three existing council-hub skills all assume a connected server. Shipped with the repo's first plugin marketplace ({sha:0cb8f8b}). Validator clean, rubric v2 self-scored ~78, **unverified** (no trigger evals yet — capped at 89).

## 2026-09-21 — 0.1.1: correct the DHCP-drift troubleshooting entry

- **Trigger:** a peer session that had just hit the failure tested the mechanism and corrected the first-hand account this skill was written from.
- **Class:** wrong. The entry said the container "starts, passes its own healthcheck, and is unreachable on every port", implying Docker silently drops bindings. It does not: starting a container against a missing bind address fails loudly with `ports are not available: … bind: can't assign requested address`. The hazard is narrower and the wording hid it — it only reaches an **already-running** container whose address disappears underneath it, which under `restart: always` may never restart and so never surfaces the error.
- **Change:** rewrote the entry around the narrower mechanism, said that restarting by hand is the right instinct because it converts silence into the real error, and added why `docker ps` is the wrong check — the healthcheck runs inside the container (`wget http://localhost:4000`) and cannot observe host-side publishing, so "healthy" and "unreachable from every client" are compatible states.
- **Evidence:** rubric v2 (self-scored): ~78 → ~80 (B1 +1: the corrected mechanism is the non-obvious part, and the original wording would have sent a reader down a wrong path; C3 +1: names the action that surfaces the real error). Validator clean.
- **Outcome:** Accepted.
