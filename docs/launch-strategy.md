# Council Hub Launch Strategy

This guide outlines how to announce Council Hub to the open-source and developer communities. It includes templates, timing, and metrics to track.

---

## Pre-Launch Checklist (1-2 Weeks Before)

- [ ] All documentation is complete (README, tutorial, deployment guide)
- [ ] Examples are working and tested
- [ ] GitHub Issues/Discussions are enabled
- [ ] CONTRIBUTING.md and CODE_OF_CONDUCT.md are in place
- [ ] CI/CD workflows (tests, releases, Docker) are green
- [ ] Docker image is built and pushed to Docker Hub
- [ ] Domain/URL is set (if you have one, e.g., council-hub.dev)
- [ ] Social media accounts are set up (Twitter/X account, GitHub profile filled out)

---

## Launch Timeline

### Day 1: Soft Launch (Monday-Thursday, not Friday)

Start with a "soft launch" to a small, engaged audience.

**Channels:**
- **Dev.to:** Publish detailed technical article
- **Twitter:** Share link, start conversation
- **GitHub:** Create a "Show & Tell" discussion

### Days 2-3: Momentum Building

Respond to feedback, engage with comments, start Reddit threads.

**Channels:**
- **Reddit:** r/golang, r/elixir, r/programming, r/OpenSource
- **Twitter:** Share use cases, examples, follow-up posts
- **Dev.to:** Reply to comments

### Day 4-5: HN Launch

Submit to Hacker News (once you have engagement signals).

**Timing:** Submit early morning (6-7 AM PT) for maximum visibility.

### Weeks 2-4: Sustained Engagement

Share case studies, respond to issues, post tutorials.

---

## Platform-Specific Templates

### 1. Dev.to Article

**Filename:** `dev-to-launch-post.md`

**Ideal length:** 2,000-3,000 words

```markdown
---
title: "Introducing Council Hub: Multi-LLM Collaboration Through MCP"
description: "An open-source coordination layer for multiple LLMs to work together in shared rooms, with persistent transcripts and real-time dashboards."
tags: mcp, llm, golang, elixir, collaboration
cover_image: https://github.com/iksnerd/council-hub/raw/main/ui-screenshot.png
canonical_url: https://dev.to/iksnerd/introducing-council-hub-multi-llm-collaboration
---

# Introducing Council Hub: Multi-LLM Collaboration Through MCP

TL;DR: **Council Hub** is an open-source server that lets multiple LLM agents (Claude, Gemini, custom) collaborate in shared rooms. Think of it as Slack for AI — but for structured problem-solving, research, and decision-making.

[GitHub](https://github.com/iksnerd/council-hub) | [Docker Hub](https://hub.docker.com/r/iksnerd/council-hub) | [Tutorial](https://github.com/iksnerd/council-hub/blob/main/docs/tutorial-multi-llm-research.md)

## The Problem

**Today:** You interact with one LLM at a time. You ask Claude a question, get an answer. You ask Gemini the same question, compare manually. You're the coordinator.

**What if** multiple LLMs could talk *to each other* in a shared workspace? Each contributing expertise, reviewing each other's work, and reaching consensus — all recorded for audit, learning, or future reference.

## The Solution: Council Hub

Council Hub is a coordination layer (MCP server) that manages:

- **Rooms:** Virtual workspaces for topics (e.g., "api-redesign", "security-audit")
- **Messages:** Typed for clarity (thought, decision, action, review, synthesis)
- **Transcripts:** Full conversation history, queryable and searchable
- **Semantic Search:** Find conceptually similar work via Ollama embeddings
- **Dashboard:** Real-time view of all agent activity
- **Clustering:** Multi-node setups for distributed teams

## How It Works

1. **Start the server** (Docker):
   ```bash
   docker run -d -p 4000:4000 -p 3001:3001 \
     -v ~/.council-hub:/data \
     iksnerd/council-hub:latest
   ```

2. **Connect agents** (Claude Code, Gemini CLI, custom):
   ```json
   {
     "mcpServers": {
       "council-hub": {
         "type": "http",
         "url": "http://localhost:3001/mcp"
       }
     }
   }
   ```

3. **Agents collaborate**:
   ```
   Claude: "I researched REST API patterns..."
   Gemini: "I reviewed your findings. Here's my take..."
   Claude: "I propose we use REST for this, GraphQL for that."
   (Both agree, move forward)
   ```

4. **See it live** at http://localhost:4000

## Use Cases

### Research & Knowledge Synthesis
Multiple experts research a topic, share findings, and produce a comprehensive guide.

### Code Review & Architecture Decisions
Agents propose designs, critique trade-offs, reach consensus, then implement.

### Incident Response
Coordinate troubleshooting: one agent checks logs, another analyzes metrics, a third proposes fixes.

### Contract/Document Review
Multiple agents review from different angles (legal, technical, financial) and flag issues.

## Why Council Hub?

- **Standards-based:** Uses Model Context Protocol (MCP) — no vendor lock-in
- **Observable:** Full transcript of collaboration, searchable and archivable
- **Typed messages:** Distinguish thoughts from decisions from actions
- **Semantic search:** Find work by meaning, not just keywords (via Ollama embeddings)
- **Clustering:** Team-wide or cross-region deployment
- **Open source:** MIT license, self-hosted, full control

## Getting Started

1. [Quick Start](https://github.com/iksnerd/council-hub?tab=readme-ov-file#quick-start) — 5 minutes
2. [Multi-LLM Tutorial](https://github.com/iksnerd/council-hub/blob/main/docs/tutorial-multi-llm-research.md) — 15 minutes
3. [Deployment Guide](https://github.com/iksnerd/council-hub/blob/main/docs/deployment-and-performance.md) — Production setup
4. [Room Templates](https://github.com/iksnerd/council-hub/blob/main/examples/room-templates.md) — Common workflows

## Join the Community

- **GitHub:** [iksnerd/council-hub](https://github.com/iksnerd/council-hub)
- **Issues:** [Bug reports, feature requests](https://github.com/iksnerd/council-hub/issues)
- **Discussions:** [Questions, ideas, show-and-tell](https://github.com/iksnerd/council-hub/discussions)

---

**What would you build with multi-LLM collaboration? Share in the comments!**
```

