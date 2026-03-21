#!/usr/bin/env bash
# Host-run code-server (OSS) with published port. Dockpipe's container runner does not map host
# ports; this script invokes docker directly so the browser can reach code-server.
set -euo pipefail

WORKDIR="${DOCKPIPE_WORKDIR:-$PWD}"
WORKDIR="$(cd "$WORKDIR" && pwd)"

# Git Bash / MSYS maps "/work" to e.g. C:/Program Files/Git/work when invoking docker.exe; "//work" disables that.
# OSTYPE/uname checks can miss some shells; WINDIR is set on Windows hosts.
CWORK="/work"
_u="$(uname -s 2>/dev/null || true)"
if [[ "${OSTYPE:-}" == msys* ]] || [[ "${OSTYPE:-}" == cygwin* ]] || [[ "${OSTYPE:-}" == win32 ]] \
  || [[ "$_u" == MINGW* ]] || [[ "$_u" == MSYS* ]] \
  || [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]]; then
  CWORK="//work"
fi
unset _u

# Fail before pull/run so we never open a browser when the daemon is down (Docker Desktop stopped, etc.).
if ! docker version >/dev/null 2>&1; then
  printf '[dockpipe] Docker is not reachable — start Docker Desktop (or Linux: sudo systemctl start docker), then re-run.\n' >&2
  printf '[dockpipe] Tip: dockpipe doctor\n' >&2
  exit 1
fi

IMAGE="${CODE_SERVER_IMAGE:-codercom/code-server:latest}"

# Host port: unset / auto / random → pick IANA dynamic range (49152–65535). Set CODE_SERVER_PORT to pin (e.g. 8080).
_pick_host_port() {
  echo $((49152 + RANDOM % 16384))
}
PORT="${CODE_SERVER_PORT:-}"
if [[ -z "${PORT}" ]] || [[ "${PORT}" == "auto" ]] || [[ "${PORT}" == "random" ]]; then
  PORT="$(_pick_host_port)"
fi
if ! [[ "${PORT}" =~ ^[0-9]+$ ]]; then
  printf '[dockpipe] CODE_SERVER_PORT must be a number, auto, or random (got %q)\n' "${CODE_SERVER_PORT:-}" >&2
  exit 1
fi
if [[ "${PORT}" -lt 1 ]] || [[ "${PORT}" -gt 65535 ]]; then
  printf '[dockpipe] CODE_SERVER_PORT must be 1-65535 (got %s)\n' "${PORT}" >&2
  exit 1
fi

NAME="${CODE_SERVER_CONTAINER_NAME:-dockpipe-code-server-${PORT}}"
URL="http://127.0.0.1:${PORT}/"

# CODE_SERVER_* are read from the process environment (Dockpipe passes workflow vars, .env, and your shell).
# CODE_SERVER_WAIT: 1 = keep dockpipe running until the app window closes or Ctrl+C; 0 = exit after start.
CODE_SERVER_WAIT="${CODE_SERVER_WAIT:-1}"
# CODE_SERVER_LAUNCH: app = Edge/Chrome --app (standalone window); none = do not launch anything.
CODE_SERVER_LAUNCH="${CODE_SERVER_LAUNCH:-app}"
# CODE_SERVER_AUTH: none (default) = no login on 127.0.0.1; password = PASSWORD env + login page.
CODE_SERVER_AUTH="${CODE_SERVER_AUTH:-none}"
# How to know when to stop: connections = TCP clients to 127.0.0.1:PORT (browser closed → disconnects).
# process = legacy PID/profile polling (CODE_SERVER_WAIT_SIGNAL=process).
CODE_SERVER_WAIT_SIGNAL="${CODE_SERVER_WAIT_SIGNAL:-connections}"
# TCP disconnect detection (connections mode)
CODE_SERVER_CONNECT_POLL_SEC="${CODE_SERVER_CONNECT_POLL_SEC:-0.25}"
CODE_SERVER_DISCONNECT_POLL_SEC="${CODE_SERVER_DISCONNECT_POLL_SEC:-0.08}"
# After count hits "session ended", wait this long and re-check once (avoids flapping; cheaper than many debounce polls).
CODE_SERVER_DISCONNECT_CONFIRM_SEC="${CODE_SERVER_DISCONNECT_CONFIRM_SEC:-0.15}"
# If peak connection count was >= CODE_SERVER_DISCONNECT_MULTI_THRESHOLD, allow shutdown when count <= this
# (default 1: last lingering TCP often holds 4s+ before clearing; strict 0 waits for that). Set 0 for strict zero only.
CODE_SERVER_DISCONNECT_TAIL_MAX="${CODE_SERVER_DISCONNECT_TAIL_MAX:-1}"
CODE_SERVER_DISCONNECT_MULTI_THRESHOLD="${CODE_SERVER_DISCONNECT_MULTI_THRESHOLD:-2}"
# App window title (Chromium --window-name). Empty = omit (browser default / page title).
CODE_SERVER_BROWSER_WINDOW_TITLE="${CODE_SERVER_BROWSER_WINDOW_TITLE:-VS Code}"

