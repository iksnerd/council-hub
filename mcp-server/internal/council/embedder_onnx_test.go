package council

import (
	"context"
	"math"
	"os"
	"runtime"
	"testing"
)

// onnxRuntimeAvailable checks if the ONNX Runtime shared library is findable.
func onnxRuntimeAvailable() bool {
	if p, ok := os.LookupEnv("ONNXRUNTIME_LIB_PATH"); ok && p != "" {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/opt/homebrew/lib/libonnxruntime.dylib",
			"/usr/local/lib/libonnxruntime.dylib",
		}
	case "linux":
		candidates = []string{
			"/usr/lib/libonnxruntime.so",
			"/usr/lib/x86_64-linux-gnu/libonnxruntime.so",
			"/usr/lib/aarch64-linux-gnu/libonnxruntime.so",
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// testModelDir returns the model directory for ONNX tests, or skips the test.
func testModelDir(t *testing.T) string {
	t.Helper()

	dir := os.Getenv("COUNCIL_ONNX_MODEL_DIR")
	if dir == "" {
		dir = "../../../models/all-MiniLM-L6-v2"
	}

	// Check model files exist
	for _, f := range []string{"model.onnx", "tokenizer.json"} {
		if _, err := os.Stat(dir + "/" + f); err != nil {
			t.Skipf("ONNX model not available (%s/%s) — download with: make download-model", dir, f)
		}
	}

	// Check ONNX Runtime is available
	if !onnxRuntimeAvailable() {
		t.Skip("ONNX Runtime not available — set ONNXRUNTIME_LIB_PATH or install libonnxruntime")
	}

	return dir
}

func TestONNXEmbedderInterface(t *testing.T) {
	// Compile-time check: ONNXEmbedder implements Embedder
	var _ Embedder = (*ONNXEmbedder)(nil)
}

func TestNewONNXEmbedder(t *testing.T) {
	dir := testModelDir(t)

	e, err := NewONNXEmbedder(dir)
	if err != nil {
		t.Fatalf("NewONNXEmbedder: %v", err)
	}
	defer e.Close()
}

func TestNewONNXEmbedderMissingDir(t *testing.T) {
	_, err := NewONNXEmbedder("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing model directory")
	}
}

func TestONNXEmbedderEmbed(t *testing.T) {
	dir := testModelDir(t)

	e, err := NewONNXEmbedder(dir)
	if err != nil {
		t.Fatalf("NewONNXEmbedder: %v", err)
	}
	defer e.Close()

	vec, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != EmbedDim {
		t.Errorf("expected %d dimensions, got %d", EmbedDim, len(vec))
	}

	// Verify non-zero output
	allZero := true
	for _, v := range vec {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("embedding is all zeros")
	}
}

func TestONNXEmbedderSimilarity(t *testing.T) {
	dir := testModelDir(t)

	e, err := NewONNXEmbedder(dir)
	if err != nil {
		t.Fatalf("NewONNXEmbedder: %v", err)
	}
	defer e.Close()

	ctx := context.Background()

	vecA, err := e.Embed(ctx, "the cat sat on the mat")
	if err != nil {
		t.Fatalf("Embed A: %v", err)
	}
	vecB, err := e.Embed(ctx, "a kitten was sitting on a rug")
	if err != nil {
		t.Fatalf("Embed B: %v", err)
	}
	vecC, err := e.Embed(ctx, "quantum chromodynamics governs strong nuclear interactions")
	if err != nil {
		t.Fatalf("Embed C: %v", err)
	}

	// Similar sentences should have higher cosine similarity than dissimilar ones
	simAB := cosine(vecA, vecB)
	simAC := cosine(vecA, vecC)

	if simAB <= simAC {
		t.Errorf("expected sim(cat/kitten) > sim(cat/quantum): %.4f <= %.4f", simAB, simAC)
	}
}

func TestONNXEmbedderEmptyString(t *testing.T) {
	dir := testModelDir(t)

	e, err := NewONNXEmbedder(dir)
	if err != nil {
		t.Fatalf("NewONNXEmbedder: %v", err)
	}
	defer e.Close()

	vec, err := e.Embed(context.Background(), "")
	if err != nil {
		t.Fatalf("Embed empty: %v", err)
	}
	if len(vec) != EmbedDim {
		t.Errorf("expected %d dimensions for empty string, got %d", EmbedDim, len(vec))
	}
}

// cosine computes cosine similarity between two float32 vectors.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
