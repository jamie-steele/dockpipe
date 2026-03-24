#!/usr/bin/env bash
# Shared helpers for cursor-dev (cursor-print-next-steps.sh and cursor-dev-session.sh).
# Not meant to be run directly.

# True on Git Bash / MSYS / Windows-style environments (path conversion, tasklist, etc.).
cursor_dev_is_msysish() {
  [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]] || [[ "${OSTYPE:-}" == msys* ]] || [[ "${OSTYPE:-}" == cygwin* ]] || [[ "${OSTYPE:-}" == win32 ]]
}

cursor_dev_set_workdir() {
  W="${DOCKPIPE_WORKDIR:-$PWD}"
  W="$(cd "$W" && pwd)"
}

# Optional: set CURSOR_DEV_SKIP_DOCKER_CHECK=1 if your workflow has no container step (custom YAML).
# Loose check: used by cursor-print-next-steps.sh — if docker is missing from PATH, still returns 0
# so host-only hints can run; if docker exists but the daemon is down, fail.
cursor_dev_docker_preflight() {
  CURSOR_DEV_SKIP_DOCKER_CHECK="${CURSOR_DEV_SKIP_DOCKER_CHECK:-0}"
  if [[ "${CURSOR_DEV_SKIP_DOCKER_CHECK}" != "1" ]] && command -v docker >/dev/null 2>&1; then
    if ! docker version >/dev/null 2>&1; then
      printf '[dockpipe] Docker daemon is not reachable.\n' >&2
      printf '  Start Docker Desktop (or Linux: sudo systemctl start docker), then re-run.\n' >&2
      return 1
    fi
  fi
  return 0
}

# Strict check for cursor-dev-session.sh: session needs docker on PATH and a reachable daemon
# (unless CURSOR_DEV_SKIP_DOCKER_CHECK=1).
cursor_dev_require_docker_for_session() {
  CURSOR_DEV_SKIP_DOCKER_CHECK="${CURSOR_DEV_SKIP_DOCKER_CHECK:-0}"
  if [[ "${CURSOR_DEV_SKIP_DOCKER_CHECK}" == "1" ]]; then
    return 0
  fi
  if ! command -v docker >/dev/null 2>&1; then
    printf '[dockpipe] docker not found in PATH — cannot start the cursor-dev session container.\n' >&2
    printf '  Install Docker and ensure `docker` is on PATH, or set CURSOR_DEV_SKIP_DOCKER_CHECK=1 for a custom flow.\n' >&2
    return 1
  fi
  if ! docker version >/dev/null 2>&1; then
    printf '[dockpipe] Docker daemon is not reachable.\n' >&2
    printf '  Start Docker Desktop (or Linux: sudo systemctl start docker), then re-run.\n' >&2
    return 1
  fi
  return 0
}

# CURSOR_DEV_LAUNCH: none = print instructions only; cli = try Cursor CLI / known install paths (default).
# CURSOR_DEV_WAITABLE: set by try_launch_cursor — 1 if we spawned a launcher we can wait(1) on (includes open -a).
# CURSOR_LAUNCHED_EXE: absolute path to the Cursor binary we launched (used to detect when the GUI exited).
# CURSOR_DEV_CMD: optional path to cursor executable.

# Git Bash: pass a Windows path to Cursor.exe; MSYS2_ARG_CONV_EXCL avoids mangling the exe path.
cursor_dev_host_folder_arg() {
  local raw="${1:-}"
  if cursor_dev_is_msysish && command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$raw" 2>/dev/null || printf '%s' "$raw"
  else
    printf '%s' "$raw"
  fi
}

launch_cursor() {
  CURSOR_DEV_WAITABLE=1
  local exe="$1"
  shift
  if [[ -f "$exe" ]]; then
    :
  elif command -v "$exe" >/dev/null 2>&1; then
    exe=$(command -v "$exe")
  else
    return 1
  fi
  if command -v readlink >/dev/null 2>&1 && readlink -f "$exe" >/dev/null 2>&1; then
    CURSOR_LAUNCHED_EXE="$(readlink -f "$exe")"
  else
    CURSOR_LAUNCHED_EXE="$exe"
  fi
  local folder
  folder=$(cursor_dev_host_folder_arg "${1:-}")
  printf '[dockpipe] Opening Cursor: %s\n' "$exe" >&2
  if cursor_dev_is_msysish; then
    MSYS2_ARG_CONV_EXCL='*' "$exe" "$folder" >/dev/null 2>&1 &
  else
    "$exe" "$folder" >/dev/null 2>&1 &
  fi
  LAUNCH_PID=$!
  return 0
}

# Return 0 if ps output suggests the launched Cursor binary is still running (Linux / non-macOS Unix).
cursor_dev_cursor_gui_running_unix() {
  if [[ -z "${CURSOR_LAUNCHED_EXE:-}" ]]; then
    return 1
  fi
  if ! command -v ps >/dev/null 2>&1; then
    return 1
  fi
  if ps -ww -eo args= 2>/dev/null | grep -Fq -- "${CURSOR_LAUNCHED_EXE}"; then
    return 0
  fi
  # Fallback when ps truncates or the process argv differs (snap/flatpak).
  local base
  base=$(basename "${CURSOR_LAUNCHED_EXE}")
  if [[ -n "$base" ]] && command -v pgrep >/dev/null 2>&1; then
    pgrep -x "$base" >/dev/null 2>&1 && return 0
  fi
  return 1
}

