package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Room represents a council room.
type Room struct {
	ID           string
	Description  string
	Status       string
	Project      string
	TechStack    string
	Tags         string
	SystemPrompt string
	RelatedRooms string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Message represents a message in a council room.
type Message struct {
	ID          int64
	RoomID      string
	Author      string
	Content     string
	MessageType string
	IsSummary   bool
	ReplyTo     int64
	Pinned      bool
	Timestamp   time.Time
}

// messageColumns is the canonical column list for SELECT queries on messages.
const messageColumns = `id, room_id, author, content, message_type, is_summary, reply_to, pinned, timestamp`

// scanMessage scans a single row into a Message struct. Use with messageColumns.
func scanMessage(scanner interface{ Scan(...any) error }) (Message, error) {
	var m Message
	err := scanner.Scan(&m.ID, &m.RoomID, &m.Author, &m.Content, &m.MessageType, &m.IsSummary, &m.ReplyTo, &m.Pinned, &m.Timestamp)
	return m, err
}

// CouncilServer holds the database, mutex, MCP server, and logger.
type CouncilServer struct {
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
	mcp    *mcp.Server
	logger *slog.Logger
}

// NewCouncilServer creates a new CouncilServer with an initialized SQLite database.
func NewCouncilServer(dbPath string, logger *slog.Logger) (*CouncilServer, error) {
	dsn := dbPath + "?_journal=WAL&_busy_timeout=5000"
	if dbPath == ":memory:" {
		dsn = ":memory:"
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite handles concurrency via WAL mode, but we still configure
	// the pool to avoid "database is locked" under heavy concurrent reads.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "council-hub",
		Version: "0.3.2",
	}, &mcp.ServerOptions{
		Logger:       logger,
		Capabilities: &mcp.ServerCapabilities{},
	})

	return &CouncilServer{
		db:     db,
		dbPath: dbPath,
		mcp:    mcpServer,
		logger: logger,
	}, nil
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS rooms (
		id TEXT PRIMARY KEY,
		description TEXT,
		status TEXT DEFAULT 'active',
		project TEXT DEFAULT '',
		tech_stack TEXT DEFAULT '',
		tags TEXT DEFAULT '',
		system_prompt TEXT DEFAULT '',
		related_rooms TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		room_id TEXT,
		author TEXT,
		content TEXT,
		message_type TEXT DEFAULT 'message',
		is_summary BOOLEAN DEFAULT 0,
		reply_to INTEGER DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(room_id) REFERENCES rooms(id)
	);`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Migrate existing databases: add new columns if they don't exist.
	// These use constant defaults so ALTER TABLE works with SQLite.
	migrations := []string{
		`ALTER TABLE rooms ADD COLUMN related_rooms TEXT DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN reply_to INTEGER DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN pinned BOOLEAN DEFAULT 0`,
	}
	for _, m := range migrations {
		db.Exec(m) // Ignore "duplicate column" errors for already-migrated DBs
	}

	return nil
}

func (cs *CouncilServer) createRoom(id, description, project, techStack, tags, systemPrompt, relatedRooms string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	_, err := cs.db.Exec(
		`INSERT OR IGNORE INTO rooms (id, description, project, tech_stack, tags, system_prompt, related_rooms) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, description, project, techStack, tags, systemPrompt, relatedRooms,
	)
	return err
}

func (cs *CouncilServer) postMessage(roomID, author, content, messageType string, replyTo int64) (int64, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if messageType == "" {
		messageType = "message"
	}

	result, err := cs.db.Exec(
		`INSERT INTO messages (room_id, author, content, message_type, reply_to) VALUES (?, ?, ?, ?, ?)`,
		roomID, author, content, messageType, replyTo,
	)
	if err != nil {
		return 0, err
	}

	// Update room's updated_at — best-effort, don't fail the post on this
	_, _ = cs.db.Exec(`UPDATE rooms SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, roomID)

	id, err := result.LastInsertId()
	return id, err
}

func (cs *CouncilServer) updateMessage(messageID int64, newContent, newMessageType string) (*Message, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if newMessageType != "" {
		_, err := cs.db.Exec(
			`UPDATE messages SET content = ?, message_type = ? WHERE id = ?`,
			newContent, newMessageType, messageID,
		)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := cs.db.Exec(
			`UPDATE messages SET content = ? WHERE id = ?`,
			newContent, messageID,
		)
		if err != nil {
			return nil, err
		}
	}

	m, err := scanMessage(cs.db.QueryRow(
		fmt.Sprintf(`SELECT %s FROM messages WHERE id = ?`, messageColumns),
		messageID,
	))
	if err != nil {
		return nil, err
	}

	// Update room's updated_at
	_, _ = cs.db.Exec(`UPDATE rooms SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, m.RoomID)

	return &m, nil
}

