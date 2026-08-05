import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import type { Config } from "./config.js";
import type { Poller } from "./poller.js";

// Every tool response below wraps a single text block in this shape.
function textResult(text: string, isError = false) {
  return isError
    ? { content: [{ type: "text" as const, text }], isError: true }
    : { content: [{ type: "text" as const, text }] };
}

export function createServer(config: Config, poller: Poller): Server {
  const server = new Server(
    { name: "council-hub-channel", version: "0.1.0" },
    {
      capabilities: {
        experimental: {
          "claude/channel": {},
        },
        tools: {},
      },
      instructions:
        'New messages from council-hub rooms arrive as <channel source="council-hub" room_id="..." author="..." message_type="..."> notifications. ' +
        "Read them and respond if relevant. Use the council_reply tool to post back to a room.",
    }
  );

  server.setRequestHandler(ListToolsRequestSchema, async () => ({
    tools: [
      {
        name: "watch_room",
        description:
          "Subscribe to a council-hub room. You will receive <channel> notifications for new messages in that room.",
        inputSchema: {
          type: "object" as const,
          properties: {
            room_id: { type: "string", description: "Room ID to watch" },
          },
          required: ["room_id"],
        },
      },
      {
        name: "unwatch_all",
        description: "Unsubscribe from all rooms at once. Use before watch_room to focus on a single room.",
        inputSchema: { type: "object" as const, properties: {} },
      },
      {
        name: "unwatch_room",
        description: "Unsubscribe from a council-hub room. Stops notifications for that room.",
        inputSchema: {
          type: "object" as const,
          properties: {
            room_id: { type: "string", description: "Room ID to stop watching" },
          },
          required: ["room_id"],
        },
      },
      {
        name: "list_watched_rooms",
        description: "List the council-hub rooms currently being watched for notifications.",
        inputSchema: { type: "object" as const, properties: {} },
      },
      {
        name: "council_reply",
        description:
          "Post a message to a council-hub room. Use this to respond to notifications or participate in ongoing discussions.",
        inputSchema: {
          type: "object" as const,
          properties: {
            room_id: {
              type: "string",
              description: "ID of the room to post to",
            },
            content: {
              type: "string",
              description: "Message content",
            },
            message_type: {
              type: "string",
              enum: [
                "message",
                "thought",
                "draft",
                "decision",
                "plan",
                "action",
                "synthesis",
                "review",
                "critique",
                "note",
              ],
              description: "Type of message (default: message)",
            },
            reply_to: {
              type: "string",
              description: "Optional message ID this is replying to",
            },
          },
          required: ["room_id", "content"],
        },
      },
    ],
  }));

  server.setRequestHandler(CallToolRequestSchema, async (req) => {
    const { name } = req.params;

    if (name === "watch_room") {
      const { room_id } = req.params.arguments as { room_id: string };
      return textResult(poller.watchRoom(room_id));
    }

    if (name === "unwatch_all") {
      return textResult(poller.unwatchAll());
    }

    if (name === "unwatch_room") {
      const { room_id } = req.params.arguments as { room_id: string };
      return textResult(poller.unwatchRoom(room_id));
    }

    if (name === "list_watched_rooms") {
      const rooms = poller.listWatched();
      return textResult(rooms.length ? rooms.join("\n") : "Not watching any rooms");
    }

    if (name !== "council_reply") {
      throw new Error(`Unknown tool: ${name}`);
    }

    const args = (req.params.arguments ?? {}) as Record<string, string>;
    const { room_id, content, message_type = "message", reply_to = "" } = args;

    if (!room_id || !content) {
      return textResult("Error: room_id and content are required", true);
    }

    try {
      const result = await postToRoom(config, { room_id, content, message_type, reply_to });
      return textResult(result);
    } catch (err) {
      return textResult(`Error posting to room: ${err}`, true);
    }
  });

  return server;
}

