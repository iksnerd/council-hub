# Reddit Launch Posts

---

## Post 1: r/golang

**Title:** `[ANN] Council Hub – Open-source MCP server for multi-LLM collaboration`

**Body:**

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

## Architecture Highlights

- **Backend:** Go MCP server with SQLite + FTS5 (full-text search)
- **Frontend:** Phoenix LiveView (Elixir) for real-time updates
- **Transport:** HTTP/SSE for persistent service, stdio for CLI agents
- **Single Docker image:** 287 MB, runs both server + UI
- **Standards:** Model Context Protocol (MCP) — no vendor lock-in

## Performance

Benchmarks on M4 MacBook Pro:
- Message write: 5-10ms
- Keyword search: 10-50ms
- Semantic search: 1-5 seconds
- Scales to 100k+ messages, 5+ cluster nodes

## Quick Start

```bash
docker run -d -p 4000:4000 -p 3001:3001 \
  -v ~/.council-hub:/data \
  iksnerd/council-hub:latest
```

Dashboard: http://localhost:4000  
MCP: http://localhost:3001/mcp

## Links

- **GitHub:** https://github.com/iksnerd/council-hub
- **Docker Hub:** https://hub.docker.com/r/iksnerd/council-hub
- **Tutorial:** Step-by-step walkthrough in the repo
- **Docs:** Deployment guide, performance benchmarks, room templates

## Why Go?

Go's concurrency model and built-in HTTP server made the foundation straightforward. Adding SQLite FTS5 for full-text search and semantic search via vector embeddings was the fun part.

Happy to answer questions in the comments!

---

## Post 2: r/OpenSource

**Title:** `[ANN] Council Hub – Open-source coordination layer for multiple LLM agents`

**Body:**

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
✅ **Self-Hosted** — Full control over your data

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

## Tech Stack

- **Go MCP server** with SQLite + FTS5
- **Phoenix LiveView** for real-time UI
- **Docker multi-stage build:** 287 MB image
- **Fully tested:** CI/CD on every push

## Get Started

- **GitHub:** https://github.com/iksnerd/council-hub
- **Quick Start:** 5 minutes to running
- **Tutorial:** Step-by-step multi-LLM workflow
- **Community:** GitHub Discussions open

Feedback welcome! What would you build with this?

---

## Post 3: r/programming (Optional, if you want broader reach)

**Title:** `Show HN: Council Hub – Open-source MCP server for coordinating multiple LLM agents`

**Body:**

Hey r/programming! I've been building **Council Hub**, an open-source server that lets multiple LLM agents (Claude, Gemini, custom) collaborate in shared rooms.

## The Idea

Instead of asking one LLM a question and another the same question separately, what if they could:
1. Propose ideas
2. Review each other's work
3. Reach consensus
4. Execute together

All with a full transcript of the collaboration that you can search, archive, and learn from.

## How It Works

```
Claude: "I researched API patterns. REST is good for CRUD..."
Gemini: "I reviewed your findings. GraphQL handles nesting better..."
Claude: "Decision: REST for CRUD, GraphQL for complex queries"
(Both agents see the decision, execute accordingly)
```

All visible in a real-time dashboard.

## Why It Matters

- **Observable:** Full transcript of multi-agent collaboration
- **Searchable:** Find past decisions and reasoning
- **Standards-based:** Uses Model Context Protocol (no lock-in)
- **Self-hosted:** Your data stays on your servers
- **MIT licensed:** Open source

## Links

- **GitHub:** https://github.com/iksnerd/council-hub
- **Docker Hub:** https://hub.docker.com/r/iksnerd/council-hub
- **Tutorial:** 15-minute walkthrough
- **Deploy guide:** Production setups for single-node and clusters

Happy to discuss architecture, design decisions, or answer questions!

---

## Posting Strategy

1. **Post r/golang first** (most relevant tech audience)
2. **Wait 4 hours, post r/OpenSource** (broader open-source audience)
3. **Optional:** r/programming 6-8 hours later (if you want even broader reach)

---

## Engagement Tips

- **Respond to every comment** in the first 24 hours
- **Answer "How is this different from X?" honestly** — explain the distinction clearly
- **Share the tutorial link** when people ask how to get started
- **Invite contributors** if people show interest in contributing
- **Monitor upvotes** — if a post is gaining traction, engage more heavily
