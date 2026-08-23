#!/usr/bin/env python3
"""Post a message to a council-hub room over the MCP StreamableHTTP transport.

Used by .githooks/post-commit, and usable by hand.

Why not the simpler /api/ui/post endpoint: it is gated to loopback, and behind
Docker's bridge NAT the container sees the gateway address (172.x) for every
published-port request from the host -- so a host-side caller is always
`forbidden`. That gate is correct and load-bearing (it is that endpoint's only
auth); the caller is what has to change. DOCKERHUB.md documents the same NAT
limitation for the Cluster Settings page.

The MCP endpoint requires a session: initialize -> capture Mcp-Session-Id ->
notifications/initialized -> tools/call. A bare tools/call is rejected with
`method "tools/call" is invalid during session initialization`.
"""

import argparse
import json
import os
import sys
import urllib.error
import urllib.request

ACCEPT = "application/json, text/event-stream"


def rpc(url: str, payload: dict, session: str | None = None, want_headers: bool = False):
    headers = {"Content-Type": "application/json", "Accept": ACCEPT}
    if session:
        headers["Mcp-Session-Id"] = session
    req = urllib.request.Request(
        url, data=json.dumps(payload).encode(), headers=headers, method="POST")
    with urllib.request.urlopen(req, timeout=10) as resp:
        body = resp.read().decode()
        sid = resp.headers.get("Mcp-Session-Id")
    # The transport may answer as SSE ("event: message\ndata: {...}") or plain JSON.
    for line in body.splitlines():
        line = line.strip()
        if line.startswith("data:"):
            body = line[5:].strip()
            break
    parsed = json.loads(body) if body else {}
    return (parsed, sid) if want_headers else parsed


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--room", required=True)
    ap.add_argument("--author", required=True)
    ap.add_argument("--type", default="action")
    ap.add_argument("--url", default=os.environ.get(
        "COUNCIL_MCP_URL", "http://127.0.0.1:3001/mcp"))
    ap.add_argument("--message", help="message body; read from stdin when omitted")
    args = ap.parse_args()

    message = args.message if args.message is not None else sys.stdin.read()
    if not message.strip():
        print("empty message", file=sys.stderr)
        return 2

    init, session = rpc(args.url, {
        "jsonrpc": "2.0", "id": 0, "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "council-hub-git-hook", "version": "1.0.0"},
        },
    }, want_headers=True)
    if init.get("error"):
        print(f"initialize failed: {init['error'].get('message')}", file=sys.stderr)
        return 1
    if not session:
        print("initialize returned no Mcp-Session-Id", file=sys.stderr)
        return 1

    # The server rejects tools/call until this arrives.
    try:
        rpc(args.url, {"jsonrpc": "2.0", "method": "notifications/initialized"}, session)
    except (urllib.error.URLError, json.JSONDecodeError):
        pass  # notification, no response body expected

    out = rpc(args.url, {
        "jsonrpc": "2.0", "id": 1, "method": "tools/call",
        "params": {
            "name": "post_to_room",
            "arguments": {
                "room_id": args.room,
                "author": args.author,
                "message_type": args.type,
                "message": message,
            },
        },
    }, session)

    if out.get("error"):
        print(f"post failed: {out['error'].get('message')}", file=sys.stderr)
        return 1
    text = "".join(c.get("text", "") for c in out.get("result", {}).get("content", []))
    if "Error:" in text:
        print(text.strip().splitlines()[0], file=sys.stderr)
        return 1
    print(text.strip().splitlines()[0] if text.strip() else "posted")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except urllib.error.URLError as e:
        print(f"server unreachable: {e}", file=sys.stderr)
        sys.exit(1)
