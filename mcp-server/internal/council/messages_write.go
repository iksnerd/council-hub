package council

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// PostMessage posts a message to a room. Existing callers use this signature.
func (s *Server) PostMessage(roomID, author, content, messageType string, replyTo string) (string, error) {
	return s.postMessageCore(roomID, author, content, messageType, replyTo, "")
}

// PostMessageWithMentions posts a message that explicitly mentions other agents.
// mentions is a CSV of agent names (e.g. "claude,gemini-cli"). Agents can query
// get_mentions on startup to find messages addressed to them.
func (s *Server) PostMessageWithMentions(roomID, author, content, messageType, replyTo, mentions string) (string, error) {
	return s.postMessageCore(roomID, author, content, messageType, replyTo, mentions)
}

func (s *Server) postMessageCore(roomID, author, content, messageType, replyTo, mentions string) (string, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if messageType == "" {
		messageType = "message"
	}

	id := uuid.Must(uuid.NewV7()).String()
	_, err := s.DB.Exec(
		`INSERT INTO messages (id, room_id, author, content, message_type, reply_to, mentions) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, roomID, author, content, messageType, replyTo, mentions,
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

	// Generate embedding asynchronously (non-fatal)
	s.EmbedAsync("message_vectors", id, content)

	return id, nil
}

// ErrContentChanged is returned by UpdateMessage when the message content has
// changed since the caller last read it (optimistic concurrency failure).
type ErrContentChanged struct {
	CurrentContent string
}

func (e *ErrContentChanged) Error() string {
	return "content changed since last read — re-read before updating"
}

func (s *Server) UpdateMessage(messageID string, newContent, newMessageType string) (*Message, error) {
	return s.UpdateMessageWithExpected(messageID, newContent, newMessageType, "")
}

// UpdateMessageWithExpected updates a message, optionally checking that the
// current content matches expectedContent before writing (optimistic locking).
// If expectedContent is "" the check is skipped (same as UpdateMessage).
// If the content has changed, returns *ErrContentChanged with the current content.
func (s *Server) UpdateMessageWithExpected(messageID, newContent, newMessageType, expectedContent string) (*Message, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Optimistic concurrency check: read current content before writing.
	if expectedContent != "" {
		var current string
		err := s.DB.QueryRow(`SELECT content FROM messages WHERE id = ?`, messageID).Scan(&current)
		if err != nil {
			return nil, err
		}
		if current != expectedContent {
			return nil, &ErrContentChanged{CurrentContent: current}
		}
	}

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

	// Re-embed if content changed (non-fatal)
	if newContent != "" {
		s.EmbedAsync("message_vectors", messageID, newContent)
	}

	return &m, nil
}

func (s *Server) MoveMessages(ids []string, targetRoomID string) (int, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Verify target room exists.
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM rooms WHERE id = ?`, targetRoomID).Scan(&count); err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, fmt.Errorf("target room '%s' not found", targetRoomID)
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids)+1)
	args[0] = targetRoomID
	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}

	res, err := s.DB.Exec(
		fmt.Sprintf(`UPDATE messages SET room_id = ? WHERE id IN (%s)`, strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		return 0, err
	}

	moved, _ := res.RowsAffected()

	// Bump target room's updated_at (best-effort).
	_, _ = s.DB.Exec(`UPDATE rooms SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, targetRoomID)

	return int(moved), nil
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

	// Clean up vectors (best-effort, already holding lock)
	s.deleteVectorsLocked("message_vectors", ids)

	return res.RowsAffected()
}
