package council

import (
	"database/sql"
	"fmt"
	"strings"
)

// GetMessageByID returns a single message by its ID.
func (s *Server) GetMessageByID(id string) (Message, error) {
	return scanMessage(s.DB.QueryRow(
		fmt.Sprintf(`SELECT %s FROM messages WHERE id = ?`, messageColumns), id,
	))
}

// GetMessagesFromIDInclusive returns all messages in roomID with id >= fromID, in chronological order.
// UUID v7 IDs are time-ordered, so this correctly captures the starting message and everything after it.
func (s *Server) GetMessagesFromIDInclusive(roomID, fromID string) ([]Message, error) {
	rows, err := s.DB.Query(fmt.Sprintf(`
		SELECT %s FROM messages WHERE room_id = ? AND id >= ? ORDER BY id ASC`, messageColumns),
		roomID, fromID,
	)
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

func (s *Server) GetPinnedMessage(roomID string) (*Message, error) {
	m, err := scanMessage(s.DB.QueryRow(
		fmt.Sprintf(`SELECT %s FROM messages WHERE room_id = ? AND pinned = 1 LIMIT 1`, messageColumns),
		roomID,
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Server) GetMessagesByIDs(ids []string) ([]Message, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`SELECT %s FROM messages WHERE id IN (%s) ORDER BY id ASC`, messageColumns, strings.Join(placeholders, ","))
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

func (s *Server) GetRecentMessages(roomID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// Verify room exists
	_, err := s.GetRoom(roomID)
	if err != nil {
		return nil, fmt.Errorf("room '%s' not found", roomID)
	}

	// Get last N messages in reverse, then flip to chronological
	rows, err := s.DB.Query(fmt.Sprintf(`SELECT %s FROM messages WHERE room_id = ? ORDER BY id DESC LIMIT ?`, messageColumns), roomID, limit)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// GetMessagesAfterID returns messages with ID > afterID for a room, in chronological order.
func (s *Server) GetMessagesAfterID(roomID string, afterID string) ([]Message, error) {
	rows, err := s.DB.Query(fmt.Sprintf(`
		SELECT %s
		FROM messages
		WHERE room_id = ? AND id > ?
		ORDER BY timestamp ASC`, messageColumns),
		roomID, afterID,
	)
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

// GetLatestPerType returns the most recent message for each message_type in a room.
func (s *Server) GetLatestPerType(roomID string) ([]Message, error) {
	// Return up to 2 most recent messages per type so agents see both the latest
	// and its predecessor (useful when the latest superseded an earlier key message).
	rows, err := s.DB.Query(fmt.Sprintf(`
		SELECT %s
		FROM (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY message_type ORDER BY id DESC) as rn
			FROM messages
			WHERE room_id = ? AND is_summary = 0
		) ranked
		WHERE rn <= 2
		ORDER BY message_type, id DESC`, messageColumns),
		roomID,
	)
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

func (s *Server) SearchMessages(query, author, messageType, roomID, project, since, until string, limit int) ([]Message, error) {
	where := `WHERE 1=1`
	var args []any
	join := ""
	orderBy := `ORDER BY m.timestamp DESC`

	if query != "" {
		join += ` JOIN messages_fts f ON m.rowid = f.rowid`
		var terms []string
		for _, word := range strings.Fields(query) {
			clean := strings.ReplaceAll(word, "\"", "")
			if clean != "" {
				terms = append(terms, "\""+clean+"\"")
			}
		}
		if len(terms) > 0 {
			where += ` AND messages_fts MATCH ?`
			args = append(args, strings.Join(terms, " AND "))
			orderBy = `ORDER BY bm25(messages_fts), m.timestamp DESC`
		}
	}
	if author != "" {
		where += ` AND m.author = ?`
		args = append(args, author)
	}
	if messageType != "" {
		where += ` AND m.message_type = ?`
		args = append(args, messageType)
	}
	if roomID != "" {
		// Support comma-separated room IDs for batch filtering
		parts := strings.Split(roomID, ",")
		if len(parts) == 1 {
			where += ` AND m.room_id = ?`
			args = append(args, strings.TrimSpace(parts[0]))
		} else {
			placeholders := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					placeholders = append(placeholders, "?")
					args = append(args, p)
				}
			}
			if len(placeholders) > 0 {
				where += ` AND m.room_id IN (` + strings.Join(placeholders, ",") + `)`
			}
		}
	}
	if project != "" {
		join += ` JOIN rooms r ON m.room_id = r.id`
		where += ` AND r.project = ?`
		args = append(args, normalizeProject(project))
	}
	if since != "" {
		where += ` AND m.timestamp >= ?`
		args = append(args, since)
	}
	if until != "" {
		where += ` AND m.timestamp <= ?`
		args = append(args, until)
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Prefix each column with "m." for the join-capable query
	prefixed := "m." + strings.ReplaceAll(messageColumns, ", ", ", m.")
	q := fmt.Sprintf(`SELECT %s FROM messages m%s %s %s LIMIT ?`, prefixed, join, where, orderBy)
	args = append(args, limit)

	rows, err := s.DB.Query(q, args...)
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