# Block until Cursor GUI is gone (Windows: Cursor.exe; Unix: CURSOR_LAUNCHED_EXE + fallbacks).
# Uses CURSOR_DEV_POLL_SEC between polls. On Unix, waits up to CURSOR_DEV_GUI_APPEAR_SEC (default 90)
# for the GUI to show before giving up (keeps session container if Cursor never appeared).
cursor_dev_wait_for_cursor_gui_exit() {
  local _poll="${CURSOR_DEV_POLL_SEC:-1}"
  local _appear="${CURSOR_DEV_GUI_APPEAR_SEC:-90}"
  if cursor_dev_is_msysish && command -v tasklist >/dev/null 2>&1; then
    local _start _deadline
    _start=$(date +%s)
    _deadline=$((_start + _appear))
    while (( $(date +%s) < _deadline )); do
      if tasklist //FI "IMAGENAME eq Cursor.exe" 2>/dev/null | grep -qi 'Cursor.exe'; then
        printf '[cursor-dev] Waiting for Cursor.exe to exit — then stopping the session container.\n' >&2
        while tasklist //FI "IMAGENAME eq Cursor.exe" 2>/dev/null | grep -qi 'Cursor.exe'; do
          sleep "$_poll"
        done
        return 0
      fi
      sleep "$_poll"
    done
    printf '[cursor-dev] Cursor.exe did not appear within %ss — leaving the session container running (Ctrl+C or docker stop).\n' "$_appear" >&2
    return 1
  fi
  local os
  os=$(uname -s 2>/dev/null || echo unknown)
  if [[ "$os" == "Darwin" ]]; then
    local _ds _dd
    _ds=$(date +%s)
    _dd=$((_ds + _appear))
    while (( $(date +%s) < _dd )); do
      if pgrep -f 'Cursor.app' >/dev/null 2>&1; then
        printf '[cursor-dev] Waiting for Cursor.app to exit — then stopping the session container.\n' >&2
        while pgrep -f 'Cursor.app' >/dev/null 2>&1; do
          sleep "$_poll"
        done
        return 0
      fi
      sleep "$_poll"
    done
    printf '[cursor-dev] Cursor.app did not appear within %ss — leaving the session container running (Ctrl+C or docker stop).\n' "$_appear" >&2
    return 1
  fi
  # Linux / other Unix: launcher often returns before Electron exists — wait for first appearance.
  if [[ -z "${CURSOR_LAUNCHED_EXE:-}" ]]; then
    return 1
  fi
  local _start _deadline
  _start=$(date +%s)
  _deadline=$((_start + _appear))
  while (( $(date +%s) < _deadline )); do
    if cursor_dev_cursor_gui_running_unix; then
      printf '[cursor-dev] Waiting for Cursor to exit — then stopping the session container.\n' >&2
      while cursor_dev_cursor_gui_running_unix; do
        sleep "$_poll"
      done
      return 0
    fi
    sleep "$_poll"
  done
  printf '[cursor-dev] Cursor GUI did not appear within %ss — leaving the session container running (Ctrl+C or docker stop).\n' "$_appear" >&2
  return 1
}

try_launch_cursor() {
  LAUNCH_PID=""
  CURSOR_LAUNCHED_EXE=""
  CURSOR_DEV_LAUNCH="${CURSOR_DEV_LAUNCH:-cli}"
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

  # Windows: prefer native Cursor.exe before `cursor` in PATH (PATH may be a WSL shim or non-GUI launcher).
  if [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]]; then
    local _lp="${LOCALAPPDATA:-${USERPROFILE:-$HOME}/AppData/Local}"
    for c in \
      "${_lp}/Programs/cursor/Cursor.exe" \
      "/c/Users/${USER:-${USERNAME:-}}/AppData/Local/Programs/cursor/Cursor.exe" \
      "${LOCALAPPDATA}/Programs/cursor/Cursor.exe"; do
      if [[ -f "$c" ]]; then
        launch_cursor "$c" "$W"
        return 0
      fi
    done
  fi

  if command -v cursor >/dev/null 2>&1; then
    launch_cursor "$(command -v cursor)" "$W"
    return 0
  fi

  if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]]; then
    local mac_bin="/Applications/Cursor.app/Contents/Resources/app/bin/cursor"
    if [[ -x "$mac_bin" ]]; then
      launch_cursor "$mac_bin" "$W"
      return 0
    fi
    if command -v open >/dev/null 2>&1; then
      printf '[dockpipe] Opening Cursor via open -a Cursor (folder path).\n' >&2
      CURSOR_DEV_WAITABLE=1
      CURSOR_LAUNCHED_EXE="/Applications/Cursor.app/Contents/MacOS/Cursor"
      open -a Cursor "$W" >/dev/null 2>&1 &
      LAUNCH_PID=$!
      return 0
    fi
  fi

  for c in /usr/share/cursor/bin/cursor /usr/local/bin/cursor /opt/cursor/bin/cursor; do
    if [[ -x "$c" ]]; then
      launch_cursor "$c" "$W"
      return 0
    fi
  done

  return 1
}

cursor_dev_print_instructions() {
  printf '\n[cursor-dev] Next step on the host:\n' >&2
  printf '  Open the Cursor app → File → Open Folder →\n' >&2
  printf '  %s\n' "$W" >&2
}

cursor_dev_footer() {
  printf '\nRemote SSH / Dev Containers are separate setups; this template only prepares the repo.\n' >&2
  printf 'For a browser-based editor (code-server), use: dockpipe --workflow vscode\n' >&2
}