if [[ "${CODE_SERVER_AUTH}" == "none" ]]; then
  :
elif [[ -z "${CODE_SERVER_PASSWORD:-}" ]]; then
  if command -v openssl >/dev/null 2>&1; then
    CODE_SERVER_PASSWORD="$(openssl rand -hex 8)"
  else
    CODE_SERVER_PASSWORD="dockpipe-$(date +%s)"
  fi
  printf '[dockpipe] CODE_SERVER_PASSWORD was unset; using a generated value (set it in workflow vars, .env, or the shell).\n' >&2
fi

docker rm -f "$NAME" 2>/dev/null || true

if [[ "${DOCKPIPE_SKIP_PULL:-}" != "1" ]]; then
  printf '[dockpipe] Pulling image %s …\n' "$IMAGE" >&2
  docker pull "$IMAGE" >&2
fi

args=(
  run -d --rm --name "$NAME"
  -p "127.0.0.1:${PORT}:8080"
  -v "${WORKDIR}:${CWORK}"
  -w "${CWORK}"
)

# Clear image defaults so --auth none wins (some images set PASSWORD in Dockerfile).
if [[ "${CODE_SERVER_AUTH}" == "none" ]]; then
  args+=( -e "PASSWORD=" )
else
  args+=( -e "PASSWORD=${CODE_SERVER_PASSWORD}" )
fi

# Match bind-mount ownership on Unix hosts (Docker Desktop on Windows often omits -u).
case "${OSTYPE:-}" in
  linux-gnu*|darwin*)
    args+=( -u "$(id -u):$(id -g)" )
    ;;
esac

if [[ "${CODE_SERVER_AUTH}" == "none" ]]; then
  args+=( "$IMAGE" --bind-addr "0.0.0.0:8080" --auth none "${CWORK}" )
else
  args+=( "$IMAGE" --bind-addr "0.0.0.0:8080" "${CWORK}" )
fi

docker "${args[@]}" >/dev/null

printf '\n[dockpipe] code-server (OSS) is running.\n' >&2
printf '  Listen:   127.0.0.1:%s only (not LAN)\n' "${PORT}" >&2
printf '  URL:      %s\n' "$URL" >&2
if [[ "${CODE_SERVER_AUTH}" == "none" ]]; then
  printf '  Auth:     none (set CODE_SERVER_AUTH=password for a login page)\n' >&2
else
  printf '  Password: %s\n' "$CODE_SERVER_PASSWORD" >&2
fi
printf '  Stop:     close the app window, or: docker stop %s\n' "$NAME" >&2
printf '\nThird-party image (%s); not Microsoft VS Code. See template README.\n' "$IMAGE" >&2

# Browser profile: stable per host port avoids Edge/Chrome "first run" on every launch (fresh mktemp did that).
# Override with CODE_SERVER_BROWSER_PROFILE_DIR; set CODE_SERVER_BROWSER_PROFILE_EPHEMERAL=1 to delete profile on exit.
_is_windows() { [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]]; }

if [[ -n "${CODE_SERVER_BROWSER_PROFILE_DIR:-}" ]]; then
  EDGE_DATA_DIR="${CODE_SERVER_BROWSER_PROFILE_DIR}"
