#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_ROOT="$(cd "${DOCKPIPE_PACKAGE_ROOT:-$SCRIPT_DIR/../..}" && pwd)"
REPO_ROOT="$(cd "$PACKAGE_ROOT/../.." && pwd)"

OUT_DIR="$REPO_ROOT/bin/.dockpipe/tooling/bin"
BUILD_DIR="$REPO_ROOT/bin/.dockpipe/build"
VERSION_FILE="$REPO_ROOT/VERSION"
GOEXE="$(go env GOEXE)"

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
go build -C "$PACKAGE_ROOT/lib" -trimpath -ldflags "$ldflags" -o "$OUT_DIR/skills-render$GOEXE" ./cmd/skills-render
go build -C "$PACKAGE_ROOT/lib" -trimpath -ldflags "$ldflags" -o "$OUT_DIR/orchestrate-helper$GOEXE" ./cmd/orchestrate-helper

printf '[dockpipe] package build source: built %s, %s, %s, and %s\n' "$OUT_DIR/dorkpipe" "$OUT_DIR/mcpd" "$OUT_DIR/skills-render$GOEXE" "$OUT_DIR/orchestrate-helper$GOEXE" >&2
