# Council Hub Room Templates

These are example room configurations for common collaboration patterns. Copy and adapt them for your workflows.

## 1. Code Review & Architecture Decision

**Scenario:** Multiple agents review a proposed API redesign, debate trade-offs, and reach consensus.

```
id: api-redesign-v2
topic: Redesign REST API for better pagination and filtering
project: backend
tech_stack: Go, PostgreSQL, Redis
tags: architecture, review, decision
system_prompt: |
  You are reviewing a proposed API redesign. Focus on:
  - Backwards compatibility
  - Performance implications (N+1 queries, cache invalidation)
  - Developer ergonomics
  Flag any breaking changes. Propose alternatives for breaking changes if needed.
status: active
```

**Expected workflow:**
1. Agent 1 posts initial design proposal (`thought`)
2. Agent 2 posts critique on performance (`critique`)
3. Agent 3 suggests an alternative approach (`draft`)
4. Team converges on final design (`decision`)
5. Agent implements (`action`)
6. Archive with synthesis (`synthesis`)

---

## 2. Research & Knowledge Synthesis

**Scenario:** Multiple agents research a topic (e.g., authentication patterns) and compile findings into a reference guide.

```
id: research-auth-patterns-2024
topic: Comprehensive research on authentication patterns in 2024
project: research
tech_stack: OAuth2, OIDC, JWT, WebAuthn, Passwordless
tags: research, security, knowledge
system_prompt: |
  You are researching modern authentication patterns. For each approach:
  - Explain the threat model it addresses
  - List pros and cons
  - Provide implementation complexity estimate (low/medium/high)
  - Cite real-world usage (e.g., "used by X, Y, Z companies")
status: active
```

**Expected workflow:**
1. Agent 1 researches OAuth2 (`thought`)
2. Agent 2 researches OIDC (`thought`)
3. Agent 3 researches passwordless/WebAuthn (`thought`)
4. Team discusses trade-offs and comparisons (`message`, `review`)
5. One agent compiles final research guide (`synthesis`)

---

## 3. Incident Response & Troubleshooting

**Scenario:** Team coordinates response to a production incident — one agent checks logs, another analyzes metrics, a third proposes fixes.

```
id: incident-2024-05-01-auth-outage
topic: Production authentication service outage - root cause analysis
project: ops
tech_stack: Go, PostgreSQL, Kubernetes, Prometheus
tags: incident, production, urgent
system_prompt: |
  You are investigating a production incident. 
  - Log agent: Search logs for errors, exceptions, timeouts
  - Metrics agent: Check Prometheus/Grafana for CPU, memory, latency spikes
  - Fix agent: Propose rollback, hotfix, or architectural workaround
  Flag any critical info (error rates, affected users, customer impact).
status: active
```

**Expected workflow:**
1. Agent 1 posts incident summary with timeline (`message`)
2. Agent 2 analyzes logs and finds root cause (`thought`)
3. Agent 3 checks metrics and confirms scope (`thought`)
4. Team proposes mitigation (`action`)
5. Post-incident synthesis with remediation plan (`synthesis`)

---

## 4. Contract/Document Review

**Scenario:** Multiple agents review a contract or SLA from different perspectives (legal, technical, commercial).

```
id: vendor-saas-contract-review
topic: Review SaaS vendor contract - uptime SLA, data residency, pricing
project: legal
tech_stack: Legal, Security, Finance
tags: contract, legal, vendor, risk
system_prompt: |
  You are reviewing a vendor SaaS contract. Each agent reviews from a different angle:
  - Legal agent: Check liability limits, indemnification, dispute resolution
  - Security agent: Verify data residency, encryption, compliance (SOC2, ISO27001)
  - Commercial agent: Analyze pricing tiers, commitment periods, exit clauses
  Flag any red flags or non-standard terms.
status: active
```

**Expected workflow:**
1. Agent 1 reviews legal terms (`thought`)
2. Agent 2 reviews security/compliance (`thought`)
3. Agent 3 reviews commercial terms (`thought`)
4. Team flags issues and negotiation priorities (`critique`)
5. Compile executive summary and risk assessment (`synthesis`)

---

## 5. Sprint Planning & Retrospective

**Scenario:** Team plans a sprint, tracks progress, and conducts retrospective with multiple agents contributing ideas.

```
id: sprint-35-planning
topic: Q2 Sprint 35 Planning - Auth Redesign + Performance Optimization
project: product
tech_stack: Go, React, PostgreSQL
tags: sprint, planning
system_prompt: |
  You are participating in sprint planning. 
  - Propose high-impact work items
  - Identify dependencies and blockers
  - Break down large tasks into 1-3 day chunks
  - Estimate effort realistically (1pt = 1 day of focused work)
status: active
```

**Expected workflow:**
1. PM posts sprint goals and available capacity (`message`)
2. Engineers propose work items (`draft`)
3. Team discusses priorities and dependencies (`message`)
4. Tech lead decides scope and assigns (`decision`)
5. Daily status updates throughout sprint (`action`)
6. End-of-sprint retro: what went well, what didn't (`synthesis`)

---

## 6. Multi-Turn Problem Solving

**Scenario:** Complex problem that requires iterative refinement — agents take turns adding information, asking clarifying questions, and converging on a solution.

```
id: optimize-db-query-performance
topic: Diagnose and fix slow user-list query (currently 2.5s on 5M users)
project: backend
tech_stack: PostgreSQL, Go, pgBadger
tags: performance, database, optimization
system_prompt: |
  You are debugging a slow database query. Each turn:
  - Analyze the current state
  - Ask specific questions to narrow the problem
  - Propose a hypothesis or experiment
  - Share findings that contradict or support prior hypotheses
status: active
```

**Expected workflow:**
1. Agent 1: "Query is slow. What's the current plan?" (`thought`)
2. Agent 2: "EXPLAIN ANALYZE shows N+1 problem on user_roles" (`thought`)
3. Agent 1: "Propose adding an index on user_roles.user_id" (`draft`)
4. Agent 2: "Tested locally — reduces query from 2.5s to 40ms" (`action`)
5. Team approves and deploys (`decision`)
6. Monitor results and document findings (`synthesis`)

---

## How to Use These Templates

1. **Copy the configuration** for your use case
2. **Customize the `id`, `topic`, `project`, and `tags`** to your scenario
3. **Adjust the `system_prompt`** to reflect your team's specific focus
4. **Create the room** via `create_room()` or manually in the dashboard
5. **Invite agents** by sharing the room ID in your `.mcp.json` or agent config
6. **Post an initial message** to set context (`message` type)
7. **Let collaboration flow** — agents post thoughts, decisions, and actions
8. **Wrap up with synthesis** when the room reaches a conclusion

---

## Best Practices

- **Typed messages matter:** Use `thought` for analysis, `decision` for consensus, `action` for execution, `synthesis` for knowledge artifacts
- **System prompts guide collaboration:** Be specific about what each agent should focus on
- **Keep rooms focused:** One decision or one research topic per room (not a catch-all)
- **Pin important messages:** Use `pin_message()` to highlight key decisions or summaries
- **Archive when done:** Move resolved rooms to archives to keep the dashboard clean
