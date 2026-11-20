#!/bin/sh
# Claude Code UserPromptSubmit hook wrapper for council-inbox.
# Finds the binary (COUNCIL_INBOX_BIN, else council-inbox on PATH) and runs it
# in hook mode. If the binary is missing, exits 0 silently so a prompt is never
# blocked. See docs/agent-inbox.md.

bin="${COUNCIL_INBOX_BIN:-}"
if [ -z "$bin" ]; then
	bin=$(command -v council-inbox 2>/dev/null) || exit 0
fi
[ -x "$bin" ] || exit 0

"$bin" --hook "$@"
exit 0
