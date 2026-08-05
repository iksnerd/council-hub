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

// maxPrefixCandidates bounds how many rows ResolveMessageID displays for an
// ambiguous prefix — enough to show the ambiguity is real without rendering an
// unbounded candidate list.
const maxPrefixCandidates = 6

// minResolvePrefixLen is the shortest prefix ResolveMessageID will attempt to
// match. Every display surface prints 8-char prefixes (#%.8s), so anything
// shorter never came from our own output — and short prefixes both match too
// promiscuously to be safe addresses and force wide index scans.
const minResolvePrefixLen = 8

// PrefixCandidate is one match for an ambiguous ID prefix, with enough context
// (room, author, type) to tell candidates apart at a glance.
type PrefixCandidate struct {
	ID          string
	RoomID      string
	Author      string
	MessageType string
}

// ErrAmbiguousMessageID is returned by ResolveMessageID when a prefix matches
// more than one message. Candidates is capped at maxPrefixCandidates; Truncated
// is true if more matches exist beyond what's listed.
type ErrAmbiguousMessageID struct {
	Prefix     string
	Candidates []PrefixCandidate
	Truncated  bool
}

func (e *ErrAmbiguousMessageID) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "'%s' is an ambiguous prefix — multiple messages match:", e.Prefix)
	for _, c := range e.Candidates {
		fmt.Fprintf(&b, "\n  #%s in %s by %s (%s)", c.ID, c.RoomID, c.Author, c.MessageType)
	}
	if e.Truncated {
		b.WriteString("\n  ...and more")
	}
	b.WriteString("\n\nUse a longer prefix, or the full message ID (from search_messages or get_links), to disambiguate.")
	return b.String()
}

// ResolveMessageID resolves a possibly-truncated message ID to its full UUIDv7.
// Every display surface (transcripts, notebook entries, get_links, re:/supersedes
// annotations) prints an 8-char prefix (#%.8s) for readability, but UUIDv7's
// first 8 hex chars are only the top bits of a millisecond timestamp — messages
// posted within the same ~65s window collide on it. Every ID-taking tool used to
// require an exact full ID, so a prefix copied from a transcript silently
// resolved to "not found" with no hint why. This makes those prefixes into real
// addresses: an exact match wins immediately (so already-full IDs are untouched
// and incur no extra query cost beyond the initial lookup); otherwise a prefix
// of at least minResolvePrefixLen chars resolves if unique, or fails loudly
// naming the candidates if ambiguous. Shorter non-exact input is rejected as
// not-found rather than matched — it never came from our own display output.
func (s *Server) ResolveMessageID(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("message id is required")
	}

	var exact string
	err := s.DB.QueryRow(`SELECT id FROM messages WHERE id = ?`, id).Scan(&exact)
	if err == nil {
		return exact, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	if len(id) < minResolvePrefixLen {
		return "", fmt.Errorf("message '%s' not found (prefixes must be at least %d characters; use the 8-char #prefix printed in transcripts, or the full ID)", id, minResolvePrefixLen)
	}

	// Range scan instead of LIKE: `id LIKE ? || '%'` can't use the primary-key
	// index under the default BINARY collation, so it full-scans messages. UUIDv7
	// IDs are lowercase hex plus '-', all of which sort below 'g', so prefix+"g"
	// is a strict upper bound for every id sharing the prefix — the pair of
	// comparisons becomes an index range seek. Fetch one row past the display cap
	// so Truncated ("...and more") is exact, not a guess.
	rows, qerr := s.DB.Query(
		`SELECT id, room_id, author, message_type FROM messages WHERE id >= ? AND id < ? ORDER BY id LIMIT ?`,
		id, id+"g", maxPrefixCandidates+1,
	)
	if qerr != nil {
		return "", qerr
	}
	defer func() { _ = rows.Close() }()

	var candidates []PrefixCandidate
	for rows.Next() {
		var c PrefixCandidate
		if serr := rows.Scan(&c.ID, &c.RoomID, &c.Author, &c.MessageType); serr != nil {
			continue
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("message '%s' not found (checked as both a full ID and a prefix)", id)
	case 1:
		return candidates[0].ID, nil
	default:
		truncated := len(candidates) > maxPrefixCandidates
		if truncated {
			candidates = candidates[:maxPrefixCandidates]
		}
		return "", &ErrAmbiguousMessageID{Prefix: id, Candidates: candidates, Truncated: truncated}
	}
}

// GetRevisionHistory returns every version of a message in chronological order
// (oldest → newest), walking the append-only revises chain. messageID may name any
// node in the chain — the walk finds the head, then follows revises back to the
// root. A message that's never been edited returns a single-element slice.
func (s *Server) GetRevisionHistory(messageID string) ([]Message, error) {
	if _, err := s.GetMessageByID(messageID); err != nil {
		return nil, err
	}
	id := s.headOfRevisionChain(messageID)
	var chain []Message
	for i := 0; i < 1000 && id != ""; i++ { // bound against any accidental cycle
		m, err := s.GetMessageByID(id)
		if err != nil {
			break
		}
		chain = append(chain, m)
		id = m.Revises
	}
	// The head read can race a concurrent purge and leave the chain empty even
	// though the message existed at the top of this call; never hand back an empty
	// slice with a nil error — callers index chain[len-1].
	if len(chain) == 0 {
		return nil, fmt.Errorf("message #%.8s not found", messageID)
	}
	// chain is newest → oldest; flip to chronological.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
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
		fmt.Sprintf(`SELECT %s FROM messages WHERE room_id = ? AND pinned = 1 AND `+headClause("")+` LIMIT 1`, messageColumns),
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

	// Get last N messages in reverse, then flip to chronological. Collapse to head
	// revisions; retracted nodes stay (they render as tombstones).
	rows, err := s.DB.Query(fmt.Sprintf(`SELECT %s FROM messages WHERE room_id = ? AND `+headClause("")+` ORDER BY id DESC LIMIT ?`, messageColumns), roomID, limit)
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
		WHERE room_id = ? AND id > ? AND `+headClause("")+`
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
			WHERE room_id = ? AND is_summary = 0 AND `+liveClause("")+`
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
	// Search surfaces only current, live nodes: head revisions, never retracted
	// tombstones. Old revisions linger in the FTS index but are filtered here.
	where := `WHERE 1=1 AND ` + liveClause("m")
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
