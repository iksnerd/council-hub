package main

import (
	"testing"
)

// RWMutex: concurrent reads don't block each other
func TestConcurrentReads(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "concurrent-room", withProject("proj"), withTechStack("Go"), withTags("tag"))
	for i := 0; i < 10; i++ {
		mustPost(t, cs, "concurrent-room", "Claude", "msg")
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := cs.listRooms("", "", "", "")
			if err != nil {
				t.Errorf("concurrent listRooms failed: %v", err)
			}
			_, err = cs.getTranscript("concurrent-room")
			if err != nil {
				t.Errorf("concurrent getTranscript failed: %v", err)
			}
			_, err = cs.searchMessages("msg", "", "", "", "", 10)
			if err != nil {
				t.Errorf("concurrent searchMessages failed: %v", err)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// RWMutex: concurrent reads with writes don't corrupt data
func TestConcurrentReadsAndWrites(t *testing.T) {
	cs := setupTestServer(t)
	mustCreateRoom(t, cs, "rw-room")

	done := make(chan bool, 20)

	// 10 writers
	for i := 0; i < 10; i++ {
		go func(n int) {
			_, err := cs.postMessage("rw-room", "Writer", "msg", "message", 0)
			if err != nil {
				t.Errorf("concurrent write %d failed: %v", n, err)
			}
			done <- true
		}(i)
	}

	// 10 readers running concurrently with writers
	for i := 0; i < 10; i++ {
		go func(n int) {
			_, err := cs.getRecentMessages("rw-room", 5)
			if err != nil {
				t.Errorf("concurrent read %d failed: %v", n, err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify all 10 messages were written
	msgs, _ := cs.getTranscript("rw-room")
	if len(msgs) != 10 {
		t.Errorf("expected 10 messages after concurrent writes, got %d", len(msgs))
	}
}

// Connection pool: verify MaxOpenConns is set (functional test)
func TestConnectionPoolConfig(t *testing.T) {
	cs := setupTestServer(t)

	stats := cs.db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("expected MaxOpenConnections=1, got %d", stats.MaxOpenConnections)
	}
}