elif _is_windows; then
  _lp="${LOCALAPPDATA:-${USERPROFILE:-$HOME}/AppData/Local}"
  EDGE_DATA_DIR="${_lp}/dockpipe/code-server-edge-${PORT}"
else
  EDGE_DATA_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/dockpipe/code-server-edge-${PORT}"
fi
unset -f _is_windows
mkdir -p "$EDGE_DATA_DIR" 2>/dev/null || true
export EDGE_DATA_DIR
EDGE_TAG="$(basename "$EDGE_DATA_DIR")"
BROWSER_PID=""
DELETE_PROFILE_ON_EXIT=0
if [[ "${CODE_SERVER_BROWSER_PROFILE_EPHEMERAL:-}" == "1" ]]; then
  DELETE_PROFILE_ON_EXIT=1
fi

# Shared Chromium/Edge flags: fewer first-run / promo surfaces; window title where supported (Chromium 120+).
build_browser_flags() {
  local _extra
  BROWSER_FLAGS=(
    --no-first-run
    --no-default-browser-check
    --disable-infobars
    --disable-session-crashed-bubble
    --disable-default-apps
    --disable-sync
    --disable-background-networking
    --disable-component-update
    --disable-features=ChromeWhatsNewUI,OptimizationHints,InterestFeedContentSuggestions,TranslateUI,msEdgeImplicitSignin,msEdgePinningWizard,msEdgeOnRampFRE,msEdgeShoppingTrigger
  )
  if [[ -n "${CODE_SERVER_BROWSER_WINDOW_TITLE:-}" ]]; then
    BROWSER_FLAGS+=( --window-name="${CODE_SERVER_BROWSER_WINDOW_TITLE}" )
  fi
  if [[ -n "${CODE_SERVER_BROWSER_EXTRA_FLAGS:-}" ]]; then
    read -r -a _extra <<< "${CODE_SERVER_BROWSER_EXTRA_FLAGS}"
    BROWSER_FLAGS+=( "${_extra[@]}" )
  fi
}

# App-style window via Chromium --app + dedicated user-data-dir (close window → process exits → we stop docker).
launch_app_window() {
  BROWSER_PID=""
  build_browser_flags
  if [[ "${CODE_SERVER_LAUNCH}" == "none" ]] || [[ "${CODE_SERVER_LAUNCH}" == "0" ]]; then
    return 0
  fi
  if [[ "${CODE_SERVER_LAUNCH}" != "app" ]]; then
    printf '[dockpipe] Unknown CODE_SERVER_LAUNCH=%s (use app or none).\n' "${CODE_SERVER_LAUNCH}" >&2
    return 0
  fi

  # Windows (Git Bash paths)
  if [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]]; then
    for edge in \
      "/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" \
      "/c/Program Files/Microsoft/Edge/Application/msedge.exe"; do
      if [[ -f "$edge" ]]; then
        printf '[dockpipe] Opening in Microsoft Edge (app window; close window to stop code-server)…\n' >&2
        "$edge" "${BROWSER_FLAGS[@]}" --user-data-dir="$EDGE_DATA_DIR" --app="$URL" >/dev/null 2>&1 &
        BROWSER_PID=$!
        return 0
      fi
    done
    for chrome in \
      "/c/Program Files/Google/Chrome/Application/chrome.exe" \
      "/c/Program Files (x86)/Google/Chrome/Application/chrome.exe"; do
      if [[ -f "$chrome" ]]; then
        printf '[dockpipe] Opening in Google Chrome (app window; close window to stop code-server)…\n' >&2
        "$chrome" "${BROWSER_FLAGS[@]}" --user-data-dir="$EDGE_DATA_DIR" --app="$URL" >/dev/null 2>&1 &
        BROWSER_PID=$!
        return 0
      fi
    done
    printf '[dockpipe] No Edge/Chrome found. Open this URL yourself:\n  %s\n' "$URL" >&2
    return 0
  fi

  # macOS
  if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]]; then
    mac_edge="/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"
    mac_chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    if [[ -f "$mac_edge" ]]; then
      printf '[dockpipe] Opening in Microsoft Edge (app window; close window to stop code-server)…\n' >&2
      "$mac_edge" "${BROWSER_FLAGS[@]}" --user-data-dir="$EDGE_DATA_DIR" --app="$URL" >/dev/null 2>&1 &
      BROWSER_PID=$!
      return 0
    fi
    if [[ -f "$mac_chrome" ]]; then
      printf '[dockpipe] Opening in Google Chrome (app window; close window to stop code-server)…\n' >&2
      "$mac_chrome" "${BROWSER_FLAGS[@]}" --user-data-dir="$EDGE_DATA_DIR" --app="$URL" >/dev/null 2>&1 &
      BROWSER_PID=$!
      return 0
    fi
    printf '[dockpipe] Install Microsoft Edge or Google Chrome, or open:\n  %s\n' "$URL" >&2
    return 0
  fi

  # Linux: prefer real binaries so "wait" tracks the browser — /usr/bin/google-chrome is often a wrapper
  # that exits immediately after spawning the real process.
  for b in \
    /opt/google/chrome/chrome \
    /usr/lib/chromium/chromium \
    /usr/lib/chromium-browser/chromium-browser; do
    if [[ -x "$b" ]]; then
      printf '[dockpipe] Opening %s (app window; close window to stop code-server)…\n' "$b" >&2
      "$b" "${BROWSER_FLAGS[@]}" --user-data-dir="$EDGE_DATA_DIR" --app="$URL" >/dev/null 2>&1 &
      BROWSER_PID=$!
      return 0
    fi
  done
  for c in microsoft-edge-stable microsoft-edge google-chrome-stable google-chrome chromium chromium-browser; do
    if command -v "$c" >/dev/null 2>&1; then
      printf '[dockpipe] Opening in %s (app window; close window to stop code-server)…\n' "$c" >&2
      "$c" "${BROWSER_FLAGS[@]}" --user-data-dir="$EDGE_DATA_DIR" --app="$URL" >/dev/null 2>&1 &
      BROWSER_PID=$!
      return 0
    fi
  done
  printf '[dockpipe] No chromium-based browser in PATH. Open:\n  %s\n' "$URL" >&2
}

