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
	// notebookNudgeMinDecisions is how many decision/action messages a project must
	// accumulate (across all its rooms) before the linter nudges it to compile a
	// curated notebook — enough deliberation that an integration view would help.
	notebookNudgeMinDecisions = 8
)

// LintResult holds the outcome of a linter sweep.
type LintResult struct {
	NeedsSynthesis []string
	Stale          []string
	StalePin       []string
	StalePlan      []string
	Incoherent     []string
	NeedsNotebook  []string // projects with lots of decided work but no curated notebook
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
	inc := s.lintIncoherent()
	nn := s.lintProjectsNeedingNotebook()
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
	return LintResult{NeedsSynthesis: ns, Stale: st, StalePin: sp, StalePlan: spl, Incoherent: inc, NeedsNotebook: nn}
}

// FlaggedRooms returns the IDs of active rooms currently carrying each of the
// given Knowledge-Linter tags, regardless of which sweep applied them, keyed by
// tag — one room-table scan for all tags rather than one per tag. The lint*
// sweeps below only ever return rooms newly flagged *this* sweep (so they don't
// re-post the same warning every cycle) — reporting that delta as "current
// health" made a room flagged in an earlier sweep invisible until it happened
// to flip again during the exact call. This queries the live tag state instead.
func (s *Server) FlaggedRooms(tags ...string) map[string][]string {
	flagged := make(map[string][]string, len(tags))
	rows, err := s.DB.Query(`SELECT id, COALESCE(tags, '') FROM rooms WHERE status = 'active'`)
	if err != nil {
		s.Logger.Error("Linter: failed to query flagged rooms", "error", err)
		return flagged
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id, roomTags string
		if err := rows.Scan(&id, &roomTags); err != nil {
			continue
		}
		for _, tag := range tags {
			if hasTag(roomTags, tag) {
				flagged[tag] = append(flagged[tag], id)
			}
		}
	}
	return flagged
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

// flagAndNotify tags a room with the given Knowledge-Linter tag and posts the
// notice as a system message. Callers filter out already-tagged rooms first
// (hasTag) — this always applies the tag. The shared tail of every lint* sweep:
// tag under lock, post the notice, log. Returns true if the tag update
// succeeded (a post failure is logged but doesn't make the flag itself fail).
func (s *Server) flagAndNotify(roomID, currentTags, tag, content string) bool {
	newTags := appendTag(currentTags, tag)
	s.Mu.Lock()
	_, err := s.DB.Exec(`UPDATE rooms SET tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newTags, roomID)
	s.Mu.Unlock()
	if err != nil {
		s.Logger.Error("Linter: failed to update tags", "room_id", roomID, "tag", tag, "error", err)
		return false
	}
	if _, err := s.PostMessage(roomID, "system", content, "message", ""); err != nil {
		s.Logger.Error("Linter: failed to post message", "room_id", roomID, "tag", tag, "error", err)
	}
	return true
}

// flagRooms runs a lint query returning (id, tags) rows, skips rooms already
// carrying tag, flags + notifies the rest via flagAndNotify, and returns the
// newly flagged room IDs. The shared shape behind every lint* sweep below
// except lintStalePins (its query carries an extra per-row field for its log
// line) and lintIncoherent (its candidates come from a union of two queries).
func (s *Server) flagRooms(query string, args []any, tag, content, queryErrContext string) []string {
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		s.Logger.Error("Linter: failed to query "+queryErrContext, "error", err)
		return nil
	}
	defer func() { _ = rows.Close() }()

	type candidate struct{ id, tags string }
	var candidates []candidate
	for rows.Next() {
		var id, tags string
		if err := rows.Scan(&id, &tags); err != nil {
			continue
		}
		if !hasTag(tags, tag) {
			candidates = append(candidates, candidate{id, tags})
		}
	}

	var flagged []string
	for _, c := range candidates {
		if s.flagAndNotify(c.id, c.tags, tag, content) {
			s.Logger.Info("Linter: flagged room", "tag", tag, "room_id", c.id)
			flagged = append(flagged, c.id)
		}
	}
	return flagged
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
	content := "### Knowledge Linter\nThis room contains decisions but lacks a `synthesis` message. " +
		"Please read the deliberation and compile a structured article using `post_to_room(message_type=\"synthesis\")`."
	return s.flagRooms(query, []any{synthesisMinDecisions, synthesisMinMessages}, "needs-synthesis", content, "rooms needing synthesis")
}

func (s *Server) lintStaleRooms() []string {
	query := `
		SELECT id, tags FROM rooms
		WHERE status = 'active'
		  AND created_at < datetime('now', '-1 day')
		  AND (SELECT MAX(timestamp) FROM messages WHERE room_id = rooms.id) < datetime('now', '-7 days')
	`
	content := "### Knowledge Linter\nThis room has been inactive for over 7 days. " +
		"Please review the context and either update the `status` to `paused`/`resolved`, or post an update to resume work."
	return s.flagRooms(query, nil, "stale", content, "stale rooms")
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

	content := "### Knowledge Linter\nThe pinned summary in this room may no longer reflect the live " +
		"state — newer `decision`/`action` updates have landed since it, or a superseding message exists. " +
		"Consider posting a fresh `synthesis` and pinning it (`pin_message`)."

	var flagged []string
	for _, c := range candidates {
		if s.flagAndNotify(c.id, c.tags, "stale-pin", content) {
			s.Logger.Info("Linter: flagged room for stale pin", "room_id", c.id, "updates_since_pin", c.cnt)
			flagged = append(flagged, c.id)
		}
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
	content := "### Knowledge Linter\nThis room has a `plan` (specified work awaiting execution) with no " +
		"follow-on `action`. If the work is done, post an `action` referencing it; if it's been dropped, " +
		"note that and resolve the room."
	return s.flagRooms(query, nil, "stale-plan", content, "stale-plan rooms")
}

// lintIncoherent is the Coherence linter (Engelbart's CoDIAK Integration leg): it
// flags active rooms holding an unresolved coherence problem in the link graph. It
// reads the explicit typed edges added in v0.41.0 (E2) and gives the inert
// `contradicts`/`duplicates` relations teeth. Two signals, both purely structural
// (no embedder required):
//
//   - a live `contradicts` edge between two same-room messages where neither side has
//     been superseded and no `synthesis` has been posted since the newer of the two —
//     the room asserts two conflicting statements and has never reconciled them.
//   - a `duplicates` edge between two `synthesis` messages where neither has been
//     superseded — the same article compiled twice (often across rooms), the
//     fragmentation the shared DKR exists to prevent. Both endpoint rooms are flagged.
//
// The fix in either case: supersede the losing message (declares a winner) or post a
// reconciling synthesis. The `incoherent` tag auto-clears in postMessageCore on a
// synthesis or superseding post, and the next sweep re-adds it if the conflict stands
// (same self-healing contract as the other flags — no permanent false positives).
func (s *Server) lintIncoherent() []string {
	// room_id -> the reason phrase rendered in the system note. A room flagged for a
	// contradiction keeps that reason even if it also has a duplicate edge — the
	// contradiction is the sharper signal, so it's reported first.
	reasons := map[string]string{}

	scanRooms := func(query, reason string, overwrite bool) {
		rows, err := s.DB.Query(query)
		if err != nil {
			s.Logger.Error("Linter: failed to query incoherent rooms", "error", err)
			return
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				continue
			}
			if _, seen := reasons[id]; seen && !overwrite {
				continue
			}
			reasons[id] = reason
		}
	}

	// 1. Live, un-reconciled contradiction within a single room. UUIDv7 ids are
	// time-ordered, so MAX(m.id, m2.id) is the newer endpoint and `syn.id > …` means
	// "a synthesis posted after the contradiction stood".
	contradictionQ := `
		SELECT DISTINCT m.room_id
		FROM message_links l
		JOIN messages m  ON m.id = l.from_id
		JOIN messages m2 ON m2.id = l.to_id
		JOIN rooms r ON r.id = m.room_id
		WHERE l.relation = 'contradicts'
		  AND m.room_id = m2.room_id
		  AND r.status = 'active'
		  AND NOT EXISTS (SELECT 1 FROM messages x WHERE x.supersedes = m.id)
		  AND NOT EXISTS (SELECT 1 FROM messages x WHERE x.supersedes = m2.id)
		  AND NOT EXISTS (
		    SELECT 1 FROM messages syn
		    WHERE syn.room_id = m.room_id
		      AND syn.message_type = 'synthesis'
		      AND syn.id > MAX(m.id, m2.id)
		  )
	`
	scanRooms(contradictionQ,
		"two messages here are linked `contradicts`, but neither has been superseded and no later `synthesis` reconciles them. "+
			"Resolve the conflict: `supersedes` the obsolete message, or post (and pin) a `synthesis` that reconciles the two.",
		true)

	// 2. Duplicate syntheses (either room of the `duplicates` edge). Cross-room dupes
	// are the headline case — the same knowledge compiled twice in separate rooms.
	duplicateQ := `
		SELECT DISTINCT room_id FROM (
			SELECT m.room_id AS room_id
			FROM message_links l
			JOIN messages m  ON m.id = l.from_id
			JOIN messages m2 ON m2.id = l.to_id
			JOIN rooms r ON r.id = m.room_id
			WHERE l.relation = 'duplicates'
			  AND m.message_type = 'synthesis' AND m2.message_type = 'synthesis'
			  AND r.status = 'active'
			  AND NOT EXISTS (SELECT 1 FROM messages x WHERE x.supersedes = m.id)
			  AND NOT EXISTS (SELECT 1 FROM messages x WHERE x.supersedes = m2.id)
			UNION
			SELECT m2.room_id AS room_id
			FROM message_links l
			JOIN messages m  ON m.id = l.from_id
			JOIN messages m2 ON m2.id = l.to_id
			JOIN rooms r ON r.id = m2.room_id
			WHERE l.relation = 'duplicates'
			  AND m.message_type = 'synthesis' AND m2.message_type = 'synthesis'
			  AND r.status = 'active'
			  AND NOT EXISTS (SELECT 1 FROM messages x WHERE x.supersedes = m.id)
			  AND NOT EXISTS (SELECT 1 FROM messages x WHERE x.supersedes = m2.id)
		)
	`
	scanRooms(duplicateQ,
		"a `synthesis` here is linked `duplicates` to another synthesis that hasn't been superseded — the same article compiled twice. "+
			"Consolidate: keep one canonical synthesis and `supersedes` the duplicate (or merge them and supersede both).",
		false)

	var flagged []string
	for id, reason := range reasons {
		var tags string
		if err := s.DB.QueryRow(`SELECT COALESCE(tags, '') FROM rooms WHERE id = ?`, id).Scan(&tags); err != nil {
			continue
		}
		if hasTag(tags, "incoherent") {
			continue
		}
		content := "### Knowledge Linter\nThis room has an unresolved coherence problem: " + reason
		if s.flagAndNotify(id, tags, "incoherent", content) {
			s.Logger.Info("Linter: flagged room as incoherent", "room_id", id)
			flagged = append(flagged, id)
		}
	}

	// Self-correct: clear `incoherent` from any active room that still carries the tag
	// but no longer qualifies. A coherence conflict can be resolved in a different room
	// than the one whose flag persists (a cross-room duplicate is fixed by superseding
	// one synthesis), so the event-driven clear in postMessageCore can't catch every
	// case — the sweep reconciles the rest, keeping the flag free of stale positives.
	clearRows, err := s.DB.Query(`SELECT id, COALESCE(tags, '') FROM rooms WHERE status = 'active' AND tags LIKE '%incoherent%'`)
	if err == nil {
		type stale struct{ id, tags string }
		var toClear []stale
		for clearRows.Next() {
			var id, tags string
			if err := clearRows.Scan(&id, &tags); err != nil {
				continue
			}
			if _, stillFlagged := reasons[id]; !stillFlagged && hasTag(tags, "incoherent") {
				toClear = append(toClear, stale{id, tags})
			}
		}
		_ = clearRows.Close()
		for _, c := range toClear {
			s.Mu.Lock()
			_, _ = s.DB.Exec(`UPDATE rooms SET tags = ? WHERE id = ?`, removeTag(c.tags, "incoherent"), c.id)
			s.Mu.Unlock()
		}
	}

	return flagged
}

// lintProjectsNeedingNotebook nudges projects that have accumulated real
// deliberation (notebookNudgeMinDecisions+ decision/action messages across their
// rooms) but have no curated notebook to integrate it. Unlike the other linters,
// this flags a *project*, not a room — projects have no inbox to post into and no
// tags to set — so it's report-only: it surfaces in check_room_health (and the
// janitor playbook) as a prompt to run edit_notebook(action=create, project=…) and
// compile the scattered decisions into one addressable view. Global notebooks
// (project = ”) don't count as a project's notebook.
func (s *Server) lintProjectsNeedingNotebook() []string {
	query := `
		SELECT r.project
		FROM messages m
		JOIN rooms r ON r.id = m.room_id
		WHERE r.project != ''
		  AND m.message_type IN ('decision', 'action')
		  AND NOT EXISTS (SELECT 1 FROM notebooks n WHERE n.project = r.project)
		GROUP BY r.project
		HAVING COUNT(*) >= ?
		ORDER BY r.project
	`
	rows, err := s.DB.Query(query, notebookNudgeMinDecisions)
	if err != nil {
		s.Logger.Error("Linter: failed to query projects needing a notebook", "error", err)
		return nil
	}
	defer func() { _ = rows.Close() }()

	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			continue
		}
		projects = append(projects, p)
	}
	if len(projects) > 0 {
		s.Logger.Info("Linter: projects with decided work but no notebook", "projects", projects)
	}
	return projects
}