func (cs *CouncilServer) pinMessage(roomID string, messageID int64) (bool, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Verify message exists and belongs to the room
	var currentlyPinned bool
	var actualRoomID string
	err := cs.db.QueryRow(`SELECT room_id, pinned FROM messages WHERE id = ?`, messageID).Scan(&actualRoomID, &currentlyPinned)
	if err != nil {
		return false, err
	}
	if actualRoomID != roomID {
		return false, fmt.Errorf("message #%d belongs to room '%s', not '%s'", messageID, actualRoomID, roomID)
	}

	if currentlyPinned {
		// Toggle off
		_, err := cs.db.Exec(`UPDATE messages SET pinned = 0 WHERE id = ?`, messageID)
		return false, err
	}

	// Unpin any existing pinned message in this room
	_, _ = cs.db.Exec(`UPDATE messages SET pinned = 0 WHERE room_id = ? AND pinned = 1`, roomID)

	// Pin the target
	_, err = cs.db.Exec(`UPDATE messages SET pinned = 1 WHERE id = ?`, messageID)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (cs *CouncilServer) getPinnedMessage(roomID string) (*Message, error) {
	m, err := scanMessage(cs.db.QueryRow(
		fmt.Sprintf(`SELECT %s FROM messages WHERE room_id = ? AND pinned = 1 LIMIT 1`, messageColumns),
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

func (cs *CouncilServer) updateStatus(roomID, status string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	res, err := cs.db.Exec(
		`UPDATE rooms SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, roomID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("room '%s' not found", roomID)
	}
	return nil
}

func (cs *CouncilServer) updateRoom(roomID, description, project, techStack, tags, systemPrompt, relatedRooms string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Build dynamic UPDATE — only set fields that are non-empty.
	setClauses := []string{"updated_at = CURRENT_TIMESTAMP"}
	var args []any

	if description != "" {
		setClauses = append(setClauses, "description = ?")
		args = append(args, description)
	}
	if project != "" {
		setClauses = append(setClauses, "project = ?")
		args = append(args, project)
	}
	if techStack != "" {
		setClauses = append(setClauses, "tech_stack = ?")
		args = append(args, techStack)
	}
	if tags != "" {
		setClauses = append(setClauses, "tags = ?")
		args = append(args, tags)
	}
	if systemPrompt != "" {
		setClauses = append(setClauses, "system_prompt = ?")
		args = append(args, systemPrompt)
	}
	if relatedRooms != "" {
		setClauses = append(setClauses, "related_rooms = ?")
		args = append(args, relatedRooms)
	}

	query := fmt.Sprintf("UPDATE rooms SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	args = append(args, roomID)

	res, err := cs.db.Exec(query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("room '%s' not found", roomID)
	}
	return nil
}

func (cs *CouncilServer) getMessagesByIDs(ids []int64) ([]Message, error) {
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
	rows, err := cs.db.Query(query, args...)
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

func (cs *CouncilServer) getRecentMessages(roomID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// Verify room exists
	_, err := cs.getRoom(roomID)
	if err != nil {
		return nil, fmt.Errorf("room '%s' not found", roomID)
	}

	// Get last N messages in reverse, then flip to chronological
	rows, err := cs.db.Query(fmt.Sprintf(`SELECT %s FROM messages WHERE room_id = ? ORDER BY id DESC LIMIT ?`, messageColumns), roomID, limit)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

func (cs *CouncilServer) deleteRoom(roomID string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	res, err := cs.db.Exec(`DELETE FROM rooms WHERE id = ?`, roomID)
	if err != nil {
		return fmt.Errorf("delete room '%s': %w", roomID, err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("room '%s' not found", roomID)
	}

	if _, err := cs.db.Exec(`DELETE FROM messages WHERE room_id = ?`, roomID); err != nil {
		return fmt.Errorf("delete messages for room '%s': %w", roomID, err)
	}
	return nil
}

// RoomStats holds aggregate statistics for a room.
type RoomStats struct {
	RoomID          string
	Status          string
	MessageCount    int
	LatestMessageID int64
	Participants    map[string]int // author -> message count
	TypeCounts      map[string]int // message_type -> count
	FirstMessage    time.Time
	LastMessage     time.Time
}

func (cs *CouncilServer) searchMessages(query, author, messageType, roomID, project string, limit int) ([]Message, error) {
	where := `WHERE 1=1`
	var args []any
	join := ""

	if query != "" {
		where += ` AND m.content LIKE '%' || ? || '%'`
		args = append(args, query)
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
		where += ` AND m.room_id = ?`
		args = append(args, roomID)
	}
	if project != "" {
		join = ` JOIN rooms r ON m.room_id = r.id`
		where += ` AND r.project = ?`
		args = append(args, project)
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	q := fmt.Sprintf(`SELECT m.id, m.room_id, m.author, m.content, m.message_type, m.is_summary, m.reply_to, m.pinned, m.timestamp FROM messages m%s %s ORDER BY m.timestamp DESC LIMIT ?`, join, where)
	args = append(args, limit)

	rows, err := cs.db.Query(q, args...)
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

func (cs *CouncilServer) getRoomStats(roomID string) (RoomStats, error) {
	var stats RoomStats
	stats.RoomID = roomID
	stats.Participants = make(map[string]int)
	stats.TypeCounts = make(map[string]int)

	// Verify room exists and get status
	var status string
	err := cs.db.QueryRow(`SELECT status FROM rooms WHERE id = ?`, roomID).Scan(&status)
	if err != nil {
		return stats, fmt.Errorf("room '%s' not found", roomID)
	}
	stats.Status = status

	// Get aggregate stats + latest message ID
	var firstMsg, lastMsg sql.NullString
	var latestID sql.NullInt64
	err = cs.db.QueryRow(`SELECT COUNT(*), MIN(timestamp), MAX(timestamp), MAX(id) FROM messages WHERE room_id = ?`, roomID).
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
		stats.LatestMessageID = latestID.Int64
	}

	// Get per-author counts
	rows, err := cs.db.Query(`SELECT author, COUNT(*) FROM messages WHERE room_id = ? GROUP BY author ORDER BY COUNT(*) DESC`, roomID)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var author string
		var count int
		if err := rows.Scan(&author, &count); err != nil {
			return stats, err
		}
		stats.Participants[author] = count
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	// Get per-type counts
	typeRows, err := cs.db.Query(`SELECT message_type, COUNT(*) FROM messages WHERE room_id = ? AND is_summary = 0 GROUP BY message_type ORDER BY COUNT(*) DESC`, roomID)
	if err != nil {
		return stats, err
	}
	defer typeRows.Close()

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

func (cs *CouncilServer) deleteMessages(ids []int64) (int64, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if len(ids) == 0 {
		return 0, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`DELETE FROM messages WHERE id IN (%s)`, strings.Join(placeholders, ","))
	res, err := cs.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func (cs *CouncilServer) archiveRoom(roomID string) (string, error) {
	room, err := cs.getRoom(roomID)
	if err != nil {
		return "", fmt.Errorf("room '%s' not found", roomID)
	}

	messages, err := cs.getTranscript(roomID)
	if err != nil {
		return "", fmt.Errorf("failed to read transcript: %w", err)
	}

	transcript := formatTranscript(room, messages)

	// Derive archive dir from DB path
	archiveDir := filepath.Join(filepath.Dir(cs.dbPath), "archives")
	if cs.dbPath == ":memory:" {
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

func (cs *CouncilServer) getRoom(roomID string) (Room, error) {
	var r Room
	err := cs.db.QueryRow(
		`SELECT id, description, status, project, tech_stack, tags, system_prompt, related_rooms, created_at, updated_at FROM rooms WHERE id = ?`,
		roomID,
	).Scan(&r.ID, &r.Description, &r.Status, &r.Project, &r.TechStack, &r.Tags, &r.SystemPrompt, &r.RelatedRooms, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return r, err
	}
	return r, nil
}

// getTranscript returns summaries + all individual messages after the latest summary.
func (cs *CouncilServer) getTranscript(roomID string) ([]Message, error) {
	rows, err := cs.db.Query(fmt.Sprintf(`
		SELECT %s
		FROM messages
		WHERE room_id = ?
		  AND (is_summary = 1 OR id > COALESCE(
		      (SELECT MAX(id) FROM messages WHERE room_id = ? AND is_summary = 1), 0
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

// getMessagesAfterID returns messages with ID > afterID for a room, in chronological order.
func (cs *CouncilServer) getMessagesAfterID(roomID string, afterID int64) ([]Message, error) {
	rows, err := cs.db.Query(fmt.Sprintf(`
		SELECT %s
		FROM messages
		WHERE room_id = ? AND id > ?
		ORDER BY timestamp ASC`, messageColumns),
		roomID, afterID,
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

// getLatestPerType returns the most recent message for each message_type in a room.
func (cs *CouncilServer) getLatestPerType(roomID string) ([]Message, error) {
	rows, err := cs.db.Query(`
		SELECT m.id, m.room_id, m.author, m.content, m.message_type, m.is_summary, m.reply_to, m.pinned, m.timestamp
		FROM messages m
		INNER JOIN (
			SELECT message_type, MAX(id) as max_id
			FROM messages
			WHERE room_id = ? AND is_summary = 0
			GROUP BY message_type
		) latest ON m.id = latest.max_id
		ORDER BY m.timestamp DESC`,
		roomID,
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

// getRoomsNeedingSummary returns room IDs with more than threshold unsummarized messages.
func (cs *CouncilServer) getRoomsNeedingSummary(threshold int) ([]string, error) {
	rows, err := cs.db.Query(`
		SELECT room_id
		FROM messages
		WHERE is_summary = 0
		  AND id > COALESCE(
		      (SELECT MAX(m2.id) FROM messages m2 WHERE m2.room_id = messages.room_id AND m2.is_summary = 1), 0
		  )
		GROUP BY room_id
		HAVING COUNT(*) > ?`,
		threshold,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

// getUnsummarizedMessages returns messages after the latest summary for a room.
func (cs *CouncilServer) getUnsummarizedMessages(roomID string) ([]Message, error) {
	rows, err := cs.db.Query(fmt.Sprintf(`
		SELECT %s
		FROM messages
		WHERE room_id = ?
		  AND is_summary = 0
		  AND id > COALESCE(
		      (SELECT MAX(id) FROM messages WHERE room_id = ? AND is_summary = 1), 0
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

// insertSummary inserts a summary message into a room.
func (cs *CouncilServer) insertSummary(roomID, summary string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	_, err := cs.db.Exec(
		`INSERT INTO messages (room_id, author, content, message_type, is_summary) VALUES (?, ?, ?, 'message', 1)`,
		roomID, "System", summary,
	)
	if err != nil {
		return err
	}

	_, _ = cs.db.Exec(`UPDATE rooms SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, roomID)
	return nil
}

// listRooms returns rooms matching optional filters.
func (cs *CouncilServer) listRooms(project, tag, status, search string) ([]Room, error) {
	query := `SELECT id, description, status, project, tech_stack, tags, system_prompt, related_rooms, created_at, updated_at FROM rooms WHERE 1=1`
	var args []any

	if project != "" {
		query += ` AND project = ?`
		args = append(args, project)
	}
	if tag != "" {
		query += ` AND (',' || tags || ',') LIKE '%,' || ? || ',%'`
		args = append(args, tag)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if search != "" {
		query += ` AND (id LIKE '%' || ? || '%' OR description LIKE '%' || ? || '%' OR tags LIKE '%' || ? || '%')`
		args = append(args, search, search, search)
	}

	query += ` ORDER BY updated_at DESC`

	rows, err := cs.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.Description, &r.Status, &r.Project, &r.TechStack, &r.Tags, &r.SystemPrompt, &r.RelatedRooms, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

// getMessageCounts returns a map of room_id -> message count for all rooms.
func (cs *CouncilServer) getMessageCounts() map[string]int {
	counts := make(map[string]int)
	rows, err := cs.db.Query(`SELECT room_id, COUNT(*) FROM messages GROUP BY room_id`)
	if err != nil {
		return counts
	}
	defer rows.Close()

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
