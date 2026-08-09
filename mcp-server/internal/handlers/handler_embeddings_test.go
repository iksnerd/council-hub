package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// stubEmbedder returns a fixed vector for any input — enough to exercise the
// on-demand trigger paths without a real Ollama server.
type stubEmbedder struct {
	vec []float32
}

func (m *stubEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return m.vec, nil
}

func waitForEmbedJobIdle(t *testing.T, reg *Registry) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&reg.Server.EmbedJobRunning) == 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHandleRegenerateEmbeddingsNoEmbedder(t *testing.T) {
	reg := setupHandlerTest(t)

	res, _, _ := reg.handleRegenerateEmbeddings(context.Background(), nil, RegenerateEmbeddingsInput{})
	if !strings.Contains(resultText(res), "not enabled") {
		t.Errorf("expected 'not enabled' error, got: %s", resultText(res))
	}
}

func TestHandleRegenerateEmbeddingsStartsBackfill(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "embed-room")
	reg.Server.PostMessage("embed-room", "alice", "hello world", "thought", "")
	reg.Server.Embedder = &stubEmbedder{vec: make([]float32, 768)}

	res, _, _ := reg.handleRegenerateEmbeddings(context.Background(), nil, RegenerateEmbeddingsInput{})
	text := resultText(res)
	if !strings.Contains(text, "Started a backfill") {
		t.Errorf("expected backfill-started message, got: %s", text)
	}
	waitForEmbedJobIdle(t, reg)

	_, msgIndexed, _, _ := reg.Server.EmbeddingCoverage()
	if msgIndexed != 1 {
		t.Errorf("expected the message to be embedded, got msgIndexed=%d", msgIndexed)
	}
}

func TestHandleRegenerateEmbeddingsFullMode(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "embed-room")
	reg.Server.PostMessage("embed-room", "alice", "hello world", "thought", "")
	reg.Server.Embedder = &stubEmbedder{vec: make([]float32, 768)}

	res, _, _ := reg.handleRegenerateEmbeddings(context.Background(), nil, RegenerateEmbeddingsInput{Full: "true"})
	if !strings.Contains(resultText(res), "full re-embed") {
		t.Errorf("expected full-re-embed message, got: %s", resultText(res))
	}
	waitForEmbedJobIdle(t, reg)
}

func TestHandleRegenerateEmbeddingsAlreadyRunning(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.Server.Embedder = &stubEmbedder{vec: make([]float32, 768)}

	atomic.StoreInt32(&reg.Server.EmbedJobRunning, 1)
	defer atomic.StoreInt32(&reg.Server.EmbedJobRunning, 0)

	res, _, _ := reg.handleRegenerateEmbeddings(context.Background(), nil, RegenerateEmbeddingsInput{})
	if !strings.Contains(resultText(res), "already running") {
		t.Errorf("expected 'already running' message, got: %s", resultText(res))
	}
}

func TestUIBackfillEmbeddingsHandlerNoEmbedder(t *testing.T) {
	reg := setupHandlerTest(t)
	handler := reg.UIBackfillEmbeddingsHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/ui/backfill_embeddings", strings.NewReader("{}"))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body uiBackfillEmbeddingsResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error == "" || body.Started {
		t.Fatalf("expected an error and started=false, got: %+v", body)
	}
}

func TestUIBackfillEmbeddingsHandlerStarts(t *testing.T) {
	reg := setupHandlerTest(t)
	mustCreateRoom(t, reg.Server, "embed-room")
	reg.Server.PostMessage("embed-room", "alice", "hello world", "thought", "")
	reg.Server.Embedder = &stubEmbedder{vec: make([]float32, 768)}
	handler := reg.UIBackfillEmbeddingsHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/ui/backfill_embeddings", strings.NewReader(`{"full": false}`))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler(rec, req)

	var body uiBackfillEmbeddingsResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !body.Started || body.Error != "" {
		t.Fatalf("expected started=true and no error, got: %+v", body)
	}
	if body.MsgTotal != 1 || body.MsgIndexed != 0 {
		t.Fatalf("expected a before-snapshot of 0/1 messages, got %d/%d", body.MsgIndexed, body.MsgTotal)
	}
	waitForEmbedJobIdle(t, reg)
}

func TestUIBackfillEmbeddingsHandlerRejectsNonLocalhost(t *testing.T) {
	reg := setupHandlerTest(t)
	reg.Server.Embedder = &stubEmbedder{vec: make([]float32, 768)}
	handler := reg.UIBackfillEmbeddingsHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/ui/backfill_embeddings", strings.NewReader("{}"))
	req.RemoteAddr = "10.0.0.5:12345"
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non-localhost caller, got %d", rec.Code)
	}
}
