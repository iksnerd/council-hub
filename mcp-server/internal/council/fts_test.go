package council

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestSearchMessages(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("search-room-1", "Room 1", "proj", "", "", "", "")
	s.CreateRoom("search-room-2", "Room 2", "proj", "", "", "", "")
	s.PostMessage("search-room-1", "Claude", "JWT token validation is broken", "thought", "")
	s.PostMessage("search-room-1", "Gemini", "I agree about the JWT issue", "review", "")
	s.PostMessage("search-room-2", "Claude", "Database migration complete", "action", "")

	// Search by keyword
	msgs, err := s.SearchMessages("JWT", "", "", "", "", "", "", 20)
	if err != nil {
		t.Fatalf("searchMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages with 'JWT', got %d", len(msgs))
	}

	// Search by author
	msgs, _ = s.SearchMessages("", "Claude", "", "", "", "", "", 20)
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages from Claude, got %d", len(msgs))
	}

	// Search by message type
	msgs, _ = s.SearchMessages("", "", "review", "", "", "", "", 20)
	if len(msgs) != 1 {
		t.Errorf("expected 1 review message, got %d", len(msgs))
	}

	// Search scoped to room
	msgs, _ = s.SearchMessages("", "Claude", "", "search-room-2", "", "", "", 20)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message from Claude in search-room-2, got %d", len(msgs))
	}

	// No results
	msgs, _ = s.SearchMessages("nonexistent", "", "", "", "", "", "", 20)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestSearchMessagesGlobal(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("search-global-a", "Room A", "proj", "", "", "", "")
	s.CreateRoom("search-global-b", "Room B", "proj", "", "", "", "")
	s.PostMessage("search-global-a", "Claude", "BEP 44 analysis here", "thought", "")
	s.PostMessage("search-global-b", "Gemini", "BEP 46 analysis here", "review", "")

	msgs, err := s.SearchMessages("BEP", "", "", "", "", "", "", 20)
	if err != nil {
		t.Fatalf("searchMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 global results, got %d", len(msgs))
	}
	rooms := map[string]bool{}
	for _, m := range msgs {
		rooms[m.RoomID] = true
	}
	if len(rooms) != 2 {
		t.Errorf("expected results from 2 rooms, got %d", len(rooms))
	}
}

func TestSearchMessagesSnippetLength(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("search-snippet", "Snippet test", "", "", "", "", "")
	longContent := "searchword " + strings.Repeat("A", 400)
	s.PostMessage("search-snippet", "Claude", longContent, "message", "")

	msgs, err := s.SearchMessages("searchword", "", "", "search-snippet", "", "", "", 1)
	if err != nil {
		t.Fatalf("searchMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if len(msgs[0].Content) != 411 {
		t.Errorf("expected full 411 char content from DB, got %d", len(msgs[0].Content))
	}
}

func TestSearchMessagesLimitClamping(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("limit-room", "Limit test", "", "", "", "", "")
	s.PostMessage("limit-room", "Claude", "keyword", "message", "")

	msgs, _ := s.SearchMessages("keyword", "", "", "", "", "", "", -5)
	if len(msgs) != 1 {
		t.Errorf("expected 1 result with negative limit (default 20), got %d", len(msgs))
	}

	msgs, _ = s.SearchMessages("keyword", "", "", "", "", "", "", 200)
	if len(msgs) != 1 {
		t.Errorf("expected 1 result with oversized limit (clamped 100), got %d", len(msgs))
	}
}

func TestSearchMessagesMultiWord(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("multi-room", "Multi-word search", "", "", "", "", "")
	s.PostMessage("multi-room", "Claude", "distributed cluster architecture", "thought", "")
	s.PostMessage("multi-room", "Gemini", "single node mode", "review", "")

	msgs, err := s.SearchMessages("distributed cluster", "", "", "", "", "", "", 20)
	if err != nil {
		t.Fatalf("SearchMessages multi-word failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 result for 'distributed cluster', got %d", len(msgs))
	}

	msgs, _ = s.SearchMessages("distributed nonexistent", "", "", "", "", "", "", 20)
	if len(msgs) != 0 {
		t.Errorf("expected 0 results for 'distributed nonexistent', got %d", len(msgs))
	}
}

func TestSearchMessagesSince(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("since-room", "Since test", "", "", "", "", "")
	past := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	future := time.Now().UTC().Add(1 * time.Hour).Format("2006-01-02 15:04:05")
	s.PostMessage("since-room", "Claude", "message now", "message", "")

	since := past
	msgs, err := s.SearchMessages("", "", "", "since-room", "", since, "", 20)
	if err != nil {
		t.Fatalf("SearchMessages since failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message since %s, got %d", since, len(msgs))
	}

	msgs, _ = s.SearchMessages("", "", "", "since-room", "", future, "", 20)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages since %s, got %d", future, len(msgs))
	}
}

