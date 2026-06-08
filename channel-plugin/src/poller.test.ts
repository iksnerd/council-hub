import { test, expect, afterEach } from "bun:test";
import { Database } from "bun:sqlite";
import { rmSync } from "fs";
import { tmpdir } from "os";
import { join } from "path";
import { Poller } from "./poller.js";
import type { Config } from "./config.js";

// Each test gets a throwaway SQLite file mirroring the council schema. The
// Poller opens it read-only, exactly as it does against the live DB.
const created: string[] = [];

afterEach(() => {
  for (const p of created.splice(0)) {
    for (const suffix of ["", "-wal", "-shm"]) rmSync(p + suffix, { force: true });
  }
});

function makeDb(): { path: string; db: Database } {
  const path = join(tmpdir(), `chtest-${Math.random().toString(36).slice(2)}.db`);
  created.push(path);
  const db = new Database(path);
  db.exec("PRAGMA journal_mode=WAL");
  db.exec("CREATE TABLE rooms (id TEXT PRIMARY KEY, status TEXT)");
  db.exec(
    "CREATE TABLE messages (id TEXT PRIMARY KEY, room_id TEXT, author TEXT, content TEXT, message_type TEXT, timestamp TEXT)"
  );
  return { path, db };
}

function cfg(path: string, over: Partial<Config> = {}): Config {
  return {
    dbPath: path,
    pollInterval: 1000,
    rooms: "*",
    mcpUrl: "",
    author: "me",
    debug: false,
    ...over,
  };
}

function addRoom(db: Database, id: string, status = "active"): void {
  db.query("INSERT INTO rooms (id, status) VALUES (?, ?)").run(id, status);
}

function addMsg(db: Database, id: string, room: string, author: string): void {
  db.query(
    "INSERT INTO messages (id, room_id, author, content, message_type, timestamp) VALUES (?, ?, ?, ?, 'message', '2026-01-01')"
  ).run(id, room, author, `content ${id}`);
}

// Sortable stand-ins for UUIDv7 ids (the cursor relies on lexicographic order).
const ID = (n: number) => String(n).padStart(4, "0");

test("delivers new messages, skips self, and never replays history", async () => {
  const { path, db } = makeDb();
  addRoom(db, "r1");
  addMsg(db, ID(1), "r1", "other"); // exists before the poller starts

  const got: Array<{ meta: Record<string, string> }> = [];
  const p = new Poller(cfg(path), async (n) => {
    got.push(n);
  });
  (p as any).init(); // seeds cursor at ID(1), watches r1

  await (p as any).poll();
  expect(got.length).toBe(0); // no replay of pre-existing history

  addMsg(db, ID(2), "r1", "me"); // our own message — should be skipped
  addMsg(db, ID(3), "r1", "other"); // genuinely new — should deliver
  await (p as any).poll();

  expect(got.map((g) => g.meta.message_id)).toEqual([ID(3)]);
  expect(got[0].meta.author).toBe("other");
  p.stop();
});

test("does not advance the cursor past a failed notification", async () => {
  const { path, db } = makeDb();
  addRoom(db, "r1");
  addMsg(db, ID(1), "r1", "other");

  let shouldFail = true;
  const got: Array<{ meta: Record<string, string> }> = [];
  const p = new Poller(cfg(path), async (n) => {
    if (shouldFail) throw new Error("delivery boom");
    got.push(n);
  });
  (p as any).init();

  addMsg(db, ID(2), "r1", "other");
  await (p as any).poll(); // notify throws → message not delivered, cursor held
  expect(got.length).toBe(0);

  shouldFail = false;
  await (p as any).poll(); // retried on the next tick
  expect(got.map((g) => g.meta.message_id)).toEqual([ID(2)]);
  p.stop();
});

test("prunes rooms that are no longer active", () => {
  const { path, db } = makeDb();
  addRoom(db, "r1");
  addRoom(db, "r2");
  const p = new Poller(cfg(path), async () => {});
  (p as any).init();
  expect(p.listWatched().sort()).toEqual(["r1", "r2"]);

  db.query("UPDATE rooms SET status = 'resolved' WHERE id = 'r2'").run();
  (p as any).refreshRooms();
  expect(p.listWatched()).toEqual(["r1"]);
  p.stop();
});

test("watch_room validates existence and survives pruning", () => {
  const { path, db } = makeDb();
  addRoom(db, "r1", "resolved"); // inactive, so not auto-watched
  const p = new Poller(cfg(path, { rooms: [] }), async () => {});
  (p as any).init();
  expect(p.listWatched()).toEqual([]);

  expect(p.watchRoom("ghost")).toContain("not found");
  expect(p.watchRoom("r1")).toContain("Now watching");
  expect(p.listWatched()).toEqual(["r1"]);

  (p as any).refreshRooms(); // r1 is inactive but manually watched → kept
  expect(p.listWatched()).toEqual(["r1"]);
  p.stop();
});
