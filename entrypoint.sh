#!/bin/bash
set -e

# ── Stdio mode: run Go MCP server directly (for CLI agents) ──
if [ "$COUNCIL_TRANSPORT" = "stdio" ]; then
  exec council-hub
fi

# ── HTTP mode: run Go MCP server + Phoenix UI ──

# Generate SECRET_KEY_BASE if not provided
if [ -z "$SECRET_KEY_BASE" ]; then
  export SECRET_KEY_BASE=$(head -c 48 /dev/urandom | base64)
fi

echo "=== Council Hub ==="
echo "  MCP server: http://0.0.0.0:${COUNCIL_HTTP_ADDR#:}/mcp"
echo "  Web UI:     http://0.0.0.0:${PORT}"
echo "  Database:   ${COUNCIL_DB}"
echo "==================="

# Trap signals to clean up both processes
cleanup() {
  echo "Shutting down..."
  kill $MCP_PID $UI_PID 2>/dev/null || true
  wait $MCP_PID $UI_PID 2>/dev/null || true
  exit 0
}
trap cleanup SIGTERM SIGINT

# Start Go MCP server in background
council-hub &
MCP_PID=$!

# Start Phoenix UI in background
/app/ui/bin/council_hub_ui start &
UI_PID=$!

# Wait for both — if either exits, shut down
wait -n $MCP_PID $UI_PID
cleanup