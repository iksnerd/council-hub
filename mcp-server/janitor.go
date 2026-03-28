package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	janitorInterval         = 5 * time.Minute
	summaryMessageThreshold = 20
)

func (cs *CouncilServer) runJanitor(ctx context.Context) {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()

	cs.logger.Info("Janitor started", "interval", janitorInterval, "threshold", summaryMessageThreshold)

	for {
		select {
		case <-ctx.Done():
			cs.logger.Info("Janitor stopped")
			return
		case <-ticker.C:
			cs.janitorSweep()
		}
	}
}

func (cs *CouncilServer) janitorSweep() {
	rooms, err := cs.getRoomsNeedingSummary(summaryMessageThreshold)
	if err != nil {
		cs.logger.Error("Janitor: failed to find rooms needing summary", "error", err)
		return
	}

	for _, roomID := range rooms {
		msgs, err := cs.getUnsummarizedMessages(roomID)
		if err != nil {
			cs.logger.Error("Janitor: failed to get unsummarized messages", "room_id", roomID, "error", err)
			continue
		}

		summary := summarize(msgs)

		if err := cs.insertSummary(roomID, summary); err != nil {
			cs.logger.Error("Janitor: failed to insert summary", "room_id", roomID, "error", err)
			continue
		}

		cs.logger.Info("Janitor: summarized room", "room_id", roomID, "messages_summarized", len(msgs))
	}
}

// summarize produces a stub summary of messages.
// Replace this with a real LLM API call for production use.
func summarize(msgs []Message) string {
	var b strings.Builder

	first := msgs[0].Timestamp.Format("2006-01-02 15:04:05")
	last := msgs[len(msgs)-1].Timestamp.Format("2006-01-02 15:04:05")

	fmt.Fprintf(&b, "**Summary of %d messages (%s to %s):**\n\n", len(msgs), first, last)

	authors := make(map[string]int)
	for _, m := range msgs {
		authors[m.Author]++
	}

	b.WriteString("Participants: ")
	i := 0
	for author, count := range authors {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s (%d msgs)", author, count)
		i++
	}
	b.WriteString("\n\nKey points:\n")

	for _, m := range msgs {
		snippet := m.Content
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		// Remove newlines for compact summary
		snippet = strings.ReplaceAll(snippet, "\n", " ")
		fmt.Fprintf(&b, "- **%s:** %s\n", m.Author, snippet)
	}

	return b.String()
}
