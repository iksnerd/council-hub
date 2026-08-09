import { test, expect, beforeEach, afterEach } from "bun:test";
import { loadConfig } from "./config.js";

const ENV_KEYS = [
  "COUNCIL_DB",
  "COUNCIL_POLL_INTERVAL",
  "COUNCIL_ROOMS",
  "COUNCIL_MCP_URL",
  "COUNCIL_AUTHOR",
  "COUNCIL_CHANNEL_DEBUG",
] as const;

let saved: Record<string, string | undefined> = {};

beforeEach(() => {
  saved = {};
  for (const k of ENV_KEYS) {
    saved[k] = process.env[k];
    delete process.env[k];
  }
});

afterEach(() => {
  for (const k of ENV_KEYS) {
    if (saved[k] === undefined) delete process.env[k];
    else process.env[k] = saved[k];
  }
});

test("defaults watch all rooms when COUNCIL_ROOMS is unset", () => {
  expect(loadConfig().rooms).toBe("*");
});

test("COUNCIL_ROOMS set to an empty string falls back to '*' instead of watching nothing", () => {
  // `??` only falls back on undefined, not "" — a template that resolves an
  // env var to blank (e.g. COUNCIL_ROOMS=${VAR} with VAR unset) must not
  // silently disable all room discovery.
  process.env.COUNCIL_ROOMS = "";
  expect(loadConfig().rooms).toBe("*");
});

test("COUNCIL_ROOMS set to whitespace falls back to '*'", () => {
  process.env.COUNCIL_ROOMS = "   ";
  expect(loadConfig().rooms).toBe("*");
});

test("COUNCIL_ROOMS parses a trimmed comma-separated list", () => {
  process.env.COUNCIL_ROOMS = "design-review, impl ,ops";
  expect(loadConfig().rooms).toEqual(["design-review", "impl", "ops"]);
});

test("rejects a poll interval below the floor", () => {
  process.env.COUNCIL_POLL_INTERVAL = "100";
  expect(() => loadConfig()).toThrow(/COUNCIL_POLL_INTERVAL/);
});

test("expands a ~/ COUNCIL_DB path", () => {
  process.env.COUNCIL_DB = "~/custom/council.db";
  expect(loadConfig().dbPath).not.toContain("~");
  expect(loadConfig().dbPath.endsWith("/custom/council.db")).toBe(true);
});
