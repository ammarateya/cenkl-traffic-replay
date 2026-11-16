#!/bin/sh
# Keep a proxy process in one container; USR1 closes its current outer link.
while :; do
  "$@" &
  child=$!
  trap 'kill -TERM "$child" 2>/dev/null || true; wait "$child" 2>/dev/null || true' USR1
  wait "$child" 2>/dev/null || true
done
