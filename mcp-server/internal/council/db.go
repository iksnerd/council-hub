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
	Reactions   string // JSON: {"emoji": ["author1", "author2"], ...}
	Mentions    string // CSV of mentioned agent names, e.g. "claude,gemini-cli"
	Timestamp   time.Time
}

// messageColumns is the canonical column list for SELECT queries on messages.
const messageColumns = `id, room_id, author, content, message_type, is_summary, reply_to, pinned, reactions, mentions, timestamp`

// scanMessage scans a single row into a Message struct. Use with messageColumns.
func scanMessage(scanner interface{ Scan(...any) error }) (Message, error) {
	var m Message
	err := scanner.Scan(&m.ID, &m.RoomID, &m.Author, &m.Content, &m.MessageType, &m.IsSummary, &m.ReplyTo, &m.Pinned, &m.Reactions, &m.Mentions, &m.Timestamp)
	return m, err
}

// Server holds the database, mutex, MCP server, and logger.
type Server struct {
	DB                 *sql.DB
	DBPath             string
	Mu                 sync.RWMutex
	MCP                *mcp.Server
	Logger             *slog.Logger
	Embedder           Embedder  // nil if no embedding provider configured
	LastJanitorScan    time.Time // zero if background janitor hasn't run yet
	LastIntegrityCheck time.Time // zero if integrity check hasn't run yet
	HealCount          uint64    // total REINDEX self-heals since process start
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
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	if err := migrateMessagesToUUIDs(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to migrate message IDs to UUID: %w", err)
	}

	healed, err := healIndexes(db, logger)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to self-heal database: %w", err)
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "council-hub",
		Version: Version,
	}, &mcp.ServerOptions{
		Logger:       logger,
		Capabilities: &mcp.ServerCapabilities{},
		Instructions: "Council Hub is a multi-LLM coordination platform. Agents share state through persistent rooms backed by SQLite.\n\n" +
			"Session start (run in order):\n" +
			"1. get_mentions(author=<your-name>) — check for threads awaiting your input before anything else\n" +
			"2. get_digest — see what changed in the last 24h; note latest_message_id per active room for delta reads\n" +
			"3. load_resources(uri=council://guide) — read usage patterns on your first session in a new project\n\n" +
			"Key conventions:\n" +
			"- Prefer get_or_create_room over create_room — returns existing content and avoids duplicates\n" +
			"- Use typed messages: thought → draft → decision → action → synthesis. Avoid posting everything as 'message'\n" +
			"- After conclusions: post a synthesis, pin it, then signal_status(resolved)\n" +
			"- Call mark_read after each session; use get_digest(unread_only=true) on return to see only new activity\n" +
			"- Use search_messages for cross-room queries or filtered lookups; use read_transcript for full sequential context",
	})

	s := &Server{
		DB:                 db,
		DBPath:             dbPath,
		MCP:                mcpServer,
		Logger:             logger,
		LastIntegrityCheck: time.Now(),
	}
	if healed {
		s.HealCount = 1
	}
	return s, nil
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
		pinned BOOLEAN DEFAULT 0,
		reactions TEXT DEFAULT '{}',
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(room_id) REFERENCES rooms(id)
	);

	CREATE TABLE IF NOT EXISTS agent_cursors (
		agent TEXT NOT NULL,
		room_id TEXT NOT NULL,
		cursor_message_id TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (agent, room_id)
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		content,
		author UNINDEXED,
		room_id UNINDEXED,
		content='messages',
		content_rowid='rowid'
	);

	CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
		INSERT INTO messages_fts(rowid, content, author, room_id)
		VALUES (new.rowid, new.content, new.author, new.room_id);
	END;

	CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid, content, author, room_id)
		VALUES ('delete', old.rowid, old.content, old.author, old.room_id);
	END;

	CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid, content, author, room_id)
		VALUES ('delete', old.rowid, old.content, old.author, old.room_id);
		INSERT INTO messages_fts(rowid, content, author, room_id)
		VALUES (new.rowid, new.content, new.author, new.room_id);
	END;`

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
		`ALTER TABLE messages ADD COLUMN reactions TEXT DEFAULT '{}'`,
		`ALTER TABLE messages ADD COLUMN mentions TEXT DEFAULT ''`,
	}
	for _, m := range migrations {
		_, _ = db.Exec(m) // Ignore "duplicate column" errors for already-migrated DBs
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

	// Normalize existing project names to slug format (v0.7.2 migration).
	// Idempotent: already-normalized values are unchanged by normalizeProject.
	if projRows, err := db.Query(`SELECT id, project FROM rooms WHERE project != ''`); err == nil {
		type projUpdate struct{ id, project string }
		var projUpdates []projUpdate
		for projRows.Next() {
			var id, proj string
			if err := projRows.Scan(&id, &proj); err != nil {
				continue
			}
			if normalized := normalizeProject(proj); normalized != proj {
				projUpdates = append(projUpdates, projUpdate{id, normalized})
			}
		}
		_ = projRows.Close()
		for _, u := range projUpdates {
			_, _ = db.Exec(`UPDATE rooms SET project = ? WHERE id = ?`, u.project, u.id)
		}
	}

	// Always rebuild the FTS index on startup to ensure consistency.
	// This is fast (< 1s for typical databases) and guarantees the
	// index is correct after upgrades or schema changes.
	var msgCount int
	_ = db.QueryRow(`SELECT count(*) FROM messages`).Scan(&msgCount)
	if msgCount > 0 {
		if _, err := db.Exec(`INSERT INTO messages_fts(messages_fts) VALUES('rebuild')`); err != nil {
			return fmt.Errorf("rebuild fts index: %w", err)
		}
	}

	// Recreate sqlite-vec virtual tables if the embedding dimension changed.
	var existingDim int
	err = db.QueryRow(`SELECT vec_length(embedding) FROM message_vectors LIMIT 1`).Scan(&existingDim)
	if err == nil && existingDim != EmbedDim {
		for _, tbl := range []string{"message_vectors", "room_vectors"} {
			if _, err := db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, tbl)); err != nil {
				return fmt.Errorf("drop old vector table %s: %w", tbl, err)
			}
		}
	}

	// Create sqlite-vec virtual tables for vector search (idempotent).
	vecTables := []string{
		fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS message_vectors USING vec0(
			message_id TEXT PRIMARY KEY,
			embedding float[%d] distance_metric=cosine
		)`, EmbedDim),
		fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS room_vectors USING vec0(
			room_id TEXT PRIMARY KEY,
			embedding float[%d] distance_metric=cosine
		)`, EmbedDim),
	}
	for _, t := range vecTables {
		if _, err := db.Exec(t); err != nil {
			return fmt.Errorf("create vector table: %w", err)
		}
	}

	return nil
}

// healIndexes runs PRAGMA integrity_check at startup. If only B-tree index
// corruption is detected (typically caused by external file-indexers like
// macOS Spotlight touching the DB on a bad mount path), it runs REINDEX to
// rebuild the indexes in place. Deeper corruption aborts startup so it can
// be investigated manually instead of silently masked. Returns true if a
// REINDEX was executed.
func healIndexes(db *sql.DB, logger *slog.Logger) (bool, error) {
	issues, err := integrityCheck(db)
	if err != nil {
		return false, fmt.Errorf("integrity_check: %w", err)
	}
	if len(issues) == 0 {
		return false, nil
	}

	if !isIndexOnlyCorruption(issues) {
		return false, fmt.Errorf("database corruption beyond indexes, manual intervention required: %s", strings.Join(issues, "; "))
	}

	logger.Warn("index corruption detected on startup; running REINDEX",
		"issue_count", len(issues), "first_issue", issues[0])
	if _, err := db.Exec(`REINDEX`); err != nil {
		return false, fmt.Errorf("REINDEX failed: %w", err)
	}

	post, err := integrityCheck(db)
	if err != nil {
		return true, fmt.Errorf("post-reindex integrity_check: %w", err)
	}
	if len(post) > 0 {
		return true, fmt.Errorf("REINDEX did not resolve all issues: %s", strings.Join(post, "; "))
	}

	logger.Info("index self-heal completed", "fixed_issues", len(issues))
	return true, nil
}

// integrityCheck returns the list of non-"ok" lines from PRAGMA integrity_check.
// A healthy database returns an empty slice.
func integrityCheck(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`PRAGMA integrity_check`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var issues []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		if s != "ok" {
			issues = append(issues, s)
		}
	}
	return issues, rows.Err()
}

// isIndexOnlyCorruption returns true iff every issue line mentions "index",
// which SQLite's integrity checker uses for index-level complaints like
// "wrong # of entries in index X" or "row N missing from index X".
// Even if this classifier is too lenient, the post-REINDEX integrity check
// in healIndexes catches any residual damage and returns a hard error.
func isIndexOnlyCorruption(issues []string) bool {
	if len(issues) == 0 {
		return false
	}
	for _, s := range issues {
		if !strings.Contains(s, "index") {
			return false
		}
	}
	return true
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
			_ = rows.Close()
			return fmt.Errorf("scan old message: %w", err)
		}
		oldRows = append(oldRows, r)
	}
	_ = rows.Close()
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
		_ = tx.Rollback()
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
		_ = tx.Rollback()
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
			_ = tx.Rollback()
			return fmt.Errorf("insert migrated message: %w", err)
		}
	}

	if _, err := tx.Exec(`DROP TABLE messages_old`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("drop old messages table: %w", err)
	}

	return tx.Commit()
}
