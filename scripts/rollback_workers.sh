#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="${CLAWX_BIN_DIR:-$HOME/.local/bin}"
TARGET_BIN="${CLAWX_TARGET_BIN:-$BIN_DIR/clawx}"
BACKUP_BIN="${CLAWX_BACKUP_BIN:-$BIN_DIR/clawx.prev}"

if [ ! -f "$BACKUP_BIN" ]; then
  echo "rollback_failed backup_not_found=$BACKUP_BIN" >&2
  exit 1
fi

install -m 0755 "$BACKUP_BIN" "$TARGET_BIN"
echo "rollback_complete target=$TARGET_BIN from=$BACKUP_BIN"