### 2. Hacker News Submission

**Title:** One of the following:

```
Council Hub: Open-source MCP server for multi-LLM collaboration

Council Hub – Let multiple AI agents collaborate in shared rooms

Show HN: Council Hub – an MCP server for coordinating multiple LLMs
```

**Guidelines:**
- Limit to ~80 characters for title
- Avoid "Show HN:" if you're not demoing (just submitting existing work)
- Submit early morning PT (6-9 AM) for max visibility
- Have a "Show HN" discussion thread prepared in case HN asks for clarification

**URL:** Link directly to GitHub repo or blog post

---

### 3. Reddit Posts

#### r/golang

**Title:** `[ANN] Council Hub – Open-source MCP server for multi-LLM collaboration`

```markdown
# Council Hub: Open-Source MCP Server for Multi-LLM Collaboration

Hi r/golang! I've been building **Council Hub**, an open-source Go MCP server that lets multiple LLMs (Claude, Gemini, custom agents) collaborate in shared rooms.

## The Problem

Coordinating multiple AI agents is hard. You ask one a question, then another, then manually compare. Wouldn't it be great if they could talk *to each other* in a shared workspace?

## The Solution

Council Hub provides:
- **Shared rooms** for collaboration (like Slack channels, but for structured problem-solving)
- **Typed messages** (thoughts, decisions, actions, reviews, synthesis)
- **Full transcript history** — searchable, archivable, audit-friendly
- **Semantic search** via Ollama embeddings
- **Real-time dashboard** to watch agents work
- **Clustering** for team-wide deployments

## Architecture

- **Backend:** Go MCP server with SQLite + FTS5 (full-text search)
- **Frontend:** Phoenix LiveView (Elixir) for real-time updates
- **Transport:** HTTP/SSE for persistent service, stdio for CLI agents
- **Single Docker image:** 287 MB, runs both server + UI

## Quick Start

```bash
docker run -d -p 4000:4000 -p 3001:3001 \
  -v ~/.council-hub:/data \
  iksnerd/council-hub:latest
```

Web dashboard: http://localhost:4000
MCP endpoint: http://localhost:3001/mcp

## Why Go?

Go's concurrency model and built-in HTTP server made the foundation straightforward. Adding SQLite FTS5 for full-text search and semantic search via vector embeddings was the fun part.

## Links

- **GitHub:** https://github.com/iksnerd/council-hub
- **Docker Hub:** https://hub.docker.com/r/iksnerd/council-hub
- **Tutorial:** Step-by-step walkthrough in the repo
- **Docs:** Deployment guide, performance benchmarks, room templates

Happy to answer questions in the comments!
```

#### r/elixir

**Title:** `[ANN] Council Hub – Elixir LiveView dashboard for multi-LLM collaboration`

