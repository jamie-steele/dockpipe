#!/usr/bin/env bash
# Host helper for the sandbox-steam workflow: prints workdir + Steam/sandbox notes.
# Does not install Steam or change dockpipe core — extend this script for your machine.
#
# Launch resolution (first match wins when STEAM_SANDBOX_LAUNCH=1):
#   STEAM_CMD              — path to the steam binary (single path, executable)
#   STEAM_USE_FLATPAK=1    — prefer Flatpak before PATH `steam` (see STEAM_FLATPAK_APP)
#   `steam` on PATH
#   flatpak app installed  — default com.valvesoftware.Steam (override with STEAM_FLATPAK_APP)
set -euo pipefail

WORK="${DOCKPIPE_WORKDIR:-$PWD}"
if [[ -d "$WORK" ]]; then
  WORK="$(cd "$WORK" && pwd)"
fi

echo "[sandbox-steam] DOCKPIPE_WORKDIR (your project): ${WORK}"
echo "[sandbox-steam] Repo root (templates/scripts): ${DOCKPIPE_REPO_ROOT:-<unset>}"
echo ""

STEAM_FLATPAK_APP="${STEAM_FLATPAK_APP:-com.valvesoftware.Steam}"

flatpak_has_app() {
  command -v flatpak >/dev/null 2>&1 || return 1
  flatpak info "$1" >/dev/null 2>&1
}

# Prints one line: "path:/abs/path/to/steam" or "flatpak:AppID"
resolve_steam_launcher() {
  if [[ -n "${STEAM_CMD:-}" ]]; then
    if [[ ! -x "$STEAM_CMD" ]]; then
      echo "[sandbox-steam] STEAM_CMD is not an executable file: ${STEAM_CMD}" >&2
      return 1
    fi
    echo "path:$(cd "$(dirname "$STEAM_CMD")" && pwd)/$(basename "$STEAM_CMD")"
    return 0
  fi

  if [[ "${STEAM_USE_FLATPAK:-0}" == "1" ]] && flatpak_has_app "$STEAM_FLATPAK_APP"; then
    echo "flatpak:${STEAM_FLATPAK_APP}"
    return 0
  fi

  if command -v steam >/dev/null 2>&1; then
    echo "path:$(command -v steam)"
    return 0
  fi

  if flatpak_has_app "$STEAM_FLATPAK_APP"; then
    echo "flatpak:${STEAM_FLATPAK_APP}"
    return 0
  fi

  return 1
}

if resolved="$(resolve_steam_launcher 2>/dev/null)"; then
  case "$resolved" in
    path:*)
      echo "[sandbox-steam] Resolved Steam launcher: ${resolved#path:}"
      ;;
    flatpak:*)
      echo "[sandbox-steam] Resolved Steam launcher: flatpak run ${resolved#flatpak:}"
      ;;
  esac
else
  echo "[sandbox-steam] No Steam launcher found (install Steam, add to PATH, or install ${STEAM_FLATPAK_APP} via Flatpak)."
fi
echo ""

echo "Steam + stronger isolation (host-specific — pick what your distro supports):"
echo "  • Flatpak:  flatpak run ${STEAM_FLATPAK_APP}"
echo "  • firejail: firejail --apparmor steam   (if firejail is installed)"
echo "  • bubblewrap: wrap steam with bwrap + read-only roots (advanced)"
echo ""
echo "Optional: export STEAM_SANDBOX_LAUNCH=1 to exec the resolved launcher (blocks until Steam exits)."
echo "          Usually run Steam manually instead."
echo ""

if [[ "${STEAM_SANDBOX_LAUNCH:-0}" != "1" ]]; then
  exit 0
fi

if ! resolved="$(resolve_steam_launcher)"; then
  echo "[sandbox-steam] STEAM_SANDBOX_LAUNCH=1 but no launcher resolved; set STEAM_CMD or install Steam/Flatpak." >&2
  exit 1
fi

case "$resolved" in
  path:*)
    exec "${resolved#path:}" "$@"
    ;;
  flatpak:*)
    exec flatpak run "${resolved#flatpak:}" "$@"
    ;;
  *)
    echo "[sandbox-steam] internal error: bad resolution ${resolved}" >&2
    exit 1
    ;;
esac