// The Go server speaks MCP over StreamableHTTP, which requires a session before
// any tools/call: initialize -> capture the Mcp-Session-Id header -> send
// notifications/initialized. We cache the session and re-handshake if it's lost
// (e.g. the server restarted). Posting a bare tools/call — as this plugin used
// to — fails with `method "tools/call" is invalid during session initialization`.
let mcpSession: string | null = null;

type RpcResponse = { result?: unknown; error?: { message: string } };

// Parse a StreamableHTTP response, which may be a single JSON object or an SSE
// stream of `data:` lines. Returns the first JSON-RPC payload found.
async function parseRpcResponse(resp: Response): Promise<RpcResponse> {
  const contentType = resp.headers.get("content-type") ?? "";
  if (contentType.includes("text/event-stream")) {
    const text = await resp.text();
    for (const line of text.split("\n")) {
      if (line.startsWith("data: ")) return JSON.parse(line.slice(6)) as RpcResponse;
    }
    return {};
  }
  return (await resp.json()) as RpcResponse;
}

// Establish an MCP session and return its id. Must run before any tools/call.
async function mcpHandshake(config: Config): Promise<string> {
  const resp = await fetch(config.mcpUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json, text/event-stream",
    },
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: 0,
      method: "initialize",
      params: {
        protocolVersion: "2024-11-05",
        capabilities: {},
        clientInfo: { name: "council-hub-channel", version: "0.1.0" },
      },
    }),
  });
  if (!resp.ok) {
    throw new Error(`MCP initialize failed: HTTP ${resp.status}: ${await resp.text()}`);
  }
  const sessionId = resp.headers.get("mcp-session-id");
  await resp.text(); // drain the SSE body so the stream completes
  if (!sessionId) {
    throw new Error("MCP initialize returned no Mcp-Session-Id header");
  }
  // Complete the handshake — the server rejects tools/call until this arrives.
  await fetch(config.mcpUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json, text/event-stream",
      "Mcp-Session-Id": sessionId,
    },
    body: JSON.stringify({ jsonrpc: "2.0", method: "notifications/initialized" }),
  }).catch(() => {});
  return sessionId;
}

async function postToRoom(
  config: Config,
  args: { room_id: string; content: string; message_type: string; reply_to: string }
): Promise<string> {
  const body = {
    jsonrpc: "2.0",
    id: 1,
    method: "tools/call",
    params: {
      name: "post_to_room",
      arguments: {
        room_id: args.room_id,
        author: config.author,
        message: args.content,
        message_type: args.message_type,
        reply_to: args.reply_to || undefined,
      },
    },
  };

  // Two attempts: the second forces a fresh session if the first failed because
  // the cached session was missing or stale (e.g. the server restarted).
  for (let attempt = 0; attempt < 2; attempt++) {
    if (!mcpSession) mcpSession = await mcpHandshake(config);

    const resp = await fetch(config.mcpUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
        "Mcp-Session-Id": mcpSession,
      },
      body: JSON.stringify(body),
    });

    // 404/400 means the session is unknown/expired — drop it and retry once.
    if ((resp.status === 404 || resp.status === 400) && attempt === 0) {
      mcpSession = null;
      continue;
    }
    if (!resp.ok) {
      throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
    }

    const json = await parseRpcResponse(resp);
    if (json.error) {
      const m = json.error.message ?? "";
      // Session lost mid-flight (e.g. server restarted): re-handshake once.
      if (attempt === 0 && /session|initializ/i.test(m)) {
        mcpSession = null;
        continue;
      }
      throw new Error(m);
    }
    return extractText(json.result);
  }
  throw new Error("post_to_room failed after re-establishing the MCP session");
}

function extractText(result: unknown): string {
  if (!result || typeof result !== "object") return "Posted";
  const r = result as { content?: Array<{ type: string; text: string }> };
  return r.content?.[0]?.text ?? "Posted";
}
