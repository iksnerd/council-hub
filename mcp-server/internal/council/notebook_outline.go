package council

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Notebook is a curated, addressable outline over the project ledger — the
// Phase 2 "DKR spine" of the dev notebook. Where read_notebook(project)
// compiles a timeline automatically, a notebook outline is assembled by hand:
// prose sections interleaved with references to ledger messages.
type Notebook struct {
	ID         string
	Project    string
	Title      string
	EntryCount int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OutlineEntry is one row of a notebook outline. Kind "prose" carries freeform
// markdown authored in the notebook; kind "ref" transcludes a ledger message
// by ID; kind "room_ref" transcludes a room's live state — status, topic, and
// its latest decision/action — which makes a notebook of room_refs a living
// work list: resolving the room flips the entry, no list maintenance. All refs
// resolve at render time, never copied, so the outline can't drift from the
// ledger. Ref* fields are populated by GetOutline; RefFound is false when the
// referenced message/room no longer exists locally.
type OutlineEntry struct {
	ID       string
	Position int
	Kind     string // "ref" | "room_ref" | "query_ref" | "prose" | "task"
	RefID    string
	Prose    string

	// task only: a first-class checklist item. Prose carries the label; Status is
	// one of "open" | "doing" | "done", set with SetTaskStatus. Unlike a room_ref
	// (which derives its state from a room's live status), a task owns its state
	// directly — the lightweight counterpart for work that doesn't warrant its own
	// room. Non-task entries carry the column default "open" but ignore it.
	Status string

	RefRoomID  string
	RefAuthor  string
	RefType    string
	RefContent string
	RefPinned  bool
	RefTime    time.Time
	RefRepo    string
	RefFound   bool

	// room_ref only: the room's live status and topic.
	RefStatus string
	RefTopic  string
}

// validOutlineKinds gates the entry kinds accepted by AddOutlineEntry.
var validOutlineKinds = map[string]bool{"ref": true, "room_ref": true, "query_ref": true, "prose": true, "task": true}

// validTaskStatuses gates the states a task entry can hold: open (backlog),
// doing (actively worked), done (finished).
var validTaskStatuses = map[string]bool{"open": true, "doing": true, "done": true}

// CreateNotebook creates an empty notebook outline. The ID is the stable
// address used by edit_notebook and read_notebook(notebook_id=...). An empty
// project makes the notebook global: it can transclude messages from any room
// and is listed alongside every project's notebooks (cross-project TODOs,
// reading lists, standing checklists).
func (s *Server) CreateNotebook(id, project, title string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("notebook id is required")
	}
	project = normalizeProject(project)

	res, err := s.DB.Exec(
		`INSERT OR IGNORE INTO notebooks (id, project, title) VALUES (?, ?, ?)`,
		id, project, strings.TrimSpace(title),
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("notebook '%s' already exists", id)
	}
	return nil
}

// GetNotebook returns a notebook's metadata including its entry count.
func (s *Server) GetNotebook(id string) (Notebook, error) {
	var n Notebook
	err := s.DB.QueryRow(`
		SELECT id, project, title, created_at, updated_at,
		       (SELECT COUNT(*) FROM notebook_entries WHERE notebook_id = notebooks.id)
		FROM notebooks WHERE id = ?`, id,
	).Scan(&n.ID, &n.Project, &n.Title, &n.CreatedAt, &n.UpdatedAt, &n.EntryCount)
	if err == sql.ErrNoRows {
		return n, fmt.Errorf("notebook '%s' not found", id)
	}
	return n, err
}

