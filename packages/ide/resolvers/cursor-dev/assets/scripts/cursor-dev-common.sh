#!/usr/bin/env bash
# Shared helpers for cursor-dev (cursor-print-next-steps.sh and cursor-dev-session.sh).
# Not meant to be run directly.

# True on Git Bash / MSYS / Windows-style environments (path conversion, tasklist, etc.).
cursor_dev_is_msysish() {
  [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]] || [[ "${OSTYPE:-}" == msys* ]] || [[ "${OSTYPE:-}" == cygwin* ]] || [[ "${OSTYPE:-}" == win32 ]]
}

# Invoke docker with MSYS path conversion disabled when needed (Git Bash + docker.exe).
cursor_dev_docker() {
  if cursor_dev_is_msysish; then
    MSYS2_ARG_CONV_EXCL="*" docker "$@"
  else
    docker "$@"
  fi
}

cursor_dev_set_workdir() {
  eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
  W="$(dockpipe_sdk workdir)"
  export W
}

cursor_dev_global_state_root() {
  if [[ -n "${DOCKPIPE_STATE_DIR:-}" ]]; then
    printf '%s' "$DOCKPIPE_STATE_DIR"
  else
    printf '%s/bin/.dockpipe' "$W"
  fi
}

cursor_dev_state_root() {
  if [[ -n "${DOCKPIPE_PACKAGE_STATE_DIR:-}" ]]; then
    printf '%s' "$DOCKPIPE_PACKAGE_STATE_DIR"
  else
    printf '%s/packages/cursor-dev' "$(cursor_dev_global_state_root)"
  fi
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
# CURSOR_DEV_DOCKER_WAIT_SEC: seconds to poll for docker info (default 120). Set 0 to fail immediately
# if the daemon is down (old behavior).
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
  local max="${CURSOR_DEV_DOCKER_WAIT_SEC:-120}"
  if [[ "${max}" == "0" ]]; then
    if ! docker info >/dev/null 2>&1; then
      printf '[dockpipe] Docker daemon is not reachable.\n' >&2
      printf '  Start Docker Desktop (or Linux: sudo systemctl start docker), then re-run.\n' >&2
      return 1
    fi
    return 0
  fi
  local i=0
  while (( i < max )); do
    if docker info >/dev/null 2>&1; then
      (( i > 0 )) && printf '[dockpipe] Docker daemon is up.\n' >&2
      return 0
    fi
    if (( i == 0 )); then
      printf '[dockpipe] Waiting for Docker daemon (up to %ss)…\n' "$max" >&2
    fi
    sleep 1
    ((i++)) || true
  done
  printf '[dockpipe] Docker daemon is not reachable after %ss.\n' "$max" >&2
  printf '  Start Docker Desktop (or Linux: sudo systemctl start docker), then re-run.\n' >&2
  printf '  Or set CURSOR_DEV_DOCKER_WAIT_SEC=0 to fail immediately.\n' >&2
  return 1
}

# Wait until the session container is running (docker run -d can return before status is "running").
cursor_dev_wait_container_running() {
  local name="$1"
  local max="${2:-120}"
  local i=0
  while (( i < max )); do
    local st
    st=$(cursor_dev_docker inspect -f '{{.State.Status}}' "$name" 2>/dev/null) || return 1
    if [[ "$st" == "running" ]]; then
      return 0
    fi
    sleep 0.5
    ((i++)) || true
  done
  return 1
}

# Hex-encode a string the same way VS Code / Cursor use for dev-container authority (UTF-8 bytes → hex digits).
cursor_dev_hex_encode_utf8() {
  printf '%s' "$1" | od -A n -t x1 | tr -d ' \n'
}

