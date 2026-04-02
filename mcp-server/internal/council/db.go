package council

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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
	ID          string
	RoomID      string
	Author      string
	Content     string
	MessageType string
	IsSummary   bool
	ReplyTo     string // UUID of parent message, or ""
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

// Server holds the database, mutex, MCP server, and logger.
type Server struct {
	DB     *sql.DB
	DBPath string
	Mu     sync.RWMutex
	MCP    *mcp.Server
	Logger *slog.Logger
}

// NewServer creates a new Server with an initialized SQLite database.
func NewServer(dbPath string, logger *slog.Logger) (*Server, error) {
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

	if err := migrateMessagesToUUIDs(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate message IDs to UUID: %w", err)
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "council-hub",
		Version: "0.6.1",
	}, &mcp.ServerOptions{
		Logger:       logger,
		Capabilities: &mcp.ServerCapabilities{},
	})

	return &Server{
		DB:     db,
		DBPath: dbPath,
		MCP:    mcpServer,
		Logger: logger,
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
		id TEXT PRIMARY KEY,
		room_id TEXT,
		author TEXT,
		content TEXT,
		message_type TEXT DEFAULT 'message',
		is_summary BOOLEAN DEFAULT 0,
		reply_to TEXT DEFAULT '',
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
		`ALTER TABLE messages ADD COLUMN reply_to TEXT DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN pinned BOOLEAN DEFAULT 0`,
	}
	for _, m := range migrations {
		db.Exec(m) // Ignore "duplicate column" errors for already-migrated DBs
	}

	// Indexes — CREATE IF NOT EXISTS is idempotent, safe to run every startup.
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_messages_room_id ON messages(room_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_room_id_id ON messages(room_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_room_id_timestamp ON messages(room_id, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_room_id_is_summary ON messages(room_id, is_summary)`,
		`CREATE INDEX IF NOT EXISTS idx_rooms_project ON rooms(project)`,
		`CREATE INDEX IF NOT EXISTS idx_rooms_status ON rooms(status)`,
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}

	return nil
}

// migrateMessagesToUUIDs detects an old integer-ID messages schema and converts it to UUID v7.
// It is idempotent: if messages.id is already TEXT, it returns immediately.
// All data is preserved; reply_to cross-references are translated using the old→new ID map.
func migrateMessagesToUUIDs(db *sql.DB) error {
	// Check if migration is needed: look for INTEGER type on messages.id
	var colType string
	err := db.QueryRow(`SELECT type FROM pragma_table_info('messages') WHERE name='id'`).Scan(&colType)
	if err != nil || strings.ToUpper(colType) != "INTEGER" {
		return nil // already migrated, or table doesn't exist yet
	}

	// Load all existing messages ordered by old integer ID (= insertion order)
	type oldRow struct {
		OldID       int64
		RoomID      string
		Author      string
		Content     string
		MessageType string
		IsSummary   bool
		OldReplyTo  int64
		Pinned      bool
		Timestamp   string
	}

	rows, err := db.Query(`SELECT id, room_id, author, content, message_type, is_summary, reply_to, pinned, timestamp FROM messages ORDER BY id ASC`)
	if err != nil {
		return fmt.Errorf("read old messages: %w", err)
	}

	var oldRows []oldRow
	for rows.Next() {
		var r oldRow
		if err := rows.Scan(&r.OldID, &r.RoomID, &r.Author, &r.Content, &r.MessageType, &r.IsSummary, &r.OldReplyTo, &r.Pinned, &r.Timestamp); err != nil {
			rows.Close()
			return fmt.Errorf("scan old message: %w", err)
		}
		oldRows = append(oldRows, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate old messages: %w", err)
	}

	// Build old int64 → new UUID v7 mapping (processed in order, so UUIDs are chronological)
	idMap := make(map[int64]string, len(oldRows))
	for _, r := range oldRows {
		idMap[r.OldID] = uuid.Must(uuid.NewV7()).String()
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}

	if _, err := tx.Exec(`ALTER TABLE messages RENAME TO messages_old`); err != nil {
		tx.Rollback()
		return fmt.Errorf("rename old table: %w", err)
	}

	if _, err := tx.Exec(`CREATE TABLE messages (
		id TEXT PRIMARY KEY,
		room_id TEXT,
		author TEXT,
		content TEXT,
		message_type TEXT DEFAULT 'message',
		is_summary BOOLEAN DEFAULT 0,
		reply_to TEXT DEFAULT '',
		pinned BOOLEAN DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(room_id) REFERENCES rooms(id)
	)`); err != nil {
		tx.Rollback()
		return fmt.Errorf("create new messages table: %w", err)
	}

	for _, r := range oldRows {
		newReplyTo := ""
		if r.OldReplyTo > 0 {
			if ref, ok := idMap[r.OldReplyTo]; ok {
				newReplyTo = ref
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO messages (id, room_id, author, content, message_type, is_summary, reply_to, pinned, timestamp) VALUES (?,?,?,?,?,?,?,?,?)`,
			idMap[r.OldID], r.RoomID, r.Author, r.Content, r.MessageType, r.IsSummary, newReplyTo, r.Pinned, r.Timestamp,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("insert migrated message: %w", err)
		}
	}

	if _, err := tx.Exec(`DROP TABLE messages_old`); err != nil {
		tx.Rollback()
		return fmt.Errorf("drop old messages table: %w", err)
	}

	return tx.Commit()
}
