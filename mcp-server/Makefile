.PHONY: help build test fmt vet lint clean run all download-model

BINARY = council-hub
BUILD_DIR = build

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the binary
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -tags sqlite_fts5 -o $(BUILD_DIR)/$(BINARY) .
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

test: ## Run tests
	CGO_ENABLED=1 go test -tags sqlite_fts5 -v -count=1 ./...
fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || \
	 (test -x "$$(go env GOPATH)/bin/golangci-lint" && "$$(go env GOPATH)/bin/golangci-lint" run) || \
	 (echo "golangci-lint not found. Install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" && exit 1)

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	go clean

run: build ## Build and run locally
	./$(BUILD_DIR)/$(BINARY)

download-model: ## Download ONNX MiniLM model for local semantic search
	@mkdir -p models/all-MiniLM-L6-v2
	@echo "Downloading all-MiniLM-L6-v2 model (~90MB)..."
	@wget -q --show-progress -O models/all-MiniLM-L6-v2/model.onnx \
		"https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"
	@wget -q --show-progress -O models/all-MiniLM-L6-v2/tokenizer.json \
		"https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"
	@echo "Model downloaded to models/all-MiniLM-L6-v2/"

all: fmt vet lint test build ## Format, vet, lint, test, and build