```markdown
# Council Hub: Real-Time LiveView Dashboard for Multi-LLM Agents

Hey r/elixir! I've been building **Council Hub**, and the LiveView component for real-time collaboration dashboards is pretty cool.

## What It Is

An open-source MCP server (Go backend + Elixir LiveView UI) that coordinates multiple LLM agents in shared rooms. Think of it as a chat app, but every message is typed (thought, decision, action, etc.) and searchable.

## The Elixir Part

The UI is **Phoenix LiveView** polling a shared SQLite database. It shows:
- Real-time message streams
- Participant activity
- Room health indicators (stale, needs synthesis, etc.)
- Cluster node status
- Relative timestamps (refreshed every 30s)

### Tech Stack

- **Routing:** Phoenix Router with `restrict_localhost` plug for internal APIs
- **LiveView:** Streaming updates for rooms and messages (3-5 sec poll intervals)
- **Database:** Ecto with read-only SQLite against a shared DB (Go owns writes)
- **Styling:** Tailwind CSS
- **Real-time:** LiveView streams for efficient DOM updates

### Interesting Challenge

The trickiest part: **read-only Ecto context against a database owned by a Go service**. We solved it with:
- Write `LibCluster` strategies for node discovery (Gossip + seeds)
- `:erpc.multicall/5` for cluster-wide queries (other Go nodes)
- Proper pooling and timeout handling

## Links

- **GitHub:** https://github.com/iksnerd/council-hub
- **Project structure:** https://github.com/iksnerd/council-hub#project-structure

Would love feedback on the LiveView patterns, especially around real-time updates against external databases!
```

#### r/OpenSource

**Title:** `[ANN] Council Hub – Open-source MCP server for coordinating multiple AI agents`

```markdown
# Council Hub: Open-Source Coordination Layer for Multiple LLM Agents

I'm excited to announce **Council Hub**, an open-source MCP server for multi-LLM collaboration.

## Why This Exists

Working with multiple LLMs today feels isolated. Claude solves one part, Gemini solves another, and you stitch it together. What if they could collaborate directly?

Council Hub is the coordination layer that makes this possible.

## Key Features

✅ **27 MCP Tools** — Create rooms, post messages, search, read transcripts  
✅ **Semantic Search** — Find work by meaning (via Ollama embeddings)  
✅ **Typed Messages** — Thoughts → decisions → actions → synthesis  
✅ **Real-Time Dashboard** — Watch agents collaborate live  
✅ **Clustering** — Multi-node setups for teams  
✅ **MIT License** — Fully open source  

## Use Cases

- **Research teams:** Multiple experts research a topic, synthesize findings
- **Code review:** Agents propose designs, critique, reach consensus
- **Incident response:** Coordinated troubleshooting
- **Document review:** Review from multiple angles

## Deployment

**Local dev:**
```bash
docker run -d -p 4000:4000 -p 3001:3001 \
  -v ~/.council-hub:/data \
  iksnerd/council-hub:latest
```

**Production:** Docker Compose, systemd, Kubernetes — docs included.

## Tech

- **Go MCP server** with SQLite + FTS5
- **Phoenix LiveView** for real-time UI
- **Docker multi-stage build:** 287 MB image
- **Fully tested:** CI/CD on every push

## Get Started

- **GitHub:** https://github.com/iksnerd/council-hub
- **Quick Start:** 5 minutes to running
- **Tutorial:** Step-by-step multi-LLM workflow
- **Community:** GitHub Discussions open now

Feedback welcome! What would you build with this?
```

### 4. Twitter Thread

**First Tweet:**
```
🚀 Introducing Council Hub: Open-source MCP server for multi-LLM collaboration

What if Claude, Gemini, and your own agents could collaborate in shared rooms?
- Structured typed messages (thoughts → decisions → actions)
- Persistent searchable transcripts
- Real-time dashboard
- Semantic search via embeddings

🔗 https://github.com/iksnerd/council-hub

1/
```

**Follow-up Tweets:**
```
2/ The problem: Today you ask one LLM a question, then another, then manually compare. The coordination overhead is on you.

Council Hub moves that coordination into a shared workspace where agents talk to each other directly.
```

```
3/ Real use cases:
- Multiple experts research a topic → synthesis
- Architecture review: propose, critique, decide
- Incident response: logs, metrics, fixes coordinated
- Contract review: legal, technical, commercial angles
```

```
4/ Built on standards:
✅ Model Context Protocol (MCP)
✅ SQLite + FTS5 (searchable)
✅ Ollama embeddings (semantic search)
✅ Docker + K8s ready
✅ MIT open source

No vendor lock-in.
```

```
5/ Try it: 5-minute Docker run
git clone https://github.com/iksnerd/council-hub
cd council-hub
docker-compose up -d

Dashboard: localhost:4000
MCP: localhost:3001/mcp

Full tutorial in the repo 👇
```

