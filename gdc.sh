#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ -x "$SCRIPT_DIR/gdc" ]; then
  exec "$SCRIPT_DIR/gdc" "$@"
fi

if [ -x "$SCRIPT_DIR/gdc-linux-amd64" ]; then
  exec "$SCRIPT_DIR/gdc-linux-amd64" "$@"
fi

exec go run "$SCRIPT_DIR/cmd/gdc" "$@"
