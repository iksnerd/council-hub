package council

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// GetTranscript returns summaries + all individual messages after the latest summary.
func (s *Server) GetTranscript(roomID string) ([]Message, error) {
	rows, err := s.DB.Query(fmt.Sprintf(`
		SELECT %s
		FROM messages
		WHERE room_id = ?
		  AND (is_summary = 1 OR id > COALESCE(
		      (SELECT MAX(id) FROM messages WHERE room_id = ? AND is_summary = 1), ''
		  ))
		ORDER BY timestamp ASC`, messageColumns),
		roomID, roomID,
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

// GetUnsummarizedMessages returns messages after the latest summary for a room.
func (s *Server) GetUnsummarizedMessages(roomID string) ([]Message, error) {
	rows, err := s.DB.Query(fmt.Sprintf(`
		SELECT %s
		FROM messages
		WHERE room_id = ?
		  AND is_summary = 0
		  AND id > COALESCE(
		      (SELECT MAX(id) FROM messages WHERE room_id = ? AND is_summary = 1), ''
		  )
		ORDER BY timestamp ASC`, messageColumns),
		roomID, roomID,
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

// InsertSummary inserts a summary message into a room.
func (s *Server) InsertSummary(roomID, summary string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	id := uuid.Must(uuid.NewV7()).String()
	_, err := s.DB.Exec(
		`INSERT INTO messages (id, room_id, author, content, message_type, is_summary) VALUES (?, ?, ?, ?, 'message', 1)`,
		id, roomID, "System", summary,
	)
	if err != nil {
		return err
	}

	_, _ = s.DB.Exec(`UPDATE rooms SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, roomID)
	return nil
}

func (s *Server) ArchiveRoom(roomID string) (string, error) {
	room, err := s.GetRoom(roomID)
	if err != nil {
		return "", fmt.Errorf("room '%s' not found", roomID)
	}

	messages, err := s.GetTranscript(roomID)
	if err != nil {
		return "", fmt.Errorf("failed to read transcript: %w", err)
	}

	transcript := FormatTranscript(room, messages)

	// Derive archive dir from DB path
	archiveDir := filepath.Join(filepath.Dir(s.DBPath), "archives")
	if s.DBPath == ":memory:" {
		archiveDir = "archives"
	}

	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}

	archivePath := filepath.Join(archiveDir, roomID+".md")
	if err := os.WriteFile(archivePath, []byte(transcript), 0644); err != nil {
		return "", fmt.Errorf("failed to write archive: %w", err)
	}

	return archivePath, nil
}
