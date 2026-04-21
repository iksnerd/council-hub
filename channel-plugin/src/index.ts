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

function shutdown() {
  poller.stop();
  process.exit(0);
}

await server.connect(transport);
poller.start();
