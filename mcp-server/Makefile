.PHONY: help build test fmt vet lint clean run all

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

all: fmt vet lint test build ## Format, vet, lint, test, and build