// ListNotebooks returns notebooks, most recently updated first. With a project
// it returns that project's notebooks plus global ones (project = "") — global
// notebooks belong to every view. With an empty project it returns everything.
func (s *Server) ListNotebooks(project string) ([]Notebook, error) {
	query := `
		SELECT id, project, title, created_at, updated_at,
		       (SELECT COUNT(*) FROM notebook_entries WHERE notebook_id = notebooks.id)
		FROM notebooks`
	var args []any
	if project != "" {
		query += ` WHERE project IN (?, '')`
		args = append(args, normalizeProject(project))
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var notebooks []Notebook
	for rows.Next() {
		var n Notebook
		if err := rows.Scan(&n.ID, &n.Project, &n.Title, &n.CreatedAt, &n.UpdatedAt, &n.EntryCount); err != nil {
			return nil, err
		}
		notebooks = append(notebooks, n)
	}
	return notebooks, rows.Err()
}

// DeleteNotebook removes a notebook and its entries. Referenced ledger
// messages are untouched — refs are pointers, not copies.
func (s *Server) DeleteNotebook(id string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	res, err := s.DB.Exec(`DELETE FROM notebooks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("notebook '%s' not found", id)
	}
	_, err = s.DB.Exec(`DELETE FROM notebook_entries WHERE notebook_id = ?`, id)
	return err
}

// ErrAlreadyReferenced signals that a ref-like entry (ref/room_ref/query_ref)
// pointing at the same target already exists in the notebook, so AddOutlineEntry
// no-opped instead of appending a duplicate. EntryID is the pre-existing entry.
// Callers should treat this as a benign no-op, not a failure — the work-list
// stays free of silent duplicate refs across long / multi-session work.
type ErrAlreadyReferenced struct {
	EntryID string
	Kind    string
	RefID   string
}

func (e *ErrAlreadyReferenced) Error() string {
	return fmt.Sprintf("%s '%s' is already referenced in this notebook (entry %s)", e.Kind, e.RefID, e.EntryID)
}

// AddOutlineEntry appends or inserts an entry. kind "ref" requires refID to
// name an existing local message; kind "prose" requires non-empty prose.
// afterEntryID "" appends at the end; otherwise the entry lands directly after
// the named entry. Returns the new entry's ID.
func (s *Server) AddOutlineEntry(notebookID, kind, refID, prose, afterEntryID string) (string, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !validOutlineKinds[kind] {
		return "", fmt.Errorf("invalid kind '%s' (must be 'ref' or 'prose')", kind)
	}
	if _, err := s.getNotebookLocked(notebookID); err != nil {
		return "", err
	}
	switch kind {
	case "ref":
		refID = strings.TrimSpace(refID)
		var exists int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM messages WHERE id = ?`, refID).Scan(&exists); err != nil {
			return "", err
		}
		if exists == 0 {
			return "", fmt.Errorf("message '%s' not found — refs must point at an existing local message", refID)
		}
		prose = ""
	case "room_ref":
		refID = strings.TrimSpace(refID)
		var exists int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM rooms WHERE id = ?`, refID).Scan(&exists); err != nil {
			return "", err
		}
		if exists == 0 {
			return "", fmt.Errorf("room '%s' not found — room_refs must point at an existing local room", refID)
		}
		prose = ""
	case "query_ref":
		// A query_ref transcludes "the latest <type> in <room>", resolved live at
		// render time (structural addressing — no frozen message ID). ref_id is
		// "room_id:message_type".
		roomID, msgType, ok := parseQueryRef(refID)
		if !ok {
			return "", fmt.Errorf("query_ref ref_id must be 'room_id:message_type' (e.g. 'auth-room:synthesis'), got '%s'", refID)
		}
		var exists int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM rooms WHERE id = ?`, roomID).Scan(&exists); err != nil {
			return "", err
		}
		if exists == 0 {
			return "", fmt.Errorf("room '%s' not found — query_refs must point at an existing local room", roomID)
		}
		refID = roomID + ":" + msgType
		prose = ""
	case "prose":
		if strings.TrimSpace(prose) == "" {
			return "", fmt.Errorf("prose is required for a prose entry")
		}
		refID = ""
	case "task":
		// A task's label lives in prose; it starts open (done=0).
		if strings.TrimSpace(prose) == "" {
			return "", fmt.Errorf("prose (the task label) is required for a task entry")
		}
		refID = ""
	}

	// Dedup ref-like entries: a ref/room_ref/query_ref pointing at a target already
	// in this notebook is a no-op, not a second entry. Prose and tasks legitimately
	// repeat, so they're exempt. The refID is already canonical here (query_ref is
	// "room:type"). Keeps the self-sorting cockpit from drifting on repeat adds.
	if kind == "ref" || kind == "room_ref" || kind == "query_ref" {
		var existingID string
		switch err := s.DB.QueryRow(
			`SELECT id FROM notebook_entries WHERE notebook_id = ? AND kind = ? AND ref_id = ? LIMIT 1`,
			notebookID, kind, refID,
		).Scan(&existingID); err {
		case nil:
			return existingID, &ErrAlreadyReferenced{EntryID: existingID, Kind: kind, RefID: refID}
		case sql.ErrNoRows:
			// fall through to insert
		default:
			return "", err
		}
	}

	ids, err := s.outlineEntryIDsLocked(notebookID)
	if err != nil {
		return "", err
	}

	newID := uuid.Must(uuid.NewV7()).String()
	order, err := insertAfter(ids, newID, afterEntryID)
	if err != nil {
		return "", err
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`INSERT INTO notebook_entries (id, notebook_id, position, kind, ref_id, prose) VALUES (?, ?, 0, ?, ?, ?)`,
		newID, notebookID, kind, refID, prose,
	); err != nil {
		return "", err
	}
	if err := renumberEntries(tx, order); err != nil {
		return "", err
	}
	if err := touchNotebook(tx, notebookID); err != nil {
		return "", err
	}
	return newID, tx.Commit()
}