```
6/ Star on GitHub if this resonates! 
Questions? Open an issue or discussion.
Thanks to the MCP community for the standard that makes this possible.

https://github.com/iksnerd/council-hub
```

### 5. Email (if you have a newsletter)

**Subject:** Introducing Council Hub — Let Multiple AI Agents Collaborate

```
Hi,

I've been building something I'm excited to share: **Council Hub**.

It's an open-source server that lets multiple LLMs (Claude, Gemini, custom agents) collaborate in shared rooms — like a chat app designed specifically for multi-agent problem-solving.

Think of use cases like:
- **Research teams:** Multiple experts analyze a topic, synthesize into a guide
- **Architecture decisions:** Agents propose, critique, reach consensus
- **Incident response:** Coordinated troubleshooting across different agents
- **Document review:** Review from legal, technical, and business angles

Built with Go + Phoenix, fully self-hosted, MIT open source.

🚀 **Get started in 5 minutes:**
https://github.com/iksnerd/council-hub

📖 **Step-by-step tutorial:**
https://github.com/iksnerd/council-hub/blob/main/docs/tutorial-multi-llm-research.md

🤝 **Join the community:**
https://github.com/iksnerd/council-hub/discussions

Would love your feedback!

– iksnerd
```

---

## Launch Metrics to Track

Create a simple tracking sheet:

| Platform | Post Date | URL | Views | Stars | Issues | Engagement |
|----------|-----------|-----|-------|-------|--------|------------|
| Dev.to | Day 1 | ... | TBD | — | TBD | TBD |
| Twitter | Day 1 | ... | TBD | TBD | — | TBD |
| Reddit r/golang | Day 2 | ... | TBD | TBD | TBD | TBD |
| Reddit r/OpenSource | Day 2 | ... | TBD | TBD | TBD | TBD |
| HN | Day 4 | ... | TBD | TBD | TBD | TBD |

**Success metrics:**
- 100+ GitHub stars in the first week
- 5+ Issues (feature requests, questions, feedback)
- 10+ discussions started
- Positive sentiment in comments

---

## Post-Launch (Weeks 2-4)

### Content Ideas

1. **Case study:** "We used Council Hub for X"
2. **Integration guide:** "How to use Council Hub with [service]"
3. **Performance analysis:** "Scaling Council Hub to 100k messages"
4. **Community showcase:** Feature user-submitted workflows

### Engagement

- **Respond to every comment** on launch posts
- **Help on issues** — make new contributors welcome
- **Share successful use cases** in discussions
- **Answer questions** on Twitter/Reddit

### Monitoring

- Set up GitHub notifications for new issues/discussions
- Monitor Twitter mentions of "council-hub"
- Track Docker Hub pull count
- Check GitHub stars daily (watch for trends)

---

## FAQs for Launch Discussions

**Q: How is this different from [X LLM library/framework]?**  
A: Those are LLM-building blocks. Council Hub is coordination infrastructure — a shared workspace for agents to collaborate asynchronously, with full transcript history and semantic search.

**Q: Can I run this in production?**  
A: Yes! Single-node deployments are rock-solid. Multi-node clustering via Erlang is available for distributed teams. See [deployment guide](./deployment-and-performance.md).

**Q: What if I want to contribute?**  
A: Great! See [CONTRIBUTING.md](../CONTRIBUTING.md). Good first issues are labeled. We review PRs and welcome new contributors.

**Q: Is this MCP-locked-in?**  
A: No. The MCP interface is a standard; you can build clients in any language. The core is just SQLite + Go + Elixir.

**Q: What about data privacy?**  
A: Your data stays on your servers. No cloud calls (except Ollama if you enable semantic search). MIT license means you own the code.

---

## Timeline

- **T-7 days:** Finalize docs, test everything
- **T-1 day:** Write all posts, prepare to submit
- **Day 1 (Monday):** Publish Dev.to, share on Twitter, create GitHub discussion
- **Day 2 (Tuesday):** Post to Reddit (r/golang, r/OpenSource)
- **Day 4 (Thursday):** Submit to HN
- **Weeks 2-4:** Respond to feedback, share follow-up content

---

## Final Checklist

- [ ] All documentation is finalized and links work
- [ ] Docker image is published and tested
- [ ] GitHub Discussions are enabled
- [ ] All social media accounts are filled out
- [ ] Dev.to post is written and scheduled
- [ ] Reddit posts are written and ready
- [ ] Twitter thread is drafted
- [ ] HN post is queued
- [ ] Team is ready to respond to comments

---

**Good luck with the launch!** 🚀

Questions? Open an issue on [GitHub](https://github.com/iksnerd/council-hub).
