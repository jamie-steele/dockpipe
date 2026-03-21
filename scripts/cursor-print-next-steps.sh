#!/usr/bin/env bash
# Host-only follow-up (skip_container step). Not a Cursor product integration.
# Same configuration ideas as the vscode template: vars / env / optional launcher, documented in README.
set -euo pipefail

W="${DOCKPIPE_WORKDIR:-$PWD}"
W="$(cd "$W" && pwd)"

# CURSOR_DEV_LAUNCH: none = print instructions only; cli = try Cursor CLI / known install paths (default).
CURSOR_DEV_LAUNCH="${CURSOR_DEV_LAUNCH:-cli}"
# CURSOR_DEV_WAIT: 1 = wait for the launcher process we started (best-effort; may attach to existing Cursor).
CURSOR_DEV_WAIT="${CURSOR_DEV_WAIT:-0}"
# Override executable path when auto-detection is wrong.
CURSOR_DEV_CMD="${CURSOR_DEV_CMD:-}"

launch_cursor() {
  local exe="$1"
  shift
  if [[ -f "$exe" ]]; then
    :
  elif command -v "$exe" >/dev/null 2>&1; then
    exe=$(command -v "$exe")
  else
    return 1
  fi
  printf '[dockpipe] Opening Cursor: %s\n' "$exe" >&2
  "$exe" "$@" >/dev/null 2>&1 &
  LAUNCH_PID=$!
  return 0
}

try_launch_cursor() {
  LAUNCH_PID=""
  if [[ "${CURSOR_DEV_LAUNCH}" == "none" ]] || [[ "${CURSOR_DEV_LAUNCH}" == "0" ]]; then
    return 1
  fi
  if [[ "${CURSOR_DEV_LAUNCH}" != "cli" ]]; then
    printf '[dockpipe] Unknown CURSOR_DEV_LAUNCH=%s (use none or cli).\n' "${CURSOR_DEV_LAUNCH}" >&2
    return 1
  fi

  if [[ -n "${CURSOR_DEV_CMD:-}" ]]; then
    if launch_cursor "${CURSOR_DEV_CMD}" "$W"; then
      return 0
    fi
    printf '[dockpipe] CURSOR_DEV_CMD not found or not runnable: %s\n' "${CURSOR_DEV_CMD}" >&2
    return 1
  fi

  if command -v cursor >/dev/null 2>&1; then
    launch_cursor "$(command -v cursor)" "$W"
    return 0
  fi

  # Windows (Git Bash): typical install location
  if [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]]; then
    local _lp="${LOCALAPPDATA:-${USERPROFILE:-$HOME}/AppData/Local}"
    for c in \
      "${_lp}/Programs/cursor/Cursor.exe" \
      "/c/Users/${USER}/AppData/Local/Programs/cursor/Cursor.exe" \
      "${LOCALAPPDATA}/Programs/cursor/Cursor.exe"; do
      if [[ -f "$c" ]]; then
        launch_cursor "$c" "$W"
        return 0
      fi
    done
  fi

  # macOS
  if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]]; then
    local mac_bin="/Applications/Cursor.app/Contents/Resources/app/bin/cursor"
    if [[ -x "$mac_bin" ]]; then
      launch_cursor "$mac_bin" "$W"
      return 0
    fi
    if command -v open >/dev/null 2>&1; then
      printf '[dockpipe] Opening Cursor via open -a Cursor (folder path).\n' >&2
      open -a Cursor "$W" >/dev/null 2>&1 &
      LAUNCH_PID=$!
      return 0
    fi
  fi

  # Linux: common locations
  for c in /usr/share/cursor/bin/cursor /usr/local/bin/cursor /opt/cursor/bin/cursor; do
    if [[ -x "$c" ]]; then
      launch_cursor "$c" "$W"
      return 0
    fi
  done

  return 1
}

printf '\n[cursor-dev] Next step on the host:\n' >&2
printf '  Open the Cursor app → File → Open Folder →\n' >&2
printf '  %s\n' "$W" >&2

if try_launch_cursor; then
  if [[ "${CURSOR_DEV_WAIT}" == "1" ]] && [[ -n "${LAUNCH_PID:-}" ]]; then
    printf '[dockpipe] Waiting for launcher PID %s (Ctrl+C stops dockpipe).\n' "${LAUNCH_PID}" >&2
    wait "${LAUNCH_PID}" || true
  fi
else
  printf '\n[cursor-dev] Cursor CLI not found in PATH and no default install path matched.\n' >&2
  printf '  Install Cursor from https://cursor.com and/or add the “cursor” shell command from the app.\n' >&2
fi

printf '\nRemote SSH / Dev Containers are separate setups; this template only prepares the repo.\n' >&2
printf 'For a browser-based editor (code-server), use: dockpipe --workflow vscode\n' >&2
