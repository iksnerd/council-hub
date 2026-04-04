package council

import (
	"context"
	"strings"
	"time"
)

const (
	janitorInterval = 1 * time.Hour
)

func (s *Server) RunJanitor(ctx context.Context) {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()

	s.Logger.Info("Knowledge Linter (Janitor) started", "interval", janitorInterval)

	for {
		select {
		case <-ctx.Done():
			s.Logger.Info("Janitor stopped")
			return
		case <-ticker.C:
			s.JanitorSweep()
		}
	}
}

func (s *Server) JanitorSweep() {
	s.lintNeedsSynthesis()
	s.lintStaleRooms()
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

func (s *Server) lintNeedsSynthesis() {
	query := `
		SELECT id, tags FROM rooms
		WHERE status IN ('active', 'resolved')
		  AND EXISTS (SELECT 1 FROM messages WHERE room_id = rooms.id AND message_type = 'decision')
		  AND NOT EXISTS (SELECT 1 FROM messages WHERE room_id = rooms.id AND message_type = 'synthesis')
	`
	rows, err := s.DB.Query(query)
	if err != nil {
		s.Logger.Error("Janitor: failed to query rooms needing synthesis", "error", err)
		return
	}
	defer rows.Close()

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

	for _, c := range candidates {
		newTags := appendTag(c.tags, "needs-synthesis")
		s.Mu.Lock()
		_, err := s.DB.Exec(`UPDATE rooms SET tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newTags, c.id)
		s.Mu.Unlock()
		if err != nil {
			s.Logger.Error("Janitor: failed to update tags", "room_id", c.id, "error", err)
			continue
		}

		content := "### Knowledge Linter\nThis room contains decisions but lacks a `synthesis` message. " +
			"Please read the deliberation and compile a structured article using `post_to_room(message_type=\"synthesis\")`."
		if _, err := s.PostMessage(c.id, "system", content, "message", ""); err != nil {
			s.Logger.Error("Janitor: failed to post linter message", "room_id", c.id, "error", err)
		} else {
			s.Logger.Info("Janitor: flagged room for synthesis", "room_id", c.id)
		}
	}
}

func (s *Server) lintStaleRooms() {
	query := `
		SELECT id, tags FROM rooms
		WHERE status = 'active'
		  AND (SELECT MAX(timestamp) FROM messages WHERE room_id = rooms.id) < datetime('now', '-7 days')
	`
	rows, err := s.DB.Query(query)
	if err != nil {
		s.Logger.Error("Janitor: failed to query stale rooms", "error", err)
		return
	}
	defer rows.Close()

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
			s.Logger.Info("Janitor: flagged room as stale", "room_id", c.id)
		}
	}
}
