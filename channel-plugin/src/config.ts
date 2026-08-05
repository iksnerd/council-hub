import { homedir } from "os";
import { join } from "path";

export interface Config {
  dbPath: string;
  pollInterval: number;
  rooms: string[] | "*";
  mcpUrl: string;
  author: string;
  debug: boolean;
}

function expandHome(p: string): string {
  if (p.startsWith("~/")) return join(homedir(), p.slice(2));
  return p;
}

export function loadConfig(): Config {
  const dbPath = expandHome(
    process.env.COUNCIL_DB ?? "~/.council-hub/council.db"
  );

  const pollInterval = parseInt(process.env.COUNCIL_POLL_INTERVAL ?? "3000", 10);
  if (isNaN(pollInterval) || pollInterval < 500) {
    throw new Error("COUNCIL_POLL_INTERVAL must be a number >= 500");
  }

  const roomsEnv = (process.env.COUNCIL_ROOMS ?? "*").trim();
  const rooms: string[] | "*" =
    roomsEnv === "*" ? "*" : roomsEnv.split(",").map((r) => r.trim()).filter(Boolean);

  const mcpUrl = process.env.COUNCIL_MCP_URL ?? "http://localhost:3001/mcp";
  const author = process.env.COUNCIL_AUTHOR ?? "claude-code";
  const debug = process.env.COUNCIL_CHANNEL_DEBUG === "1";

  return { dbPath, pollInterval, rooms, mcpUrl, author, debug };
}
