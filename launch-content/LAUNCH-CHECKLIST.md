# Council Hub Launch Checklist

**Timeline:** Launch Day (Monday-Thursday, not Friday)

---

## Pre-Launch (72 hours before)

- [ ] Test all links in Dev.to article
- [ ] Verify Docker image is published to Docker Hub
- [ ] Confirm GitHub Discussions are enabled
- [ ] Review README for typos/broken links
- [ ] Create GitHub release notes for v0.28.0 (if not already done)
- [ ] Schedule tweets in advance if using a scheduling tool
- [ ] Prepare Firefox/Chrome tabs:
  - [ ] Dev.to editor (draft ready to publish)
  - [ ] Twitter compose
  - [ ] Reddit r/golang
  - [ ] Reddit r/OpenSource
  - [ ] HN (for Day 4)
  - [ ] Discord communities (list below)

---

## Launch Day (Day 1 – Monday-Thursday)

### Morning (6-10 AM PT)

**9:00 AM:**
- [ ] Publish Dev.to article (copy from `launch-content/dev-to-article.md`)
- [ ] Tweet link to Dev.to
  - [ ] Use all 6 tweets from `launch-content/twitter-thread.md`
  - [ ] Post ~15-30 minutes apart, or as a thread
- [ ] Verify both posts are live

**10:00 AM:**
- [ ] Post to Discord communities (use templates from `launch-content/discord-announcement.md`)
  - [ ] Golang Discord
  - [ ] Elixir Discord
  - [ ] LLM/AI communities
  - [ ] Open source communities
  - [ ] Your personal communities
- [ ] Monitor comments/reactions

### Afternoon (12-6 PM PT)

**12:00 PM:**
- [ ] Start responding to comments
- [ ] Answer Dev.to comments
- [ ] Answer Twitter replies
- [ ] Answer Discord messages

**3:00 PM:**
- [ ] Monitor star count on GitHub
- [ ] Monitor new issues/discussions

**6:00 PM:**
- [ ] Continue engagement (aim to respond within 1 hour of comments)

---

## Day 2-3 (Tuesday-Wednesday)

### Morning

- [ ] Post to Reddit r/golang (copy from `launch-content/reddit-posts.md`, Post 1)
- [ ] Wait 4 hours

- [ ] Post to Reddit r/OpenSource (copy from `launch-content/reddit-posts.md`, Post 2)

- [ ] Tweet follow-up content (examples in `launch-content/twitter-thread.md`)

### Throughout Day

- [ ] Respond to all Reddit comments
- [ ] Answer questions on Twitter
- [ ] Monitor GitHub stars/issues/discussions

---

## Day 4 (Thursday or Friday)

### Morning (6-9 AM PT)

- [ ] Submit to Hacker News
  - [ ] Title: "Council Hub – Open-source MCP server for multi-LLM collaboration"
  - [ ] URL: https://github.com/iksnerd/council-hub
  - [ ] Text: Short description (optional)

- [ ] Be ready to engage on HN immediately
  - [ ] Monitor for comments
  - [ ] Respond to questions
  - [ ] Address concerns

### Throughout Day

- [ ] Post follow-up tweets
- [ ] Respond to HN comments
- [ ] Monitor GitHub activity
- [ ] Celebrate milestone(s) if reached (100 stars, 10 issues, etc.)

---

## Sustained Engagement (Week 2-4)

### Daily (First 2 weeks)

- [ ] Respond to new issues within 24 hours
- [ ] Answer discussions
- [ ] Share user feedback on Twitter
- [ ] Monitor Discord communities

### Weekly (Weeks 3-4)

- [ ] Post case study or tutorial update
- [ ] Share performance/deployment guide updates
- [ ] Feature a community contribution if available
- [ ] Check star count and trending status

---

## Discord Communities to Post In

**Elixir:**
- Elixir Official Discord
- Your personal Elixir channels

**Go:**
- Go Discord (if exists)
- Golang subreddits
- Your personal Go channels

**LLM/AI:**
- Anthropic Discord (if available)
- LLM-focused communities
- Your personal AI channels

**Open Source:**
- Open source communities
- Dev.to Discord
- Your personal OSS channels

**Other:**
- Rust, Python, JavaScript communities that might care
- Enterprise/DevOps communities (Kubernetes)
- Your company/team channels

---

## Success Metrics

### Target (First Week)

- [ ] 100+ GitHub stars
- [ ] 5+ new issues/discussions
- [ ] Positive sentiment in comments
- [ ] 3+ blog posts/shares linking to project
- [ ] 10+ Discord reactions across communities

### Stretch Goals

- [ ] 500+ GitHub stars
- [ ] 50+ issues/discussions
- [ ] Featured on trending page (Reddit, HN, Twitter)
- [ ] First external contribution
- [ ] First user case study

---

## Content Files

All templates are in `launch-content/`:

- `dev-to-article.md` — Copy entire content into Dev.to editor (markdown format)
- `twitter-thread.md` — 6 individual tweets + 3 follow-up templates
- `reddit-posts.md` — 3 subreddit-specific posts (r/golang, r/OpenSource, r/programming)
- `discord-announcement.md` — 4 community-specific announcements
- `LAUNCH-CHECKLIST.md` — This file

---

## Common Questions During Launch

### "Why should I care about this?"

Use this elevator pitch:
> "Council Hub lets multiple LLM agents collaborate in shared rooms instead of working in isolation. Full transcripts, semantic search, and a real-time dashboard. Self-hosted, MIT licensed, no vendor lock-in."

### "How is this different from [X]?"

Reference the guide in `docs/launch-strategy.md` FAQ section.

### "Can I contribute?"

Point to: https://github.com/iksnerd/council-hub/blob/main/CONTRIBUTING.md

### "When is [feature] coming?"

Honest answer: "Open an issue to request it. We prioritize based on community feedback."

---

## Post-Launch (After Week 1)

- [ ] Compile launch metrics (stars, issues, discussions, engagement)
- [ ] Write a follow-up post ("Launch recap: what we learned")
- [ ] Share user feedback publicly (with permission)
- [ ] Plan next steps (features, improvements, partnerships)
- [ ] Thank early adopters and contributors
- [ ] Schedule content for Weeks 2-4

---

## Notes

- **Respond generously:** First week is crucial. Respond to every comment, even critical ones.
- **Be authentic:** Share your real motivation, challenges, and roadmap.
- **Celebrate wins:** Share milestones (100 stars, first issue, first PR, etc.)
- **Invite feedback:** Ask what people would build with Council Hub.
- **Have fun:** This is exciting! Let that enthusiasm show.

---

**Good luck! 🚀**

Questions? Open an issue: https://github.com/iksnerd/council-hub/issues
