#!/usr/bin/env bash
# Action: write a patch file of uncommitted changes to /work/dockpipe.patch (or DOCKPIPE_PATCH_PATH).
# Useful for applying results outside the container without committing.
set -euo pipefail

cd "${DOCKPIPE_CONTAINER_WORKDIR:-/work}"
OUT="${DOCKPIPE_PATCH_PATH:-/work/dockpipe.patch}"

if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  echo "[dockpipe action] Not a git repo; skipping export-patch." >&2
  exit "$DOCKPIPE_EXIT_CODE"
fi

if git diff --quiet HEAD 2>/dev/null && [[ -z "$(git status --porcelain)" ]]; then
  echo "[dockpipe action] No changes; no patch written." >&2
  exit "$DOCKPIPE_EXIT_CODE"
fi

git diff HEAD > "${OUT}" 2>/dev/null || true
git diff --cached HEAD >> "${OUT}" 2>/dev/null || true
echo "[dockpipe action] Patch written to ${OUT}" >&2
exit "$DOCKPIPE_EXIT_CODE"
