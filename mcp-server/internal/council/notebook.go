package council

import (
	"fmt"
	"strings"
)

// NotebookEntry is one item in a project notebook timeline: a typed message
// woven from one of the project's rooms. Repo is the owning room's repo so
// {sha:...} tokens resolve against the room the entry came from — rooms in
// the same project may point at different repositories.
type NotebookEntry struct {
	Message
	Repo string `json:"repo"`
}

// DefaultNotebookTypes are the message types compiled into a notebook when the
// caller doesn't specify any: the decision → action → synthesis spine of a
// project plus note (human journal entries), skipping exploratory chatter.
var DefaultNotebookTypes = []string{"decision", "action", "synthesis", "note"}

const (
	defaultNotebookLimit = 100
	maxNotebookLimit     = 500
)

// GetNotebookEntries returns typed messages from every room in a project as a
// single chronological timeline. UUIDv7 message IDs sort lexicographically by
// creation time, so ORDER BY id weaves all rooms together without a timestamp
// merge, and afterID works as a cross-room delta cursor. When limit truncates,
// the most recent entries are kept (the result stays chronological).
func (s *Server) GetNotebookEntries(project string, types []string, since, until, afterID string, limit int) ([]NotebookEntry, error) {
	project = normalizeProject(project)
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	if len(types) == 0 {
		types = DefaultNotebookTypes
	}
	if limit <= 0 {
		limit = defaultNotebookLimit
	}
	if limit > maxNotebookLimit {
		limit = maxNotebookLimit
	}

	// Normalize timestamps — accept both "2026-03-31T12:00:00" and
	// "2026-03-31 12:00:00" (stored format uses a space).
	since = strings.ReplaceAll(since, "T", " ")
	until = strings.ReplaceAll(until, "T", " ")

	typePlaceholders := make([]string, len(types))
	args := []any{project}
	for i, t := range types {
		typePlaceholders[i] = "?"
		args = append(args, strings.TrimSpace(t))
	}

	prefixed := "m." + strings.ReplaceAll(messageColumns, ", ", ", m.")
	query := fmt.Sprintf(`
		SELECT %s, COALESCE(r.repo, '')
		FROM messages m
		JOIN rooms r ON m.room_id = r.id
		WHERE r.project = ? AND m.is_summary = 0 AND m.message_type IN (%s)`,
		prefixed, strings.Join(typePlaceholders, ","))

	if since != "" {
		query += ` AND m.timestamp >= ?`
		args = append(args, since)
	}
	if until != "" {
		query += ` AND m.timestamp <= ?`
		args = append(args, until)
	}
	if afterID != "" {
		query += ` AND m.id > ?`
		args = append(args, afterID)
	}

	query += ` ORDER BY m.id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []NotebookEntry
	for rows.Next() {
		var e NotebookEntry
		if err := rows.Scan(&e.ID, &e.RoomID, &e.Author, &e.Content, &e.MessageType, &e.IsSummary, &e.ReplyTo, &e.Pinned, &e.Reactions, &e.Mentions, &e.Timestamp, &e.Repo); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Flip DESC-limited rows back to chronological order.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, nil
}

// CountRoomsInProject returns how many rooms belong to a project. Used to
// distinguish "project has no notebook-worthy messages yet" from "no such
// project" in the read_notebook handler.
func (s *Server) CountRoomsInProject(project string) (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM rooms WHERE project = ?`, normalizeProject(project)).Scan(&count)
	return count, err
}
