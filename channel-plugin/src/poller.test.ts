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

test("a newly-discovered room's backlog survives the shared cursor advancing past it", async () => {
  const { path, db } = makeDb();
  addRoom(db, "r1");
  const got: Array<{ meta: Record<string, string> }> = [];
  const p = new Poller(cfg(path), async (n) => {
    got.push(n);
  });
  (p as any).init(); // cursor seeded at "" (nothing posted yet), r1 watched

  // r2 is created and posted to, but refreshRooms() hasn't run yet — so it
  // isn't in watchedRooms when its message lands.
  addRoom(db, "r2");
  addMsg(db, ID(1), "r2", "other");

  // Meanwhile r1 (already watched) gets a *later* message, which the shared
  // global cursor advances past — this is what used to leapfrog r2's backlog.
  addMsg(db, ID(2), "r1", "other");
  await (p as any).poll();
  expect(got.map((g) => g.meta.message_id)).toEqual([ID(2)]);

  // Discovery happens now, after the cursor has already moved past ID(1).
  (p as any).refreshRooms();
  expect(p.listWatched().sort()).toEqual(["r1", "r2"]);

  await (p as any).poll();
  expect(got.map((g) => g.meta.message_id).sort()).toEqual([ID(1), ID(2)]);

  // And exactly once — subsequent ticks must not re-deliver the backlog.
  await (p as any).poll();
  expect(got.length).toBe(2);
  p.stop();
});

test("a room added mid-tick is not promoted before its catch-up query runs", async () => {
  const { path, db } = makeDb();
  addRoom(db, "r1");
  const got: string[] = [];
  let addRoomMidNotify = false;
  const p = new Poller(cfg(path), async (n) => {
    got.push(n.meta.message_id);
    if (addRoomMidNotify) {
      addRoomMidNotify = false;
      // watch_room fires while poll() is suspended on this notify — the room
      // lands in catchupFloor after this tick's queries have already run.
      p.watchRoom("r2");
    }
  });
  (p as any).init(); // cursor and sessionFloor seeded at "" (no messages yet)

  // r2 exists with a backlog message, but isn't watched yet.
  addRoom(db, "r2");
  addMsg(db, ID(1), "r2", "other");
  addMsg(db, ID(2), "r1", "other");

  addRoomMidNotify = true;
  await (p as any).poll(); // delivers ID(2); r2 is added mid-notify
  expect(got).toEqual([ID(2)]);

  // r2's floor == sessionFloor == this tick's starting cursor (the exact
  // race) — but it was never queried this tick, so it must not be promoted.
  expect((p as any).catchupFloor.has("r2")).toBe(true);

  await (p as any).poll(); // r2's catch-up query finally runs
  expect(got.sort()).toEqual([ID(1), ID(2)]);
  p.stop();
});

test("re-watching a pruned room does not replay already-delivered messages", async () => {
  const { path, db } = makeDb();
  addRoom(db, "r1");
  addRoom(db, "r2");
  const got: string[] = [];
  const p = new Poller(cfg(path), async (n) => {
    got.push(n.meta.message_id);
  });
  (p as any).init();

  addMsg(db, ID(1), "r1", "other");
  await (p as any).poll();
  expect(got).toEqual([ID(1)]);

  // r1 resolves and is pruned.
  db.query("UPDATE rooms SET status = 'resolved' WHERE id = 'r1'").run();
  (p as any).refreshRooms();
  expect(p.listWatched()).toEqual(["r2"]);

  // While pruned: r1 gets a message, and r2 pushes the shared cursor past it.
  addMsg(db, ID(2), "r1", "other");
  addMsg(db, ID(3), "r2", "other");
  await (p as any).poll();
  expect(got).toEqual([ID(1), ID(3)]);

  // r1 reactivates and is rediscovered: no duplicate of ID(1), but the
  // message posted while pruned (ID(2), below the shared cursor) is still
  // delivered — the room resumes catch-up from its remembered floor.
  db.query("UPDATE rooms SET status = 'active' WHERE id = 'r1'").run();
  (p as any).refreshRooms();
  await (p as any).poll();
  await (p as any).poll();
  expect(got).toEqual([ID(1), ID(3), ID(2)]);
  p.stop();
});

test("manually re-watching an unwatched room does not replay delivered messages", async () => {
  const { path, db } = makeDb();
  addRoom(db, "r1");
  const got: string[] = [];
  const p = new Poller(cfg(path), async (n) => {
    got.push(n.meta.message_id);
  });
  (p as any).init();

  addMsg(db, ID(1), "r1", "other");
  await (p as any).poll();
  expect(got).toEqual([ID(1)]);

  p.unwatchRoom("r1");
  expect(p.watchRoom("r1")).toContain("Now watching");
  await (p as any).poll(); // must not re-deliver ID(1)
  expect(got).toEqual([ID(1)]);

  addMsg(db, ID(2), "r1", "other");
  await (p as any).poll(); // new messages still flow
  expect(got).toEqual([ID(1), ID(2)]);
  p.stop();
});

test("a quiet catch-up room is promoted on an empty result instead of polling forever", async () => {
  const { path, db } = makeDb();
  addRoom(db, "r1");
  addMsg(db, ID(1), "r1", "other"); // pre-existing → seeds cursor at ID(1)
  const p = new Poller(cfg(path), async () => {});
  (p as any).init();

  // Advance the shared cursor past the session floor.
  addMsg(db, ID(2), "r1", "other");
  await (p as any).poll();

  // Spy on fetchRows to see which rooms get individual catch-up queries.
  const catchupQueried: string[][] = [];
  const orig = (p as any).fetchRows.bind(p);
  (p as any).fetchRows = (est: string[], catchup: string[]) => {
    catchupQueried.push([...catchup]);
    return orig(est, catchup);
  };

  // r2 appears with no messages at all: its floor (sessionFloor) sits below
  // the shared cursor, so nothing can ever advance it via delivered rows.
  addRoom(db, "r2");
  (p as any).refreshRooms();
  await (p as any).poll(); // catch-up query returns empty → gap proven closed
  expect(catchupQueried).toEqual([["r2"]]);

  await (p as any).poll();
  await (p as any).poll();
  expect(catchupQueried).toEqual([["r2"], [], []]); // no more individual queries
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
