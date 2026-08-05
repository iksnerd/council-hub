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

// Mirrors the Row fields above — shared by every message query in fetchRows.
const MESSAGE_COLUMNS = "id, room_id, author, content, message_type, timestamp";

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
  // Set once, at startup seeding, and never moved again — the absolute floor
  // for this session (never replay anything older than this).
  private sessionFloor = "";
  // Rooms that started being watched *after* the shared cursor had already
  // advanced past messages they posted in the gap: room X posts at T0, room Y
  // (already watched) posts at T1 > T0 and pushes `cursor` past T0 before X is
  // even discovered (the room-refresh cadence lags poll by up to 30s), so the
  // shared-cursor query `id > cursor` would skip X's T0 message forever. Such a
  // room gets its own floor (starting at sessionFloor) instead of the shared
  // cursor until its backlog catches up to where the shared cursor already was
  // — then it's promoted into the ordinary shared-cursor path.
  private catchupFloor = new Map<string, string>();
  // Delivered-through positions for rooms that have left the watch set
  // (pruned or manually unwatched). A room REdiscovered later re-enters
  // catch-up from this floor instead of sessionFloor: everything at or below
  // it was already delivered this session, so re-seeding at sessionFloor
  // would replay it as duplicate notifications. A room with no entry here was
  // never watched this session — it seeds at sessionFloor, because a
  // genuinely new room may hold messages older than the shared cursor that
  // were never delivered (the leapfrog case catch-up mode exists for).
  private lastFloor = new Map<string, string>();
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
      this.poll().catch((err) => this.logError(`Poll cycle error: ${err}`));
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
      this.logError(`DB not found at ${this.config.dbPath}, will retry`);
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
      this.logError(`Failed to open DB: ${err}`);
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
      this.sessionFloor = this.cursor;
      this.cursorSeeded = true;
    } catch (err) {
      this.logError(`Cursor seed error: ${err}`);
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
      this.logError(`Room refresh error: ${err}`);
      return;
    }

    const eligible = (id: string): boolean =>
      activeIds.has(id) &&
      (this.config.rooms === "*" || this.config.rooms.includes(id)) &&
      !this.manuallyUnwatched.has(id);

    // Add newly-active rooms.
    for (const id of activeIds) {
      if (eligible(id) && !this.watchedRooms.has(id)) {
        this.startWatching(id);
        this.log(`Watching room: ${id}`);
      }
    }

    // Prune auto-watched rooms that are no longer active or were deleted, so we
    // don't poll dead rooms forever. Rooms added via watch_room are kept.
    for (const id of this.watchedRooms) {
      if (this.manuallyWatched.has(id) || eligible(id)) continue;
      this.stopWatching(id);
      this.log(`Unwatching inactive/removed room: ${id}`);
    }
  }

  // Enter catch-up mode for a newly-(re)watched room. First discovery seeds at
  // sessionFloor (the room's backlog may predate the shared cursor); a
  // rediscovered room resumes from where delivery previously stopped, so
  // already-delivered history isn't replayed. See the lastFloor field comment.
  private seedCatchup(id: string): void {
    this.catchupFloor.set(id, this.lastFloor.get(id) ?? this.sessionFloor);
  }

  // Record how far delivery got in a room leaving the watch set, so a later
  // re-watch resumes there. A catch-up room is delivered through its own
  // floor; an established room through the shared cursor (the cursor never
  // advances past an undelivered message, so everything at or below it in an
  // established room has been delivered).
  private rememberFloor(id: string): void {
    this.lastFloor.set(id, this.catchupFloor.get(id) ?? this.cursor);
  }

  // Begins tracking a newly-(re)watched room: adds it to the watch set and
  // enters catch-up mode so its pre-discovery backlog isn't lost.
  private startWatching(id: string): void {
    this.watchedRooms.add(id);
    this.seedCatchup(id);
  }

  // Stops tracking a room leaving the watch set: remembers how far delivery
  // got (for a clean resume on rediscovery) and drops it from both the watch
  // set and any in-flight catch-up state.
  private stopWatching(id: string): void {
    this.rememberFloor(id);
    this.watchedRooms.delete(id);
    this.catchupFloor.delete(id);
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
      if (!this.cursorSeeded) {
        this.seedCursor();
        if (!this.cursorSeeded) return;
      }
      if (this.watchedRooms.size === 0) return;

      // Snapshot before this tick's established query runs (it isn't mutated
      // until rows are processed below) — used to decide which catchup rooms
      // have fully closed their gap by the end of this tick.
      const cursorSnapshot = this.cursor;
      const ids = [...this.watchedRooms];
      const establishedIds = ids.filter((id) => !this.catchupFloor.has(id));
      const catchupIds = ids.filter((id) => this.catchupFloor.has(id));

      let rows: Row[];
      try {
        rows = this.fetchRows(establishedIds, catchupIds);
      } catch (err) {
        this.logError(`Poll query error: ${err}`);
        // DB may have been locked/rotated — reopen next tick.
        this.db?.close();
        this.db = null;
        return;
      }

      for (const row of rows) {
        // Skip our own messages, but still advance past them.
        if (row.author === this.config.author) {
          this.advanceCursor(row);
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
          this.logError(`Notify failed for ${row.id}, will retry: ${err}`);
          break;
        }

        this.advanceCursor(row);
      }

      // End-of-tick promotion. Only rooms whose catch-up query actually ran
      // this tick (the catchupIds snapshot) are candidates — a room added to
      // catchupFloor mid-tick (refreshRooms timer or watch_room firing while
      // a notify await was pending) hasn't been queried yet, and promoting it
      // here would skip its backlog forever.
      const roomsWithRows = new Set(rows.map((r) => r.room_id));
      for (const roomId of catchupIds) {
        const floor = this.catchupFloor.get(roomId);
        if (floor === undefined) continue; // unwatched mid-tick
        // An empty catch-up result proves the room has nothing after its
        // floor, so its gap up to this tick's starting cursor is closed even
        // though no row advanced the floor — without this, a quiet room would
        // stay in catch-up mode issuing its own query every tick forever.
        if (!roomsWithRows.has(roomId) && cursorSnapshot > floor) {
          this.catchupFloor.set(roomId, cursorSnapshot);
        }
        // A floor that has reached this tick's starting cursor means the gap
        // is fully closed — anything beyond that point is already reflected
        // in the shared cursor via advanceCursor, so the room merges into the
        // ordinary shared-cursor path with no risk of re-delivery.
        if (this.catchupFloor.get(roomId)! >= cursorSnapshot) {
          this.catchupFloor.delete(roomId);
        }
      }
    } finally {
      this.polling = false;
    }
  }

  // Every row (established or catchup) advances the shared cursor if it's the
  // newest seen so far, and advances its room's individual catchup floor if it
  // has one — see the catchupFloor field comment for why both are needed.
  private advanceCursor(row: Row): void {
    if (row.id > this.cursor) this.cursor = row.id;
    if (this.catchupFloor.has(row.room_id)) this.catchupFloor.set(row.room_id, row.id);
  }

  private fetchRows(establishedIds: string[], catchupIds: string[]): Row[] {
    if (!this.db) return [];
    const rows: Row[] = [];

    if (establishedIds.length > 0) {
      const placeholders = establishedIds.map(() => "?").join(",");
      rows.push(
        ...this.db
          .query<Row, string[]>(
            `SELECT ${MESSAGE_COLUMNS}
             FROM messages
             WHERE room_id IN (${placeholders}) AND id > ?
             ORDER BY id ASC`
          )
          .all(...establishedIds, this.cursor)
      );
    }

    // Each catchup room has its own floor (potentially well behind the shared
    // cursor), so it's queried individually rather than sharing one IN(...)
    // clause with a single lower bound.
    for (const roomId of catchupIds) {
      const floor = this.catchupFloor.get(roomId) ?? this.sessionFloor;
      rows.push(
        ...this.db
          .query<Row, [string, string]>(
            `SELECT ${MESSAGE_COLUMNS}
             FROM messages
             WHERE room_id = ? AND id > ?
             ORDER BY id ASC`
          )
          .all(roomId, floor)
      );
    }

    return rows.sort((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
  }

  watchRoom(id: string): string {
    if (!this.roomExists(id)) return `Room not found: ${id}`;
    this.manuallyUnwatched.delete(id);
    this.manuallyWatched.add(id);
    if (this.watchedRooms.has(id)) return `Already watching ${id}`;
    this.startWatching(id);
    this.log(`Manually watching room: ${id}`);
    return `Now watching ${id}`;
  }

  unwatchAll(): string {
    const count = this.watchedRooms.size;
    for (const id of this.watchedRooms) {
      this.manuallyUnwatched.add(id);
      this.stopWatching(id);
    }
    this.manuallyWatched.clear();
    return `Stopped watching ${count} room(s)`;
  }

  unwatchRoom(id: string): string {
    if (!this.watchedRooms.has(id)) return `Not currently watching ${id}`;
    this.stopWatching(id);
    this.manuallyWatched.delete(id);
    this.manuallyUnwatched.add(id);
    this.log(`Manually unwatched room: ${id}`);
    return `Stopped watching ${id}`;
  }

  listWatched(): string[] {
    return [...this.watchedRooms];
  }

  // Genuine failures (bad DB path, query errors, dropped notifications) must be
  // visible without COUNCIL_CHANNEL_DEBUG=1 — a wrong config previously looked
  // identical to "nothing happening" with no way to tell why. Routine chatter
  // (watch/unwatch bookkeeping) stays behind the debug flag via log().
  private logError(msg: string): void {
    process.stderr.write(`[council-hub-channel] ${msg}\n`);
  }

  private log(msg: string): void {
    if (this.config.debug) {
      process.stderr.write(`[council-hub-channel] ${msg}\n`);
    }
  }
}
