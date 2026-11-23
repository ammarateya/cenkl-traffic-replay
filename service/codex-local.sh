#!/bin/sh
set -eu
: "${CENKL_CODEX_REAL_BIN:?Set CENKL_CODEX_REAL_BIN to the Codex CLI path}"
if [ "${1:-}" != exec ]; then
  echo 'Codex SDK command was not exec' >&2
  exit 2
fi
shift
exec "$CENKL_CODEX_REAL_BIN" exec --ignore-user-config --disable plugins --disable apps "$@"
