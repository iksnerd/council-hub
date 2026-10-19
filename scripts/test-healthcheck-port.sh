#!/bin/sh
# Checks the port expansion the Dockerfile HEALTHCHECK uses in COUNCIL_UI=off
# mode. The expansion is read out of the Dockerfile itself, so the test fails
# if the real line regresses rather than a copy of it.
set -u
cd "$(dirname "$0")/.." || exit 1

expr=$(grep -o 'localhost:\${p[^}]*}' Dockerfile | head -1 | sed 's/^localhost://')
if [ -z "$expr" ]; then
  echo "FAIL: no localhost:\${p...} expansion found in Dockerfile"
  exit 1
fi

fail=0
check() {
  addr=$1 want=$2
  got=$(p=$addr; eval "printf %s \"$expr\"")
  if [ "$got" = "$want" ]; then
    echo "ok   $addr -> $got"
  else
    echo "FAIL $addr -> '$got' (want '$want') using $expr"
    fail=1
  fi
}
check ':3001' 3001
check '0.0.0.0:3001' 3001
check '127.0.0.1:3001' 3001
check '[::]:3001' 3001
exit $fail
