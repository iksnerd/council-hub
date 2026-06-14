package council

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// PostMessage posts a message to a room. Existing callers use this signature.
func (s *Server) PostMessage(roomID, author, content, messageType string, replyTo string) (string, error) {
	return s.postMessageCore(roomID, author, content, messageType, replyTo, "", "")
}

// PostMessageWithMentions posts a message that explicitly mentions other agents.
// mentions is a CSV of agent names (e.g. "claude,gemini-cli"). Agents can query
// get_mentions on startup to find messages addressed to them.
func (s *Server) PostMessageWithMentions(roomID, author, content, messageType, replyTo, mentions string) (string, error) {
	return s.postMessageCore(roomID, author, content, messageType, replyTo, mentions, "")
}

// PostMessageWithRefs is the full post path: like PostMessageWithMentions plus an
// optional supersedes link (the ID of a message this one replaces, e.g. an earlier
// synthesis). Renders as "supersedes #x" so tooling can dim the dead version.
func (s *Server) PostMessageWithRefs(roomID, author, content, messageType, replyTo, mentions, supersedes string) (string, error) {
	return s.postMessageCore(roomID, author, content, messageType, replyTo, mentions, supersedes)
}

func (s *Server) postMessageCore(roomID, author, content, messageType, replyTo, mentions, supersedes string) (string, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if messageType == "" {
		messageType = "message"
	}

	id := uuid.Must(uuid.NewV7()).String()
	_, err := s.DB.Exec(
		`INSERT INTO messages (id, room_id, author, content, message_type, reply_to, mentions, supersedes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, roomID, author, content, messageType, replyTo, mentions, supersedes,
	)
	if err != nil {
		return "", err
	}

	// Auto-clear linter status tags the moment the condition that set them no longer holds:
	//   - a synthesis post clears `needs-synthesis`
	//   - an action post clears `stale-plan` (its id is newer than every plan, so the
	//     latest plan now has a following action — the handoff was picked up)
	//   - a synthesis or superseding post clears `incoherent` — a synthesis reconciles a
	//     contradiction, and superseding declares a winner over a contradiction/duplicate;
	//     the next sweep re-adds the flag if a conflict still stands.
	//   - any genuine (non-system) activity clears `stale` — the room is live again.
	// The linter posts as author "system", so its own flag messages must not self-clear.
	if messageType == "synthesis" || messageType == "action" || supersedes != "" || author != "system" {
		var tags string
		if err := s.DB.QueryRow(`SELECT COALESCE(tags, '') FROM rooms WHERE id = ?`, roomID).Scan(&tags); err == nil {
			newTags := tags
			if messageType == "synthesis" {
				newTags = removeTag(newTags, "needs-synthesis")
			}
			if messageType == "action" {
				newTags = removeTag(newTags, "stale-plan")
			}
			if messageType == "synthesis" || supersedes != "" {
				newTags = removeTag(newTags, "incoherent")
			}
			if author != "system" {
				newTags = removeTag(newTags, "stale")
			}
			if newTags != tags {
				_, _ = s.DB.Exec(`UPDATE rooms SET tags = ? WHERE id = ?`, newTags, roomID)
			}
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

// ErrAlreadyRevised is returned when a caller tries to edit a node that a newer
// revision has already superseded. Edits must target the head of the chain so we
// never fork it; HeadID points the caller at the current version to edit instead.
type ErrAlreadyRevised struct {
	HeadID string
}

func (e *ErrAlreadyRevised) Error() string {
	return fmt.Sprintf("message already revised — edit the current version #%.8s instead", e.HeadID)
}

func (s *Server) UpdateMessage(messageID string, newContent, newMessageType string) (*Message, error) {
	return s.UpdateMessageWithExpected(messageID, newContent, newMessageType, "", "")
}

// UpdateMessageWithExpected edits a message by appending a new revision rather than
// overwriting in place: the prior version is preserved as an immutable node, and
// the new node points back at it via `revises` (the NLS Journal property — nothing
// is destroyed, every version stays addressable). The old node is flagged
// revised = 1 so reads collapse to the head; the new node inherits the old one's
// structural role (reply_to, mentions, supersedes, and the pin, which follows the
// head). Returns the new head message.
//
// If expectedContent is non-empty it must match the current content (optimistic
// locking) or *ErrContentChanged is returned. Editing an already-superseded node
// returns *ErrAlreadyRevised. author attributes the edit; "" inherits the original
// author.
func (s *Server) UpdateMessageWithExpected(messageID, newContent, newMessageType, expectedContent, author string) (*Message, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Read the node being edited. It must exist, be the head (not already revised),
	// and not be retracted.
	var (
		roomID, origAuthor, curContent, curType string
		curReplyTo, curMentions, curSupersedes  string
		curPinned, revised                      bool
		retractedAt                             sql.NullTime
	)
	err := s.DB.QueryRow(
		`SELECT room_id, author, content, message_type, reply_to, mentions, supersedes, pinned, revised, retracted_at FROM messages WHERE id = ?`,
		messageID,
	).Scan(&roomID, &origAuthor, &curContent, &curType, &curReplyTo, &curMentions, &curSupersedes, &curPinned, &revised, &retractedAt)
	if err != nil {
		return nil, err
	}
	if revised {
		head := s.headOfRevisionChain(messageID)
		return nil, &ErrAlreadyRevised{HeadID: head}
	}
	if retractedAt.Valid {
		return nil, fmt.Errorf("message #%.8s is retracted; restore it before editing", messageID)
	}
	if expectedContent != "" && curContent != expectedContent {
		return nil, &ErrContentChanged{CurrentContent: curContent}
	}

	newType := curType
	if newMessageType != "" {
		newType = newMessageType
	}
	if author == "" {
		author = origAuthor
	}

	// Append the new revision as a fresh immutable node pointing back at the old one.
	newID := uuid.Must(uuid.NewV7()).String()
	if _, err := s.DB.Exec(
		`INSERT INTO messages (id, room_id, author, content, message_type, reply_to, mentions, supersedes, revises, pinned) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newID, roomID, author, newContent, newType, curReplyTo, curMentions, curSupersedes, messageID, curPinned,
	); err != nil {
		return nil, err
	}

	// Flag the old node as a superseded revision and move the pin to the head, so
	// reads (which filter revised = 0) surface exactly one current version.
	if _, err := s.DB.Exec(`UPDATE messages SET revised = 1, pinned = 0 WHERE id = ?`, messageID); err != nil {
		return nil, err
	}

	// Drop the superseded node's vector: it's filtered out of semantic search by
	// liveClause anyway, so keeping it only bloats the vector table per edit. The
	// row itself stays (revision history walks it); only the embedding is dropped.
	s.deleteVectorsLocked("message_vectors", []string{messageID})

	m, err := scanMessage(s.DB.QueryRow(
		fmt.Sprintf(`SELECT %s FROM messages WHERE id = ?`, messageColumns),
		newID,
	))
	if err != nil {
		return nil, err
	}

	_, _ = s.DB.Exec(`UPDATE rooms SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, roomID)
	s.EmbedAsync("message_vectors", newID, newContent)

	return &m, nil
}

// headOfRevisionChain walks revises pointers forward from a (possibly old) node to
// the current head — the newest version that nothing else revises. Used to point a
// caller who edited a stale node at the version they should edit instead.
func (s *Server) headOfRevisionChain(messageID string) string {
	id := messageID
	for i := 0; i < 1000; i++ { // bound the walk against any accidental cycle
		var next string
		err := s.DB.QueryRow(`SELECT id FROM messages WHERE revises = ?`, id).Scan(&next)
		if err != nil || next == "" {
			return id
		}
		id = next
	}
	return id
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

// RetractMessages tombstones messages instead of destroying them: it sets
// retracted_at/retracted_by but leaves the rows, their content, and their links
// intact. Retracted nodes still render (as "[retracted]") and their graph edges
// stay valid — the immutable counterpart to deletion. Already-retracted messages
// are skipped. Returns the number newly retracted.
func (s *Server) RetractMessages(ids []string, retractedBy string) (int64, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if len(ids) == 0 {
		return 0, nil
	}

	placeholders := make([]string, len(ids))
	args := []any{retractedBy}
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}

	q := fmt.Sprintf(
		`UPDATE messages SET retracted_at = CURRENT_TIMESTAMP, retracted_by = ? WHERE id IN (%s) AND retracted_at IS NULL`,
		strings.Join(placeholders, ","),
	)
	res, err := s.DB.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RestoreMessages reverses a retraction: it clears retracted_at/retracted_by so the
// tombstoned node renders normally again. Retraction is meant to be reversible —
// only PurgeMessages is final. Returns the number of messages newly restored.
func (s *Server) RestoreMessages(ids []string) (int64, error) {
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

	q := fmt.Sprintf(
		`UPDATE messages SET retracted_at = NULL, retracted_by = '' WHERE id IN (%s) AND retracted_at IS NOT NULL`,
		strings.Join(placeholders, ","),
	)
	res, err := s.DB.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeMessages permanently destroys messages — the deliberate escape hatch from
// immutability, for content that must not persist (a leaked secret, PII). Unlike
// RetractMessages it hard-deletes the rows, cascade-cleans their links so the graph
// never dangles, and drops their vectors. Prefer RetractMessages for everything
// else.
func (s *Server) PurgeMessages(ids []string) (int64, error) {
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

	// Cascade-clean any links that reference the purged messages (either endpoint),
	// so the link graph never points at a missing message.
	inList := strings.Join(placeholders, ",")
	_, _ = s.DB.Exec(
		fmt.Sprintf(`DELETE FROM message_links WHERE from_id IN (%s) OR to_id IN (%s)`, inList, inList),
		append(append([]any{}, args...), args...)...,
	)

	// Clean up vectors (best-effort, already holding lock)
	s.deleteVectorsLocked("message_vectors", ids)

	return res.RowsAffected()
}
