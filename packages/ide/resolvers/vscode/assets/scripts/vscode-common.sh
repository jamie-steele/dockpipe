#!/usr/bin/env bash
# Shared helpers for vscode session scripts.

vscode_is_msysish() {
  [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]] || [[ "${OSTYPE:-}" == msys* ]] || [[ "${OSTYPE:-}" == cygwin* ]] || [[ "${OSTYPE:-}" == win32 ]]
}

vscode_docker() {
  if vscode_is_msysish; then
    MSYS2_ARG_CONV_EXCL="*" docker "$@"
  else
    docker "$@"
  fi
}

vscode_set_workdir() {
  W="$(pwd)"
  export W
}

vscode_global_state_root() {
  dockpipe get state_dir
}

vscode_state_root() {
  dockpipe scope --package vscode .
}

vscode_container_state_root() {
  local state_root
  state_root="$(vscode_state_root)"
  case "$state_root" in
    "$W"/*) printf '/work/%s' "${state_root#"$W"/}" ;;
    *) printf '%s' "$state_root" ;;
  esac
}

vscode_container_global_state_root() {
  local state_root
  state_root="$(vscode_global_state_root)"
  case "$state_root" in
    "$W"/*) printf '/work/%s' "${state_root#"$W"/}" ;;
    *) printf '%s' "$state_root" ;;
  esac
}

vscode_home_dir() {
  printf '%s/home' "$(vscode_state_root)"
}

vscode_remote_server_dir() {
  printf '%s/.vscode-server' "$(vscode_home_dir)"
}

vscode_require_docker_for_session() {
  VSCODE_SKIP_DOCKER_CHECK="${VSCODE_SKIP_DOCKER_CHECK:-0}"
  if [[ "${VSCODE_SKIP_DOCKER_CHECK}" == "1" ]]; then
    return 0
  fi
  if ! command -v docker >/dev/null 2>&1; then
    printf '[dockpipe] docker not found in PATH — cannot start the vscode session container.\n' >&2
    return 1
  fi
  local max="${VSCODE_DOCKER_WAIT_SEC:-120}"
  if [[ "${max}" == "0" ]]; then
    if ! docker info >/dev/null 2>&1; then
      printf '[dockpipe] Docker daemon is not reachable.\n' >&2
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
  return 1
}

vscode_wait_container_running() {
  local name="$1"
  local max="${2:-120}"
  local i=0
  while (( i < max )); do
    local st
    st=$(vscode_docker inspect -f '{{.State.Status}}' "$name" 2>/dev/null) || return 1
    if [[ "$st" == "running" ]]; then
      return 0
    fi
    sleep 0.5
    ((i++)) || true
  done
  return 1
}

vscode_attached_container_folder_uri() {
  local container_name="$1"
  local path_in_container="${2:-/work}"
  local cid
  cid=$(vscode_docker inspect -f '{{.Id}}' "$container_name" 2>/dev/null) || return 1
  cid=${cid#sha256:}
  local hex
  hex=$(printf '%s' "$cid" | od -A n -t x1 | tr -d ' \n')
  printf 'vscode-remote://attached-container+%s%s' "$hex" "$path_in_container"
}

vscode_resolve_exe() {
  if [[ -n "${VSCODE_CMD:-}" ]]; then
    if [[ -x "${VSCODE_CMD}" ]] || command -v "${VSCODE_CMD}" >/dev/null 2>&1; then
      printf '%s' "${VSCODE_CMD}"
      return 0
    fi
    return 1
  fi
  if vscode_is_msysish; then
    local local_app="${LOCALAPPDATA:-${USERPROFILE:-$HOME}/AppData/Local}"
    for candidate in \
      "${local_app}/Programs/Microsoft VS Code/Code.exe" \
      "${local_app}/Programs/VS Code/Code.exe" \
      "/c/Program Files/Microsoft VS Code/Code.exe" \
      "/c/Program Files (x86)/Microsoft VS Code/Code.exe"; do
      if [[ -f "$candidate" ]]; then
        printf '%s' "$candidate"
        return 0
      fi
    done
  fi
  if command -v code >/dev/null 2>&1; then
    command -v code
    return 0
  fi
  if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]]; then
    local mac_bin="/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code"
    if [[ -x "$mac_bin" ]]; then
      printf '%s' "$mac_bin"
      return 0
    fi
  fi
  for candidate in /usr/bin/code /usr/local/bin/code /snap/bin/code; do
    if [[ -x "$candidate" ]]; then
      printf '%s' "$candidate"
      return 0
    fi
  done
  return 1
}

launch_vscode_folder_uri() {
  local exe="$1"
  local uri="$2"
  if [[ -f "$exe" ]]; then
    :
  elif command -v "$exe" >/dev/null 2>&1; then
    exe=$(command -v "$exe")
  else
    return 1
  fi
  printf '[dockpipe] Opening VS Code attached to container (--folder-uri) at /work\n' >&2
  VSCODE_WAITABLE=1
  if vscode_is_msysish; then
    MSYS2_ARG_CONV_EXCL='*' "$exe" --new-window --wait --folder-uri "$uri" >/dev/null 2>&1 &
  else
    setsid "$exe" --new-window --wait --folder-uri "$uri" </dev/null >/dev/null 2>&1 &
  fi
  VSCODE_LAUNCH_PID=$!
  return 0
}

vscode_host_running() {
  if vscode_is_msysish && command -v tasklist >/dev/null 2>&1; then
    tasklist //FI "IMAGENAME eq Code.exe" 2>/dev/null | grep -qi 'Code.exe'
    return $?
  fi
  if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]]; then
    pgrep -f 'Visual Studio Code.app' >/dev/null 2>&1
    return $?
  fi
  local uid
  uid=$(id -u)
  if command -v pgrep >/dev/null 2>&1; then
    pgrep -u "$uid" -x code >/dev/null 2>&1 && return 0
    pgrep -u "$uid" -x Code >/dev/null 2>&1 && return 0
    pgrep -u "$uid" -f '/usr/share/code|code --type|code-url-handler|/code/code' >/dev/null 2>&1 && return 0
  fi
  return 1
}

vscode_age_seconds_of_file() {
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

vscode_fs_remote_active() {
  local dir="${1:-$(vscode_remote_server_dir)}"
  local quiet="${VSCODE_REMOTE_FS_QUIET_SEC:-90}"
  [[ "${VSCODE_REMOTE_FS_SIGNAL:-1}" == "1" ]] || return 1
  [[ -d "$dir" ]] || return 1
  find "$dir" -type f -newermt "-${quiet} seconds" -print -quit 2>/dev/null | grep -q .
}

vscode_tcp_session_to_container() {
  local name="$1"
  local ip
  ip=$(vscode_docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$name" 2>/dev/null) || return 1
  [[ -n "$ip" ]] || return 1
  if command -v ss >/dev/null 2>&1; then
    ss -tn state established 2>/dev/null | grep -qF "${ip}:" && return 0
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP -sTCP:ESTABLISHED 2>/dev/null | grep -qF "${ip}:" && return 0
  fi
  return 1
}

_vscode_remote_exec_ps_probe() {
  local name="$1"
  vscode_docker exec "$name" sh -c '
    if command -v pgrep >/dev/null 2>&1; then
      pgrep -f "vscode-server" >/dev/null 2>&1 && exit 0
      pgrep -f "code-reh" >/dev/null 2>&1 && exit 0
    fi
    _line=$(ps -eww -o args= 2>/dev/null | tr "\n" " ")
    case "$_line" in
      *vscode-server*|*code-reh*) exit 0 ;;
    esac
    if [ -d /proc ]; then
      for f in /proc/[0-9]*/cmdline; do
        [ -r "$f" ] || continue
        _line=$(tr "\0" " " <"$f" 2>/dev/null || true)
        case "$_line" in
          *vscode-server*|*code-reh*) exit 0 ;;
        esac
      done
    fi
    exit 1
  ' 2>/dev/null
}

