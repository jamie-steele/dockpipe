#!/usr/bin/env bash
set -euo pipefail

eval "$(dockpipe sdk)"
dockpipe_sdk init-script

REPO_ROOT="${ROOT}"

if [[ -n "${DOCKPIPE_SKILLS_RENDER_BIN:-}" ]]; then
  RENDER_BIN="${DOCKPIPE_SKILLS_RENDER_BIN}"
else
  RENDER_BIN="$(dockpipe_sdk require tooling-bin skills-render || true)"
fi

if [[ -z "${RENDER_BIN:-}" ]]; then
  DOCKPIPE_BIN="$(dockpipe_sdk require dockpipe-bin)"
  if [[ -x "${DOCKPIPE_BIN:-}" ]]; then
    "$DOCKPIPE_BIN" package build source --workdir "$REPO_ROOT" --only dorkpipe
    RENDER_BIN="$(dockpipe_sdk require tooling-bin skills-render || true)"
  fi
fi

if [[ -z "${RENDER_BIN:-}" ]]; then
  echo "skills-render: compiled helper not found at $REPO_ROOT/bin/.dockpipe/tooling/bin/skills-render(.exe)" >&2
  echo "Run: ${DOCKPIPE_BIN:-dockpipe} package build source --workdir $REPO_ROOT --only dorkpipe" >&2
  exit 1
fi

exec "$RENDER_BIN" "$@"
