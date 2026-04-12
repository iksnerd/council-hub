package council

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

// ONNXEmbedder generates embeddings using an all-MiniLM-L6-v2 ONNX model loaded from disk.
// Runs entirely in-process — no external service needed.
// Requires ONNX Runtime shared library and a model directory containing model.onnx + tokenizer.json.
type ONNXEmbedder struct {
	tk      *tokenizer.Tokenizer
	session *ort.DynamicAdvancedSession
}

// NewONNXEmbedder creates an in-process embedder loading the model from modelDir.
// modelDir must contain model.onnx and tokenizer.json (all-MiniLM-L6-v2 from HuggingFace).
func NewONNXEmbedder(modelDir string) (*ONNXEmbedder, error) {
	// Load tokenizer
	tokData, err := os.ReadFile(filepath.Join(modelDir, "tokenizer.json"))
	if err != nil {
		return nil, fmt.Errorf("read tokenizer.json: %w", err)
	}
	tk, err := pretrained.FromReader(bytes.NewReader(tokData))
	if err != nil {
		return nil, fmt.Errorf("parse tokenizer: %w", err)
	}

	// Initialize ONNX Runtime (idempotent per process)
	if libPath, ok := os.LookupEnv("ONNXRUNTIME_LIB_PATH"); ok {
		ort.SetSharedLibraryPath(libPath)
	}
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("init ONNX runtime: %w", err)
	}

	// Load model
	modelData, err := os.ReadFile(filepath.Join(modelDir, "model.onnx"))
	if err != nil {
		return nil, fmt.Errorf("read model.onnx: %w", err)
	}

	inputNames := []string{"input_ids", "attention_mask", "token_type_ids"}
	outputNames := []string{"last_hidden_state"}

	session, err := ort.NewDynamicAdvancedSessionWithONNXData(modelData, inputNames, outputNames, nil)
	if err != nil {
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	return &ONNXEmbedder{tk: tk, session: session}, nil
}

// Embed generates a 384-dimensional embedding for the given text using mean pooling.
func (e *ONNXEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Tokenize
	encoding, err := e.tk.EncodeSingle(text, true)
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}

	seqLen := len(encoding.Ids)

	// Build input tensors
	inputIDs := make([]int64, seqLen)
	attentionMask := make([]int64, seqLen)
	tokenTypeIDs := make([]int64, seqLen)

	for i, id := range encoding.Ids {
		inputIDs[i] = int64(id)
	}
	for i, mask := range encoding.AttentionMask {
		attentionMask[i] = int64(mask)
	}
	for i, tid := range encoding.TypeIds {
		tokenTypeIDs[i] = int64(tid)
	}

	shape := ort.NewShape(1, int64(seqLen))

	inputIDsTensor, err := ort.NewTensor(shape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("input_ids tensor: %w", err)
	}
	defer func() { _ = inputIDsTensor.Destroy() }()

	attMaskTensor, err := ort.NewTensor(shape, attentionMask)
	if err != nil {
		return nil, fmt.Errorf("attention_mask tensor: %w", err)
	}
	defer func() { _ = attMaskTensor.Destroy() }()

	typeTensor, err := ort.NewTensor(shape, tokenTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("token_type_ids tensor: %w", err)
	}
	defer func() { _ = typeTensor.Destroy() }()

	// Output tensor: last_hidden_state has shape [1, seq_len, 384]
	outShape := ort.NewShape(1, int64(seqLen), int64(EmbedDim))
	outTensor, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		return nil, fmt.Errorf("output tensor: %w", err)
	}
	defer func() { _ = outTensor.Destroy() }()

	// Run inference
	err = e.session.Run(
		[]ort.Value{inputIDsTensor, attMaskTensor, typeTensor},
		[]ort.Value{outTensor},
	)
	if err != nil {
		return nil, fmt.Errorf("ONNX run: %w", err)
	}

	// Mean pooling: average token embeddings weighted by attention mask
	hidden := outTensor.GetData() // flat [seq_len * 384]
	return meanPool(hidden, attentionMask, seqLen, EmbedDim), nil
}

// meanPool averages token embeddings weighted by the attention mask.
func meanPool(hidden []float32, mask []int64, seqLen, dim int) []float32 {
	result := make([]float32, dim)
	var maskSum float32
	for t := 0; t < seqLen; t++ {
		m := float32(mask[t])
		maskSum += m
		for d := 0; d < dim; d++ {
			result[d] += hidden[t*dim+d] * m
		}
	}
	if maskSum > 0 {
		for d := range result {
			result[d] /= maskSum
		}
	}
	return result
}

// Close releases ONNX Runtime resources.
func (e *ONNXEmbedder) Close() error {
	if e.session != nil {
		_ = e.session.Destroy()
	}
	return ort.DestroyEnvironment()
}
