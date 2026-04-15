package council

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewOllamaEmbedderDefaultModel(t *testing.T) {
	e := NewOllamaEmbedder("http://localhost:11434", "")
	if e.Model != "embeddinggemma:300m" {
		t.Errorf("expected default model 'embeddinggemma:300m', got: %s", e.Model)
	}
}

func TestNewOllamaEmbedderCustomModel(t *testing.T) {
	e := NewOllamaEmbedder("http://localhost:11434", "mxbai-embed-large")
	if e.Model != "mxbai-embed-large" {
		t.Errorf("expected custom model, got: %s", e.Model)
	}
}

func makeFloat64Vec(dim int, seed float64) []float64 {
	v := make([]float64, dim)
	for i := range v {
		v[i] = seed + float64(i)*0.001
	}
	return v
}

func ollamaServer(t *testing.T, vec []float64, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			w.Write([]byte(body)) //nolint:errcheck
			return
		}
		resp := ollamaEmbedResponse{Embeddings: [][]float64{vec}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
}

func TestOllamaEmbedderSuccess(t *testing.T) {
	vec := makeFloat64Vec(EmbedDim, 0.5)
	srv := ollamaServer(t, vec, http.StatusOK, "")
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-model")
	result, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(result) != EmbedDim {
		t.Errorf("expected %d dimensions, got %d", EmbedDim, len(result))
	}
	if result[0] != float32(vec[0]) {
		t.Errorf("expected first element %v, got %v", float32(vec[0]), result[0])
	}
}

func TestOllamaEmbedderNon200Status(t *testing.T) {
	srv := ollamaServer(t, nil, http.StatusInternalServerError, "model not found")
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-model")
	_, err := e.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got: %v", err)
	}
}

func TestOllamaEmbedderMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json")) //nolint:errcheck
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-model")
	_, err := e.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

func TestOllamaEmbedderEmptyEmbeddings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaEmbedResponse{Embeddings: [][]float64{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-model")
	_, err := e.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for empty embeddings")
	}
	if !strings.Contains(err.Error(), "no embeddings") {
		t.Errorf("expected 'no embeddings' error, got: %v", err)
	}
}

func TestOllamaEmbedderSmallerThanDim(t *testing.T) {
	// Raw vector shorter than EmbedDim — should use available length
	smallVec := makeFloat64Vec(100, 1.0)
	srv := ollamaServer(t, smallVec, http.StatusOK, "")
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-model")
	result, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(result) != 100 {
		t.Errorf("expected 100 dimensions (raw length), got %d", len(result))
	}
}

func TestOllamaEmbedderLargerThanDim(t *testing.T) {
	// Raw vector longer than EmbedDim — should truncate to EmbedDim
	largeVec := makeFloat64Vec(EmbedDim*2, 2.0)
	srv := ollamaServer(t, largeVec, http.StatusOK, "")
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-model")
	result, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(result) != EmbedDim {
		t.Errorf("expected truncation to %d, got %d", EmbedDim, len(result))
	}
}

func TestOllamaEmbedderNetworkError(t *testing.T) {
	// Use an address with no server
	e := NewOllamaEmbedder("http://127.0.0.1:19999", "test-model")
	_, err := e.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestOllamaEmbedderContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until client disconnects
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	e := NewOllamaEmbedder(srv.URL, "test-model")
	_, err := e.Embed(ctx, "hello")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
