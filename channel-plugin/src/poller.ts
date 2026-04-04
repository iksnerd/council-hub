import { Database } from "bun:sqlite";
import { existsSync } from "fs";
import type { Config } from "./config.js";

const MAX_CONTENT_LENGTH = 4000;
const ROOM_REFRESH_INTERVAL = 30_000;

interface Row {
  id: string;
  room_id: string;
  author: string;
  content: string;
  message_type: string;
  timestamp: string;
}

export type NotifyFn = (params: {
  content: string;
  meta: Record<string, string>;
}) => Promise<void>;

export class Poller {
  private db: Database | null = null;
  private lastSeenId = new Map<string, string>();
  private watchedRooms = new Set<string>();
  private pollTimer: Timer | null = null;
  private roomRefreshTimer: Timer | null = null;

  constructor(
    private config: Config,
    private notify: NotifyFn
  ) {}

  start(): void {
    this.init();
    this.pollTimer = setInterval(() => this.poll(), this.config.pollInterval);
    this.roomRefreshTimer = setInterval(
      () => this.refreshRooms(),
      Math.max(ROOM_REFRESH_INTERVAL, this.config.pollInterval * 10)
    );
  }

  stop(): void {
    if (this.pollTimer) clearInterval(this.pollTimer);
    if (this.roomRefreshTimer) clearInterval(this.roomRefreshTimer);
    this.db?.close();
    this.db = null;
  }

  private openDb(): Database | null {
    if (!existsSync(this.config.dbPath)) {
      this.log(`DB not found at ${this.config.dbPath}, will retry`);
      return null;
    }
    try {
      const db = new Database(this.config.dbPath, { readonly: true });
      db.exec("PRAGMA journal_mode=WAL");
      db.exec("PRAGMA busy_timeout=5000");
      return db;
    } catch (err) {
      this.log(`Failed to open DB: ${err}`);
      return null;
    }
  }

  private init(): void {
    this.db = this.openDb();
    if (!this.db) return;
    this.refreshRooms();
  }

  private refreshRooms(): void {
    if (!this.db) {
      this.db = this.openDb();
      if (!this.db) return;
    }

    try {
      const rows = this.db
        .query<{ id: string }, []>("SELECT id FROM rooms WHERE status = 'active'")
        .all();

      for (const { id } of rows) {
        if (this.config.rooms !== "*" && !this.config.rooms.includes(id)) continue;

        if (!this.watchedRooms.has(id)) {
          this.watchedRooms.add(id);
          // Seed with latest message ID so we don't replay history
          const latest = this.db
            .query<{ id: string }, [string]>(
              "SELECT id FROM messages WHERE room_id = ? ORDER BY id DESC LIMIT 1"
            )
            .get(id);
          this.lastSeenId.set(id, latest?.id ?? "");
          this.log(`Watching room: ${id} (from id: ${latest?.id ?? "start"})`);
        }
      }
    } catch (err) {
      this.log(`Room refresh error: ${err}`);
    }
  }

  private poll(): void {
    if (!this.db) {
      this.db = this.openDb();
      if (!this.db) return;
      this.refreshRooms();
    }

    for (const roomId of this.watchedRooms) {
      try {
        const lastId = this.lastSeenId.get(roomId) ?? "";
        const rows = this.db
          .query<Row, [string, string]>(
            `SELECT id, room_id, author, content, message_type, timestamp
             FROM messages
             WHERE room_id = ? AND id > ?
             ORDER BY id ASC`
          )
          .all(roomId, lastId);

        for (const row of rows) {
          // Skip self-echo
          if (row.author === this.config.author) {
            this.lastSeenId.set(roomId, row.id);
            continue;
          }

          const content =
            row.content.length > MAX_CONTENT_LENGTH
              ? row.content.slice(0, MAX_CONTENT_LENGTH) +
                "\n...[truncated — use read_room to see full message]"
              : row.content;

          this.notify({
            content,
            meta: {
              room_id: row.room_id,
              author: row.author,
              message_type: row.message_type,
              timestamp: row.timestamp,
              message_id: row.id,
            },
          }).catch((err) => this.log(`Notify error: ${err}`));

          this.lastSeenId.set(roomId, row.id);
        }
      } catch (err) {
        this.log(`Poll error for room ${roomId}: ${err}`);
        // DB may have been locked/rotated — reopen next tick
        this.db?.close();
        this.db = null;
        break;
      }
    }
  }

  private log(msg: string): void {
    if (this.config.debug) {
      process.stderr.write(`[council-hub-channel] ${msg}\n`);
    }
  }
}
