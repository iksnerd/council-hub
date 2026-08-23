#!/usr/bin/env python3
"""Report commits since the last tag that have no council-hub ledger entry.

The backstop for the ledger gap. The `Council-Room:` commit trailer (see
.githooks/post-commit) removes the common case by making the ledger entry a
byproduct of the commit; this catches what the trailer misses -- a commit made
without it is the same failure in a new coat.

It deliberately does NOT guess semantic coverage. Matching a commit subject to a
prose post by keyword produces confident nonsense, and a coverage report that is
wrong is worse than one that admits what it cannot see. The only claim it makes
is exact: a commit is "linked" when its short sha appears verbatim in a ledger
message. Everything else is listed as unverified for a human to judge.

Read-only against the same SQLite file the Go server owns (WAL, so this is safe
against a live server).

Usage:
    scripts/ledger-check.py [--since <tag>] [--project <slug>] [--db <path>]
"""

import argparse
import os
import re
import sqlite3
import subprocess
import sys
import time

# Matches read_notebook's default type set. `note` counts: the question here is
# "did this work reach the ledger at all", and a note is a real record even though
# it is not changelog material. Excluding it produced a false positive on the first
# commit that used the hook with Council-Type: note.
LEDGER_TYPES = ("action", "decision", "synthesis", "note")


def git(*args: str) -> str:
    return subprocess.run(
        ["git", *args], capture_output=True, text=True, check=True
    ).stdout.strip()


def main() -> int:
    repo_root = git("rev-parse", "--show-toplevel")

    ap = argparse.ArgumentParser()
    ap.add_argument("--since", help="tag or ref to diff from (default: most recent tag)")
    ap.add_argument("--project", default=os.path.basename(repo_root),
                    help="council-hub project slug (default: repo directory name)")
    ap.add_argument("--db", default=os.environ.get(
        "COUNCIL_DB", os.path.expanduser("~/.council-hub/council.db")))
    args = ap.parse_args()

    since = args.since
    if not since:
        try:
            since = git("describe", "--tags", "--abbrev=0")
        except subprocess.CalledProcessError:
            print("no tags in this repo -- pass --since <ref>", file=sys.stderr)
            return 2

    # Tag date bounds the ledger window. %aI is the author date in ISO-8601 with
    # an offset; SQLite stores UTC, so normalise before comparing.
    tag_iso = git("log", "-1", "--format=%aI", since)
    tag_utc = subprocess.run(
        ["python3", "-c",
         "import datetime,sys;print(datetime.datetime.fromisoformat(sys.argv[1])"
         ".astimezone(datetime.timezone.utc).strftime('%Y-%m-%d %H:%M:%S'))", tag_iso],
        capture_output=True, text=True, check=True).stdout.strip()

    log = git("log", f"{since}..HEAD", "--format=%h\x1f%s")
    commits = [tuple(line.split("\x1f", 1)) for line in log.splitlines() if line]

    if not os.path.exists(args.db):
        print(f"ledger not found at {args.db} (set COUNCIL_DB)", file=sys.stderr)
        return 2

    query = """SELECT m.id, m.room_id, m.message_type, m.content
                 FROM messages m JOIN rooms r ON r.id = m.room_id
                WHERE r.project = ?
                  AND m.message_type IN (?, ?, ?, ?)
                  AND m.timestamp >= ?
                  AND m.revised = 0
                ORDER BY m.id"""
    params = (args.project, *LEDGER_TYPES, tag_utc)

    # A read-only connection that opens mid-WAL-checkpoint can see an inconsistent
    # snapshot and raise "database disk image is malformed" against a database that
    # is perfectly fine -- observed live, with PRAGMA integrity_check returning ok
    # moments later. Retry before believing it, so this tool never cries corruption
    # at a healthy server.
    rows = None
    for attempt in range(3):
        try:
            con = sqlite3.connect(f"file:{args.db}?mode=ro", uri=True)
            rows = con.execute(query, params).fetchall()
            con.close()
            break
        except sqlite3.DatabaseError as e:
            if attempt == 2:
                print(f"ledger read failed after 3 attempts: {e}", file=sys.stderr)
                print("If this persists, check the server's /health "
                      "(last_integrity_check) before assuming corruption.",
                      file=sys.stderr)
                return 2
            time.sleep(0.3)

    haystack = "\n".join(r[3] for r in rows)

    print(f"Range:   {since}..HEAD  (since {tag_utc} UTC)")
    print(f"Project: {args.project}")
    print(f"Ledger:  {len(rows)} {'/'.join(LEDGER_TYPES)} post(s)")
    print(f"Commits: {len(commits)}\n")

    for mid, room, mtype, content in rows:
        first = content.strip().splitlines()[0] if content.strip() else ""
        print(f"  #{mid[:8]} [{room}] {mtype}: {first[:78]}")
    if rows:
        print()

    unlinked = [(sha, subj) for sha, subj in commits if sha not in haystack]

    if not commits:
        print("No commits since the last tag.")
        return 0

    if not unlinked:
        print(f"All {len(commits)} commit(s) are cited by sha in the ledger.")
        return 0

    print(f"{len(unlinked)} commit(s) not cited by sha in any ledger post:\n")
    for sha, subj in unlinked:
        print(f"  {sha}  {subj}")
    print("\nNot proof of a gap -- a post can cover a commit without naming its sha.")
    print("Judge each one, and add a `Council-Room:` trailer next time so the")
    print("entry is a byproduct of the commit rather than a thing to remember.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
