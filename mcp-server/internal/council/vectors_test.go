package council

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// mockEmbedder returns a fixed vector for any input.
type mockEmbedder struct {
	vec []float32
	err error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return m.vec, m.err
}

func makeVec(seed float32) []float32 {
	vec := make([]float32, EmbedDim)
	for i := range vec {
		vec[i] = seed + float32(i)*0.001
	}
	return vec
}

func TestStoreAndSearchVector(t *testing.T) {
	s := setupTestServer(t)

	s.CreateRoom("test-room", "Test room", "proj", "", "", "", "")
	id, _ := s.PostMessage("test-room", "alice", "authentication concerns with tokens", "thought", "")

	vec := makeVec(0.1)
	if err := s.StoreVector("message_vectors", id, vec); err != nil {
		t.Fatalf("StoreVector failed: %v", err)
	}

	// Upsert: storing again should not error
	if err := s.StoreVector("message_vectors", id, vec); err != nil {
		t.Fatalf("StoreVector upsert failed: %v", err)
	}
}

func TestDeleteVectorsLocked(t *testing.T) {
	s := setupTestServer(t)

	s.CreateRoom("test-room", "Test room", "proj", "", "", "", "")
	id1, _ := s.PostMessage("test-room", "alice", "msg one", "thought", "")
	id2, _ := s.PostMessage("test-room", "bob", "msg two", "thought", "")

	_ = s.StoreVector("message_vectors", id1, makeVec(0.1))
	_ = s.StoreVector("message_vectors", id2, makeVec(0.2))

	s.Mu.Lock()
	s.deleteVectorsLocked("message_vectors", []string{id1, id2})
	s.Mu.Unlock()

	// Vectors should be gone — store again should work (confirms delete worked)
	if err := s.StoreVector("message_vectors", id1, makeVec(0.3)); err != nil {
		t.Fatalf("StoreVector after delete failed: %v", err)
	}
}

func TestRoomVectors(t *testing.T) {
	s := setupTestServer(t)

	vec := makeVec(0.5)
	s.CreateRoom("vec-room", "Auth migration room", "proj", "", "", "Handle auth tokens", "")

	if err := s.StoreVector("room_vectors", "vec-room", vec); err != nil {
		t.Fatalf("StoreVector room failed: %v", err)
	}

	// Upsert
	if err := s.StoreVector("room_vectors", "vec-room", makeVec(0.6)); err != nil {
		t.Fatalf("StoreVector room upsert failed: %v", err)
	}
}

func TestEmbedAsync(t *testing.T) {
	s := setupTestServer(t)
	// No embedder set — should be a no-op
	s.EmbedAsync("message_vectors", "fake-id", "some text")
	// No crash = pass
}

func TestSearchMessagesSemantic_NoEmbedder(t *testing.T) {
	s := setupTestServer(t)
	_, err := s.SearchMessagesSemantic("test query", "", "", "", "", "", "", 10)
	if err == nil {
		t.Fatal("expected error when no embedder configured")
	}
}

func TestSearchMessagesSemantic_WithMock(t *testing.T) {
	s := setupTestServer(t)

	s.CreateRoom("test-room", "Test", "proj", "", "", "", "")
	id1, _ := s.PostMessage("test-room", "alice", "login flow implementation", "thought", "")
	id2, _ := s.PostMessage("test-room", "bob", "database migration plan", "action", "")
	id3, _ := s.PostMessage("test-room", "alice", "auth token refresh logic", "decision", "")

	// Store vectors: id1 and id3 are "near" the query, id2 is "far"
	nearVec := makeVec(0.1)
	farVec := makeVec(0.9)
	_ = s.StoreVector("message_vectors", id1, nearVec)
	_ = s.StoreVector("message_vectors", id2, farVec)
	_ = s.StoreVector("message_vectors", id3, makeVec(0.11)) // close to nearVec

	// Set up mock embedder that returns the "near" vector for any query
	s.Embedder = &mockEmbedder{vec: nearVec}

	messages, err := s.SearchMessagesSemantic("authentication", "", "", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("SearchMessagesSemantic failed: %v", err)
	}

	if len(messages) == 0 {
		t.Fatal("expected results from semantic search")
	}

	// First result should be id1 (exact match to query vector)
	if messages[0].ID != id1 {
		t.Errorf("expected first result to be %s, got %s", id1, messages[0].ID)
	}
}