func TestSearchMessagesUntil(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("until-room", "Until test", "", "", "", "", "")
	past := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	future := time.Now().UTC().Add(1 * time.Hour).Format("2006-01-02 15:04:05")
	s.PostMessage("until-room", "Claude", "message now", "message", "")

	msgs, err := s.SearchMessages("", "", "", "until-room", "", "", past, 20)
	if err != nil {
		t.Fatalf("SearchMessages until failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages until %s, got %d", past, len(msgs))
	}

	msgs, _ = s.SearchMessages("", "", "", "until-room", "", "", future, 20)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message until %s, got %d", future, len(msgs))
	}
}

func TestSearchMessagesSinceAndUntil(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("range-room", "Range test", "", "", "", "", "")
	past := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	future := time.Now().UTC().Add(1 * time.Hour).Format("2006-01-02 15:04:05")
	s.PostMessage("range-room", "Claude", "message now", "message", "")

	msgs, err := s.SearchMessages("", "", "", "range-room", "", past, future, 20)
	if err != nil {
		t.Fatalf("SearchMessages range failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message in range, got %d", len(msgs))
	}
}

func TestSearchMessagesByProject(t *testing.T) {
	s := setupTestServer(t)
	mustCreateRoom(t, s, "proj-room-a", withProject("alpha"))
	mustCreateRoom(t, s, "proj-room-b", withProject("beta"))
	mustPost(t, s, "proj-room-a", "Claude", "shared keyword")
	mustPost(t, s, "proj-room-b", "Gemini", "shared keyword")

	msgs, err := s.SearchMessages("keyword", "", "", "", "alpha", "", "", 20)
	if err != nil {
		t.Fatalf("SearchMessages with project filter failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message from project alpha, got %d", len(msgs))
	}
	if msgs[0].RoomID != "proj-room-a" {
		t.Errorf("expected proj-room-a, got %s", msgs[0].RoomID)
	}
}

func TestSearchMessagesFTSSync(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("fts-room", "FTS Sync", "", "", "", "", "")

	// 1. Insert
	id, _ := s.PostMessage("fts-room", "Alice", "This is the original content about bananas", "message", "")

	msgs, err := s.SearchMessages("bananas", "", "", "fts-room", "", "", "", 10)
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for 'bananas', got %d", len(msgs))
	}

	// 2. Edit — append-only: a new head node carries "apples"; the original "bananas"
	// node is preserved but flagged revised, so search (which filters to revised = 0)
	// stops matching "bananas" and starts matching "apples".
	head, err := s.UpdateMessage(id, "This is the updated content about apples", "message")
	if err != nil {
		t.Fatalf("UpdateMessage failed: %v", err)
	}

	msgs, _ = s.SearchMessages("bananas", "", "", "fts-room", "", "", "", 10)
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages for 'bananas' after update, got %d", len(msgs))
	}

	msgs, _ = s.SearchMessages("apples", "", "", "fts-room", "", "", "", 10)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for 'apples', got %d", len(msgs))
	}

	// 3. Purge the head (hard delete fires the FTS delete trigger)
	s.PurgeMessages([]string{head.ID})
	msgs, _ = s.SearchMessages("apples", "", "", "fts-room", "", "", "", 10)
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages for 'apples' after delete, got %d", len(msgs))
	}
}

