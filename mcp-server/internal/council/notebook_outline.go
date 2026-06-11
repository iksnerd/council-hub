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
// by ID — resolved live at render time, never copied, so the outline can't
// drift from the ledger. Ref* fields are populated by GetOutline; RefFound is
// false when the referenced message no longer exists locally.
type OutlineEntry struct {
	ID       string
	Position int
	Kind     string // "ref" | "prose"
	RefID    string
	Prose    string

	RefRoomID  string
	RefAuthor  string
	RefType    string
	RefContent string
	RefPinned  bool
	RefTime    time.Time
	RefRepo    string
	RefFound   bool
}

// validOutlineKinds gates the entry kinds accepted by AddOutlineEntry.
var validOutlineKinds = map[string]bool{"ref": true, "prose": true}

// CreateNotebook creates an empty notebook outline. The ID is the stable
// address used by edit_notebook and read_notebook(notebook_id=...).
func (s *Server) CreateNotebook(id, project, title string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("notebook id is required")
	}
	project = normalizeProject(project)
	if project == "" {
		return fmt.Errorf("project is required")
	}

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

// ListNotebooks returns notebooks, optionally filtered by project, most
// recently updated first.
func (s *Server) ListNotebooks(project string) ([]Notebook, error) {
	query := `
		SELECT id, project, title, created_at, updated_at,
		       (SELECT COUNT(*) FROM notebook_entries WHERE notebook_id = notebooks.id)
		FROM notebooks`
	var args []any
	if project != "" {
		query += ` WHERE project = ?`
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
	case "prose":
		if strings.TrimSpace(prose) == "" {
			return "", fmt.Errorf("prose is required for a prose entry")
		}
		refID = ""
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
	if kind != "prose" {
		return fmt.Errorf("entry '%s' is a ref — only prose entries can be edited (update the referenced message instead)", entryID)
	}

	if _, err := s.DB.Exec(`UPDATE notebook_entries SET prose = ? WHERE id = ?`, prose, entryID); err != nil {
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

	rows, err := s.DB.Query(`
		SELECT e.id, e.position, e.kind, e.ref_id, e.prose,
		       m.id IS NOT NULL,
		       COALESCE(m.room_id, ''), COALESCE(m.author, ''), COALESCE(m.message_type, ''),
		       COALESCE(m.content, ''), COALESCE(m.pinned, 0), m.timestamp, COALESCE(r.repo, '')
		FROM notebook_entries e
		LEFT JOIN messages m ON e.kind = 'ref' AND m.id = e.ref_id
		LEFT JOIN rooms r ON r.id = m.room_id
		WHERE e.notebook_id = ?
		ORDER BY e.position ASC`, notebookID)
	if err != nil {
		return n, nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []OutlineEntry
	for rows.Next() {
		var e OutlineEntry
		var ts sql.NullTime
		if err := rows.Scan(&e.ID, &e.Position, &e.Kind, &e.RefID, &e.Prose,
			&e.RefFound, &e.RefRoomID, &e.RefAuthor, &e.RefType,
			&e.RefContent, &e.RefPinned, &ts, &e.RefRepo); err != nil {
			return n, nil, err
		}
		if ts.Valid {
			e.RefTime = ts.Time
		}
		entries = append(entries, e)
	}
	return n, entries, rows.Err()
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
