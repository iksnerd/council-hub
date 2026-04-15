package council

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// EmbedDim is the standardized embedding dimension (embeddinggemma:300m native output).
const EmbedDim = 768

// OllamaEmbedder generates embeddings via an Ollama HTTP API.
type OllamaEmbedder struct {
	BaseURL string
	Model   string
	Client  *http.Client
	Logger  *slog.Logger
}

// NewOllamaEmbedder creates an embedder that calls the Ollama /api/embed endpoint.
func NewOllamaEmbedder(baseURL, model string, logger *slog.Logger) *OllamaEmbedder {
	if model == "" {
		model = "embeddinggemma:300m"
	}
	return &OllamaEmbedder{
		BaseURL: baseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 2 * time.Minute},
		Logger:  logger,
	}
}

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(ollamaEmbedRequest{
		Model: o.Model,
		Input: text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := o.Client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("embed request cancelled: %w", ctx.Err())
		}
		if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "Timeout") {
			o.Logger.Warn("Ollama embed timed out — model may still be loading into memory",
				"model", o.Model, "timeout", o.Client.Timeout)
			return nil, fmt.Errorf("ollama timed out (model %q may still be loading — retry in a moment)", o.Model)
		}
		o.Logger.Warn("Ollama embed request failed", "model", o.Model, "error", err)
		return nil, fmt.Errorf("embed request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if elapsed > 5*time.Second {
		o.Logger.Info("Ollama embed slow — model was likely loading", "model", o.Model, "elapsed", elapsed.Round(time.Millisecond))
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		o.Logger.Warn("Ollama returned error", "model", o.Model, "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}

	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Convert float64 to float32 and truncate to EmbedDim (Matryoshka)
	raw := result.Embeddings[0]
	dim := EmbedDim
	if len(raw) < dim {
		dim = len(raw)
	}
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		vec[i] = float32(raw[i])
	}

	return vec, nil
}
