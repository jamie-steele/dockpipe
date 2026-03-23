#!/usr/bin/env bash
# Host workflow: run a Flathub app id with flatpak (host install). Set FLATHUB_APP_ID (e.g. com.valvesoftware.Steam).
# Pipeon can set this via "Extra dockpipe env" or dockpipe --env FLATHUB_APP_ID=...
set -euo pipefail

if [[ -z "${FLATHUB_APP_ID:-}" ]]; then
  echo "flathub-host: FLATHUB_APP_ID is not set. Example: dockpipe --env FLATHUB_APP_ID=com.valvesoftware.Steam --workflow flathub-host --workdir \$PWD" >&2
  exit 1
fi

if ! command -v flatpak >/dev/null 2>&1; then
  echo "flathub-host: flatpak not found on PATH" >&2
  exit 1
fi

exec flatpak run "${FLATHUB_APP_ID}" "$@"
