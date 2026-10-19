#!/bin/sh
# Tests for .githooks/post-commit: when does a commit become a ledger post, and
# when is it suppressed as a duplicate? Runs in a throwaway repo with a fake
# scripts/council-post.py that records each call, so no hub is needed.
#
#   sh scripts/test-post-commit.sh
set -eu

HOOK=${HOOK:-$(cd "$(dirname "$0")/.." && pwd)/.githooks/post-commit}
[ -f "$HOOK" ] || { echo "hook not found: $HOOK" >&2; exit 2; }

TMP=$(mktemp -d)
POSTS_LOG="$TMP/posts.log"
export POSTS_LOG
trap 'rm -rf "$TMP"' EXIT
FAILS=0

setup_repo() {
  rm -rf "$TMP/repo" && mkdir -p "$TMP/repo/.githooks" "$TMP/repo/scripts" && cd "$TMP/repo"
  git init -q -b main .
  git config user.name tester && git config user.email t@example.com
  git config commit.gpgsign false && git config tag.gpgsign false
  git config core.hooksPath .githooks
  cp "$HOOK" .githooks/post-commit && chmod +x .githooks/post-commit
  # fake poster: one line per call, "room|subject" taken from stdin's first line
  cat > scripts/council-post.py <<'PY'
import sys
room = sys.argv[sys.argv.index("--room") + 1]
first = sys.stdin.readline().strip()
import os
open(os.environ["POSTS_LOG"], "a").write(room + "|" + first + "\n")
PY
  : > "$POSTS_LOG"
  git add -A && git -c core.hooksPath=/dev/null commit -q -m "base"
}

commit_file() { # name subject room
  printf '%s\n' "$1-$(date +%s%N 2>/dev/null || date +%s)-$RANDOM" > "$1.txt"
  git add "$1.txt"
  git commit -q -m "$2

Council-Room: $3" 2>/dev/null
}

posts() { grep -c . "$POSTS_LOG" 2>/dev/null || echo 0; }

expect() { # label want
  got=$(posts)
  if [ "$got" = "$2" ]; then echo "PASS $1 (posts=$got)"; else echo "FAIL $1: want $2 posts, got $got"; FAILS=$((FAILS+1)); fi
}

# 1. a trailer posts once
setup_repo
commit_file a "feat: one" room-x
expect "trailer posts once" 1

# 2. amending the commit does not post again
git commit -q --amend --no-edit 2>/dev/null
expect "amend is suppressed" 1

# 3. no trailer, no post
printf x > b.txt && git add b.txt && git commit -q -m "chore: no trailer" 2>/dev/null
expect "no trailer, no post" 1

# 4. a different subject in the same room posts
commit_file c "feat: two" room-x
expect "different subject posts" 2

# 5. same subject, different room, posts
commit_file d "feat: two" room-y
expect "same subject in another room posts" 3

# 6. THE BUG: a rebase replays the commit onto a new parent; same subject, same
#    room, new parent. It is the same piece of work and must not post again.
setup_repo
git checkout -q -b topic
commit_file e "feat: replayed" room-x
git checkout -q main
printf y > upstream.txt && git add upstream.txt && git -c core.hooksPath=/dev/null commit -q -m "upstream moved"
git checkout -q topic
git rebase main >/dev/null 2>&1
expect "rebase onto a new parent is suppressed" 1

# 6b. a rebase that replays SEVERAL commits must suppress every one of them
setup_repo
git checkout -q -b topic2
commit_file h1 "feat: first of two" room-x
commit_file h2 "feat: second of two" room-x
git checkout -q main
printf z > upstream2.txt && git add upstream2.txt && git -c core.hooksPath=/dev/null commit -q -m "upstream moved again"
git checkout -q topic2
git rebase main >/dev/null 2>&1
expect "rebase replaying two commits is suppressed" 2

# 7. after the suppression window the same subject posts again
setup_repo
commit_file f "feat: later" room-x
# age the recorded post past the window ("seen <epoch> <room> <subject>" lines)
M="$(git rev-parse --git-dir)/council-last-post"
OLD=$(( $(date +%s) - 7200 ))
sed "s/^seen [0-9][0-9]* /seen $OLD /" "$M" > "$M.new" && mv "$M.new" "$M"
commit_file g "feat: later" room-x
expect "same subject after the window posts again" 2

echo
if [ "$FAILS" -eq 0 ]; then echo "all post-commit tests passed"; else echo "$FAILS post-commit test(s) failed"; exit 1; fi
