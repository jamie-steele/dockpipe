#!/usr/bin/env bash
# Shared release-packaging status helpers.

run_with_elapsed_status() {
  local label="$1"
  shift
  local start pid status elapsed spin i output line
  start="$(date +%s)"
  if [[ -t 2 ]]; then
    output="$(mktemp "${TMPDIR:-/tmp}/dockpipe-status.XXXXXX")"
    "$@" >"${output}" 2>&1 &
    pid=$!
    spin='|/-\'
    i=0
    while kill -0 "$pid" 2>/dev/null; do
      elapsed=$(( "$(date +%s)" - start ))
      printf '\r[dockpipe] %s... %s %ss' "$label" "${spin:i++%${#spin}:1}" "$elapsed" >&2
      sleep 0.2
    done
    if wait "$pid"; then
      status=0
    else
      status=$?
    fi
    elapsed=$(( "$(date +%s)" - start ))
    printf '\r\033[K' >&2
    while IFS= read -r line || [[ -n "$line" ]]; do
      printf '%s\n' "$line" >&2
    done <"${output}"
    rm -f "${output}"
    if (( status == 0 )); then
      echo "[dockpipe] ${label}... done in ${elapsed}s" >&2
    else
      echo "[dockpipe] ${label}... failed after ${elapsed}s" >&2
    fi
    return "$status"
  fi
  echo "[dockpipe] ${label}..." >&2
  "$@" &
  pid=$!
  if wait "$pid"; then
    status=0
  else
    status=$?
  fi
  elapsed=$(( "$(date +%s)" - start ))
  if (( status == 0 )); then
    echo "[dockpipe] ${label}... done in ${elapsed}s" >&2
  else
    echo "[dockpipe] ${label}... failed after ${elapsed}s" >&2
  fi
  return "$status"
}