// UpdateOutlineEntry replaces the markdown of a prose entry. Ref entries are
// immutable by design — change the ledger message instead, the outline
// transcludes it live.
func (s *Server) UpdateOutlineEntry(entryID, prose string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if strings.TrimSpace(prose) == "" {
		return fmt.Errorf("prose is required")
	}

	var kind, notebookID string
	err := s.DB.QueryRow(`SELECT kind, notebook_id FROM notebook_entries WHERE id = ?`, entryID).Scan(&kind, &notebookID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("entry '%s' not found", entryID)
	}
	if err != nil {
		return err
	}
	if kind != "prose" && kind != "task" {
		return fmt.Errorf("entry '%s' is a ref — only prose and task entries can be edited (update the referenced message instead)", entryID)
	}

	if _, err := s.DB.Exec(`UPDATE notebook_entries SET prose = ? WHERE id = ?`, prose, entryID); err != nil {
		return err
	}
	_, err = s.DB.Exec(`UPDATE notebooks SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, notebookID)
	return err
}

// SetTaskStatus moves a task entry between open / doing / done. Only task entries
// own a status (a room_ref derives its state from its room, a prose/ref entry has
// none), so setting the status of anything else is an error.
func (s *Server) SetTaskStatus(entryID, status string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !validTaskStatuses[status] {
		return fmt.Errorf("invalid task status '%s' (must be open, doing, or done)", status)
	}

	var kind, notebookID string
	err := s.DB.QueryRow(`SELECT kind, notebook_id FROM notebook_entries WHERE id = ?`, entryID).Scan(&kind, &notebookID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("entry '%s' not found", entryID)
	}
	if err != nil {
		return err
	}
	if kind != "task" {
		return fmt.Errorf("entry '%s' is a %s, not a task — only task entries have a status", entryID, kind)
	}

	if _, err := s.DB.Exec(`UPDATE notebook_entries SET status = ? WHERE id = ?`, status, entryID); err != nil {
		return err
	}
	_, err = s.DB.Exec(`UPDATE notebooks SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, notebookID)
	return err
}