vscode_container_remote_server_running() {
  local name="$1"
  local probe="${2:-${VSCODE_REMOTE_PROBE:-full}}"
  local max_age="${VSCODE_MARKER_MAX_AGE_SEC:-60}"
  local marker
  marker="$(vscode_state_root)/remote_active"
  local age value
  if [[ -f "$marker" ]]; then
    age=$(vscode_age_seconds_of_file "$marker") || age=999999
    if [[ "$age" -lt "$max_age" ]]; then
      value=$(tr -d ' \n' <"$marker" 2>/dev/null || echo 0)
      if [[ "$value" == "1" ]]; then
        return 0
      fi
      if [[ "$probe" == "dual" ]] && [[ "$value" == "0" ]]; then
        return 1
      fi
    fi
  fi
  if vscode_tcp_session_to_container "$name"; then
    return 0
  fi
  if [[ "$probe" != "dual" ]] && vscode_fs_remote_active "$(vscode_remote_server_dir)"; then
    return 0
  fi
  _vscode_remote_exec_ps_probe "$name"
}

vscode_wait_dual_session_end() {
  local name="$1"
  local poll="${VSCODE_POLL_SEC:-1}"
  local appear="${VSCODE_GUI_APPEAR_SEC:-90}"
  local idle_need="${VSCODE_REMOTE_SERVER_IDLE_POLLS:-3}"
  local host_seen=0
  local remote_seen=0
  local idle_streak=0
  local never_seen=0
  local no_seen_need="${VSCODE_HOST_CLEAR_POLLS_NO_SEEN:-0}"
  local no_signal_streak=0
  local start deadline
  start=$(date +%s)
  deadline=$((start + appear))
  while (( $(date +%s) < deadline )); do
    if vscode_host_running; then
      host_seen=1
      printf '[vscode] VS Code running on host — monitoring for session end.\n' >&2
      break
    fi
    if vscode_container_remote_server_running "$name" dual; then
      remote_seen=1
      printf '[vscode] Remote server in container — monitoring for session end.\n' >&2
      break
    fi
    sleep "$poll"
  done
  if [[ "$host_seen" == "0" ]] && [[ "$remote_seen" == "0" ]]; then
    never_seen=1
    printf '[vscode] No VS Code / remote server detected within %ss — still monitoring (slow start is OK).\n' "$appear" >&2
  fi
  while true; do
    local host_running=0
    local remote_running=0
    if vscode_host_running; then
      host_running=1
      host_seen=1
      never_seen=0
    fi
    if [[ "$host_running" == "0" ]] && { [[ "$host_seen" == "1" ]] || [[ "$remote_seen" == "1" ]]; }; then
      printf '[vscode] VS Code is no longer running on the host — stopping session container.\n' >&2
      return 0
    fi
    if vscode_container_remote_server_running "$name" dual; then
      remote_running=1
      remote_seen=1
      never_seen=0
      idle_streak=0
    elif [[ "$remote_seen" == "1" ]]; then
      ((idle_streak++)) || true
      if (( idle_streak >= idle_need )); then
        printf '[vscode] Remote session inside the container went idle — stopping session container.\n' >&2
        return 0
      fi
    fi
    if [[ "$host_running" == "0" ]] && [[ "$never_seen" == "1" ]] && [[ "$remote_running" == "0" ]]; then
      ((no_signal_streak++)) || true
    else
      no_signal_streak=0
    fi
    if [[ "$host_running" == "0" ]] && [[ "$never_seen" == "1" ]] && [[ "$no_seen_need" =~ ^[0-9]+$ ]] && (( no_seen_need > 0 )) && (( no_signal_streak >= no_seen_need )); then
      printf '[vscode] No host VS Code and no in-container remote for %s consecutive polls — stopping session container.\n' "$no_signal_streak" >&2
      return 0
    fi
    sleep "$poll"
  done
}

vscode_wait_for_session_end() {
  local name="$1"
  if [[ -n "${VSCODE_LAUNCH_PID:-}" ]] && [[ "${VSCODE_WAITABLE:-0}" == "1" ]]; then
    if wait "${VSCODE_LAUNCH_PID}" 2>/dev/null; then
      printf '[vscode] VS Code launcher exited — stopping session container.\n' >&2
      return 0
    fi
  fi
  vscode_wait_dual_session_end "$name"
}
