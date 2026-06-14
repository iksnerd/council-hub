package council

import "database/sql"

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