// RemoveOutlineEntry deletes one entry and renumbers the rest.
func (s *Server) RemoveOutlineEntry(entryID string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	notebookID, err := s.entryNotebookLocked(entryID)
	if err != nil {
		return err
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM notebook_entries WHERE id = ?`, entryID); err != nil {
		return err
	}

	ids, err := outlineEntryIDsTx(tx, notebookID)
	if err != nil {
		return err
	}
	if err := renumberEntries(tx, ids); err != nil {
		return err
	}
	if err := touchNotebook(tx, notebookID); err != nil {
		return err
	}
	return tx.Commit()
}

// MoveOutlineEntry repositions an entry: afterEntryID "" moves it to the top,
// otherwise it lands directly after the named entry.
func (s *Server) MoveOutlineEntry(entryID, afterEntryID string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if entryID == afterEntryID {
		return fmt.Errorf("cannot move an entry after itself")
	}

	notebookID, err := s.entryNotebookLocked(entryID)
	if err != nil {
		return err
	}

	ids, err := s.outlineEntryIDsLocked(notebookID)
	if err != nil {
		return err
	}

	// Remove the entry, then re-insert at the requested spot. Unlike add
	// (where "" means append), move treats "" as "to the top".
	found := false
	remaining := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != entryID {
			remaining = append(remaining, id)
		} else {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("entry '%s' not found", entryID)
	}

	var order []string
	if afterEntryID == "" {
		order = append([]string{entryID}, remaining...)
	} else {
		order, err = insertAfter(remaining, entryID, afterEntryID)
		if err != nil {
			return err
		}
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := renumberEntries(tx, order); err != nil {
		return err
	}
	if err := touchNotebook(tx, notebookID); err != nil {
		return err
	}
	return tx.Commit()
}

// GetOutline returns a notebook and its entries in order, with ref entries
// transcluded: the referenced message's room, author, type, content,
// timestamp, and the owning room's repo (for {sha:...} resolution) are
// resolved live. A ref whose message has been deleted comes back with
// RefFound=false rather than failing the read.
func (s *Server) GetOutline(notebookID string) (Notebook, []OutlineEntry, error) {
	n, err := s.GetNotebook(notebookID)
	if err != nil {
		return n, nil, err
	}

	// Message refs resolve via m/r; room refs resolve via rr, with the room's
	// latest decision/action pulled in as the entry's content — so a work-list
	// entry always shows where that thread of work currently stands.
	rows, err := s.DB.Query(`
		SELECT e.id, e.position, e.kind, e.ref_id, e.prose, COALESCE(e.status, 'open'),
		       (m.id IS NOT NULL OR rr.id IS NOT NULL),
		       COALESCE(m.room_id, rr.id, ''),
		       COALESCE(m.author, lm.author, ''),
		       COALESCE(m.message_type, lm.message_type, ''),
		       COALESCE(m.content, lm.content, ''),
		       COALESCE(m.pinned, 0),
		       COALESCE(m.timestamp, lm.timestamp),
		       COALESCE(r.repo, rr.repo, ''),
		       COALESCE(rr.status, ''), COALESCE(rr.description, ''),
		       m.retracted_at, COALESCE(m.retracted_by, '')
		FROM notebook_entries e
		LEFT JOIN messages m ON e.kind = 'ref' AND m.id = e.ref_id
		LEFT JOIN rooms r ON r.id = m.room_id
		LEFT JOIN rooms rr ON e.kind = 'room_ref' AND rr.id = e.ref_id
		LEFT JOIN messages lm ON lm.id = (
			SELECT id FROM messages
			WHERE room_id = rr.id AND message_type IN ('decision', 'action') AND `+liveClause("")+`
			ORDER BY id DESC LIMIT 1
		)
		WHERE e.notebook_id = ?
		ORDER BY e.position ASC`, notebookID)
	if err != nil {
		return n, nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []OutlineEntry
	for rows.Next() {
		var e OutlineEntry
		// COALESCE strips the column's datetime affinity, so the driver hands
		// back the raw stored string — parse it instead of scanning NullTime.
		var ts sql.NullString
		var retractedAt sql.NullTime
		var retractedBy string
		if err := rows.Scan(&e.ID, &e.Position, &e.Kind, &e.RefID, &e.Prose, &e.Status,
			&e.RefFound, &e.RefRoomID, &e.RefAuthor, &e.RefType,
			&e.RefContent, &e.RefPinned, &ts, &e.RefRepo,
			&e.RefStatus, &e.RefTopic, &retractedAt, &retractedBy); err != nil {
			return n, nil, err
		}
		if ts.Valid {
			e.RefTime = parseStoredTime(ts.String)
		}
		if e.Kind == "ref" {
			e.RefContent = DisplayContent(Message{Content: e.RefContent, RetractedAt: retractedAt, RetractedBy: retractedBy})
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return n, nil, err
	}

	// Resolve query_refs in Go (a follow-up query each) — "latest <type> in <room>",
	// live at render time. Cleaner than cramming the "room:type" split into the main
	// SQL. A query that currently matches nothing resolves as RefFound=false.
	for i := range entries {
		switch entries[i].Kind {
		case "query_ref":
			s.resolveQueryRef(&entries[i])
		case "ref":
			s.resolveRefHead(&entries[i])
		}
	}
	return n, entries, nil
}

// resolveRefHead re-resolves a "ref" entry to the head of its revision chain.
// The main query joins on the entry's stored ref_id directly, which is a fixed
// address; if that message has since been edited (revises appends a new node
// and flags the old one revised), the join keeps transcluding the stale prior
// version forever. Walk forward to the current head and re-fetch its content.
func (s *Server) resolveRefHead(e *OutlineEntry) {
	if !e.RefFound {
		return
	}
	headID := s.headOfRevisionChain(e.RefID)
	if headID == e.RefID {
		return // not revised, nothing to do
	}
	m, err := scanMessage(s.DB.QueryRow(fmt.Sprintf(`SELECT %s FROM messages WHERE id = ?`, messageColumns), headID))
	if err != nil {
		return
	}
	e.RefAuthor = m.Author
	e.RefType = m.MessageType
	e.RefContent = DisplayContent(m)
	e.RefPinned = m.Pinned
	e.RefTime = m.Timestamp
}

// parseQueryRef splits a query_ref ref_id "room_id:message_type" into its parts.
// The room_id itself never contains a colon (slugified), so the LAST colon
// separates room from type.
func parseQueryRef(ref string) (roomID, msgType string, ok bool) {
	ref = strings.TrimSpace(ref)
	i := strings.LastIndex(ref, ":")
	if i <= 0 || i == len(ref)-1 {
		return "", "", false
	}
	return strings.TrimSpace(ref[:i]), strings.TrimSpace(ref[i+1:]), true
}

// resolveQueryRef fills a query_ref entry with the latest live (head, non-retracted)
// message of its type in its room, evaluated now.
func (s *Server) resolveQueryRef(e *OutlineEntry) {
	roomID, msgType, ok := parseQueryRef(e.RefID)
	if !ok {
		return
	}
	m, err := scanMessage(s.DB.QueryRow(fmt.Sprintf(`
		SELECT %s FROM messages
		WHERE room_id = ? AND message_type = ? AND is_summary = 0 AND `+liveClause("")+`
		ORDER BY id DESC LIMIT 1`, messageColumns), roomID, msgType))
	if err != nil {
		return // no match yet → RefFound stays false
	}
	var repo string
	_ = s.DB.QueryRow(`SELECT COALESCE(repo, '') FROM rooms WHERE id = ?`, roomID).Scan(&repo)
	e.RefFound = true
	e.RefRoomID = m.RoomID
	e.RefAuthor = m.Author
	e.RefType = m.MessageType
	e.RefContent = m.Content
	e.RefPinned = m.Pinned
	e.RefTime = m.Timestamp
	e.RefRepo = repo
}

// parseStoredTime parses a raw SQLite timestamp string in the formats the
// driver itself accepts for DATETIME columns.
func parseStoredTime(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05.999999999-07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// --- internal helpers ---

func (s *Server) getNotebookLocked(id string) (string, error) {
	var got string
	err := s.DB.QueryRow(`SELECT id FROM notebooks WHERE id = ?`, id).Scan(&got)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("notebook '%s' not found", id)
	}
	return got, err
}

func (s *Server) entryNotebookLocked(entryID string) (string, error) {
	var notebookID string
	err := s.DB.QueryRow(`SELECT notebook_id FROM notebook_entries WHERE id = ?`, entryID).Scan(&notebookID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("entry '%s' not found", entryID)
	}
	return notebookID, err
}

func (s *Server) outlineEntryIDsLocked(notebookID string) ([]string, error) {
	rows, err := s.DB.Query(`SELECT id FROM notebook_entries WHERE notebook_id = ? ORDER BY position ASC`, notebookID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanIDs(rows)
}

func outlineEntryIDsTx(tx *sql.Tx, notebookID string) ([]string, error) {
	rows, err := tx.Query(`SELECT id FROM notebook_entries WHERE notebook_id = ? ORDER BY position ASC`, notebookID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanIDs(rows)
}

func scanIDs(rows *sql.Rows) ([]string, error) {
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// insertAfter returns ids with newID inserted directly after afterID, or
// appended when afterID is "" and the list came from an add (callers pass the
// existing order). For moves, afterID "" means the top of the list.
func insertAfter(ids []string, newID, afterID string) ([]string, error) {
	if afterID == "" {
		return append(ids, newID), nil
	}
	out := make([]string, 0, len(ids)+1)
	found := false
	for _, id := range ids {
		out = append(out, id)
		if id == afterID {
			out = append(out, newID)
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("after_entry_id '%s' not found in this notebook", afterID)
	}
	return out, nil
}

// renumberEntries writes positions 1..n following the given order. Outlines
// are small (tens of entries), so a full renumber per mutation is simpler and
// safer than sparse positions.
func renumberEntries(tx *sql.Tx, orderedIDs []string) error {
	for i, id := range orderedIDs {
		if _, err := tx.Exec(`UPDATE notebook_entries SET position = ? WHERE id = ?`, i+1, id); err != nil {
			return err
		}
	}
	return nil
}

func touchNotebook(tx *sql.Tx, notebookID string) error {
	_, err := tx.Exec(`UPDATE notebooks SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, notebookID)
	return err
}
