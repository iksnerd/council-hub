import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import type { Config } from "./config.js";

export function createServer(config: Config): Server {
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
                "decision",
                "code",
                "review",
                "action",
                "critique",
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
    if (req.params.name !== "council_reply") {
      throw new Error(`Unknown tool: ${req.params.name}`);
    }

    const args = req.params.arguments as Record<string, string>;
    const { room_id, content, message_type = "message", reply_to = "" } = args;

    if (!room_id || !content) {
      return {
        content: [{ type: "text", text: "Error: room_id and content are required" }],
        isError: true,
      };
    }

    try {
      const result = await postToRoom(config, { room_id, content, message_type, reply_to });
      return { content: [{ type: "text", text: result }] };
    } catch (err) {
      return {
        content: [{ type: "text", text: `Error posting to room: ${err}` }],
        isError: true,
      };
    }
  });

  return server;
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

  const resp = await fetch(config.mcpUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json, text/event-stream",
    },
    body: JSON.stringify(body),
  });

  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
  }

  const contentType = resp.headers.get("content-type") ?? "";

  // Handle SSE response (stream of events)
  if (contentType.includes("text/event-stream")) {
    const text = await resp.text();
    // Extract data lines from SSE
    for (const line of text.split("\n")) {
      if (line.startsWith("data: ")) {
        const json = JSON.parse(line.slice(6));
        if (json.result) return extractText(json.result);
        if (json.error) throw new Error(json.error.message);
      }
    }
    return "Posted";
  }

  const json = await resp.json() as { result?: unknown; error?: { message: string } };
  if (json.error) throw new Error(json.error.message);
  return extractText(json.result);
}

function extractText(result: unknown): string {
  if (!result || typeof result !== "object") return "Posted";
  const r = result as { content?: Array<{ type: string; text: string }> };
  return r.content?.[0]?.text ?? "Posted";
}
