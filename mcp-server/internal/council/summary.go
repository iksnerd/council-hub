package council

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ArchiveEntry holds metadata about a single archived room transcript.
type ArchiveEntry struct {
	RoomID     string
	Path       string
	Size       int64
	ArchivedAt time.Time
}

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

	epitaph := buildEpitaph(room, messages)
	transcript := epitaph + FormatTranscript(room, messages)

	archiveDir := s.archiveDir()
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}

	archivePath := filepath.Join(archiveDir, roomID+".md")
	if err := os.WriteFile(archivePath, []byte(transcript), 0644); err != nil {
		return "", fmt.Errorf("failed to write archive: %w", err)
	}

	return archivePath, nil
}

// buildEpitaph generates a brief summary block from the last decision and action messages.
func buildEpitaph(room Room, messages []Message) string {
	var lastDecision, lastAction *Message
	for i := range messages {
		m := &messages[i]
		switch m.MessageType {
		case "decision":
			lastDecision = m
		case "action":
			lastAction = m
		}
	}

	if lastDecision == nil && lastAction == nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Summary\n\n")
	if lastDecision != nil {
		excerpt := lastDecision.Content
		if len(excerpt) > 300 {
			excerpt = excerpt[:300]
			if i := strings.LastIndex(excerpt, "\n"); i > 200 {
				excerpt = excerpt[:i]
			}
			excerpt += "..."
		}
		fmt.Fprintf(&b, "**Last decision** (%s by %s):\n%s\n\n", lastDecision.Timestamp.Format("2006-01-02"), lastDecision.Author, excerpt)
	}
	if lastAction != nil {
		excerpt := lastAction.Content
		if len(excerpt) > 300 {
			excerpt = excerpt[:300]
			if i := strings.LastIndex(excerpt, "\n"); i > 200 {
				excerpt = excerpt[:i]
			}
			excerpt += "..."
		}
		fmt.Fprintf(&b, "**Last action** (%s by %s):\n%s\n\n", lastAction.Timestamp.Format("2006-01-02"), lastAction.Author, excerpt)
	}
	b.WriteString("---\n\n")
	return b.String()
}

// archiveDir returns the directory where archives are stored.
func (s *Server) archiveDir() string {
	if s.DBPath == ":memory:" {
		return "archives"
	}
	return filepath.Join(filepath.Dir(s.DBPath), "archives")
}

// ListArchives scans the archives directory and returns metadata for each archived room,
// sorted by archive date descending (most recent first). Returns an empty slice if no
// archives exist.
func (s *Server) ListArchives() ([]ArchiveEntry, error) {
	dir := s.archiveDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ArchiveEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read archives directory: %w", err)
	}

	var archives []ArchiveEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		archives = append(archives, ArchiveEntry{
			RoomID:     strings.TrimSuffix(e.Name(), ".md"),
			Path:       path,
			Size:       info.Size(),
			ArchivedAt: info.ModTime(),
		})
	}

	sort.Slice(archives, func(i, j int) bool {
		return archives[i].ArchivedAt.After(archives[j].ArchivedAt)
	})

	return archives, nil
}

// ReadArchive reads and returns the contents of an archived room transcript.
func (s *Server) ReadArchive(roomID string) (string, error) {
	path := filepath.Join(s.archiveDir(), roomID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("archive for room '%s' not found", roomID)
		}
		return "", fmt.Errorf("failed to read archive: %w", err)
	}
	return string(data), nil
}
