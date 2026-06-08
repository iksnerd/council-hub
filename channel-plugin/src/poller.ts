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
  // Single global cursor. Message IDs are UUIDv7, so they sort in time order
  // across every room — one cursor lets us fetch new messages for all watched
  // rooms in a single query and never replay history.
  private cursor = "";
  private cursorSeeded = false;
  private watchedRooms = new Set<string>();
  private manuallyWatched = new Set<string>();
  private manuallyUnwatched = new Set<string>();
  private pollTimer: Timer | null = null;
  private roomRefreshTimer: Timer | null = null;
  private polling = false;

  constructor(
    private config: Config,
    private notify: NotifyFn
  ) {}

  start(): void {
    this.init();
    this.pollTimer = setInterval(() => {
      this.poll().catch((err) => this.log(`Poll cycle error: ${err}`));
    }, this.config.pollInterval);
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
      // The Go server owns the DB and keeps it in WAL mode; a read-only handle
      // can't change journal_mode, so we only set a busy timeout to wait out
      // the writer's locks.
      const db = new Database(this.config.dbPath, { readonly: true });
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
    this.seedCursor();
    this.refreshRooms();
  }

  // Seed the cursor at the newest message so we never replay history on startup.
  private seedCursor(): void {
    if (this.cursorSeeded || !this.db) return;
    try {
      const latest = this.db
        .query<{ id: string }, []>("SELECT id FROM messages ORDER BY id DESC LIMIT 1")
        .get();
      this.cursor = latest?.id ?? "";
      this.cursorSeeded = true;
    } catch (err) {
      this.log(`Cursor seed error: ${err}`);
    }
  }

  private roomExists(id: string): boolean {
    if (!this.db) return false;
    try {
      return (
        this.db
          .query<{ id: string }, [string]>("SELECT id FROM rooms WHERE id = ? LIMIT 1")
          .get(id) !== null
      );
    } catch {
      return false;
    }
  }

  private refreshRooms(): void {
    if (!this.db) {
      this.db = this.openDb();
      if (!this.db) return;
      this.seedCursor();
    }

    let activeIds: Set<string>;
    try {
      const rows = this.db
        .query<{ id: string }, []>("SELECT id FROM rooms WHERE status = 'active'")
        .all();
      activeIds = new Set(rows.map((r) => r.id));
    } catch (err) {
      this.log(`Room refresh error: ${err}`);
      return;
    }

    const eligible = (id: string): boolean =>
      activeIds.has(id) &&
      (this.config.rooms === "*" || this.config.rooms.includes(id)) &&
      !this.manuallyUnwatched.has(id);

    // Add newly-active rooms.
    for (const id of activeIds) {
      if (eligible(id) && !this.watchedRooms.has(id)) {
        this.watchedRooms.add(id);
        this.log(`Watching room: ${id}`);
      }
    }

    // Prune auto-watched rooms that are no longer active or were deleted, so we
    // don't poll dead rooms forever. Rooms added via watch_room are kept.
    for (const id of this.watchedRooms) {
      if (this.manuallyWatched.has(id) || eligible(id)) continue;
      this.watchedRooms.delete(id);
      this.log(`Unwatching inactive/removed room: ${id}`);
    }
  }

  private async poll(): Promise<void> {
    // Skip if the previous tick is still draining — a slow notify shouldn't let
    // ticks overlap and double-deliver.
    if (this.polling) return;
    this.polling = true;
    try {
      if (!this.db) {
        this.db = this.openDb();
        if (!this.db) return;
        this.seedCursor();
        this.refreshRooms();
      }
      if (this.watchedRooms.size === 0) return;

      let rows: Row[];
      try {
        const ids = [...this.watchedRooms];
        const placeholders = ids.map(() => "?").join(",");
        rows = this.db
          .query<Row, string[]>(
            `SELECT id, room_id, author, content, message_type, timestamp
             FROM messages
             WHERE room_id IN (${placeholders}) AND id > ?
             ORDER BY id ASC`
          )
          .all(...ids, this.cursor);
      } catch (err) {
        this.log(`Poll query error: ${err}`);
        // DB may have been locked/rotated — reopen next tick.
        this.db?.close();
        this.db = null;
        return;
      }

      for (const row of rows) {
        // Skip our own messages, but still advance past them.
        if (row.author === this.config.author) {
          this.cursor = row.id;
          continue;
        }

        const content =
          row.content.length > MAX_CONTENT_LENGTH
            ? row.content.slice(0, MAX_CONTENT_LENGTH) +
              "\n...[truncated — use read_room to see full message]"
            : row.content;

        try {
          await this.notify({
            content,
            meta: {
              room_id: row.room_id,
              author: row.author,
              message_type: row.message_type,
              timestamp: row.timestamp,
              message_id: row.id,
            },
          });
        } catch (err) {
          // Delivery failed — leave the cursor put so this message is retried
          // next tick rather than silently dropped, and stop draining to keep
          // ordering.
          this.log(`Notify failed for ${row.id}, will retry: ${err}`);
          break;
        }

        this.cursor = row.id;
      }
    } finally {
      this.polling = false;
    }
  }

  watchRoom(id: string): string {
    if (!this.roomExists(id)) return `Room not found: ${id}`;
    this.manuallyUnwatched.delete(id);
    this.manuallyWatched.add(id);
    if (this.watchedRooms.has(id)) return `Already watching ${id}`;
    this.watchedRooms.add(id);
    this.log(`Manually watching room: ${id}`);
    return `Now watching ${id}`;
  }

  unwatchAll(): string {
    const count = this.watchedRooms.size;
    for (const id of this.watchedRooms) this.manuallyUnwatched.add(id);
    this.watchedRooms.clear();
    this.manuallyWatched.clear();
    return `Stopped watching ${count} room(s)`;
  }

  unwatchRoom(id: string): string {
    if (!this.watchedRooms.has(id)) return `Not currently watching ${id}`;
    this.watchedRooms.delete(id);
    this.manuallyWatched.delete(id);
    this.manuallyUnwatched.add(id);
    this.log(`Manually unwatched room: ${id}`);
    return `Stopped watching ${id}`;
  }

  listWatched(): string[] {
    return [...this.watchedRooms];
  }

  private log(msg: string): void {
    if (this.config.debug) {
      process.stderr.write(`[council-hub-channel] ${msg}\n`);
    }
  }
}
