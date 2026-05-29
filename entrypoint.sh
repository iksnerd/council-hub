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

# Auto-detect LAN IP when RELEASE_NODE is still the loopback default.
# Uses the default-route interface IP (works on Linux/Docker; skips on stdio mode).
if [ "${RELEASE_NODE}" = "council_hub@127.0.0.1" ]; then
  DETECTED_IP=$(ip route get 1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") {print $(i+1); exit}}')
  if [ -n "$DETECTED_IP" ] && [ "$DETECTED_IP" != "127.0.0.1" ]; then
    export RELEASE_NODE="council_hub@${DETECTED_IP}"
    echo "WARN: RELEASE_NODE not set — auto-detected as ${RELEASE_NODE}"
    echo "      Set RELEASE_NODE explicitly to suppress this warning."
  else
    echo "WARN: RELEASE_NODE is 127.0.0.1 and IP detection failed."
    echo "      Cluster peers will not be able to reach this node."
    echo "      Pass -e RELEASE_NODE=council_hub@<your-lan-ip> to fix."
  fi
fi

echo "=== Council Hub ==="
echo "  MCP server: http://0.0.0.0:${COUNCIL_HTTP_ADDR#:}/mcp"
echo "  Web UI:     http://0.0.0.0:${PORT}"
echo "  Database:   ${COUNCIL_DB}"
echo "  Node:       ${RELEASE_NODE}"
if [ -n "$COUNCIL_SEEDS" ]; then
  echo "  Seeds:      ${COUNCIL_SEEDS}"
fi
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