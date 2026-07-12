package council

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// RoomStats holds aggregate statistics for a room.
type RoomStats struct {
	RoomID           string
	Status           string
	MessageCount     int
	LatestMessageID  string
	Participants     map[string]int // author -> message count
	TypeCounts       map[string]int // message_type -> count
	FirstMessage     time.Time
	LastMessage      time.Time
	PinnedMessageID  string // ID of the pinned message, or "" if none
	MessagesSincePin int    // messages posted after the pin — a one-call "is the pin stale?" check
}

func (s *Server) GetRoomStats(roomID string) (RoomStats, error) {
	var stats RoomStats
	stats.RoomID = roomID
	stats.Participants = make(map[string]int)
	stats.TypeCounts = make(map[string]int)

	// Verify room exists and get status
	var status string
	err := s.DB.QueryRow(`SELECT status FROM rooms WHERE id = ?`, roomID).Scan(&status)
	if err != nil {
		return stats, fmt.Errorf("room '%s' not found", roomID)
	}
	stats.Status = status

	// Get aggregate stats + latest message ID
	// Counts reflect live nodes — head revisions only (revised = 0) — so stats match
	// what the transcript shows. Retracted tombstones still count (they still render).
	var firstMsg, lastMsg, latestID sql.NullString
	err = s.DB.QueryRow(`SELECT COUNT(*), MIN(timestamp), MAX(timestamp), MAX(id) FROM messages WHERE room_id = ? AND `+headClause(""), roomID).
		Scan(&stats.MessageCount, &firstMsg, &lastMsg, &latestID)
	if err != nil {
		return stats, err
	}
	if firstMsg.Valid {
		stats.FirstMessage, _ = time.Parse("2006-01-02 15:04:05", firstMsg.String)
	}
	if lastMsg.Valid {
		stats.LastMessage, _ = time.Parse("2006-01-02 15:04:05", lastMsg.String)
	}
	if latestID.Valid {
		stats.LatestMessageID = latestID.String
	}

	// Pin staleness signal: how many messages have landed since the pinned message.
	// A one-call "is the pin stale?" check that pairs with the stale-pin linter flag.
	// QueryRow holds no connection open, so this is safe before the per-author rows.
	var pinnedID sql.NullString
	_ = s.DB.QueryRow(`SELECT id FROM messages WHERE room_id = ? AND pinned = 1 LIMIT 1`, roomID).Scan(&pinnedID)
	if pinnedID.Valid && pinnedID.String != "" {
		stats.PinnedMessageID = pinnedID.String
		_ = s.DB.QueryRow(`SELECT COUNT(*) FROM messages WHERE room_id = ? AND id > ? AND `+headClause(""), roomID, pinnedID.String).
			Scan(&stats.MessagesSincePin)
	}

	// Get per-author counts
	rows, err := s.DB.Query(`SELECT author, COUNT(*) FROM messages WHERE room_id = ? AND `+headClause("")+` GROUP BY author ORDER BY COUNT(*) DESC`, roomID)
	if err != nil {
		return stats, err
	}

	for rows.Next() {
		var author string
		var count int
		if err := rows.Scan(&author, &count); err != nil {
			_ = rows.Close()
			return stats, err
		}
		stats.Participants[author] = count
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return stats, err
	}
	// Close before the next query: SetMaxOpenConns(1) means both *Rows share one
	// underlying connection, so the second query must not run while this is open.
	if err := rows.Close(); err != nil {
		return stats, err
	}

	// Get per-type counts
	typeRows, err := s.DB.Query(`SELECT message_type, COUNT(*) FROM messages WHERE room_id = ? AND is_summary = 0 AND `+headClause("")+` GROUP BY message_type ORDER BY COUNT(*) DESC`, roomID)
	if err != nil {
		return stats, err
	}
	defer func() { _ = typeRows.Close() }()

	for typeRows.Next() {
		var msgType string
		var count int
		if err := typeRows.Scan(&msgType, &count); err != nil {
			return stats, err
		}
		stats.TypeCounts[msgType] = count
	}

	return stats, typeRows.Err()
}

// DigestEntry represents one room's activity in a project digest.
type DigestEntry struct {
	RoomID          string `json:"room_id"`
	NewMessages     int    `json:"new_messages"`
	LatestAuthor    string `json:"latest_author"`
	LatestExcerpt   string `json:"latest_excerpt"`
	Tags            string `json:"tags"`
	DecisionCount   int    `json:"decision_count"`
	SynthesisCount  int    `json:"synthesis_count"`
	LatestMessageID string `json:"latest_message_id"`
}

