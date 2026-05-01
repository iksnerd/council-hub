# Discord Launch Announcements

Post these in relevant Discord communities. Adjust channel names/links as needed for each server.

---

## Template 1: Golang Discord Communities

**Channel:** #announcements or #projects

```
🚀 **Council Hub** – Open-source MCP server for multi-LLM collaboration

Hi everyone! I've just launched Council Hub, a Go-based MCP server that lets multiple LLM agents (Claude, Gemini, custom) collaborate in shared rooms.

**The idea:** Instead of asking one LLM a question and another separately, they can work together in a shared workspace, review each other's work, and reach consensus.

**What it does:**
✅ Shared rooms for collaboration
✅ Typed messages (thoughts, decisions, actions)
✅ Full searchable transcripts
✅ Semantic search via Ollama embeddings
✅ Real-time dashboard
✅ Clustering for team deployments

**Tech Stack:**
- Go MCP server with SQLite + FTS5
- Phoenix LiveView (Elixir) for real-time UI
- Single Docker image, arm64 + amd64 ready

**Get Started (5 minutes):**
```bash
docker run -d -p 4000:4000 -p 3001:3001 \
  -v ~/.council-hub:/data \
  iksnerd/council-hub:latest
```

Dashboard: localhost:4000
MCP: localhost:3001/mcp

**Links:**
🔗 GitHub: https://github.com/iksnerd/council-hub
📖 Tutorial: https://github.com/iksnerd/council-hub/blob/main/docs/tutorial-multi-llm-research.md
🐳 Docker: https://hub.docker.com/r/iksnerd/council-hub

Happy to answer questions! Feedback & contributions welcome 🎉
```

---

## Template 2: LLM/AI Discord Communities

**Channel:** #tools or #projects

```
📢 **Council Hub** – Coordinate multiple LLM agents in shared rooms

Just launched Council Hub, an open-source tool for multi-LLM collaboration.

**The problem:** Working with multiple LLMs (Claude, Gemini, etc.) is fragmented. You get answers from each separately, then manually coordinate.

**The solution:** A shared workspace where:
- Claude researches one angle
- Gemini researches another
- They review each other's work
- They reach consensus
- All recorded in a searchable transcript

**Key Features:**
🏠 Virtual rooms for topics (like Slack channels)
💬 Typed messages (thoughts, decisions, actions, synthesis)
🔍 Semantic search (find by meaning, not just keywords)
📊 Real-time dashboard
🌐 Clustering for distributed teams
📜 Full transcript history

**Try it:**
- GitHub: https://github.com/iksnerd/council-hub
- Docker: `docker run -d -p 4000:4000 -p 3001:3001 -v ~/.council-hub:/data iksnerd/council-hub:latest`
- Tutorial: https://github.com/iksnerd/council-hub/blob/main/docs/tutorial-multi-llm-research.md

Self-hosted, MIT licensed, no vendor lock-in.

Questions? Happy to discuss! ⭐
```

---

## Template 3: Elixir Discord Communities

**Channel:** #announcements or #projects

```
🎉 **Council Hub** – Phoenix LiveView dashboard for multi-LLM collaboration

Hey Elixir friends! I just launched Council Hub, and the LiveView component is pretty cool.

**Backend:** Go MCP server  
**Frontend:** Phoenix LiveView (Elixir) polling SQLite  
**Use case:** Real-time dashboard for coordinating multiple LLM agents

**The Elixir Part:**
The UI is a Phoenix LiveView that shows:
- Real-time message streams
- Participant activity
- Room health indicators
- Cluster node status
- Relative timestamps (refreshed every 30s)

Interesting technical challenge: **read-only Ecto context against a database owned by a Go service**. We solved it with LibCluster for discovery + `:erpc.multicall/5` for cluster queries.

**Links:**
- GitHub: https://github.com/iksnerd/council-hub
- Architecture docs: https://github.com/iksnerd/council-hub/blob/main/CLAUDE.md

Feedback on LiveView patterns welcome! 👂
```

---

## Template 4: Open Source Discord Communities

**Channel:** #announcements or #new-projects

```
🚀 **Council Hub** – Open-source MCP server for multi-agent LLM collaboration

Launching Council Hub, an open-source project that lets multiple LLM agents collaborate in shared rooms.

**Why:** Coordinating multiple LLMs today is manual and fragmented. Council Hub provides a shared workspace where agents can work together directly.

**Features:**
✅ 27 MCP tools for room/message management
✅ Typed messages (thoughts, decisions, actions, synthesis)
✅ Persistent searchable transcripts
✅ Semantic search via Ollama embeddings
✅ Real-time dashboard
✅ Multi-node clustering

**Tech:**
- Go + Elixir + SQLite
- Single Docker image
- Self-hosted, MIT license

**Quick Start:**
```bash
docker run -d -p 4000:4000 -p 3001:3001 \
  -v ~/.council-hub:/data \
  iksnerd/council-hub:latest
```

**Resources:**
📍 GitHub: https://github.com/iksnerd/council-hub
📚 Tutorial: https://github.com/iksnerd/council-hub/blob/main/docs/tutorial-multi-llm-research.md
🐳 Docker Hub: https://hub.docker.com/r/iksnerd/council-hub

⭐ Star on GitHub if you like it!
```

---

## Posting Strategy

1. **Post immediately** in Elixir + Golang communities (most relevant)
2. **Post in LLM/AI communities** within 2-4 hours
3. **Post in Open Source communities** 6-12 hours later
4. **Be available to answer questions** for 24-48 hours after posting
5. **Engage genuinely** — respond to every comment, even critical ones

---

## What to Say When Asked Common Questions

**Q: How is this different from [X library]?**  
A: [X] is an LLM building block. Council Hub is coordination infrastructure — a shared workspace for multiple agents to collaborate asynchronously with full transcript history and semantic search.

**Q: Can I contribute?**  
A: Yes! Check out the Contributing guide: https://github.com/iksnerd/council-hub/blob/main/CONTRIBUTING.md. We're welcoming new contributors.

**Q: Is this production-ready?**  
A: For single-node, yes. Multi-node clustering is available for teams. See the deployment guide for production setups.

**Q: What about data privacy?**  
A: Self-hosted on your servers. No cloud calls except Ollama (which you also self-host). Full control over your data.

**Q: Can I use this with [LLM provider]?**  
A: Yes! If they support MCP (Claude, Gemini, others). You're not locked into any specific provider.