cleanup_session() {
  docker stop "$NAME" 2>/dev/null || true
  if [[ -n "${BROWSER_PID:-}" ]] && kill -0 "$BROWSER_PID" 2>/dev/null; then
    kill "$BROWSER_PID" 2>/dev/null || true
  fi
}

# Windows: true if PID is still a running process (Git Bash kill -0 is unreliable for native PIDs).
windows_pid_alive() {
  local pid="$1"
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \
    "if (Get-Process -Id $pid -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }" 2>/dev/null
}

# After background launch, $! is often correct but can be a short-lived parent. Find the real
# browser process that owns the --app window (cmdline has --app=, not --type=child), within the
# process subtree of the launcher PID on Windows; on Unix scan by profile path.
resolve_browser_monitor_pid() {
  local launch="$1"
  local tag="$2"
  local dir="$3"
  [[ -n "$launch" ]] || return 0
  [[ "$launch" =~ ^[0-9]+$ ]] || return 0

  if [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]] && command -v powershell.exe >/dev/null 2>&1; then
    local qtag="${tag//\'/\'\'}"
    local out
    # Descendants of launcher PID, then pick the msedge/chrome process that carries --app= without --type= (not renderer/gpu).
    out=$(powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \
      "\$r = [int]${launch}; \$tag = '${qtag}'; \
       \$subs = [System.Collections.Generic.HashSet[int]]::new(); \
       \$q = [System.Collections.Queue]::new(); [void]\$q.Enqueue(\$r); \
       while (\$q.Count -gt 0) { \
         \$p = \$q.Dequeue(); if (\$subs.Contains(\$p)) { continue }; [void]\$subs.Add(\$p); \
         Get-CimInstance Win32_Process | Where-Object { \$_.ParentProcessId -eq \$p } | ForEach-Object { [void]\$q.Enqueue(\$_.ProcessId) } \
       }; \
       \$best = \$null; \
       Get-CimInstance Win32_Process | Where-Object { \$subs.Contains(\$_.ProcessId) } | ForEach-Object { \
         if (-not \$best -and \$_.CommandLine -and (\$_.CommandLine.IndexOf(\$tag, [StringComparison]::OrdinalIgnoreCase) -ge 0) -and \
             (\$_.CommandLine -match '--app=') -and (\$_.CommandLine -notmatch '--type=(renderer|utility|gpu-process|crashpad-handler)') -and \
             (\$_.Name -match 'msedge|chrome')) { \$best = \$_.ProcessId } \
       }; \
       if (\$best) { Write-Output \$best } else { Write-Output \$r }" 2>/dev/null | tr -d '\r' | head -n1)
    [[ "$out" =~ ^[0-9]+$ ]] && BROWSER_PID=$out
    return 0
  fi

  # Linux: launcher already the browser process?
  if [[ -r "/proc/${launch}/cmdline" ]]; then
    local cl
    cl=$(tr '\0' ' ' < "/proc/${launch}/cmdline" 2>/dev/null || true)
    if [[ "$cl" =~ --app= ]] && [[ "$cl" =~ $tag ]] && [[ ! "$cl" =~ --type= ]]; then
      return 0
    fi
  fi
  # macOS / fallback: no /proc for launch check
  if [[ "$(uname -s 2>/dev/null)" == "Darwin" ]] && [[ ! -r "/proc/${launch}/cmdline" ]]; then
    local lcl
    lcl=$(ps -p "$launch" -o args= 2>/dev/null || true)
    if [[ "$lcl" =~ --app= ]] && [[ "$lcl" =~ $tag ]] && [[ ! "$lcl" =~ --type= ]]; then
      return 0
    fi
  fi

  if command -v pgrep >/dev/null 2>&1; then
    local pid cl
    for pid in $(pgrep -f -- "--user-data-dir=${dir}" 2>/dev/null | sort -n); do
      if [[ -r "/proc/${pid}/cmdline" ]]; then
        cl=$(tr '\0' ' ' < "/proc/${pid}/cmdline")
      else
        cl=$(ps -p "$pid" -o args= 2>/dev/null || true)
      fi
      [[ "$cl" =~ --app= ]] || continue
      [[ "$cl" =~ --type= ]] && continue
      BROWSER_PID=$pid
      return 0
    done
  fi
}

