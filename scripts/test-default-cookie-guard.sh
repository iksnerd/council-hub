#!/bin/sh
# Checks the default-cookie guard in entrypoint.sh. The guard block is cut out of
# the real file (between its marker comments), so the test fails if the real code
# regresses. Usage: test-default-cookie-guard.sh [entrypoint-path]
set -u
cd "$(dirname "$0")/.." || exit 1
file=${1:-entrypoint.sh}

block=$(sed -n '/^# ── Default cookie guard ──/,/^# ── end default cookie guard ──/p' "$file")
if [ -z "$block" ]; then
  echo "FAIL: no default cookie guard block in $file"
  exit 1
fi

fail=0
# run <name> <want-exit> <cookie|-> <seeds> [allow]
run() {
  name=$1 want=$2 cookie=$3 seeds=$4 allow=${5:-0}
  if [ "$cookie" = "-" ]; then unset RELEASE_COOKIE; else RELEASE_COOKIE=$cookie; export RELEASE_COOKIE; fi
  out=$(COUNCIL_SEEDS=$seeds COUNCIL_ALLOW_DEFAULT_COOKIE=$allow sh -c "$block"'
exit 0' 2>&1)
  got=$?
  if [ "$got" != "$want" ]; then
    echo "FAIL $name: exit $got, want $want"; fail=1; return
  fi
  if [ "$want" = 1 ]; then
    case $out in
      *COUNCIL_COOKIE*RELEASE_COOKIE*COUNCIL_ALLOW_DEFAULT_COOKIE*) ;;
      *) echo "FAIL $name: refusal does not name the cookie vars and the escape hatch"; fail=1; return ;;
    esac
  fi
  echo "ok   $name"
}

run "default cookie + seeds refuses"        1 council          "10.0.0.2"
run "unset cookie + seeds refuses"          1 -                "10.0.0.2"
run "default cookie, no seeds starts"       0 council          ""
run "real cookie + seeds starts"            0 0123456789abcdef "10.0.0.2"
run "escape hatch + default + seeds starts" 0 council          "10.0.0.2" 1
exit $fail