# Build vscode-remote folder URI to attach to a running container and open path_in_container (e.g. /work).
# Matches Dev Containers "Attach to Running Container" (settingType container + short container id).
cursor_dev_dev_container_folder_uri() {
  local container_name="$1"
  local path_in_container="${2:-/work}"
  local cid
  cid=$(cursor_dev_docker inspect -f '{{.Id}}' "$container_name" 2>/dev/null) || return 1
  cid=${cid#sha256:}
  local short="${cid:0:12}"
  local json
  json=$(printf '{"settingType":"container","containerId":"%s"}' "$short")
  local hex
  hex=$(cursor_dev_hex_encode_utf8 "$json")
  printf 'vscode-remote://dev-container+%s%s' "$hex" "$path_in_container"
}

# Resolve path to Cursor CLI / app for launching (stdout = path). Returns 1 if not found.
cursor_dev_resolve_cursor_exe() {
  if [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]]; then
    local _lp="${LOCALAPPDATA:-${USERPROFILE:-$HOME}/AppData/Local}"
    for c in \
      "${_lp}/Programs/cursor/Cursor.exe" \
      "/c/Users/${USER:-${USERNAME:-}}/AppData/Local/Programs/cursor/Cursor.exe" \
      "${LOCALAPPDATA}/Programs/cursor/Cursor.exe"; do
      if [[ -f "$c" ]]; then
        printf '%s' "$c"
        return 0
      fi
    done
  fi
  if command -v cursor >/dev/null 2>&1; then
    command -v cursor
    return 0
  fi
  if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]]; then
    local mac_bin="/Applications/Cursor.app/Contents/Resources/app/bin/cursor"
    if [[ -x "$mac_bin" ]]; then
      printf '%s' "$mac_bin"
      return 0
    fi
  fi
  for c in /usr/share/cursor/bin/cursor /usr/local/bin/cursor /opt/cursor/bin/cursor; do
    if [[ -x "$c" ]]; then
      printf '%s' "$c"
      return 0
    fi
  done
  return 1
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

# Open Cursor already attached to the session container (Dev Containers) at workspace path inside the container.
launch_cursor_folder_uri() {
  CURSOR_DEV_WAITABLE=1
  local exe="$1"
  local uri="$2"
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
  printf '[dockpipe] Opening Cursor attached to container (--folder-uri) at /work\n' >&2
  if cursor_dev_is_msysish; then
    MSYS2_ARG_CONV_EXCL='*' "$exe" --folder-uri "$uri" >/dev/null 2>&1 &
  else
    "$exe" --folder-uri "$uri" >/dev/null 2>&1 &
  fi
  LAUNCH_PID=$!
  return 0
}

# Regex fragment for pgrep -f: Cursor argv paths (user installs). Keep in sync with cursor_dev_host_cursor_localhost_tcp_active.
# Covers ~/.local/share/cursor, Flatpak (com.cursor.Cursor), AppImage, snap, system packages.
# Override with CURSOR_DEV_PGREP_CURSOR_PATHS (full ERE string) for exotic installs.
cursor_dev_pgrep_cursor_path_fragment() {
  if [[ -n "${CURSOR_DEV_PGREP_CURSOR_PATHS:-}" ]]; then
    printf '%s' "$CURSOR_DEV_PGREP_CURSOR_PATHS"
    return 0
  fi
  printf '%s' '/usr/share/cursor|/opt/cursor|/opt/Cursor|/snap/cursor|flatpak.*[Cc]ursor|/AppImage|cursor-app|\.local/share/cursor|/.var/app/com.cursor.Cursor|cursor/Cursor'
}