# Count OS processes whose command line references our profile path (Edge/Chrome children).
profile_process_count_windows() {
  local tag="$1"
  local qtag="${tag//\'/\'\'}"
  powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \
    "\$tag = '${qtag}'; \$n = (Get-CimInstance Win32_Process | Where-Object { \$_.CommandLine -and (\$_.CommandLine.IndexOf(\$tag, [StringComparison]::OrdinalIgnoreCase) -ge 0) }).Count; Write-Output \$n" 2>/dev/null \
    | tr -d '\r' | head -n1
}

profile_process_count_unix() {
  local tag="$1"
  pgrep -f -- "$tag" 2>/dev/null | wc -l | tr -d ' \n'
}

# Established TCP connections to code-server on localhost (browser ↔ server). When the PWA window
# closes, these drop — more reliable than tracking Edge PIDs on Windows.
tcp_established_to_code_server() {
  local port="$1"
  local n
  # Windows: netstat is much faster than spawning PowerShell every poll (was a major source of multi-second lag).
  if [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]]; then
    if command -v netstat.exe >/dev/null 2>&1; then
      n=$(netstat.exe -ano 2>/dev/null | grep -E "127\\.0\\.0\\.1:${port}.*ESTABLISHED" | wc -l | tr -d ' ')
      n6=$(netstat.exe -ano 2>/dev/null | grep -E "\\[::1\\]:${port}.*ESTABLISHED" | wc -l | tr -d ' ')
      [[ "$n" =~ ^[0-9]+$ ]] || n=0
      [[ "$n6" =~ ^[0-9]+$ ]] || n6=0
      printf '%s' "$((n + n6))"
      return 0
    fi
    if command -v powershell.exe >/dev/null 2>&1; then
      n=$(powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \
        "\$n = @(Get-NetTCPConnection -LocalPort $port -State Established -ErrorAction SilentlyContinue | Where-Object { \$_.LocalAddress -eq '127.0.0.1' -or \$_.LocalAddress -eq '::1' }).Count; Write-Output \$n" 2>/dev/null \
        | tr -d '\r' | head -n1)
      [[ "$n" =~ ^[0-9]+$ ]] && printf '%s' "$n" || printf '0'
      return 0
    fi
  fi
  if command -v lsof >/dev/null 2>&1; then
    # IPv4 localhost; add IPv6 if present
    n=$(lsof -nP -iTCP@127.0.0.1:"$port" -sTCP:ESTABLISHED 2>/dev/null | tail -n +2 | wc -l | tr -d ' ')
    n6=$(lsof -nP -iTCP@[::1]:"$port" -sTCP:ESTABLISHED 2>/dev/null | tail -n +2 | wc -l | tr -d ' ')
    [[ "$n" =~ ^[0-9]+$ ]] || n=0
    [[ "$n6" =~ ^[0-9]+$ ]] || n6=0
    printf '%s' "$((n + n6))"
    return 0
  fi
  if command -v ss >/dev/null 2>&1; then
    n=$(ss -tn state established 2>/dev/null | grep -cE "127\\.0\\.0\\.1:${port}|\\[::1\\]:${port}" || true)
    [[ "$n" =~ ^[0-9]+$ ]] && printf '%s' "$n" || printf '0'
    return 0
  fi
  printf '0'
}

