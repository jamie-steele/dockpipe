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

now_ms() {
  date +%s%3N
}

emit_result() {
  local unit="${1:?unit}"
  local status="${2:?status}"
  local duration_ms="${3:-}"
  shift 3 || true
  local dockpipe_bin="${DOCKPIPE_BIN:-dockpipe}"
  local args=("result" "--unit" "$unit" "--status" "$status")
  if [[ -n "$duration_ms" && "$status" != "start" ]]; then
    args+=("--duration-ms" "$duration_ms")
  fi
  local field key value
  for field in "$@"; do
    [[ -n "$field" ]] || continue
    if [[ "$field" == *=* ]]; then
      key="${field%%=*}"
      value="${field#*=}"
      if [[ "$key" == "error" ]]; then
        args+=("--error" "$value")
      else
        args+=("--id" "$field")
      fi
    fi
  done
  if command -v "$dockpipe_bin" >/dev/null 2>&1 && "$dockpipe_bin" "${args[@]}"; then
    return 0
  fi
  printf '[dockpipe] unit=%s status=%s' "$unit" "$status" >&2
  if [[ -n "$duration_ms" && "$status" != "start" ]]; then
    printf ' duration_ms=%s' "$duration_ms" >&2
  fi
  for field in "$@"; do
    [[ -n "$field" ]] && printf ' %s' "$field" >&2
  done
  printf '\n' >&2
}

build_tool() {
  local tool="${1:?tool}"
  local module_dir="${2:?module dir}"
  local output="${3:?output}"
  local package="${4:?package}"
  local started_ms duration_ms rc
  emit_result "package.source.tool" "start" "" "tool=$tool" "output=$output" "package=dorkpipe"
  started_ms="$(now_ms)"
  set +e
  go build -C "$module_dir" -trimpath -ldflags "$ldflags" -o "$output" "$package"
  rc=$?
  set -e
  duration_ms="$(( $(now_ms) - started_ms ))"
  if [[ "$rc" -ne 0 ]]; then
    emit_result "package.source.tool" "fail" "$duration_ms" "tool=$tool" "output=$output" "package=dorkpipe" "error=go build exited $rc"
    return "$rc"
  fi
  emit_result "package.source.tool" "done" "$duration_ms" "tool=$tool" "output=$output" "package=dorkpipe"
}

build_tool "dorkpipe" "$PACKAGE_ROOT/lib" "$OUT_DIR/dorkpipe" "./cmd/dorkpipe"
build_tool "mcpd" "$PACKAGE_ROOT/mcp" "$OUT_DIR/mcpd" "./cmd/mcpd"
build_tool "skills-render" "$PACKAGE_ROOT/lib" "$OUT_DIR/skills-render$GOEXE" "./cmd/skills-render"
build_tool "orchestrate-helper" "$PACKAGE_ROOT/lib" "$OUT_DIR/orchestrate-helper$GOEXE" "./cmd/orchestrate-helper"
