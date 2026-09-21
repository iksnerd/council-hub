# council-hub-workflow — change log

## 2026-09-21 — 1.1.0: `get_or_create_room` is node-local

- **Trigger:** current-work task `01a0b10b`. Hit 2026-09-17 in `iksnerd/adeloc`: a session created `adeloc-use-case-fit` while the real room, `adeloc-real-world-fit`, lived on a peer node. Two rooms, same subject, neither visible to the other.
- **Class:** wrong. The skill said `get_or_create_room` "returns existing content if a room already exists under that name, so it's safe to call even when unsure." That reassurance is true locally and false across a cluster — the call does not fan out, so it matches nothing on a peer-owned room and creates a local shadow. The sentence actively encouraged the failure.
- **Change:** dropped the "safe to call even when unsure" clause. Added the node-local limit with the adeloc incident, `list_rooms(project=…, cluster_wide=true)` as the pre-check, and a second paragraph on why a clean cluster-wide result is not proof: fan-out reaches *connected* nodes only, so a peer that has dropped out is absent rather than unreachable, produces no warning, and the reply looks complete. Observed live the same day while writing this — `/health` showed one `cluster_nodes` entry during a deliberate cookie-rotation split, and `list_rooms(cluster_wide=true)` returned 30 rooms all tagged `[local]`.
- **Evidence:** rubric v2 (self-scored): ~76 → ~79 (B1 +2: removes a false guarantee and documents a silent-degradation path the tool's own docs imply is warned about; B2 +1: two dated incidents, one first-hand). Body 133 → 158 lines, which costs a little on D1. Validator clean before and after.
- **Outcome:** Accepted.

*Skill moved into `iksnerd/council-hub` from `iksnerd/skills` in {sha:0cb8f8b}; this log starts there.*