# Wait until at least one client connects, then until the session looks closed (TCP count).
# code-server often keeps multiple ESTABLISHED sockets; the last one can linger seconds — when peak>=2,
# default CODE_SERVER_DISCONNECT_TAIL_MAX=1 allows shutdown at <=1 after stable polls (see README).
wait_for_code_server_clients_disconnect() {
  local port="$1"
  local c c2 _i saw_client=0 peak=0 stable_tail=0
  local cpoll dpoll confirm tailm mult
  cpoll="${CODE_SERVER_CONNECT_POLL_SEC:-0.25}"
  dpoll="${CODE_SERVER_DISCONNECT_POLL_SEC:-0.08}"
  confirm="${CODE_SERVER_DISCONNECT_CONFIRM_SEC:-0.15}"
  tailm="${CODE_SERVER_DISCONNECT_TAIL_MAX:-1}"
  mult="${CODE_SERVER_DISCONNECT_MULTI_THRESHOLD:-2}"
  [[ "$cpoll" =~ ^[0-9]*\.?[0-9]+$ ]] || cpoll=0.25
  [[ "$dpoll" =~ ^[0-9]*\.?[0-9]+$ ]] || dpoll=0.08
  [[ "$confirm" =~ ^[0-9]*\.?[0-9]+$ ]] || confirm=0.15
  [[ "$tailm" =~ ^[0-9]+$ ]] || tailm=1
  [[ "$mult" =~ ^[0-9]+$ ]] || mult=2

  _confirm_still_done() {
    sleep "$confirm"
    c2="$(tcp_established_to_code_server "$port")"
    [[ "$c2" =~ ^[0-9]+$ ]] || c2=0
  }

  printf '[dockpipe] Waiting for a browser connection to 127.0.0.1:%s…\n' "$port" >&2
  for ((_i = 0; _i < 480; _i++)); do
    c="$(tcp_established_to_code_server "$port")"
    [[ "$c" =~ ^[0-9]+$ ]] || c=0
    if [[ "$c" -gt 0 ]]; then
      saw_client=1
      peak=$c
      printf '[dockpipe] Connected (%s TCP session(s)); will exit when you close the app window (or Ctrl+C).\n' "$c" >&2
      break
    fi
    sleep "$cpoll"
  done
  if [[ "$saw_client" -eq 0 ]]; then
    printf '[dockpipe] No TCP connection seen on 127.0.0.1:%s; waiting for the container (Ctrl+C stops it).\n' "$port" >&2
    docker wait "$NAME" || true
    return 0
  fi

  while true; do
    c="$(tcp_established_to_code_server "$port")"
    [[ "$c" =~ ^[0-9]+$ ]] || c=0
    [[ "$c" -gt "$peak" ]] && peak=$c

    # Strict zero (always): all sockets closed.
    if [[ "$c" -eq 0 ]]; then
      _confirm_still_done
      if [[ "$c2" -eq 0 ]]; then
        break
      fi
      sleep "$dpoll"
      continue
    fi

    # Fast path: had a multi-connection session; last socket often sits at 1 ESTABLISHED for seconds.
    if [[ "$peak" -ge "$mult" ]] && [[ "$tailm" -ge 1 ]] && [[ "$c" -le "$tailm" ]]; then
      stable_tail=$((stable_tail + 1))
      if [[ "$stable_tail" -ge 2 ]]; then
        _confirm_still_done
        if [[ "$c2" -le "$tailm" ]] || [[ "$c2" -eq 0 ]]; then
          break
        fi
        stable_tail=0
      fi
    else
      stable_tail=0
    fi

    sleep "$dpoll"
  done
}

