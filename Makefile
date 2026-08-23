.PHONY: help docker-build docker-run docker-stop docker-logs docker-push test-all clean

IMAGE    = iksnerd/council-hub
VERSION ?= latest
# Platforms for docker-push. arm64 only by default: the amd64 leg cannot build
# on an Apple Silicon Mac (the BEAM dies under QEMU — see CLAUDE.md), so listing
# it here only buys a long build that fails at the end. Override deliberately:
#   make docker-push PLATFORMS=linux/amd64,linux/arm64   (needs a native amd64 builder)
PLATFORMS ?= linux/arm64
DATA_DIR = $(HOME)/.council-hub

# Clustering defaults (override with e.g. make docker-run SEEDS=other@10.0.0.1)
# Or create a Makefile.local with your own values — it is gitignored.
LOCAL_IP  ?= $(shell ipconfig getifaddr en0 2>/dev/null || echo 127.0.0.1)
NODE_NAME ?= council_hub@$(LOCAL_IP)
SEEDS     ?= example@10.0.0.1
COOKIE    ?= council

-include Makefile.local

help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

docker-build: ## Build unified Docker image (native arch)
	docker build -t $(IMAGE):latest .
	@echo "Built: $(IMAGE):latest"

docker-run: ## Run council-hub (MCP on :3001, UI on :4000, cluster on :4369/:9000)
# Cluster ports are published to $(LOCAL_IP) only, not 0.0.0.0: Erlang distribution
# grants code execution to anyone holding the cookie, so it has no business listening
# on a VPN or guest interface. Peers reach the LAN address anyway, so nothing is lost.
	@mkdir -p $(DATA_DIR)
	docker run -d --name council-hub --restart always \
		-p 4000:4000 -p 3001:3001 \
		-p $(LOCAL_IP):4369:4369 -p $(LOCAL_IP):9000:9000 \
		-v $(DATA_DIR):/data \
		-e COUNCIL_TRANSPORT=http \
		-e COUNCIL_OLLAMA_URL=http://host.docker.internal:11434 \
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

docker-push: ## Build and push image to Docker Hub (VERSION=vX.Y.Z; arm64 only, see PLATFORMS)
	docker buildx build --platform $(PLATFORMS) \
		-t $(IMAGE):latest -t $(IMAGE):$(VERSION) \
		--push .
	@echo "Pushed: $(IMAGE):latest + $(IMAGE):$(VERSION) ($(PLATFORMS))"
	@echo "NOTE: this overwrote :latest. On $(PLATFORMS) alone, x86 users cannot run it."

ledger-check: ## List commits since the last tag with no council-hub ledger entry (exits 1 if any — it is a gate)
	@python3 scripts/ledger-check.py $(if $(SINCE),--since $(SINCE))

install-hooks: ## Enable the Council-Room commit trailer (per-clone, opt-in)
	git config core.hooksPath .githooks
	@echo "Hooks enabled. Add 'Council-Room: <room-id>' to a commit message to log it."
	@echo "Disable with: git config --unset core.hooksPath"

test-all: ## Run Go + Elixir tests
	cd mcp-server && make test

clean: ## Remove build artifacts
	cd mcp-server && make clean