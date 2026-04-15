package council

import (
	"context"
	"strings"
	"time"
)

const (
	janitorInterval       = 6 * time.Hour
	synthesisMinDecisions = 3
	synthesisMinMessages  = 20
	graceperiod           = 24 * time.Hour
)

// LintResult holds the outcome of a linter sweep.
type LintResult struct {
	NeedsSynthesis []string
	Stale          []string
}

func (s *Server) RunJanitor(ctx context.Context) {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()

	s.Logger.Info("Knowledge Linter started", "interval", janitorInterval)

	for {
		select {
		case <-ctx.Done():
			s.Logger.Info("Knowledge Linter stopped")
			return
		case <-ticker.C:
			s.JanitorSweep()
		}
	}
}

func (s *Server) JanitorSweep() LintResult {
	ns := s.lintNeedsSynthesis()
	st := s.lintStaleRooms()

	// Backfill any messages/rooms missing embeddings (e.g. Ollama was
	// unavailable when they were created).
	s.BackfillEmbeddings(context.Background())

	s.Mu.Lock()
	s.LastJanitorScan = time.Now()
	s.Mu.Unlock()
	return LintResult{NeedsSynthesis: ns, Stale: st}
}

// hasTag checks whether a comma-separated tag string contains an exact tag.
func hasTag(tags, tag string) bool {
	for _, t := range strings.Split(tags, ",") {
		if strings.TrimSpace(t) == tag {
			return true
		}
	}
	return false
}

// removeTag removes a tag from a comma-separated tag string, if present.
func removeTag(tags, tag string) string {
	if !hasTag(tags, tag) {
		return tags
	}
	var kept []string
	for _, t := range strings.Split(tags, ",") {
		t = strings.TrimSpace(t)
		if t != "" && t != tag {
			kept = append(kept, t)
		}
	}
	return strings.Join(kept, ",")
}

// appendTag adds a tag to a comma-separated tag string if not already present.
func appendTag(tags, tag string) string {
	if hasTag(tags, tag) {
		return tags
	}
	if tags == "" {
		return tag
	}
	return tags + "," + tag
}

func (s *Server) lintNeedsSynthesis() []string {
	query := `
		SELECT id, tags FROM rooms
		WHERE status = 'active'
		  AND created_at < datetime('now', '-1 day')
		  AND NOT EXISTS (SELECT 1 FROM messages WHERE room_id = rooms.id AND message_type = 'synthesis')
		  AND (
		    (SELECT COUNT(*) FROM messages WHERE room_id = rooms.id AND message_type = 'decision') >= ?
		    OR
		    (SELECT COUNT(*) FROM messages WHERE room_id = rooms.id) >= ?
		  )
	`
	rows, err := s.DB.Query(query, synthesisMinDecisions, synthesisMinMessages)
	if err != nil {
		s.Logger.Error("Linter: failed to query rooms needing synthesis", "error", err)
		return nil
	}
	defer func() { _ = rows.Close() }()

	type candidate struct {
		id   string
		tags string
	}
	var candidates []candidate

	for rows.Next() {
		var id, tags string
		if err := rows.Scan(&id, &tags); err != nil {
			continue
		}
		if !hasTag(tags, "needs-synthesis") {
			candidates = append(candidates, candidate{id, tags})
		}
	}

	var flagged []string
	for _, c := range candidates {
		newTags := appendTag(c.tags, "needs-synthesis")
		s.Mu.Lock()
		_, err := s.DB.Exec(`UPDATE rooms SET tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newTags, c.id)
		s.Mu.Unlock()
		if err != nil {
			s.Logger.Error("Linter: failed to update tags", "room_id", c.id, "error", err)
			continue
		}

		content := "### Knowledge Linter\nThis room contains decisions but lacks a `synthesis` message. " +
			"Please read the deliberation and compile a structured article using `post_to_room(message_type=\"synthesis\")`."
		if _, err := s.PostMessage(c.id, "system", content, "message", ""); err != nil {
			s.Logger.Error("Linter: failed to post message", "room_id", c.id, "error", err)
		} else {
			s.Logger.Info("Linter: flagged room for synthesis", "room_id", c.id)
		}
		flagged = append(flagged, c.id)
	}
	return flagged
}

func (s *Server) lintStaleRooms() []string {
	query := `
		SELECT id, tags FROM rooms
		WHERE status = 'active'
		  AND created_at < datetime('now', '-1 day')
		  AND (SELECT MAX(timestamp) FROM messages WHERE room_id = rooms.id) < datetime('now', '-7 days')
	`
	rows, err := s.DB.Query(query)
	if err != nil {
		s.Logger.Error("Linter: failed to query stale rooms", "error", err)
		return nil
	}
	defer func() { _ = rows.Close() }()

	type candidate struct {
		id   string
		tags string
	}
	var candidates []candidate

	for rows.Next() {
		var id, tags string
		if err := rows.Scan(&id, &tags); err != nil {
			continue
		}
		if !hasTag(tags, "stale") {
			candidates = append(candidates, candidate{id, tags})
		}
	}

	var flagged []string
	for _, c := range candidates {
		newTags := appendTag(c.tags, "stale")
		s.Mu.Lock()
		_, err := s.DB.Exec(`UPDATE rooms SET tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newTags, c.id)
		s.Mu.Unlock()
		if err != nil {
			continue
		}

		content := "### Knowledge Linter\nThis room has been inactive for over 7 days. " +
			"Please review the context and either update the `status` to `paused`/`resolved`, or post an update to resume work."
		if _, err := s.PostMessage(c.id, "system", content, "message", ""); err == nil {
			s.Logger.Info("Linter: flagged room as stale", "room_id", c.id)
		}
		flagged = append(flagged, c.id)
	}
	return flagged
}
