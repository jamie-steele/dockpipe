#!/usr/bin/env bash
# PID 1 replacement for cursor-dev: bootstrap line, optional remote-session monitor, then sleep forever.
# Writes /work/.dockpipe/cursor-dev/remote_active (0|1) on each poll so the host can read state without docker exec.
# Emits connect/disconnect lines to stdout → docker logs.
set -uo pipefail

poll="${CURSOR_DEV_SESSION_POLL_SEC:-2}"
monitor="${CURSOR_DEV_CONTAINER_MONITOR:-1}"

printf '%s\n' "[dockpipe] cursor-dev: idle @ /work — remote logs: .cursor-server/ (docker logs: bootstrap + session events)"
printf '%s\n' "[dockpipe] cursor-dev: monitor polling every ${poll}s — started/ended using PIDs + (after first TCP connect) established sockets so disconnect can log \"ended\" even when server PIDs linger."
if [[ "${CURSOR_DEV_SESSION_TCP_GATE:-1}" == "1" ]] && ! command -v lsof >/dev/null 2>&1; then
  printf '%s\n' "[dockpipe] cursor-dev: warning: lsof missing — TCP disconnect detection often fails (ss hides pid= in containers). Rebuild dockpipe-base-dev (templates/core/assets/images/base-dev/Dockerfile adds lsof+iproute2)."
fi

MARKER_DIR="/work/.dockpipe/cursor-dev"
mkdir -p "$MARKER_DIR" 2>/dev/null || true

if [[ -f /dockpipe-cursor-dev-common.sh ]]; then
  # shellcheck source=/dev/null
  source /dockpipe-cursor-dev-common.sh
fi

# Full argv for Cursor/VS Code remote (ps aux often truncates long node command lines).
cursor_dev_cmdline_has_remote() {
  local line
  line="$1"
  case "$line" in
    *".cursor-server"*|*"vscode-server"*|*"cursor-reh"*|*"cursor-server"*) return 0 ;;
  esac
  return 1
}

# Process / cmdline only — no filesystem (see monitor_loop for why).
cursor_dev_compute_active_process() {
  local active=0
  local _ps _line f
  if command -v pgrep >/dev/null 2>&1; then
    pgrep -f '[.]cursor-server' >/dev/null 2>&1 && active=1
    [[ "$active" -eq 0 ]] && pgrep -f 'vscode-server' >/dev/null 2>&1 && active=1
    [[ "$active" -eq 0 ]] && pgrep -f 'cursor-reh' >/dev/null 2>&1 && active=1
  fi
  if [[ "$active" -eq 0 ]]; then
    _ps=$(ps auxww 2>/dev/null || ps aux 2>/dev/null || true)
    # Avoid a bare "cursor-server" substring — it false-positives and blocks "remote session ended".
    if echo "$_ps" | grep -qF ".cursor-server"; then active=1; fi
    if [[ "$active" -eq 0 ]] && echo "$_ps" | grep -qF "vscode-server"; then active=1; fi
    if [[ "$active" -eq 0 ]] && echo "$_ps" | grep -qF "cursor-reh"; then active=1; fi
    if [[ "$active" -eq 0 ]] && echo "$_ps" | grep -qE '(^|/)cursor-server/|cursor-server --'; then active=1; fi
  fi
  if [[ "$active" -eq 0 ]]; then
    _line=$( (ps -eww -o args= 2>/dev/null || ps -e -o args= 2>/dev/null; echo "$_ps") | tr '\n' ' ' )
    case "$_line" in
      *vscode-server*|*.cursor-server*|*cursor-reh*|*/cursor-server/*|*"cursor-server --"*) active=1 ;;
    esac
  fi
  if [[ "$active" -eq 0 ]] && [[ -d /proc ]]; then
    for f in /proc/[0-9]*/cmdline; do
      [[ -r "$f" ]] || continue
      _line=$(tr '\0' ' ' <"$f" 2>/dev/null || true)
      if cursor_dev_cmdline_has_remote "$_line"; then active=1; break; fi
    done
  fi
  printf '%s' "$active"
}

# True if any cursor/vscode remote server PID has an ESTABLISHED TCP socket (client actually connected).
# Prefer lsof: ss -tnp often omits pid= in user-namespaced containers, so grep never matched and "ended" never logged.
# Disconnect drops ESTAB while Node PIDs linger — that transition is what we need for docker logs.
cursor_dev_remote_tcp_connected() {
  local pid pids ss_out
  pids=$(pgrep -f '[.]cursor-server' 2>/dev/null; pgrep -f 'vscode-server' 2>/dev/null; pgrep -f 'cursor-reh' 2>/dev/null)
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
  local prev=0 active proc seen_proc_ever=0 seen_tcp_connected=0 tcp=0
  local _tick="${CURSOR_DEV_SESSION_LOG_HEARTBEAT_SEC:-0}"
  local tcp_gate="${CURSOR_DEV_SESSION_TCP_GATE:-1}"
  while true; do
    proc=$(cursor_dev_compute_active_process)
    [[ "$proc" -eq 1 ]] && seen_proc_ever=1

    tcp=0
    # lsof or ss — do not require ss only (lsof works when -tnp pid= is missing in logs)
    if [[ "$tcp_gate" == "1" ]] && { command -v lsof >/dev/null 2>&1 || command -v ss >/dev/null 2>&1; }; then
      cursor_dev_remote_tcp_connected && tcp=1
    fi
    [[ "$tcp" -eq 1 ]] && seen_tcp_connected=1

    active=$proc
    # FS bootstrap only when we never saw remote processes: stale .cursor-server mtimes after disconnect
    # were keeping active=1 forever, so we never printed "remote session ended".
    if [[ "$proc" -eq 0 ]] && [[ "$seen_proc_ever" -eq 0 ]] && declare -F cursor_dev_fs_remote_active >/dev/null 2>&1; then
      cursor_dev_fs_remote_active "/work/.cursor-server" && active=1
    fi

    # After we've seen at least one ESTAB socket for the server, require tcp=1 for active=1. Disconnect clears
    # ESTAB while PIDs often remain — without this, docker logs never show "remote session ended".
    if [[ "$tcp_gate" == "1" ]] && { command -v lsof >/dev/null 2>&1 || command -v ss >/dev/null 2>&1; } && [[ "$seen_tcp_connected" -eq 1 ]]; then
      active=0
      [[ "$proc" -eq 1 ]] && [[ "$tcp" -eq 1 ]] && active=1
    fi

    write_marker "$active"
    if [[ "$prev" -eq 0 && "$active" -eq 1 ]]; then
      printf '%s\n' "[dockpipe] cursor-dev: remote session started"
    elif [[ "$prev" -eq 1 && "$active" -eq 0 ]]; then
      printf '%s\n' "[dockpipe] cursor-dev: remote session ended"
    fi
    if [[ "$_tick" =~ ^[0-9]+$ ]] && [[ "$_tick" -gt 0 ]]; then
      if [[ -z "${_hb:-}" ]] || (( $(date +%s) >= _hb + _tick )); then
        printf '%s\n' "[dockpipe] cursor-dev: monitor heartbeat active=%s proc=%s tcp=%s seen_tcp=%s poll=%ss" "$active" "$proc" "$tcp" "$seen_tcp_connected" "$poll"
        _hb=$(date +%s)
      fi
    fi
    prev=$active
    sleep "$poll"
  done
}

if [[ "$monitor" == "1" ]]; then
  monitor_loop &
fi

# Do not use exec: keep this shell alive as the main process so the background monitor stays attached.
sleep infinity