func TestSearchMessagesFTSRanking(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("rank-room", "FTS Rank", "", "", "", "", "")

	// Post multiple messages with varying frequencies of the target word.
	// FTS5 bm25 assigns a lower (more negative) score to better matches.
	s.PostMessage("rank-room", "Bob", "just a single keyword here", "message", "")
	s.PostMessage("rank-room", "Bob", "keyword keyword keyword keyword keyword", "message", "")
	s.PostMessage("rank-room", "Bob", "another message with keyword in it twice keyword", "message", "")

	msgs, err := s.SearchMessages("keyword", "", "", "rank-room", "", "", "", 10)
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	// The second message has "keyword" 5 times, it should rank first.
	if !strings.Contains(msgs[0].Content, "keyword keyword keyword") {
		t.Errorf("expected highest frequency match to rank first, got: %s", msgs[0].Content)
	}
}

func TestSearchMessagesFTSQuotes(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("quote-room", "FTS Quotes", "", "", "", "", "")

	s.PostMessage("quote-room", "Charlie", "We need to find this exact word", "message", "")

	// Query with quotes inside should be sanitized and still match
	msgs, err := s.SearchMessages(`"exact"`, "", "", "quote-room", "", "", "", 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for quoted query, got %d", len(msgs))
	}
}

func TestSearchMessagesFTSEmptyCleanQuery(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("empty-clean", "Room", "", "", "", "", "")
	s.PostMessage("empty-clean", "Alice", "Testing empty clean", "message", "")

	// All quotes, stripped out completely
	msgs, err := s.SearchMessages(`""""`, "", "", "", "", "", "", 20)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	// It should return the message because len(terms) == 0, so it acts like no query
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestSearchMessagesFTSCombineFilters(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("combine-room", "Room", "my-project", "", "", "", "")

	s.PostMessage("combine-room", "Alice", "We need a database index", "thought", "")
	s.PostMessage("combine-room", "Bob", "Database index created", "action", "")
	s.PostMessage("combine-room", "Alice", "Database migration done", "action", "")

	// Search "database" + author="Alice" + type="action" + project="my-project"
	msgs, err := s.SearchMessages("database", "Alice", "action", "combine-room", "my-project", "", "", 20)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, "migration") {
		t.Errorf("expected migration message, got: %s", msgs[0].Content)
	}
}

func TestInitSchemaFTSRebuild(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	defer db.Close()

	// Create schema without FTS triggers first — simulate pre-FTS database
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS rooms (
			id TEXT PRIMARY KEY,
			description TEXT DEFAULT '',
			project TEXT DEFAULT '',
			system_prompt TEXT DEFAULT '',
			tech_stack TEXT DEFAULT '',
			tags TEXT DEFAULT '',
			related_rooms TEXT DEFAULT '',
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			room_id TEXT NOT NULL,
			author TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT DEFAULT 'message',
			is_summary BOOLEAN DEFAULT 0,
			pinned BOOLEAN DEFAULT 0,
			reply_to TEXT DEFAULT '',
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(room_id) REFERENCES rooms(id)
		);
	`)
	if err != nil {
		t.Fatalf("create tables failed: %v", err)
	}

	// Insert data before FTS exists
	_, err = db.Exec(`INSERT INTO rooms (id) VALUES ('r1')`)
	if err != nil {
		t.Fatalf("insert room failed: %v", err)
	}
	_, err = db.Exec(`INSERT INTO messages (id, room_id, author, content) VALUES ('m1', 'r1', 'Alice', 'Hello world')`)
	if err != nil {
		t.Fatalf("insert message failed: %v", err)
	}

	// Now run initSchema — it should create FTS table and rebuild index
	err = initSchema(db)
	if err != nil {
		t.Fatalf("initSchema failed: %v", err)
	}

	var count int
	db.QueryRow(`SELECT count(*) FROM messages_fts`).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 fts row after rebuild, got %d", count)
	}
}
