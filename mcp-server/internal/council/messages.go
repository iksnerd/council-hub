package council

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (s *Server) PostMessage(roomID, author, content, messageType string, replyTo string) (string, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if messageType == "" {
		messageType = "message"
	}

	id := uuid.Must(uuid.NewV7()).String()
	_, err := s.DB.Exec(
		`INSERT INTO messages (id, room_id, author, content, message_type, reply_to) VALUES (?, ?, ?, ?, ?, ?)`,
		id, roomID, author, content, messageType, replyTo,
	)
	if err != nil {
		return "", err
	}

	// Automatic tag clearing for synthesis messages
	if messageType == "synthesis" {
		var tags string
		err := s.DB.QueryRow(`SELECT COALESCE(tags, '') FROM rooms WHERE id = ?`, roomID).Scan(&tags)
		if err == nil && strings.Contains(tags, "needs-synthesis") {
			parts := strings.Split(tags, ",")
			var newParts []string
			for _, p := range parts {
				if p != "needs-synthesis" && p != "" {
					newParts = append(newParts, p)
				}
			}
			newTags := strings.Join(newParts, ",")
			_, _ = s.DB.Exec(`UPDATE rooms SET tags = ? WHERE id = ?`, newTags, roomID)
		}
	}

	// Update room's updated_at — best-effort, don't fail the post on this
	_, _ = s.DB.Exec(`UPDATE rooms SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, roomID)

	return id, nil
}

func (s *Server) UpdateMessage(messageID string, newContent, newMessageType string) (*Message, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if newMessageType != "" {
		_, err := s.DB.Exec(
			`UPDATE messages SET content = ?, message_type = ? WHERE id = ?`,
			newContent, newMessageType, messageID,
		)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := s.DB.Exec(
			`UPDATE messages SET content = ? WHERE id = ?`,
			newContent, messageID,
		)
		if err != nil {
			return nil, err
		}
	}

	m, err := scanMessage(s.DB.QueryRow(
		fmt.Sprintf(`SELECT %s FROM messages WHERE id = ?`, messageColumns),
		messageID,
	))
	if err != nil {
		return nil, err
	}

	// Update room's updated_at
	_, _ = s.DB.Exec(`UPDATE rooms SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, m.RoomID)

	return &m, nil
}

func (s *Server) PinMessage(roomID string, messageID string) (bool, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Verify message exists and belongs to the room
	var currentlyPinned bool
	var actualRoomID string
	err := s.DB.QueryRow(`SELECT room_id, pinned FROM messages WHERE id = ?`, messageID).Scan(&actualRoomID, &currentlyPinned)
	if err != nil {
		return false, err
	}
	if actualRoomID != roomID {
		return false, fmt.Errorf("message %.8s belongs to room '%s', not '%s'", messageID, actualRoomID, roomID)
	}

	if currentlyPinned {
		// Toggle off
		_, err := s.DB.Exec(`UPDATE messages SET pinned = 0 WHERE id = ?`, messageID)
		return false, err
	}

	// Unpin any existing pinned message in this room
	_, _ = s.DB.Exec(`UPDATE messages SET pinned = 0 WHERE room_id = ? AND pinned = 1`, roomID)

	// Pin the target
	_, err = s.DB.Exec(`UPDATE messages SET pinned = 1 WHERE id = ?`, messageID)
	if err != nil {
		return false, err
	}

	return true, nil
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
	defer rows.Close()

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
	defer rows.Close()

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
	defer rows.Close()

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
	rows, err := s.DB.Query(`
		SELECT id, room_id, author, content, message_type, is_summary, reply_to, pinned, timestamp
		FROM (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY message_type ORDER BY id DESC) as rn
			FROM messages
			WHERE room_id = ? AND is_summary = 0
		) ranked
		WHERE rn <= 2
		ORDER BY message_type, id DESC`,
		roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
		where += ` AND m.room_id = ?`
		args = append(args, roomID)
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

	q := fmt.Sprintf(`SELECT m.id, m.room_id, m.author, m.content, m.message_type, m.is_summary, m.reply_to, m.pinned, m.timestamp FROM messages m%s %s %s LIMIT ?`, join, where, orderBy)
	args = append(args, limit)

	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

func (s *Server) DeleteMessages(ids []string) (int64, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if len(ids) == 0 {
		return 0, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	q := fmt.Sprintf(`DELETE FROM messages WHERE id IN (%s)`, strings.Join(placeholders, ","))
	res, err := s.DB.Exec(q, args...)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
