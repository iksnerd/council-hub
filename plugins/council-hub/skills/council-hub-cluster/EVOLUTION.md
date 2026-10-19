# council-hub-cluster - change log

## 2026-10-20: 1.0.0 created

- **Trigger:** Owner asked for a cluster skill if evidence justified it. Room council-hub-mcp-feedback: shadow rooms (#01a0b10c), link_messages cannot cross nodes (#01a0b10b), signal_status proxy (#01a01658), identity drift incident (#019fd0f2), cluster_wide reads (#019fd0e4, corrected by #019fd0ee), split cluster via cookie rotation.
- **Class:** missing. Cluster facts lived in a workflow section that had drifted (see council-hub-workflow entry).
- **Change:** New skill: cross-node map per tool, shadow-room trap, 'clean result is not proof', symptom table from /health fields (cluster_nodes, node_identity_warning, advertised_warning, seed_warning). Facts checked against CHANGELOG [0.63.0], tools_register.go, cluster_writes.go and CLAUDE.md's cluster section.
- **Evidence:** rubric v2, self-scored by the editing agent (not independent): Self-scored ~80 content, 90 with evals. Trigger evals: 9/9 loads, 0/6 false loads, outcome 12/15. Evals run 2026-10-20 with `claude plugin eval plugins/council-hub --runs 3 --ablation none`, final run; first run was 61/63 loads and 0/6 false loads per skill. Outcome graders are judged in a sandbox with no Council Hub server, so they grade the stated approach.
- **Outcome:** Accepted.