func TestSearchMessagesSemantic_WithFilters(t *testing.T) {
	s := setupTestServer(t)

	s.CreateRoom("room-a", "Room A", "proj-a", "", "", "", "")
	s.CreateRoom("room-b", "Room B", "proj-b", "", "", "", "")
	id1, _ := s.PostMessage("room-a", "alice", "auth stuff", "thought", "")
	id2, _ := s.PostMessage("room-b", "bob", "auth other", "decision", "")

	vec := makeVec(0.1)
	_ = s.StoreVector("message_vectors", id1, vec)
	_ = s.StoreVector("message_vectors", id2, vec)

	s.Embedder = &mockEmbedder{vec: vec}

	// Filter by room
	messages, _ := s.SearchMessagesSemantic("auth", "room-a", "", "", "", "", "", 10)
	if len(messages) != 1 || messages[0].ID != id1 {
		t.Errorf("room filter failed: expected 1 result from room-a, got %d", len(messages))
	}

	// Filter by author
	messages, _ = s.SearchMessagesSemantic("auth", "", "", "bob", "", "", "", 10)
	if len(messages) != 1 || messages[0].ID != id2 {
		t.Errorf("author filter failed: expected 1 result from bob, got %d", len(messages))
	}

	// Filter by message type
	messages, _ = s.SearchMessagesSemantic("auth", "", "", "", "decision", "", "", 10)
	if len(messages) != 1 || messages[0].ID != id2 {
		t.Errorf("type filter failed: expected 1 result with type=decision, got %d", len(messages))
	}
}

func TestBackfillEmbeddings(t *testing.T) {
	s := setupTestServer(t)

	s.CreateRoom("bf-room", "Backfill test room", "proj", "", "", "System prompt here", "")
	s.PostMessage("bf-room", "alice", "message one", "thought", "")
	s.PostMessage("bf-room", "bob", "message two", "action", "")

	// Set embedder and run backfill
	s.Embedder = &mockEmbedder{vec: makeVec(0.5)}
	s.BackfillEmbeddings(context.Background())

	// Verify vectors were created by searching
	messages, err := s.SearchMessagesSemantic("anything", "", "", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("search after backfill failed: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("expected 2 backfilled messages, got %d", len(messages))
	}
}

func TestBackfillEmbeddingsSkipsRevisedAndRetracted(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("bf-skip", "Backfill skip room", "proj", "", "", "", "")

	live, _ := s.PostMessage("bf-skip", "alice", "still live", "thought", "")
	edited, _ := s.PostMessage("bf-skip", "alice", "original wording", "thought", "")
	retracted, _ := s.PostMessage("bf-skip", "bob", "withdrawn", "thought", "")

	// Revise one (the prior version gets revised=1) and retract another; only
	// live heads should be embedded (liveClause).
	head, err := s.UpdateMessageWithExpected(edited, "new wording", "", "", "alice")
	if err != nil {
		t.Fatalf("UpdateMessageWithExpected: %v", err)
	}
	if _, err := s.RetractMessages([]string{retracted}, "bob"); err != nil {
		t.Fatalf("RetractMessages: %v", err)
	}

	s.Embedder = &mockEmbedder{vec: makeVec(0.5)}
	s.BackfillEmbeddings(context.Background())

	hasVector := func(id string) bool {
		var one int
		return s.DB.QueryRow(`SELECT 1 FROM message_vectors WHERE message_id = ?`, id).Scan(&one) == nil
	}
	if !hasVector(live) {
		t.Error("expected live message to be embedded")
	}
	if !hasVector(head.ID) {
		t.Error("expected the revision head to be embedded")
	}
	if hasVector(edited) {
		t.Error("revised (superseded) version must not be embedded")
	}
	if hasVector(retracted) {
		t.Error("retracted message must not be embedded")
	}
}

func TestEmbeddingCoverage(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("cov-room", "Coverage room", "proj", "", "", "", "")
	id, _ := s.PostMessage("cov-room", "alice", "message one", "thought", "")

	msgTotal, msgIndexed, roomTotal, roomIndexed := s.EmbeddingCoverage()
	if msgTotal != 1 || msgIndexed != 0 || roomTotal != 1 || roomIndexed != 0 {
		t.Fatalf("unexpected coverage before embedding: msgTotal=%d msgIndexed=%d roomTotal=%d roomIndexed=%d",
			msgTotal, msgIndexed, roomTotal, roomIndexed)
	}

	_ = s.StoreVector("message_vectors", id, makeVec(0.1))
	_ = s.StoreVector("room_vectors", "cov-room", makeVec(0.2))

	msgTotal, msgIndexed, roomTotal, roomIndexed = s.EmbeddingCoverage()
	if msgIndexed != 1 || roomIndexed != 1 {
		t.Fatalf("expected coverage to reflect stored vectors, got msgIndexed=%d roomIndexed=%d", msgIndexed, roomIndexed)
	}
}

func TestTriggerEmbedJob_NoEmbedder(t *testing.T) {
	s := setupTestServer(t)
	if s.TriggerEmbedJob(context.Background(), false) {
		t.Fatal("expected TriggerEmbedJob to return false with no embedder configured")
	}
}

func TestTriggerEmbedJob_StartsAndCompletesBackfill(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("trigger-room", "Trigger room", "proj", "", "", "", "")
	s.PostMessage("trigger-room", "alice", "message one", "thought", "")
	s.Embedder = &mockEmbedder{vec: makeVec(0.5)}

	if !s.TriggerEmbedJob(context.Background(), false) {
		t.Fatal("expected TriggerEmbedJob to start")
	}

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&s.EmbedJobRunning) == 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	_, msgIndexed, _, _ := s.EmbeddingCoverage()
	if msgIndexed != 1 {
		t.Fatalf("expected the triggered backfill to index the pending message, got msgIndexed=%d", msgIndexed)
	}
}

