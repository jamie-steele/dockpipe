#!/usr/bin/env bash
set -euo pipefail

ASSETS_DIR="${DOCKPIPE_ASSETS_DIR:-}"
if [[ -z "$ASSETS_DIR" ]]; then
  SCRIPT_DIR="$(dockpipe get script_dir)"
  ASSETS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
fi

PACKAGE_ROOT="${DOCKPIPE_PACKAGE_ROOT:-}"
if [[ -z "$PACKAGE_ROOT" ]]; then
  PACKAGE_ROOT="$(cd "$ASSETS_DIR/../../.." && pwd)"
fi

REPO_ROOT="$(cd "$PACKAGE_ROOT/../.." && pwd)"

if [[ -n "${DOCKPIPE_SKILLS_RENDER_BIN:-}" ]]; then
  RENDER_BIN="$DOCKPIPE_SKILLS_RENDER_BIN"
else
  for candidate in \
    "$REPO_ROOT/bin/.dockpipe/tooling/bin/skills-render" \
    "$REPO_ROOT/bin/.dockpipe/tooling/bin/skills-render.exe"
  do
    if [[ -x "$candidate" ]]; then
      RENDER_BIN="$candidate"
      break
    fi
  done
fi

if [[ -z "${RENDER_BIN:-}" ]]; then
  echo "skills-render: compiled helper not found. Run dockpipe package build source --only dorkpipe" >&2
  exit 1
fi

exec "$RENDER_BIN" "$@"
