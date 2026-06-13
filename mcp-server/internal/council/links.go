package council

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ValidLinkRelations are the typed edges an agent can assert between two messages.
// `reply` and `supersedes` are deliberately NOT here — those are column-backed on
// the message itself and surfaced by GetLinks as implicit edges, so the explicit
// link table only carries the new semantic relations.
var ValidLinkRelations = map[string]bool{
	"refines":     true,
	"contradicts": true,
	"implements":  true,
	"duplicates":  true,
	"depends-on":  true,
	"relates":     true,
}

// MessageLink is one edge in the message link graph. Implicit edges (reply,
// supersedes) are derived from message columns and have an empty ID/Author.
type MessageLink struct {
	ID       string `json:"id,omitempty"`
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Relation string `json:"relation"`
	Author   string `json:"author,omitempty"`
	Implicit bool   `json:"implicit"`
}

// CreateLink asserts a typed link from one message to another. Both endpoints must
// exist. Idempotent on (from, to, relation): re-asserting returns the existing link.
func (s *Server) CreateLink(fromID, toID, relation, author string) (string, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	relation = strings.ToLower(strings.TrimSpace(relation))
	if !ValidLinkRelations[relation] {
		return "", fmt.Errorf("invalid relation %q", relation)
	}
	if fromID == "" || toID == "" {
		return "", fmt.Errorf("from_id and to_id are required")
	}
	if fromID == toID {
		return "", fmt.Errorf("cannot link a message to itself")
	}
	for _, id := range []string{fromID, toID} {
		var one int
		if err := s.DB.QueryRow(`SELECT 1 FROM messages WHERE id = ?`, id).Scan(&one); err != nil {
			return "", fmt.Errorf("message %.8s not found", id)
		}
	}

	id := uuid.Must(uuid.NewV7()).String()
	res, err := s.DB.Exec(
		`INSERT OR IGNORE INTO message_links (id, from_id, to_id, relation, author) VALUES (?, ?, ?, ?, ?)`,
		id, fromID, toID, relation, author,
	)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Already linked — return the existing edge's ID rather than a phantom new one.
		var existing string
		_ = s.DB.QueryRow(`SELECT id FROM message_links WHERE from_id = ? AND to_id = ? AND relation = ?`,
			fromID, toID, relation).Scan(&existing)
		return existing, nil
	}
	return id, nil
}

// DeleteLink removes an explicit link by its ID.
func (s *Server) DeleteLink(linkID string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	res, err := s.DB.Exec(`DELETE FROM message_links WHERE id = ?`, linkID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("link %.8s not found", linkID)
	}
	return nil
}

// GetLinks returns a message's neighborhood in the link graph: outgoing edges (this
// message points at others) and incoming edges (others point here — the backlinks).
// Explicit typed links are merged with the implicit reply_to/supersedes edges so a
// caller sees the whole graph around a node, not just the new-style links.
func (s *Server) GetLinks(messageID string) (outgoing, incoming []MessageLink, err error) {
	var one int
	if err = s.DB.QueryRow(`SELECT 1 FROM messages WHERE id = ?`, messageID).Scan(&one); err != nil {
		return nil, nil, fmt.Errorf("message %.8s not found", messageID)
	}

	scan := func(query string) ([]MessageLink, error) {
		rows, qerr := s.DB.Query(query, messageID)
		if qerr != nil {
			return nil, qerr
		}
		defer func() { _ = rows.Close() }()
		var out []MessageLink
		for rows.Next() {
			var l MessageLink
			if serr := rows.Scan(&l.ID, &l.FromID, &l.ToID, &l.Relation, &l.Author); serr != nil {
				return nil, serr
			}
			out = append(out, l)
		}
		return out, rows.Err()
	}

	if outgoing, err = scan(`SELECT id, from_id, to_id, relation, COALESCE(author, '') FROM message_links WHERE from_id = ? ORDER BY created_at`); err != nil {
		return nil, nil, err
	}
	if incoming, err = scan(`SELECT id, from_id, to_id, relation, COALESCE(author, '') FROM message_links WHERE to_id = ? ORDER BY created_at`); err != nil {
		return nil, nil, err
	}

	// Implicit outgoing: this message's own reply_to / supersedes columns.
	var replyTo, supersedes string
	_ = s.DB.QueryRow(`SELECT COALESCE(reply_to, ''), COALESCE(supersedes, '') FROM messages WHERE id = ?`, messageID).
		Scan(&replyTo, &supersedes)
	if replyTo != "" {
		outgoing = append(outgoing, MessageLink{FromID: messageID, ToID: replyTo, Relation: "reply", Implicit: true})
	}
	if supersedes != "" {
		outgoing = append(outgoing, MessageLink{FromID: messageID, ToID: supersedes, Relation: "supersedes", Implicit: true})
	}

	// Implicit incoming: messages that reply to / supersede this one (the backlinks
	// we already render in the transcript, now queryable).
	implicitIn := func(column, relation string) error {
		rows, qerr := s.DB.Query(fmt.Sprintf(`SELECT id FROM messages WHERE %s = ? ORDER BY id`, column), messageID)
		if qerr != nil {
			return qerr
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var from string
			if serr := rows.Scan(&from); serr != nil {
				return serr
			}
			incoming = append(incoming, MessageLink{FromID: from, ToID: messageID, Relation: relation, Implicit: true})
		}
		return rows.Err()
	}
	if err = implicitIn("reply_to", "reply"); err != nil {
		return nil, nil, err
	}
	if err = implicitIn("supersedes", "supersedes"); err != nil {
		return nil, nil, err
	}

	return outgoing, incoming, nil
}
