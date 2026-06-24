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

# ── Peer auto-discovery (when COUNCIL_SEEDS is not set) ──
# Scans the local /24 subnet for EPMD (4369) then probes :3001/health to get
# the Erlang node name. Skip with COUNCIL_NO_DISCOVER=1.
if [ -z "$COUNCIL_SEEDS" ] && [ "${COUNCIL_NO_DISCOVER:-0}" != "1" ]; then
  MY_IP="${RELEASE_NODE#*@}"
  if [ "$MY_IP" = "127.0.0.1" ]; then
    echo "Skipping peer discovery: no LAN IP (RELEASE_NODE is 127.0.0.1)"
  else
    SUBNET="${MY_IP%.*}"
    echo "Scanning ${SUBNET}.0/24 for council-hub peers..."
    DISC_TMP=$(mktemp -d)
    for i in $(seq 1 254); do
      ip="${SUBNET}.${i}"
      [ "$ip" = "$MY_IP" ] && continue
      (
        if timeout 0.5 bash -c "echo >/dev/tcp/$ip/4369" 2>/dev/null; then
          result=$(wget --no-verbose --tries=1 --timeout=2 -q -O- "http://$ip:3001/health" 2>/dev/null)
          if [ -n "$result" ]; then
            node=$(echo "$result" | grep -o '"node":"[^"]*"' | head -1 | sed 's/"node":"//;s/"//')
            [ -n "$node" ] && echo "$node" > "$DISC_TMP/$i"
          fi
        fi
      ) &
    done
    wait
    DISCOVERED=""
    for f in "$DISC_TMP"/*; do
      [ -f "$f" ] || continue
      node=$(cat "$f")
      [ -n "$DISCOVERED" ] && DISCOVERED="$DISCOVERED,$node" || DISCOVERED="$node"
    done
    rm -rf "$DISC_TMP"
    if [ -n "$DISCOVERED" ]; then
      export COUNCIL_SEEDS="$DISCOVERED"
      echo "Discovered peers: $COUNCIL_SEEDS"
    else
      echo "No peers found on ${SUBNET}.0/24 (set COUNCIL_SEEDS manually or COUNCIL_NO_DISCOVER=1 to skip)"
    fi
  fi
fi

# ── Resolve bare IPs/hostnames in COUNCIL_SEEDS ──
# Values without @ are probed at :3001/health to resolve the full node@ip name.
# Supports plain IPs (192.168.0.5), MagicDNS names (boyandrenski), FQDNs, etc.
# Values already in node@ip format pass through unchanged.
if [ -n "$COUNCIL_SEEDS" ]; then
  RESOLVED=""
  IFS=',' read -ra SEEDS_ARR <<< "$COUNCIL_SEEDS"
  for seed in "${SEEDS_ARR[@]}"; do
    seed="${seed// /}"
    if echo "$seed" | grep -q '@'; then
      [ -n "$RESOLVED" ] && RESOLVED="$RESOLVED,$seed" || RESOLVED="$seed"
    else
      result=$(wget --no-verbose --tries=1 --timeout=3 -q -O- "http://$seed:3001/health" 2>/dev/null)
      if [ -n "$result" ]; then
        node=$(echo "$result" | grep -o '"node":"[^"]*"' | head -1 | sed 's/"node":"//;s/"//')
        if [ -n "$node" ]; then
          echo "Resolved $seed → $node"
          [ -n "$RESOLVED" ] && RESOLVED="$RESOLVED,$node" || RESOLVED="$node"
        else
          echo "WARN: no node name in response from $seed:3001 — skipping"
        fi
      else
        echo "WARN: $seed:3001/health unreachable — skipping"
      fi
    fi
  done
  export COUNCIL_SEEDS="$RESOLVED"
fi

echo "=== Council Hub ==="
echo "  MCP server: http://0.0.0.0:${COUNCIL_HTTP_ADDR#:}/mcp"
if [ "${COUNCIL_UI:-on}" != "off" ]; then
  echo "  Web UI:     http://0.0.0.0:${PORT}"
fi
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

# ── COUNCIL_UI=off: run the Go MCP server alone (lowest footprint — no BEAM) ──
# Drops the Phoenix dashboard, which is ~90% of the image's resident memory. Local
# reads/writes and cross-node *writes* are unaffected; only cluster_wide *reads*
# (which fan out through Phoenix's internal :erpc API) are unavailable in this mode.
if [ "${COUNCIL_UI:-on}" = "off" ]; then
  echo "  UI:         disabled (COUNCIL_UI=off) — Go MCP server only, no cluster_wide read fan-out"
  wait $MCP_PID
  cleanup
fi

# Trim the BEAM's footprint: cap schedulers and disable scheduler busy-wait so the
# read-only dashboard doesn't hold a scheduler thread per host core or spin the CPU
# at idle. Override by exporting your own ERL_FLAGS.
export ERL_FLAGS="${ERL_FLAGS:-+S 2:2 +SDio 1 +sbwt none +sbwtdcpu none +sbwtdio none}"

# Start Phoenix UI in background
/app/ui/bin/council_hub_ui start &
UI_PID=$!

# Wait for both — if either exits, shut down
wait -n $MCP_PID $UI_PID
cleanup