func TestTriggerEmbedJob_FalseWhenAlreadyRunning(t *testing.T) {
	s := setupTestServer(t)
	s.Embedder = &mockEmbedder{vec: makeVec(0.5)}

	atomic.StoreInt32(&s.EmbedJobRunning, 1)
	defer atomic.StoreInt32(&s.EmbedJobRunning, 0)

	if s.TriggerEmbedJob(context.Background(), false) {
		t.Fatal("expected TriggerEmbedJob to return false while a job is already running")
	}
	if s.TriggerEmbedJob(context.Background(), true) {
		t.Fatal("expected a full re-embed trigger to also be refused while a job is already running")
	}
}

func TestBackfillEmbeddings_SkipsWhenJobAlreadyRunning(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("skip-room", "Skip room", "proj", "", "", "", "")
	s.PostMessage("skip-room", "alice", "message one", "thought", "")
	s.Embedder = &mockEmbedder{vec: makeVec(0.5)}

	atomic.StoreInt32(&s.EmbedJobRunning, 1)
	s.BackfillEmbeddings(context.Background())
	atomic.StoreInt32(&s.EmbedJobRunning, 0)

	_, msgIndexed, _, _ := s.EmbeddingCoverage()
	if msgIndexed != 0 {
		t.Fatalf("expected BackfillEmbeddings to no-op while guard is held, got msgIndexed=%d", msgIndexed)
	}
}

// countingEmbedder tracks how many times Embed was called — used to tell a
// missing-only backfill (should skip an already-indexed row) apart from a
// full re-embed (must re-embed it anyway).
type countingEmbedder struct {
	vec   []float32
	calls int
}

func (m *countingEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	m.calls++
	return m.vec, nil
}

func TestRunFullReembedRecomputesAlreadyIndexedVectors(t *testing.T) {
	s := setupTestServer(t)
	s.CreateRoom("full-room", "Full reembed room", "proj", "", "", "", "")
	id, _ := s.PostMessage("full-room", "alice", "message one", "thought", "")

	// Cover both the message and its room, so the "already indexed" premise
	// holds for both of runBackfill's two passes.
	_ = s.StoreVector("message_vectors", id, makeVec(0.1))
	_ = s.StoreVector("room_vectors", "full-room", makeVec(0.1))

	counter := &countingEmbedder{vec: makeVec(0.9)}
	s.Embedder = counter

	// A missing-only backfill must skip both — they're already indexed.
	s.BackfillEmbeddings(context.Background())
	if counter.calls != 0 {
		t.Fatalf("expected missing-only backfill to skip already-indexed rows, got %d embed calls", counter.calls)
	}

	// A full re-embed must clear and recompute both anyway (e.g. after
	// switching COUNCIL_EMBED_MODEL, where the old vectors are stale).
	s.runFullReembed(context.Background())
	if counter.calls != 2 {
		t.Fatalf("expected full re-embed to re-embed the message and its room, got %d embed calls", counter.calls)
	}

	_, msgIndexed, _, roomIndexed := s.EmbeddingCoverage()
	if msgIndexed != 1 || roomIndexed != 1 {
		t.Fatalf("expected exactly 1 indexed message and 1 indexed room after full re-embed, got msgIndexed=%d roomIndexed=%d",
			msgIndexed, roomIndexed)
	}
}
