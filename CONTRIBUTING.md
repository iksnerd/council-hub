# Contributing to Council Hub

Thanks for contributing to Council Hub. This guide covers local setup, testing, and the pull-request process.

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

1. **Run the tests.** `go test ./...` inside `mcp-server` and `mix test` inside `ui`. The full Go and Elixir suites only run in CI on release tags (to conserve Actions minutes), so run them locally before submitting.
2. **Lint and format.**
   - Go: `golangci-lint run` in `mcp-server`.
   - Elixir: `mix format` in `ui`.
3. **Write a clear PR description.** Summarize the change and any context a reviewer needs.
4. **Secret scan.** Opening a PR triggers a gitleaks scan; it must pass before merge.

## Code of Conduct

Be respectful and considerate in the issue tracker and pull requests. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