# Wait for session end: root browser PID gone, then no stray processes mentioning profile (Windows/Edge).
wait_for_browser_profile_gone() {
  local tag="$1"
  local pid="$2"
  local is_win=0
  local c
  [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]] && is_win=1

  if [[ "$is_win" == "1" ]] && command -v powershell.exe >/dev/null 2>&1 && [[ "$pid" =~ ^[0-9]+$ ]]; then
    while windows_pid_alive "$pid"; do
      sleep 0.5
    done
    while true; do
      c="$(profile_process_count_windows "$tag")"
      [[ "$c" =~ ^[0-9]+$ ]] || c=1
      if [[ "$c" -eq 0 ]]; then
        break
      fi
      sleep 0.5
    done
    return 0
  fi

  if command -v pgrep >/dev/null 2>&1; then
    local _i
    for ((_i = 0; _i < 80; _i++)); do
      c="$(profile_process_count_unix "$tag")"
      [[ "$c" =~ ^[0-9]+$ ]] || c=0
      if [[ "$c" -gt 0 ]]; then
        break
      fi
      sleep 0.25
    done
    while true; do
      c="$(profile_process_count_unix "$tag")"
      [[ "$c" =~ ^[0-9]+$ ]] || c=1
      if [[ "$c" -eq 0 ]]; then
        break
      fi
      sleep 0.5
    done
    return 0
  fi

  SECONDS=0
  wait "$pid" || true
  if (( SECONDS < 1 )); then
    if docker inspect --format '{{.State.Running}}' "$NAME" 2>/dev/null | grep -qx true; then
      printf '[dockpipe] Browser launcher exited immediately (detached). Waiting for the code-server container (Ctrl+C stops it).\n' >&2
      docker wait "$NAME" || true
    fi
  fi
}

if [[ "${CODE_SERVER_WAIT}" == "1" ]]; then
  sleep 2
  launch_app_window
  trap cleanup_session INT TERM

  if [[ "${CODE_SERVER_WAIT_SIGNAL}" == "process" ]]; then
    if [[ -n "${BROWSER_PID:-}" ]]; then
      sleep 1
      resolve_browser_monitor_pid "$BROWSER_PID" "$EDGE_TAG" "$EDGE_DATA_DIR"
      printf '\n[dockpipe] Monitoring browser PID %s (close the app window to stop code-server). Ctrl+C also stops.\n' "${BROWSER_PID}" >&2
      wait_for_browser_profile_gone "$EDGE_TAG" "$BROWSER_PID"
    else
      printf '\n[dockpipe] No browser launcher; waiting for the code-server container (Ctrl+C stops it).\n' >&2
      docker wait "$NAME" || true
    fi
  else
    # Default: infer disconnect from the host listener (code-server ↔ browser TCP sessions).
    printf '\n[dockpipe] Using TCP sessions to 127.0.0.1:%s (close the app window to disconnect). Ctrl+C also stops.\n' "${PORT}" >&2
    wait_for_code_server_clients_disconnect "${PORT}"
  fi

  cleanup_session
  trap - INT TERM
  if [[ "$DELETE_PROFILE_ON_EXIT" == "1" ]]; then
    rm -rf "$EDGE_DATA_DIR" 2>/dev/null || true
  fi
else
  printf '\n[dockpipe] CODE_SERVER_WAIT=0 — not waiting; container keeps running in the background.\n' >&2
fi