// GetDigest returns rooms with messages since the given timestamp, plus any rooms needing attention.
func (s *Server) GetDigest(project, since string) ([]DigestEntry, error) {
	// Normalize timestamp — accept both "2026-03-31T12:00:00" and "2026-03-31 12:00:00"
	since = strings.ReplaceAll(since, "T", " ")
	project = normalizeProject(project)

	// The lm join resolves each room's latest head revision ONCE (headClause, so
	// an edited message's stale prior version never surfaces as the "latest"
	// excerpt/author) and pulls author/content/retracted_at/retracted_by from that
	// single row — the same resolved-head-id pattern as GetOutline's room_ref join.
	// retracted_at/by come along so the caller can render a tombstone via
	// DisplayContent instead of broadcasting withdrawn content.
	query := `
		SELECT m.room_id,
		       SUM(CASE WHEN m.timestamp > ? AND ` + liveClause("m") + ` THEN 1 ELSE 0 END) as new_msgs,
		       COALESCE(lm.author, '') as latest_author,
		       COALESCE(lm.content, '') as latest_content,
		       lm.retracted_at as latest_retracted_at,
		       COALESCE(lm.retracted_by, '') as latest_retracted_by,
		       COALESCE(r.tags, '') as tags,
		       (SELECT COUNT(*) FROM messages WHERE room_id = m.room_id AND message_type = 'decision' AND ` + liveClause("") + `) as decision_count,
		       (SELECT COUNT(*) FROM messages WHERE room_id = m.room_id AND message_type = 'synthesis' AND ` + liveClause("") + `) as synthesis_count,
		       COALESCE((SELECT MAX(id) FROM messages WHERE room_id = m.room_id), '') as latest_message_id
		FROM messages m
		JOIN rooms r ON m.room_id = r.id
		LEFT JOIN messages lm ON lm.id = (
			SELECT id FROM messages WHERE room_id = m.room_id AND ` + headClause("") + ` ORDER BY id DESC LIMIT 1
		)
		WHERE (m.timestamp > ? OR r.tags LIKE '%stale%' OR r.tags LIKE '%needs-synthesis%')`

	var args []any
	args = append(args, since, since)

	if project != "" {
		query += ` AND r.project = ?`
		args = append(args, project)
	}

	query += ` GROUP BY m.room_id ORDER BY MAX(m.timestamp) DESC`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []DigestEntry
	for rows.Next() {
		var d DigestEntry
		var retractedAt sql.NullTime
		var retractedBy string
		if err := rows.Scan(&d.RoomID, &d.NewMessages, &d.LatestAuthor, &d.LatestExcerpt, &retractedAt, &retractedBy, &d.Tags, &d.DecisionCount, &d.SynthesisCount, &d.LatestMessageID); err != nil {
			return nil, err
		}
		d.LatestExcerpt = DisplayContent(Message{Content: d.LatestExcerpt, RetractedAt: retractedAt, RetractedBy: retractedBy})
		entries = append(entries, d)
	}
	return entries, rows.Err()
}

// GetMessageCounts returns a map of room_id -> message count for all rooms.
func (s *Server) GetMessageCounts() map[string]int {
	counts := make(map[string]int)
	rows, err := s.DB.Query(`SELECT room_id, COUNT(*) FROM messages WHERE ` + headClause("") + ` GROUP BY room_id`)
	if err != nil {
		return counts
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var roomID string
		var count int
		if err := rows.Scan(&roomID, &count); err != nil {
			continue
		}
		counts[roomID] = count
	}
	return counts
}

// GetPinnedExcerpts returns a map of room_id -> truncated pinned message content.
func (s *Server) GetPinnedExcerpts(roomIDs []string) map[string]string {
	excerpts := make(map[string]string)
	if len(roomIDs) == 0 {
		return excerpts
	}

	placeholders := make([]string, len(roomIDs))
	args := make([]any, len(roomIDs))
	for i, id := range roomIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT room_id, content, retracted_at, retracted_by FROM messages WHERE pinned = 1 AND `+headClause("")+` AND room_id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return excerpts
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var roomID, content, retractedBy string
		var retractedAt sql.NullTime
		if err := rows.Scan(&roomID, &content, &retractedAt, &retractedBy); err != nil {
			continue
		}
		content = DisplayContent(Message{Content: content, RetractedAt: retractedAt, RetractedBy: retractedBy})
		content = strings.ReplaceAll(content, "\n", " ")
		content = TruncateRunes(content, 60, " ", 40)
		excerpts[roomID] = content
	}
	return excerpts
}

// GetLatestMessageIDs returns a map of room_id -> latest message ID for all rooms.
func (s *Server) GetLatestMessageIDs() map[string]string {
	ids := make(map[string]string)
	rows, err := s.DB.Query(`SELECT room_id, MAX(id) FROM messages GROUP BY room_id`)
	if err != nil {
		return ids
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var roomID, latestID string
		if err := rows.Scan(&roomID, &latestID); err != nil {
			continue
		}
		ids[roomID] = latestID
	}
	return ids
}

// GetRoomsNeedingSummary returns room IDs with more than threshold unsummarized messages.
func (s *Server) GetRoomsNeedingSummary(threshold int) ([]string, error) {
	rows, err := s.DB.Query(`
		SELECT room_id
		FROM messages
		WHERE is_summary = 0
		  AND id > COALESCE(
		      (SELECT MAX(m2.id) FROM messages m2 WHERE m2.room_id = messages.room_id AND m2.is_summary = 1), ''
		  )
		GROUP BY room_id
		HAVING COUNT(*) > ?`,
		threshold,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var roomIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		roomIDs = append(roomIDs, id)
	}
	return roomIDs, rows.Err()
}
