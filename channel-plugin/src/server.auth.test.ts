import { test, expect, afterEach } from "bun:test";
import type { Config } from "./config.js";
import { postToRoom, resetSessionForTests } from "./server.js";

const TOKEN = "test-token-0123456789abcdef";

// A fake MCP endpoint that speaks just enough StreamableHTTP for postToRoom:
// initialize, notifications/initialized and tools/call. It records the
// Authorization header of every request and answers 401 when it is wrong.
function startFakeHub(requireToken?: string) {
  const seen: Array<string | null> = [];
  const server = Bun.serve({
    port: 0,
    async fetch(req) {
      const authz = req.headers.get("authorization");
      seen.push(authz);
      if (requireToken && authz !== `Bearer ${requireToken}`) {
        return new Response(null, {
          status: 401,
          headers: { "WWW-Authenticate": 'Bearer realm="council-hub"' },
        });
      }
      const body = (await req.json()) as { id?: number; method: string };
      if (body.method === "initialize") {
        return new Response(JSON.stringify({ jsonrpc: "2.0", id: body.id, result: {} }), {
          headers: { "Content-Type": "application/json", "Mcp-Session-Id": "sess-1" },
        });
      }
      if (body.method === "notifications/initialized") {
        return new Response(null, { status: 202 });
      }
      return new Response(
        JSON.stringify({
          jsonrpc: "2.0",
          id: body.id,
          result: { content: [{ type: "text", text: "Posted" }] },
        }),
        { headers: { "Content-Type": "application/json" } }
      );
    },
  });
  return { server, seen, url: `http://127.0.0.1:${server.port}/mcp` };
}

function config(mcpUrl: string, authToken?: string): Config {
  return {
    dbPath: "",
    pollInterval: 3000,
    rooms: "*",
    mcpUrl,
    author: "alice",
    debug: false,
    authToken,
  };
}

const args = { room_id: "r", content: "hi", message_type: "message", reply_to: "" };
let stop: (() => void) | null = null;

afterEach(() => {
  stop?.();
  stop = null;
  resetSessionForTests();
});

test("council_reply sends the bearer token on every MCP request when set", async () => {
  const hub = startFakeHub(TOKEN);
  stop = () => hub.server.stop(true);
  const out = await postToRoom(config(hub.url, TOKEN), args);
  expect(out).toBe("Posted");
  // initialize, notifications/initialized, tools/call
  expect(hub.seen.length).toBe(3);
  for (const h of hub.seen) expect(h).toBe(`Bearer ${TOKEN}`);
});

test("council_reply sends no Authorization header when no token is set", async () => {
  const hub = startFakeHub();
  stop = () => hub.server.stop(true);
  await postToRoom(config(hub.url), args);
  expect(hub.seen.length).toBe(3);
  for (const h of hub.seen) expect(h).toBeNull();
});

test("a wrong token surfaces as an error that does not echo the token", async () => {
  const hub = startFakeHub(TOKEN);
  stop = () => hub.server.stop(true);
  const bad = "wrong-token-0123456789abc";
  let message = "";
  try {
    await postToRoom(config(hub.url, bad), args);
  } catch (err) {
    message = String(err);
  }
  expect(message).toContain("401");
  expect(message).not.toContain(bad);
});
