Taking a step back to consolidate everything into a unified blueprint is a great idea. It prevents spaghetti code later on.

Here is the complete, refined **Technical Specification for the LLM Council Hub**, incorporating the Go-based MCP server, SQLite persistence, the "Virtual Rooms" concept, and the advanced tricks we discussed.

---

# Technical Specification: LLM Council Hub

## 1. System Architecture
The system follows a **Hub-and-Spoke** topology. The Go application acts as the central state manager (the Hub), while various LLM clients (the Spokes) connect to it asynchronously.



* **The Hub (Go Server):** Runs as a background process. It implements the Model Context Protocol (MCP) over `stdio` or SSE and manages all read/write operations to the database.
* **The State (SQLite):** A single file (`council.db`) that guarantees persistence, ACID compliance for concurrent writes from different terminals, and easy integration with a future Web UI.
* **The Clients (LLMs):** Claude Code, Gemini CLI, or custom scripts. They act as autonomous agents that read the state, perform tasks, and write back their findings.

---

## 2. Database Schema (`council.db`)
The schema is designed to be lightweight but robust enough to support future features like web-based dashboards and conversation summarization.

```sql
-- Represents a specific topic, task, or "Virtual Room"
CREATE TABLE IF NOT EXISTS rooms (
    id TEXT PRIMARY KEY,             -- e.g., "auth-migration-v2"
    description TEXT,                -- e.g., "Refactoring JWT logic"
    status TEXT DEFAULT 'active',    -- 'active', 'paused', 'resolved'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- The immutable ledger of all model interactions
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id TEXT,
    author TEXT,                     -- e.g., "Claude", "Gemini", "Admin"
    content TEXT,                    -- The markdown/code payload
    is_summary BOOLEAN DEFAULT 0,    -- Used by the Janitor routine
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(room_id) REFERENCES rooms(id)
);
```

---

## 3. The MCP Contract (Tools & Resources)
This is the API surface that the LLMs will "see" when they connect to your Go server.

### A. Tools (State Mutations)
Models will call these tools to change the state of the Council.

| Tool Name | Arguments | Description |
| :--- | :--- | :--- |
| `create_room` | `id` (str), `topic` (str) | Initializes a new workspace if it doesn't exist. |
| `post_to_room` | `room_id` (str), `author` (str), `message` (str) | Appends a thought, critique, or code snippet to the room's ledger. |
| `signal_status` | `room_id` (str), `status` (str) | Allows a model to announce its state (e.g., "waiting on Gemini review", "implementing"). |

### B. Resources (State Observation)
This is how models "read the room." The Go server intercepts these URI requests and builds a dynamic document.

* **URI Template:** `council://room/{room_id}/transcript`
* **Go Server Behavior:** 1. Fetches all messages for `room_id` from SQLite.
    2. Formats them into a clear Markdown structure.
    3. Prepends a **System Context Header** (e.g., *"You are viewing the Council log. Do not repeat previous points."*).

---

## 4. Go Implementation Internals
To ensure the server doesn't crash when Claude and Gemini try to post at the exact same millisecond, the Go backend requires specific structural safeguards.

### Core Data Structures
```go
package main

import (
    "database/sql"
    "sync"
    _ "github.com/mattn/go-sqlite3"
    "github.com/modelcontextprotocol/go-sdk/server"
)

// ServerState holds the DB connection and a Mutex for thread safety
type CouncilServer struct {
    DB    *sql.DB
    Mutex sync.Mutex
    MCP   *server.Server
}

// HandlePost safely writes to SQLite
func (s *CouncilServer) HandlePost(roomID, author, content string) error {
    s.Mutex.Lock()         // Lock the state
    defer s.Mutex.Unlock() // Ensure it unlocks even if a panic occurs

    query := `INSERT INTO messages (room_id, author, content) VALUES (?, ?, ?)`
    _, err := s.DB.Exec(query, roomID, author, content)
    return err
}
```

---

## 5. Advanced Mechanics (The "Tricks")

### 1. The Context-Injection Trick
When a model requests the `council://room/...` resource, the Go server shouldn't just return raw database rows. It should compile a **Prompt-Optimized Markdown Document**.
```markdown
# 🏛️ COUNCIL ROOM: {room_id}
**Topic:** {description}
**Current Status:** {status}
---
**[2026-03-28 10:00:00] Gemini:**
I propose we use RS256 for the JWTs. Here is the schema...

**[2026-03-28 10:05:00] Claude:**
I see the schema. I will implement the Go middleware now.
---
*SYSTEM INSTRUCTION: If you are an LLM reading this, append your next action using the `post_to_room` tool.*
```

### 2. The "Janitor" Goroutine (Context Management)
To prevent the models from blowing through their token limits by reading a 500-message history, implement a background worker in Go:
1.  **Monitor:** Every 5 minutes, check if a room has > 20 un-summarized messages.
2.  **Summarize:** If yes, send those 20 messages to a fast, cheap model via an API call (e.g., Gemini Flash).
3.  **Compress:** Save the summary as a new message with `is_summary = 1`, and ignore the older individual messages in future Resource reads.

---

## 6. Execution Flow (Day-to-Day Usage)

1.  **Boot the Hub:** You run `go run main.go` in a dedicated background terminal. It spins up the SQLite DB and listens for MCP connections.
2.  **Assign Roles:**
    * Open Terminal A: `claude-code "Monitor council://room/auth-api. You are the lead developer. Execute agreed-upon plans."`
    * Open Terminal B: `gemini-cli "Review Claude's code in council://room/auth-api. You are the security auditor. Use post_to_room to raise issues."`
3.  **Observe:** You can open a database viewer (or your future Web UI) and watch the `messages` table populate as the models negotiate and write code.

---
