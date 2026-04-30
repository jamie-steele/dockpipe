#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_ROOT="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$SCRIPT_DIR/../..}" && pwd)"
REPO_ROOT="$(cd "$PACKAGE_ROOT/../.." && pwd)"
SDK_SH="$REPO_ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"

if [[ -f "$SDK_SH" ]]; then
  # shellcheck source=/dev/null
  source "$SDK_SH"
  dockpipe_sdk_refresh "$REPO_ROOT"
  PACKAGE_ROOT="$(dockpipe get package_root 2>/dev/null || printf '%s\n' "$PACKAGE_ROOT")"
  REPO_ROOT="$(dockpipe get workdir 2>/dev/null || printf '%s\n' "$REPO_ROOT")"
fi

OUT_DIR="$REPO_ROOT/bin/.dockpipe/tooling/bin"
BUILD_DIR="$REPO_ROOT/bin/.dockpipe/build"
VERSION_FILE="$REPO_ROOT/VERSION"

version="0.0.0"
if [[ -f "$VERSION_FILE" ]]; then
  version="$(tr -d ' \t\r\n' < "$VERSION_FILE")"
fi
ldflags="-s -w -X main.Version=${version}"

mkdir -p "$OUT_DIR"
mkdir -p "$BUILD_DIR/go-cache" "$BUILD_DIR/go-tmp"
export GOCACHE="${GOCACHE:-$BUILD_DIR/go-cache}"
export GOTMPDIR="${GOTMPDIR:-$BUILD_DIR/go-tmp}"

go build -C "$PACKAGE_ROOT/lib" -trimpath -ldflags "$ldflags" -o "$OUT_DIR/dorkpipe" ./cmd/dorkpipe
go build -C "$PACKAGE_ROOT/mcp" -trimpath -ldflags "$ldflags" -o "$OUT_DIR/mcpd" ./cmd/mcpd

printf '[dockpipe] package build source: built %s and %s\n' "$OUT_DIR/dorkpipe" "$OUT_DIR/mcpd" >&2
