package council

import (
	"database/sql"
	"strings"
	"time"
)

// mentionAgentKey normalizes an agent name for the mention cursor.
func mentionAgentKey(agent string) string {
	return strings.ToLower(strings.TrimSpace(agent))
}

// mentionFilter builds the shared WHERE clause of the mention queries.
func mentionFilter(author, project string) (string, []any) {
	where := `LOWER(mentions) LIKE '%'||LOWER(?)||'%' AND ` + liveClause("")
	args := []any{author}
	if project = normalizeProject(project); project != "" {
		where += ` AND room_id IN (SELECT id FROM rooms WHERE project = ?)`
		args = append(args, project)
	}
	return where, args
}

// GetMentionCursor returns the agent's mention cursor ("" when none).
func (s *Server) GetMentionCursor(agent string) (string, error) {
	var id string
	err := s.DB.QueryRow(`SELECT cursor_message_id FROM mention_cursors WHERE agent = ?`, mentionAgentKey(agent)).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// AdvanceMentionCursor moves the agent's mention cursor to messageID. It never
// moves backwards: an id at or below the stored cursor is a no-op.
func (s *Server) AdvanceMentionCursor(agent, messageID string) error {
	if messageID == "" {
		return nil
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	_, err := s.DB.Exec(`
		INSERT INTO mention_cursors (agent, cursor_message_id, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(agent) DO UPDATE SET
			cursor_message_id = excluded.cursor_message_id,
			updated_at = CURRENT_TIMESTAMP
		WHERE excluded.cursor_message_id > mention_cursors.cursor_message_id
	`, mentionAgentKey(agent), messageID)
	return err
}

// AckMentions records a delivery receipt for each id and advances the agent's
// mention cursor to the newest of them, in one transaction. Receipts use
// INSERT OR IGNORE, so a re-ack keeps the first acked_at. Ids from cluster
// peers are stored too (ids are globally unique); the rows are node-local.
func (s *Server) AckMentions(agent string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	key := mentionAgentKey(agent)
	newest := ids[0]
	for _, id := range ids {
		if id > newest {
			newest = id
		}
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, id := range ids {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO mention_acks (agent, message_id, acked_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, key, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO mention_cursors (agent, cursor_message_id, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(agent) DO UPDATE SET
			cursor_message_id = excluded.cursor_message_id,
			updated_at = CURRENT_TIMESTAMP
		WHERE excluded.cursor_message_id > mention_cursors.cursor_message_id
	`, key, newest); err != nil {
		return err
	}
	return tx.Commit()
}

// RecordAgentSeen upserts the agent's last-seen row (name normalised like the
// mention cursor key). Empty names are ignored.
func (s *Server) RecordAgentSeen(agent, tool string, at time.Time) error {
	key := mentionAgentKey(agent)
	if key == "" {
		return nil
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	_, err := s.DB.Exec(`
		INSERT INTO agent_seen (agent, last_seen_at, last_tool) VALUES (?, ?, ?)
		ON CONFLICT(agent) DO UPDATE SET last_seen_at = excluded.last_seen_at, last_tool = excluded.last_tool
	`, key, at.UTC().Format("2006-01-02 15:04:05"), tool)
	return err
}

// GetMentionsAfter returns the OLDEST limit mentions with id > afterID, oldest
// first, and how many further newer mentions remain beyond them. Taking the
// oldest (not the newest) lets a caller advance the cursor to the last id
// returned without skipping any unseen mention. Ids are time-ordered.
func (s *Server) GetMentionsAfter(author, project, afterID string, limit int) (msgs []Message, remaining int, err error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	where, args := mentionFilter(author, project)
	where += ` AND id > ?`
	args = append(args, afterID)

	rows, err := s.DB.Query(`SELECT `+messageColumns+` FROM messages WHERE `+where+` ORDER BY id ASC LIMIT ?`, append(append([]any{}, args...), limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, 0, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(msgs) == limit {
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM messages WHERE `+where+` AND id > ?`,
			append(append([]any{}, args...), msgs[len(msgs)-1].ID)...).Scan(&remaining); err != nil {
			return nil, 0, err
		}
	}
	return msgs, remaining, nil
}

// CountMentionsUpTo counts mentions with id <= cursorID: the ones a cursor hides.
func (s *Server) CountMentionsUpTo(author, project, cursorID string) (int, error) {
	if cursorID == "" {
		return 0, nil
	}
	where, args := mentionFilter(author, project)
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM messages WHERE `+where+` AND id <= ?`, append(args, cursorID)...).Scan(&n)
	return n, err
}

// GetMentions returns recent messages that explicitly mention the given author.
// Uses case-insensitive substring matching so partial names like "claude" match
// "Claude Code (Opus)", "claude-code", etc. When project is non-empty, mentions are
// scoped to rooms in that project — making the session-start get_digest(project)/
// get_mentions(project) pair symmetric.
func (s *Server) GetMentions(author, project string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	project = normalizeProject(project)

	// Mentions is a discovery surface like search: collapse to live head revisions
	// so an edited mention isn't a stale duplicate (old + new both carry the
	// mention) and a withdrawn (retracted) mention no longer pings.
	query := `SELECT ` + messageColumns + ` FROM messages
		WHERE LOWER(mentions) LIKE '%'||LOWER(?)||'%' AND ` + liveClause("")
	args := []any{author}
	if project != "" {
		query += ` AND room_id IN (SELECT id FROM rooms WHERE project = ?)`
		args = append(args, project)
	}
	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var msgs []Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// MarkRead persists a read cursor for an agent in a given room.
// The cursor is the ID of the last message the agent has read.
// Subsequent calls with a new cursorMessageID overwrite the previous value.
func (s *Server) MarkRead(agent, roomID, cursorMessageID string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	_, err := s.DB.Exec(`
		INSERT INTO agent_cursors (agent, room_id, cursor_message_id, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(agent, room_id) DO UPDATE SET
			cursor_message_id = excluded.cursor_message_id,
			updated_at = CURRENT_TIMESTAMP
	`, agent, roomID, cursorMessageID)
	return err
}

// MarkAllRead sets agent's cursor in every room on this node to that room's
// latest message id (MAX(id), the same value get_digest reports as
// latest_message_id), in one transaction. Rooms with no messages get no cursor
// and are counted in skipped.
func (s *Server) MarkAllRead(agent string) (updated, skipped int, err error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	tx, err := s.DB.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if err = tx.QueryRow(`SELECT COUNT(*) FROM rooms`).Scan(&updated); err != nil {
		return 0, 0, err
	}
	res, err := tx.Exec(`
		INSERT INTO agent_cursors (agent, room_id, cursor_message_id, updated_at)
		SELECT ?, room_id, MAX(id), CURRENT_TIMESTAMP FROM messages
		WHERE room_id IN (SELECT id FROM rooms)
		GROUP BY room_id
		ON CONFLICT(agent, room_id) DO UPDATE SET
			cursor_message_id = excluded.cursor_message_id,
			updated_at = CURRENT_TIMESTAMP
	`, agent)
	if err != nil {
		return 0, 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, 0, err
	}
	return int(n), updated - int(n), nil
}

// GetCursor returns the stored read cursor for an agent in a room.
// Returns ("", nil) when no cursor exists yet.
func (s *Server) GetCursor(agent, roomID string) (string, error) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	var cursorID string
	err := s.DB.QueryRow(
		`SELECT cursor_message_id FROM agent_cursors WHERE agent = ? AND room_id = ?`,
		agent, roomID,
	).Scan(&cursorID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return cursorID, err
}

// CursorNeighbours reports, before a mark_read, whether agent is a brand-new
// cursor name and which existing cursor names it is a case-insensitive substring
// of (or contains). get_mentions matches by substring while cursors key on the
// exact string, so 'claude' next to 'claude-code' silently shares mentions but
// not read position. Neighbours are returned sorted.
func (s *Server) CursorNeighbours(agent string) (isNew bool, neighbours []string, err error) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	var existing int
	if err = s.DB.QueryRow(`SELECT COUNT(*) FROM agent_cursors WHERE agent = ?`, agent).Scan(&existing); err != nil {
		return false, nil, err
	}
	rows, err := s.DB.Query(`SELECT DISTINCT agent FROM agent_cursors
		WHERE agent != ? AND (LOWER(agent) LIKE '%'||LOWER(?)||'%' OR LOWER(?) LIKE '%'||LOWER(agent)||'%')
		ORDER BY agent`, agent, agent, agent)
	if err != nil {
		return false, nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var a string
		if err = rows.Scan(&a); err != nil {
			return false, nil, err
		}
		neighbours = append(neighbours, a)
	}
	return existing == 0, neighbours, rows.Err()
}
