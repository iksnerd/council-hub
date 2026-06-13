package council

import (
	"context"
	"strings"
	"sync/atomic"
	"time"
)

const (
	janitorInterval       = 6 * time.Hour
	synthesisMinDecisions = 3
	synthesisMinMessages  = 20
	graceperiod           = 24 * time.Hour
	// stalePinMinUpdates is how many decision/action/synthesis messages must land
	// after the pinned message before its room is flagged `stale-pin`.
	stalePinMinUpdates = 5
)

// LintResult holds the outcome of a linter sweep.
type LintResult struct {
	NeedsSynthesis []string
	Stale          []string
	StalePin       []string
	StalePlan      []string
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
	sp := s.lintStalePins()
	spl := s.lintStalePlans()
	healed, err := healIndexes(s.DB, s.Logger)
	if err != nil {
		s.Logger.Error("Janitor: integrity check failed", "error", err)
	}
	if healed {
		atomic.AddUint64(&s.HealCount, 1)
	}
	now := time.Now()
	s.Mu.Lock()
	s.LastJanitorScan = now
	s.LastIntegrityCheck = now
	s.Mu.Unlock()
	return LintResult{NeedsSynthesis: ns, Stale: st, StalePin: sp, StalePlan: spl}
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

// lintStalePins flags active rooms whose pinned message has fallen behind the live
// state: stalePinMinUpdates or more decision/action/synthesis messages have landed
// since the pin was set. Inactivity (lintStaleRooms) catches abandoned rooms; this
// catches the more dangerous case — a busy room whose pinned TL;DR now misleads a
// cold-start agent. UUIDv7 message IDs are time-ordered, so `m.id > p.id` means
// "posted after the pin". The `stale-pin` tag auto-clears in PinMessage on re-pin.
func (s *Server) lintStalePins() []string {
	// Flag an active room's pin as stale when either (a) stalePinMinUpdates+ decision/
	// action/synthesis messages have landed since it (drift heuristic), or (b) the pin
	// has been explicitly superseded by a newer message but never re-pinned (definitive).
	query := `
		SELECT id, tags, cnt FROM (
			SELECT r.id AS id, COALESCE(r.tags, '') AS tags,
				(SELECT COUNT(*) FROM messages m
				   WHERE m.room_id = r.id AND m.id > p.id
				     AND m.message_type IN ('decision', 'action', 'synthesis')) AS cnt,
				EXISTS (SELECT 1 FROM messages s
				          WHERE s.room_id = r.id AND s.supersedes = p.id) AS superseded
			FROM rooms r
			JOIN messages p ON p.room_id = r.id AND p.pinned = 1
			WHERE r.status = 'active'
		)
		WHERE cnt >= ? OR superseded = 1
	`
	rows, err := s.DB.Query(query, stalePinMinUpdates)
	if err != nil {
		s.Logger.Error("Linter: failed to query stale-pin rooms", "error", err)
		return nil
	}
	defer func() { _ = rows.Close() }()

	type candidate struct {
		id   string
		tags string
		cnt  int
	}
	var candidates []candidate

	for rows.Next() {
		var id, tags string
		var cnt int
		if err := rows.Scan(&id, &tags, &cnt); err != nil {
			continue
		}
		if !hasTag(tags, "stale-pin") {
			candidates = append(candidates, candidate{id, tags, cnt})
		}
	}

	var flagged []string
	for _, c := range candidates {
		newTags := appendTag(c.tags, "stale-pin")
		s.Mu.Lock()
		_, err := s.DB.Exec(`UPDATE rooms SET tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newTags, c.id)
		s.Mu.Unlock()
		if err != nil {
			s.Logger.Error("Linter: failed to update stale-pin tags", "room_id", c.id, "error", err)
			continue
		}

		content := "### Knowledge Linter\nThe pinned summary in this room may no longer reflect the live " +
			"state — newer `decision`/`action` updates have landed since it, or a superseding message exists. " +
			"Consider posting a fresh `synthesis` and pinning it (`pin_message`)."
		if _, err := s.PostMessage(c.id, "system", content, "message", ""); err == nil {
			s.Logger.Info("Linter: flagged room for stale pin", "room_id", c.id, "updates_since_pin", c.cnt)
		}
		flagged = append(flagged, c.id)
	}
	return flagged
}

// lintStalePlans flags active rooms holding an unexecuted handoff: the most recent
// `plan` message has no `action` posted after it. A plan is "specified work awaiting
// execution", so a plan with no following action is work that was handed off and never
// picked up. The `stale-plan` tag auto-clears in postMessageCore when an `action` lands
// (whose UUIDv7 id is necessarily newer than every existing plan).
func (s *Server) lintStalePlans() []string {
	query := `
		SELECT r.id, COALESCE(r.tags, '')
		FROM rooms r
		JOIN (
			SELECT room_id, MAX(id) AS plan_id
			FROM messages WHERE message_type = 'plan'
			GROUP BY room_id
		) p ON p.room_id = r.id
		WHERE r.status = 'active'
		  AND NOT EXISTS (
		    SELECT 1 FROM messages a
		    WHERE a.room_id = r.id AND a.message_type = 'action' AND a.id > p.plan_id
		  )
	`
	rows, err := s.DB.Query(query)
	if err != nil {
		s.Logger.Error("Linter: failed to query stale-plan rooms", "error", err)
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
		if !hasTag(tags, "stale-plan") {
			candidates = append(candidates, candidate{id, tags})
		}
	}

	var flagged []string
	for _, c := range candidates {
		newTags := appendTag(c.tags, "stale-plan")
		s.Mu.Lock()
		_, err := s.DB.Exec(`UPDATE rooms SET tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newTags, c.id)
		s.Mu.Unlock()
		if err != nil {
			s.Logger.Error("Linter: failed to update stale-plan tags", "room_id", c.id, "error", err)
			continue
		}

		content := "### Knowledge Linter\nThis room has a `plan` (specified work awaiting execution) with no " +
			"follow-on `action`. If the work is done, post an `action` referencing it; if it's been dropped, " +
			"note that and resolve the room."
		if _, err := s.PostMessage(c.id, "system", content, "message", ""); err == nil {
			s.Logger.Info("Linter: flagged room for stale plan", "room_id", c.id)
		}
		flagged = append(flagged, c.id)
	}
	return flagged
}
