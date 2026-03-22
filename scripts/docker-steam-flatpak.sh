#!/usr/bin/env bash
# Build (if missing) and run Flathub Steam in Docker with GUI/GPU-friendly mounts.
# Linux desktop only; Flatpak-in-Docker needs --privileged for bwrap (see templates/core/bundles/steam-flatpak/assets/images/steam-flatpak/README.md).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=docker-cache-volumes.sh
source "${SCRIPT_DIR}/docker-cache-volumes.sh"

REPO_ROOT="${DOCKPIPE_REPO_ROOT:-$(cd "${SCRIPT_DIR}/.." && pwd)}"
DF="${REPO_ROOT}/templates/core/bundles/steam-flatpak/assets/images/steam-flatpak/Dockerfile"
if [[ ! -f "${DF}" ]]; then
  echo "docker-steam-flatpak: Dockerfile not found: ${DF}" >&2
  exit 1
fi

VER="$(tr -d '\n' < "${REPO_ROOT}/VERSION" 2>/dev/null || echo latest)"
IMAGE="${DOCKER_STEAM_IMAGE:-dockpipe-steam-flatpak:${VER}}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker-steam-flatpak: docker not in PATH" >&2
  exit 1
fi

if [[ "${DOCKER_STEAM_REBUILD:-0}" == "1" ]] || ! docker image inspect "${IMAGE}" >/dev/null 2>&1; then
  echo "[docker-steam-flatpak] Building ${IMAGE} (first time is large)…"
  docker build -t "${IMAGE}" -f "${DF}" "${REPO_ROOT}"
fi

XAUTH="${XAUTHORITY:-${HOME}/.Xauthority}"
DBUS_SOCK="/run/user/$(id -u)/bus"
DOCKER_ARGS=(
  --rm
  -it
  --privileged
  -u "$(id -u):$(id -g)"
  -e "DISPLAY=${DISPLAY:-:0}"
  -e "HOME=${HOME}"
  -e "XAUTHORITY=${XAUTH}"
)

if [[ -n "${WAYLAND_DISPLAY:-}" ]] && [[ -n "${XDG_RUNTIME_DIR:-}" ]]; then
  DOCKER_ARGS+=(-e "WAYLAND_DISPLAY=${WAYLAND_DISPLAY}" -e "XDG_RUNTIME_DIR=/run/user/$(id -u)")
  DOCKER_ARGS+=(-v "${XDG_RUNTIME_DIR}:${XDG_RUNTIME_DIR}")
else
  DOCKER_ARGS+=(-v /tmp/.X11-unix:/tmp/.X11-unix:rw)
  if [[ -f "${XAUTH}" ]]; then
    DOCKER_ARGS+=(-v "${XAUTH}:${XAUTH}:ro")
  fi
fi

if [[ -d /dev/dri ]]; then
  DOCKER_ARGS+=(--device /dev/dri)
fi

if [[ -S "${DBUS_SOCK}" ]]; then
  DOCKER_ARGS+=(-v "${DBUS_SOCK}:${DBUS_SOCK}")
fi

if [[ "${DOCKER_STEAM_NVIDIA:-0}" == "1" ]]; then
  DOCKER_ARGS+=(--gpus all)
fi

# Persist Flathub system dir (runtimes, refs) across container runs — disable with DOCKER_STEAM_USE_FLATPAK_CACHE_VOLUME=0
if [[ "${DOCKER_STEAM_USE_FLATPAK_CACHE_VOLUME:-1}" == "1" ]]; then
  CACHE_VOL_ARGS=()
  docker_cache_volume_append_mounts CACHE_VOL_ARGS \
    "${DOCKER_STEAM_FLATPAK_CACHE_VOLUME:-dockpipe-flatpak-system}:/var/lib/flatpak"
  DOCKER_ARGS+=("${CACHE_VOL_ARGS[@]}")
fi

echo "[docker-steam-flatpak] Running ${IMAGE} (Ctrl+C to stop)"
exec docker run "${DOCKER_ARGS[@]}" "${IMAGE}" "$@"