# Return 0 if ps output suggests the launched Cursor binary is still running (Linux / non-macOS Unix).
# Scope to current uid so we do not match another user's "cursor" or daemon.
cursor_dev_cursor_gui_running_unix() {
  local uid _frag
  uid=$(id -u)
  _frag="$(cursor_dev_pgrep_cursor_path_fragment)"
  if command -v pgrep >/dev/null 2>&1; then
    pgrep -u "$uid" -x cursor >/dev/null 2>&1 && return 0
    pgrep -u "$uid" -x Cursor >/dev/null 2>&1 && return 0
    # AppImage / snap / flatpak / user-local / install paths (argv often does not match CURSOR_LAUNCHED_EXE).
    pgrep -u "$uid" -f "$_frag" >/dev/null 2>&1 && return 0
    # Electron argv patterns (process name is not always "cursor" for -x).
    pgrep -u "$uid" -f 'cursor --type|/cursor/Cursor|Cursor\.AppImage|cursor-renderer|cursor\.exe' >/dev/null 2>&1 && return 0
  fi
  # Linux: executable path — avoid bare *cursor* (matches unrelated tools); require known Cursor install shapes.
  if [[ "$(uname -s 2>/dev/null)" == "Linux" ]] && [[ -d /proc ]]; then
    local d o t
    for d in /proc/[0-9]*; do
      [[ -r "$d/exe" ]] || continue
      t=$(readlink -f "$d/exe" 2>/dev/null || readlink "$d/exe" 2>/dev/null) || continue
      [[ "$t" == *"(deleted)"* ]] && continue
      case "$t" in
        */opt/cursor/*|*/opt/Cursor/*|*/usr/share/cursor/*|*/snap/cursor/*|*/AppImage/*[Cc]ursor*|*.AppImage/*cursor*|*.AppImage/*Cursor*|*/cursor/Cursor|*/Cursor/Cursor|*/cursor/cursor) ;;
        *"/.local/share/cursor/"*) ;;
        */.var/app/com.cursor.Cursor/*) ;;
        *) continue ;;
      esac
      o=$(stat -c %u "$d" 2>/dev/null) || continue
      [[ "$o" == "$uid" ]] && return 0
    done
  fi
  # Last resort on Linux: /proc/*/comm is short (15 chars) but reliable for the main binary name.
  if [[ "$(uname -s 2>/dev/null)" == "Linux" ]] && [[ -d /proc ]]; then
    local d c o
    for d in /proc/[0-9]*; do
      [[ -r "$d/comm" ]] || continue
      read -r c <"$d/comm" 2>/dev/null || continue
      case "$c" in
        cursor|Cursor)
          o=$(stat -c %u "$d" 2>/dev/null) || continue
          [[ "$o" == "$uid" ]] && return 0
          ;;
      esac
    done
  fi
  if [[ -n "${CURSOR_LAUNCHED_EXE:-}" ]] && command -v ps >/dev/null 2>&1; then
    if ps -u "$uid" -ww -o args= 2>/dev/null | grep -Fq -- "${CURSOR_LAUNCHED_EXE}"; then
      return 0
    fi
    local base
    base=$(basename "${CURSOR_LAUNCHED_EXE}")
    if [[ -n "$base" ]] && command -v pgrep >/dev/null 2>&1; then
      pgrep -u "$uid" -x "$base" >/dev/null 2>&1 && return 0
    fi
  fi
  return 1
}

# True if Cursor (host) appears to be running — used for shutdown detection on all OSes.
cursor_dev_host_cursor_running() {
  if cursor_dev_is_msysish && command -v tasklist >/dev/null 2>&1; then
    if tasklist //FI "IMAGENAME eq Cursor.exe" 2>/dev/null | grep -qi 'Cursor.exe'; then
      return 0
    fi
    return 1
  fi
  if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]]; then
    if pgrep -f 'Cursor.app' >/dev/null 2>&1; then
      return 0
    fi
    return 1
  fi
  cursor_dev_cursor_gui_running_unix
  return $?
}

# Age of file in seconds (for marker freshness). Returns 1 if stat fails.
cursor_dev_age_seconds_of_file() {
  local f="$1"
  local now mt
  now=$(date +%s)
  if [[ "$(uname -s)" == "Darwin" ]]; then
    mt=$(stat -f %m "$f" 2>/dev/null) || return 1
  else
    mt=$(stat -c %Y "$f" 2>/dev/null) || return 1
  fi
  printf '%s' "$((now - mt))"
}

# Recent writes under the Cursor server data dir (logs, IPC, state). Like vscode's "TCP still has clients" signal:
# when the remote stops, churn here usually drops off. Tunable via CURSOR_DEV_REMOTE_FS_QUIET_SEC.
cursor_dev_fs_remote_active() {
  local dir="${1:-}"
  if [[ -z "$dir" ]]; then
    local w="${W:-/work}"
    dir="${CURSOR_DEV_CURSOR_SERVER_DIR:-$w/.cursor-server}"
  fi
  local quiet="${CURSOR_DEV_REMOTE_FS_QUIET_SEC:-90}"
  [[ "${CURSOR_DEV_REMOTE_FS_SIGNAL:-1}" == "1" ]] || return 1
  [[ -d "$dir" ]] || return 1
  find "$dir" -type f -newermt "-${quiet} seconds" -print -quit 2>/dev/null | grep -q .
}

