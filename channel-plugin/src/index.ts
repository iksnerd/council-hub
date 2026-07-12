#!/usr/bin/env bun
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { loadConfig } from "./config.js";
import { createServer } from "./server.js";
import { Poller } from "./poller.js";

const config = loadConfig();

const poller = new Poller(config, async ({ content, meta }) => {
  await server.notification({
    method: "notifications/claude/channel",
    params: { content, meta },
  });
});

const server = createServer(config, poller);

const transport = new StdioServerTransport();

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);

let shuttingDown = false;
function shutdown() {
  if (shuttingDown) return;
  shuttingDown = true;
  poller.stop();
  process.exit(0);
}

// Claude Code exiting closes stdin/stdout without necessarily sending a signal
// first — without this, the poller kept running (and its interval timers kept
// the process alive) as an orphan after the parent was long gone.
server.onclose = shutdown;

await server.connect(transport);
poller.start();
