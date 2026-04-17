#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BIN_DIR="${CLAWX_BIN_DIR:-$HOME/.local/bin}"
TARGET_BIN="${CLAWX_TARGET_BIN:-$BIN_DIR/clawx}"
BACKUP_BIN="${CLAWX_BACKUP_BIN:-$BIN_DIR/clawx.prev}"
TMP_BIN="$ROOT_DIR/.tmp/clawx.release"
RELEASE_ARTIFACT="${CLAWX_RELEASE_ARTIFACT:-}"
RELEASE_VERSION="${CLAWX_RELEASE_VERSION:-}"

mkdir -p "$BIN_DIR" "$ROOT_DIR/.tmp"

BUILD_SOURCE="$TMP_BIN"
if [ -n "$RELEASE_ARTIFACT" ]; then
  if [ ! -f "$RELEASE_ARTIFACT" ]; then
    echo "release_failed artifact_not_found=$RELEASE_ARTIFACT" >&2
    exit 1
  fi
  BUILD_SOURCE="$RELEASE_ARTIFACT"
else
  if [ "${CLAWX_RELEASE_SKIP_GIT_PULL:-0}" != "1" ]; then
    git pull --ff-only
  fi

  go build -o "$TMP_BIN" ./cmd/clawx
fi

if [ -f "$TARGET_BIN" ]; then
  cp "$TARGET_BIN" "$BACKUP_BIN"
fi

install -m 0755 "$BUILD_SOURCE" "$TARGET_BIN"
if [ -n "$RELEASE_VERSION" ]; then
  echo "release_complete target=$TARGET_BIN backup=$BACKUP_BIN source=$BUILD_SOURCE version=$RELEASE_VERSION"
else
  echo "release_complete target=$TARGET_BIN backup=$BACKUP_BIN source=$BUILD_SOURCE"
fi
