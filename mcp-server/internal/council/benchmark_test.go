package council

import (
	"fmt"
	"testing"
)

// setupBenchServer creates a test server for benchmarks (no *testing.T cleanup).
func setupBenchServer(b *testing.B) *Server {
	b.Helper()
	s, err := NewServer(":memory:", testLogger())
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}
	b.Cleanup(func() { s.DB.Close() })
	return s
}

func seedRoom(b *testing.B, s *Server, roomID string, msgCount int) {
	b.Helper()
	if err := s.CreateRoom(roomID, "Bench room", "bench-project", "Go", "bench", "", ""); err != nil {
		b.Fatalf("CreateRoom failed: %v", err)
	}
	authors := []string{"Alice", "Bob", "Charlie"}
	types := []string{"message", "thought", "decision", "code"}
	for i := 0; i < msgCount; i++ {
		_, err := s.PostMessage(roomID, authors[i%3], fmt.Sprintf("Benchmark message %d with some content to search through", i), types[i%4], "")
		if err != nil {
			b.Fatalf("PostMessage failed: %v", err)
		}
	}
}

// --- Write operations ---

func BenchmarkPostMessage(b *testing.B) {
	s := setupBenchServer(b)
	if err := s.CreateRoom("bench-write", "Write bench", "", "", "", "", ""); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.PostMessage("bench-write", "Alice", fmt.Sprintf("msg %d", i), "message", "")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPostMessageParallel(b *testing.B) {
	s := setupBenchServer(b)
	if err := s.CreateRoom("bench-parallel", "Parallel bench", "", "", "", "", ""); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, err := s.PostMessage("bench-parallel", "Alice", fmt.Sprintf("msg %d", i), "message", "")
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// --- Read operations ---

func BenchmarkGetRecentMessages(b *testing.B) {
	s := setupBenchServer(b)
	seedRoom(b, s, "bench-recent", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.GetRecentMessages("bench-recent", 20)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetTranscript(b *testing.B) {
	s := setupBenchServer(b)
	seedRoom(b, s, "bench-transcript", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.GetTranscript("bench-transcript")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetMessagesAfterID(b *testing.B) {
	s := setupBenchServer(b)
	seedRoom(b, s, "bench-delta", 100)

	// Get a message ID near the middle to simulate a delta read
	msgs, _ := s.GetRecentMessages("bench-delta", 50)
	afterID := msgs[0].ID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.GetMessagesAfterID("bench-delta", afterID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchMessages(b *testing.B) {
	s := setupBenchServer(b)
	seedRoom(b, s, "bench-search", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.SearchMessages("content to search", "", "", "", "", "", "", 20)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// --- Room operations ---

func BenchmarkListRooms(b *testing.B) {
	s := setupBenchServer(b)
	for i := 0; i < 50; i++ {
		project := "alpha"
		if i%2 == 0 {
			project = "beta"
		}
		if err := s.CreateRoom(fmt.Sprintf("room-%d", i), "Bench room", project, "Go", "bench", "", ""); err != nil {
			b.Fatal(err)
		}
	}

	b.Run("unfiltered", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := s.ListRooms("", "", "", "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("by_project", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := s.ListRooms("alpha", "", "", "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("by_search", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := s.ListRooms("", "", "", "room-2")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// --- Aggregation ---

func BenchmarkGetRoomStats(b *testing.B) {
	s := setupBenchServer(b)
	seedRoom(b, s, "bench-stats", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.GetRoomStats("bench-stats")
		if err != nil {
			b.Fatal(err)
		}
	}
}