# Host has an established TCP session involving the container's bridge IP (Cursor ↔ remote server traffic).
# Mirrors vscode-code-server.sh counting ESTABLISHED to 127.0.0.1:PORT — here we key off the container address.
cursor_dev_tcp_session_to_container() {
  local name="$1"
  [[ "${CURSOR_DEV_REMOTE_TCP_SIGNAL:-1}" == "1" ]] || return 1
  local ip
  ip=$(cursor_dev_docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$name" 2>/dev/null) || return 1
  [[ -n "$ip" ]] || return 1
  # Match IP as address (e.g. 172.17.0.3:443) — avoids false substring matches vs longer IPs.
  if command -v ss >/dev/null 2>&1; then
    ss -tn state established 2>/dev/null | grep -qF "${ip}:" && return 0
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP -sTCP:ESTABLISHED 2>/dev/null | grep -qF "${ip}:" && return 0
  fi
  if [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]] && command -v netstat.exe >/dev/null 2>&1; then
    netstat.exe -ano 2>/dev/null | grep -qF "${ip}:" && return 0
  fi
  return 1
}

# Dev Containers / Cursor forward the remote through 127.0.0.1:<port> on the host (same shape as vscode counting
# ESTABLISHED to localhost — not always visible as TCP to the container bridge IP).
cursor_dev_host_cursor_localhost_tcp_active() {
  [[ "${CURSOR_DEV_REMOTE_HOST_LOCALHOST_TCP:-1}" == "1" ]] || return 1
  command -v lsof >/dev/null 2>&1 || return 1
  local uid _frag
  uid=$(id -u)
  _frag="$(cursor_dev_pgrep_cursor_path_fragment)"
  local pids
  pids=$(
    {
      pgrep -u "$uid" -x cursor 2>/dev/null || true
      pgrep -u "$uid" -x Cursor 2>/dev/null || true
      pgrep -u "$uid" -f "$_frag" 2>/dev/null || true
    } | sort -u
  )
  [[ -n "$pids" ]] || return 1
  local pid
  for pid in $pids; do
    [[ "$pid" =~ ^[0-9]+$ ]] || continue
    if lsof -nP -a -p "$pid" -iTCP -sTCP:ESTABLISHED 2>/dev/null | grep -qE '127\.0\.0\.1:[0-9]+|::1:[0-9]+|\[::1\]:[0-9]+'; then
      return 0
    fi
  done
  return 1
}

# pgrep/ps probe inside the session container (docker exec fallback for cursor_dev_container_remote_server_running).
_cursor_remote_exec_ps_probe() {
  local _n="$1"
  cursor_dev_docker exec "$_n" sh -c '
    if command -v pgrep >/dev/null 2>&1; then
      pgrep -f "[.]cursor-server" >/dev/null 2>&1 && exit 0
      pgrep -f "vscode-server" >/dev/null 2>&1 && exit 0
      pgrep -f "cursor-reh" >/dev/null 2>&1 && exit 0
    fi
    _ps=$(ps auxww 2>/dev/null || ps aux 2>/dev/null || true)
    echo "$_ps" | grep -qF ".cursor-server" && exit 0
    echo "$_ps" | grep -qF "vscode-server" && exit 0
    echo "$_ps" | grep -qF "cursor-reh" && exit 0
    echo "$_ps" | grep -qF "cursor-server" && exit 0
    _line=$( (ps -eww -o args= 2>/dev/null || ps -e -o args= 2>/dev/null; echo "$_ps") | tr "\n" " " )
    case "$_line" in
      *cursor-server*|*vscode-server*|*.cursor-server*|*cursor-reh*) exit 0 ;;
    esac
    if [ -d /proc ]; then
      for f in /proc/[0-9]*/cmdline; do
        [ -r "$f" ] || continue
        _line=$(tr "\0" " " <"$f" 2>/dev/null || true)
        case "$_line" in
          *.cursor-server*|*vscode-server*|*cursor-reh*|*cursor-server*) exit 0 ;;
        esac
      done
    fi
    exit 1
  ' 2>/dev/null
}

