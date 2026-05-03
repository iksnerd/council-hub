# Contributing to Council Hub

First off, thank you for considering contributing to Council Hub! It's people like you that make open-source software such a great community to learn, inspire, and create.

## Architecture Overview

Council Hub is split into two main components:
1. **Go MCP Server (`/mcp-server`)**: Handles the SQLite database, implements the Model Context Protocol (MCP) for AI agent tools, and manages the Erlang distribution logic for multi-node clusters.
2. **Elixir Phoenix UI (`/ui`)**: Provides the real-time LiveView dashboard. It connects to the Go server's cluster and queries the local SQLite database.

## Local Development Setup

### Requirements
- **Go** 1.25+
- **Elixir** 1.15+ (with Erlang/OTP 26+)
- **Docker** (optional, for running the full distributed stack)

### Quick Start

```bash
# Go MCP Server
cd mcp-server && make all          # fmt + vet + test + build

# Phoenix UI
cd ui && mix setup                 # deps + db + assets
cd ui && mix phx.server            # dev server on :4000
```

For the complete dev workflow — build/test commands, single-test syntax, release flow, environment variables, and architecture deep-dive — see **[CLAUDE.md](CLAUDE.md)**.

## Pull Request Process

1. **Ensure Tests Pass:** All code changes must be verified. Run `go test ./...` inside `mcp-server` and `mix test` inside `ui`.
2. **Lint and Format:** 
   - For Go: Run `golangci-lint run` in the `mcp-server` directory.
   - For Elixir: Run `mix format` in the `ui` directory.
3. **Draft a clear PR description:** Summarize your changes and any context for the reviewers.
4. **CI/CD Checks:** GitHub Actions will automatically run the test suites for both Go and Elixir when you open your PR. Please ensure they pass!

## Code of Conduct

Please be respectful and considerate of others when interacting in the issue tracker or pull requests. 
