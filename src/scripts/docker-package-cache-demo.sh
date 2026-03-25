#!/usr/bin/env bash
# Demo: named Docker volumes for Flathub + APT caches (survive image rebuilds / relaunches).
# Does not install apps for you — shows the pattern. See .staging/workflows/package-cache-demo/README.md
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${DOCKPIPE_REPO_ROOT:-$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel 2>/dev/null || true)}"
if [[ -z "$REPO_ROOT" ]]; then
  echo "docker-package-cache-demo: cannot find repo root (set DOCKPIPE_REPO_ROOT or run from a git checkout)" >&2
  exit 1
fi
# shellcheck source=src/core/assets/scripts/docker-cache-volumes.sh
source "${REPO_ROOT}/src/core/assets/scripts/docker-cache-volumes.sh"

FLATPAK_VOL="${DOCKER_DEMO_FLATPAK_VOL:-dockpipe-flatpak-system}"
APT_CACHE_VOL="${DOCKER_DEMO_APT_CACHE_VOL:-dockpipe-apt-cache}"
APT_LISTS_VOL="${DOCKER_DEMO_APT_LISTS_VOL:-dockpipe-apt-lists}"

CACHE_ARGS=()
docker_cache_volume_append_mounts CACHE_ARGS \
  "${FLATPAK_VOL}:/var/lib/flatpak" \
  "${APT_CACHE_VOL}:/var/cache/apt" \
  "${APT_LISTS_VOL}:/var/lib/apt/lists"

echo "[package-cache-demo] Named volumes are ready. Reuse these mounts on every run to avoid re-downloading."
echo ""
echo "Flathub (system) + APT cache paths:"
printf ' '; printf '%q ' "${CACHE_ARGS[@]}"
echo ""
echo ""
echo "Example — Debian with persistent APT cache:"
echo -n "  docker run --rm -it"
for ((i = 0; i < ${#CACHE_ARGS[@]}; i += 2)); do
  echo -n " ${CACHE_ARGS[$i]} ${CACHE_ARGS[$((i + 1))]}"
done
echo " \\"
echo "    debian:bookworm-slim bash -lc 'apt-get update && apt-get install -y curl && exec bash'"
echo ""
echo "Example — dockpipe with Flathub system cache (same volume as optional Steam script):"
echo "  dockpipe --mount ${FLATPAK_VOL}:/var/lib/flatpak --isolate steam-flatpak -- true"
echo ""

if [[ "${DOCKER_PACKAGE_CACHE_DEMO_RUN:-0}" == "1" ]]; then
  if ! command -v docker >/dev/null 2>&1; then
    echo "docker not in PATH; skipping run" >&2
    exit 1
  fi
  echo "[package-cache-demo] DOCKER_PACKAGE_CACHE_DEMO_RUN=1 — running apt-get update in debian:bookworm-slim"
  exec docker run --rm "${CACHE_ARGS[@]}" debian:bookworm-slim bash -lc 'apt-get update && echo ok'
fi