# True if a Dev Containers / Cursor remote server process is running inside the session container.
# $2 probe: "full" (default) — marker, bridge TCP, host localhost TCP, host .cursor-server fs, docker exec.
#   "dual" — for cursor_dev_wait_dual_session_end only: trust fresh remote_active from the container (same as
#   docker logs); do NOT use host .cursor-server or host localhost TCP (those stay "hot" after disconnect and
#   prevented remote idle + host-quit detection). Stale/missing marker still uses bridge TCP + docker exec.
cursor_dev_container_remote_server_running() {
  local name="$1"
  local probe="${2:-${CURSOR_DEV_REMOTE_PROBE:-full}}"
  local max_age="${CURSOR_DEV_MARKER_MAX_AGE_SEC:-60}"
  [[ -n "$name" ]] || return 1
  if [[ -n "${W:-}" ]]; then
    local mf age v
    mf="$(cursor_dev_state_root)/remote_active"
    if [[ -f "$mf" ]]; then
      age=$(cursor_dev_age_seconds_of_file "$mf") || age=999999
      if [[ "$age" -lt "$max_age" ]]; then
        v=$(tr -d ' \n' <"$mf" 2>/dev/null || echo 0)
        if [[ "$v" == "1" ]]; then
          return 0
        fi
        # dual: fresh 0 from session-idle is authoritative (host-side FS/TCP are not).
        if [[ "$probe" == "dual" ]] && [[ "$v" == "0" ]]; then
          return 1
        fi
        # full: v == 0 — fall through (monitor can miss argv; host probes help)
      fi
    fi
  fi
  if cursor_dev_tcp_session_to_container "$name"; then
    return 0
  fi
  if [[ "$probe" != "dual" ]]; then
    if cursor_dev_host_cursor_localhost_tcp_active; then
      return 0
    fi
    if [[ -n "${W:-}" ]] && cursor_dev_fs_remote_active "$(cursor_dev_state_root)/home/.cursor-server"; then
      return 0
    fi
  fi
  # Prefer pgrep + full argv (ps aux truncates long node lines). /proc scan works without procps.
  # Optional timeout avoids docker exec blocking the next poll (host quit is checked first each iteration).
  local _dex="${CURSOR_DEV_DOCKER_EXEC_TIMEOUT_SEC:-12}"
  if command -v timeout >/dev/null 2>&1 && [[ "$_dex" =~ ^[0-9]+$ ]] && (( _dex > 0 )); then
    # Subshell needs cursor_dev_docker + deps (declare -f); printf %q for container name.
    timeout "$_dex" bash -c "$(declare -f cursor_dev_is_msysish cursor_dev_docker _cursor_remote_exec_ps_probe); _cursor_remote_exec_ps_probe $(printf %q "$name")"
  else
    _cursor_remote_exec_ps_probe "$name"
  fi
}

