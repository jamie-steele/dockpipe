#!/usr/bin/env bash
# PID 1 helper for vscode remote sessions.
set -uo pipefail

poll="${VSCODE_SESSION_POLL_SEC:-2}"
monitor="${VSCODE_CONTAINER_MONITOR:-1}"
MARKER_DIR="${DOCKPIPE_PACKAGE_STATE_DIR:?DOCKPIPE_PACKAGE_STATE_DIR is required}"
remote_server_dir="${HOME:?HOME is required}/.vscode-server"

printf '%s\n' "[dockpipe] vscode: idle @ /work — remote state lives under package state"

mkdir -p "$MARKER_DIR" 2>/dev/null || true

if [[ -f /dockpipe-vscode-common.sh ]]; then
  # shellcheck source=/dev/null
  source /dockpipe-vscode-common.sh
fi

printf '%s\n' "[dockpipe] vscode: remote state root: ${MARKER_DIR}"

vscode_compute_active_process() {
  if command -v pgrep >/dev/null 2>&1; then
    pgrep -f 'vscode-server' >/dev/null 2>&1 && return 0
    pgrep -f 'code-reh' >/dev/null 2>&1 && return 0
  fi
  local line f
  line=$(ps -eww -o args= 2>/dev/null | tr '\n' ' ')
  case "$line" in
    *vscode-server*|*code-reh*) return 0 ;;
  esac
  if [[ -d /proc ]]; then
    for f in /proc/[0-9]*/cmdline; do
      [[ -r "$f" ]] || continue
      line=$(tr '\0' ' ' <"$f" 2>/dev/null || true)
      case "$line" in
        *vscode-server*|*code-reh*) return 0 ;;
      esac
    done
  fi
  return 1
}

vscode_remote_tcp_connected() {
  local pid pids ss_out
  pids=$(pgrep -f 'vscode-server' 2>/dev/null; pgrep -f 'code-reh' 2>/dev/null)
  for pid in $pids; do
    [[ "$pid" =~ ^[0-9]+$ ]] || continue
    if command -v lsof >/dev/null 2>&1; then
      if lsof -nP -a -p "$pid" -iTCP -sTCP:ESTABLISHED 2>/dev/null | grep -q .; then
        return 0
      fi
    fi
  done
  if command -v ss >/dev/null 2>&1; then
    ss_out=$(ss -tnp state established 2>/dev/null) || true
    [[ -z "$ss_out" ]] && return 1
    for pid in $pids; do
      [[ "$pid" =~ ^[0-9]+$ ]] || continue
      if printf '%s\n' "$ss_out" | grep -qE "pid=${pid}[,)]"; then
        return 0
      fi
      if printf '%s\n' "$ss_out" | grep -qF "pid=$pid"; then
        return 0
      fi
    done
  fi
  return 1
}

write_marker() {
  local v="$1"
  local tmp="${MARKER_DIR}/remote_active.tmp"
  printf '%s' "$v" >"$tmp" && mv -f "$tmp" "${MARKER_DIR}/remote_active"
}

monitor_loop() {
  local prev=0 active proc seen_proc_ever=0 seen_tcp_connected=0 tcp=0 disconnect_streak=0
  local disconnect_need="${VSCODE_REMOTE_TCP_IDLE_POLLS:-6}"
  while true; do
    active=0
    proc=0
    if vscode_compute_active_process; then
      proc=1
      seen_proc_ever=1
    fi
    tcp=0
    if command -v lsof >/dev/null 2>&1 || command -v ss >/dev/null 2>&1; then
      vscode_remote_tcp_connected && tcp=1
    fi
    [[ "$tcp" -eq 1 ]] && seen_tcp_connected=1

    active=$proc
    if [[ "$proc" -eq 0 ]] && [[ "$seen_proc_ever" -eq 0 ]] && declare -F vscode_fs_remote_active >/dev/null 2>&1; then
      if vscode_fs_remote_active "$remote_server_dir"; then
        active=1
      fi
    fi
    if [[ "$seen_tcp_connected" -eq 1 ]]; then
      if [[ "$proc" -eq 1 ]] && [[ "$tcp" -eq 1 ]]; then
        disconnect_streak=0
        active=1
      elif [[ "$proc" -eq 1 ]]; then
        ((disconnect_streak++)) || true
        if (( disconnect_streak < disconnect_need )); then
          active=1
        else
          active=0
        fi
      else
        active=0
      fi
    fi
    if [[ "$active" -eq 0 ]] && [[ "$proc" -eq 0 ]] && [[ "$seen_tcp_connected" -eq 0 ]] && declare -F vscode_fs_remote_active >/dev/null 2>&1; then
      active=1
      vscode_fs_remote_active "$remote_server_dir" || active=0
    fi
    write_marker "$active"
    if [[ "$prev" -eq 0 && "$active" -eq 1 ]]; then
      printf '%s\n' "[dockpipe] vscode: remote session started"
    elif [[ "$prev" -eq 1 && "$active" -eq 0 ]]; then
      printf '%s\n' "[dockpipe] vscode: remote session ended"
    fi
    prev=$active
    sleep "$poll"
  done
}

if [[ "$monitor" == "1" ]]; then
  monitor_loop &
fi

sleep infinity
