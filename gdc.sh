#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ "${GDC_USE_PREBUILT:-}" = "1" ]; then
  if [ -x "$SCRIPT_DIR/gdc" ]; then
    exec "$SCRIPT_DIR/gdc" "$@"
  fi
  if [ -x "$SCRIPT_DIR/gdc-linux-amd64" ]; then
    exec "$SCRIPT_DIR/gdc-linux-amd64" "$@"
  fi
  echo "GDC_USE_PREBUILT=1 but no prebuilt POSIX executable was found." >&2
  exit 1
fi

exec go run "$SCRIPT_DIR/cmd/gdc" "$@"
