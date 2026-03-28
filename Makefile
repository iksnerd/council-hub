.PHONY: help docker-build docker-run docker-stop docker-logs test-all clean

IMAGE    = council-hub
DATA_DIR = $(HOME)/Documents/council-hub

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

docker-build: ## Build unified Docker image
	docker build -t $(IMAGE):latest .
	@echo "Built: $(IMAGE):latest"

docker-run: ## Run council-hub (MCP on :3001, UI on :4000)
	@mkdir -p $(DATA_DIR)
	docker run -d --name $(IMAGE) \
		-p 4000:4000 -p 3001:3001 \
		-v $(DATA_DIR):/data \
		$(IMAGE):latest
	@echo "Council Hub running — UI: http://localhost:4000, MCP: http://localhost:3001/mcp"

docker-stop: ## Stop council-hub container
	docker stop $(IMAGE) 2>/dev/null || true
	docker rm $(IMAGE) 2>/dev/null || true

docker-logs: ## Tail container logs
	docker logs -f $(IMAGE)

test-all: ## Run Go + Elixir tests
	cd mcp-server && make test

clean: ## Remove build artifacts
	cd mcp-server && make clean