# Stop when (1) in-container remote was seen then idle for empty_need polls, or (2) host Cursor is gone after we
# once saw host or remote activity. (2) uses host_seen OR server_seen so abrupt app kill still stops the session
# even if pgrep never matched "Cursor" on the host but remote was up.
#
# We do not stop on host gone before any host_seen or server_seen — avoids tearing down before first attach.
# Optional: if the appear window passes with no host or remote signal, set CURSOR_DEV_HOST_CLEAR_POLLS_NO_SEEN
# to a positive number to stop after that many consecutive polls where BOTH host and remote probes are quiet
# (default 0 = off — avoids tearing down a long-idle container you plan to attach to later).
cursor_dev_wait_dual_session_end() {
  local name="$1"
  local _poll="${CURSOR_DEV_POLL_SEC:-1}"
  local _appear="${CURSOR_DEV_GUI_APPEAR_SEC:-90}"
  local empty_need="${CURSOR_DEV_REMOTE_SERVER_IDLE_POLLS:-3}"
  local no_seen_need="${CURSOR_DEV_HOST_CLEAR_POLLS_NO_SEEN:-0}"
  local server_seen=0
  local host_seen=0
  local empty_streak=0
  local never_seen=0
  local no_signal_streak=0
  local _start _deadline _remote _hr
  _start=$(date +%s)
  _deadline=$((_start + _appear))
  while (( $(date +%s) < _deadline )); do
    if cursor_dev_host_cursor_running; then
      host_seen=1
      printf '[cursor-dev] Cursor running on host — monitoring for session end.\n' >&2
      break
    fi
    if cursor_dev_container_remote_server_running "$name" dual; then
      server_seen=1
      printf '[cursor-dev] Remote server in container — monitoring for session end.\n' >&2
      break
    fi
    sleep "$_poll"
  done
  if [[ "$host_seen" == "0" ]] && [[ "$server_seen" == "0" ]]; then
    never_seen=1
    printf '[cursor-dev] No Cursor / remote server detected within %ss — still monitoring (slow start is OK).\n' "$_appear" >&2
  fi
  while true; do
    _hr=0
    _remote=0
    if cursor_dev_host_cursor_running; then
      host_seen=1
      never_seen=0
      _hr=1
    fi
    # Host must be evaluated before remote checks: cursor_dev_container_remote_server_running can block on
    # docker exec or stay true via host .cursor-server mtime — both delay or prevent abrupt-quit detection.
    if [[ "${CURSOR_DEV_WAIT_DEBUG:-0}" == "1" ]]; then
      printf '[cursor-dev] debug (pre-remote): host_seen=%s host_running=%s server_seen=%s never_seen=%s\n' \
        "$host_seen" "$_hr" "$server_seen" "$never_seen" >&2
    fi
    if [[ "$_hr" == "0" ]] && { [[ "$host_seen" == "1" ]] || [[ "$server_seen" == "1" ]]; }; then
      printf '[cursor-dev] Cursor not running on host — stopping session container (quit, killed, or abrupt disconnect).\n' >&2
      return 0
    fi
    if cursor_dev_container_remote_server_running "$name" dual; then
      _remote=1
      server_seen=1
      never_seen=0
      empty_streak=0
    else
      _remote=0
      if [[ "$server_seen" == "1" ]]; then
        ((empty_streak++)) || true
        if (( empty_streak >= empty_need )); then
          printf '[cursor-dev] Remote server in container stopped — stopping session container.\n' >&2
          return 0
        fi
      fi
    fi
    # Deadlock escape: if we never saw host or remote in the appear window, we used to require host_seen ||
    # server_seen before treating host quit — but then quit with missed detection never stopped. Only count
    # streak when BOTH probes say quiet (host not running + remote not up), so an attached session does not
    # get torn down just because pgrep missed Cursor on the host.
    if [[ "$_hr" == "0" ]] && [[ "$never_seen" == "1" ]] && [[ "$_remote" == "0" ]]; then
      ((no_signal_streak++)) || true
    else
      no_signal_streak=0
    fi
    if [[ "$_hr" == "0" ]] && [[ "$never_seen" == "1" ]] && [[ "$no_seen_need" =~ ^[0-9]+$ ]] && (( no_seen_need > 0 )) && (( no_signal_streak >= no_seen_need )); then
      printf '[cursor-dev] No host Cursor and no in-container remote for %s consecutive polls (nothing detected in first %ss) — stopping session container.\n' "$no_signal_streak" "$_appear" >&2
      return 0
    fi
    if [[ "${CURSOR_DEV_WAIT_DEBUG:-0}" == "1" ]]; then
      printf '[cursor-dev] debug: host_seen=%s host_running=%s server_seen=%s remote_running=%s empty_streak=%s/%s\n' \
        "$host_seen" "$_hr" "$server_seen" "$_remote" "$empty_streak" "$empty_need" >&2
    fi
    sleep "$_poll"
  done
}

