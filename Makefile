.PHONY: help docker-build docker-run docker-stop docker-logs docker-push test-all clean

IMAGE    = iksnerd/council-hub
VERSION ?= latest
DATA_DIR = $(HOME)/.council-hub

# Clustering defaults (override with e.g. make docker-run SEEDS=other@10.0.0.1)
# Or create a Makefile.local with your own values — it is gitignored.
LOCAL_IP  ?= $(shell ipconfig getifaddr en0 2>/dev/null || echo 127.0.0.1)
NODE_NAME ?= council_hub@$(LOCAL_IP)
SEEDS     ?= example@10.0.0.1
COOKIE    ?= council

-include Makefile.local

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

docker-build: ## Build unified Docker image (native arch)
	docker build -t $(IMAGE):latest .
	@echo "Built: $(IMAGE):latest"

docker-run: ## Run council-hub (MCP on :3001, UI on :4000, cluster on :4369/:9000)
	@mkdir -p $(DATA_DIR)
	docker run -d --name council-hub \
		-p 4000:4000 -p 3001:3001 -p 4369:4369 -p 9000:9000 \
		-v $(DATA_DIR):/data \
		-e COUNCIL_TRANSPORT=http \
		-e RELEASE_NODE=$(NODE_NAME) \
		-e COUNCIL_SEEDS=$(SEEDS) \
		-e RELEASE_COOKIE=$(COOKIE) \
		$(IMAGE):latest
	@echo "Council Hub running — UI: http://localhost:4000, MCP: http://localhost:3001/mcp"

docker-stop: ## Stop council-hub container
	docker stop council-hub 2>/dev/null || true
	docker rm council-hub 2>/dev/null || true

docker-logs: ## Tail container logs
	docker logs -f council-hub

docker-push: ## Build and push arm64 image to Docker Hub with correct manifest
	docker buildx build --platform linux/arm64 \
		-t $(IMAGE):latest -t $(IMAGE):$(VERSION) \
		--push .
	@echo "Pushed: $(IMAGE):latest + $(IMAGE):$(VERSION) (arm64)"

test-all: ## Run Go + Elixir tests
	cd mcp-server && make test

clean: ## Remove build artifacts
	cd mcp-server && make clean