# Block until Cursor GUI is gone (Windows: Cursor.exe; Unix: CURSOR_LAUNCHED_EXE + fallbacks).
# Uses CURSOR_DEV_POLL_SEC between polls. On Unix, waits up to CURSOR_DEV_GUI_APPEAR_SEC (default 90)
# for the GUI to show before giving up (keeps session container if Cursor never appeared).
#
# Optional $1: Docker container name. When CURSOR_DEV_SESSION_SHUTDOWN is not "host" and container_name is set,
# uses dual monitoring (host Cursor + in-container remote server). Does not require CURSOR_DEV_FOLDER_URI —
# attach can be manual or --folder-uri may fail; remote detection still runs (marker, TCP, fs, docker exec).
cursor_dev_wait_for_cursor_gui_exit() {
  local container_name="${1:-}"
  # Default "both": quit Cursor on the host OR remote session idle (close remote window). Use "host" for host-only
  # shutdown if remote heuristics misbehave (see README).
  if [[ -n "$container_name" ]] && [[ "${CURSOR_DEV_SESSION_SHUTDOWN:-both}" != "host" ]]; then
    printf '[cursor-dev] Dual shutdown mode — watching host Cursor and in-container remote (container %s).\n' "$container_name" >&2
    cursor_dev_wait_dual_session_end "$container_name"
    return $?
  fi

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
  # Linux / other Unix: do not require CURSOR_LAUNCHED_EXE (manual attach / failed CLI launch is common).
  # On Linux, never "give up" waiting for Cursor to appear: returning 1 skips docker stop and dockpipe then
  # blocks on docker wait forever. Poll until seen, then until gone.
  if [[ "$os" == "Linux" ]]; then
    local _wait_start _warned=0
    _wait_start=$(date +%s)
    printf '[cursor-dev] Waiting for Cursor on the host (open or attach the project if needed)…\n' >&2
    while ! cursor_dev_cursor_gui_running_unix; do
      sleep "$_poll"
      if [[ "$_warned" == "0" ]] && (( $(date +%s) >= _wait_start + _appear )); then
        printf '[cursor-dev] Still no Cursor process after %ss — keep polling until you quit Cursor (Ctrl+C in this terminal still stops the container).\n' "$_appear" >&2
        _warned=1
      fi
    done
    printf '[cursor-dev] Waiting for Cursor to exit — then stopping the session container.\n' >&2
    while cursor_dev_cursor_gui_running_unix; do
      sleep "$_poll"
    done
    return 0
  fi
  # Other Unix: bounded wait for first appearance (original behavior).
  local _start2 _deadline2
  _start2=$(date +%s)
  _deadline2=$((_start2 + _appear))
  while (( $(date +%s) < _deadline2 )); do
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

  # Prefer attaching to the session container with the same URI Dev Containers uses (avoids manual "attach" clicks).
  if [[ -n "${CURSOR_DEV_FOLDER_URI:-}" ]]; then
    if [[ -n "${CURSOR_DEV_CMD:-}" ]]; then
      if launch_cursor_folder_uri "${CURSOR_DEV_CMD}" "${CURSOR_DEV_FOLDER_URI}"; then
        return 0
      fi
      printf '[dockpipe] CURSOR_DEV_CMD not found or not runnable: %s\n' "${CURSOR_DEV_CMD}" >&2
      return 1
    fi
    local exe
    if exe=$(cursor_dev_resolve_cursor_exe); then
      if launch_cursor_folder_uri "$exe" "${CURSOR_DEV_FOLDER_URI}"; then
        return 0
      fi
    fi
    if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]] && command -v open >/dev/null 2>&1; then
      printf '[dockpipe] Opening Cursor via open -a (dev container URI).\n' >&2
      CURSOR_DEV_WAITABLE=1
      CURSOR_LAUNCHED_EXE="/Applications/Cursor.app/Contents/MacOS/Cursor"
      open -a Cursor --args --folder-uri "${CURSOR_DEV_FOLDER_URI}" >/dev/null 2>&1 &
      LAUNCH_PID=$!
      return 0
    fi
    printf '[dockpipe] Could not find Cursor to open dev container URI.\n' >&2
    return 1
  fi

  if [[ -n "${CURSOR_DEV_CMD:-}" ]]; then
    if launch_cursor "${CURSOR_DEV_CMD}" "$W"; then
      return 0
    fi
    printf '[dockpipe] CURSOR_DEV_CMD not found or not runnable: %s\n' "${CURSOR_DEV_CMD}" >&2
    return 1
  fi

  local exe
  if exe=$(cursor_dev_resolve_cursor_exe); then
    launch_cursor "$exe" "$W"
    return 0
  fi

  if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]] && command -v open >/dev/null 2>&1; then
    printf '[dockpipe] Opening Cursor via open -a Cursor (folder path).\n' >&2
    CURSOR_DEV_WAITABLE=1
    CURSOR_LAUNCHED_EXE="/Applications/Cursor.app/Contents/MacOS/Cursor"
    open -a Cursor "$W" >/dev/null 2>&1 &
    LAUNCH_PID=$!
    return 0
  fi

  return 1
}

cursor_dev_print_instructions() {
  printf '\n[cursor-dev] Next step on the host:\n' >&2
  if [[ -n "${CURSOR_DEV_FOLDER_URI:-}" ]]; then
    printf '  Cursor should open attached to the session container with workspace /work (repo mount).\n' >&2
    printf '  If it did not: bottom-left remote indicator → Attach to Running Container → %s.\n' "${CURSOR_DEV_CONTAINER_NAME:-<container>}" >&2
  else
    printf '  Open the Cursor app → File → Open Folder →\n' >&2
    printf '  %s\n' "$W" >&2
  fi
}

cursor_dev_footer() {
  printf '\nRemote SSH is separate; this workflow can auto-attach Cursor to the session container (see CURSOR_DEV_REMOTE_URI).\n' >&2
  printf 'For a browser-based editor, use Pipeon instead.\n' >&2
}
