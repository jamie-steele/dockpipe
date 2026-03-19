# dockpipe runner: build docker run args, then run the container (attach or detach).
# Sourced by bin/dockpipe. Expects env: DOCKPIPE_IMAGE, DOCKPIPE_ACTION, DOCKPIPE_WORKDIR,
# DOCKPIPE_DATA_VOLUME / DOCKPIPE_DATA_DIR / DOCKPIPE_NO_DATA, DOCKPIPE_REINIT, DOCKPIPE_FORCE,
# DOCKPIPE_EXTRA_MOUNTS, DOCKPIPE_EXTRA_ENV, DOCKPIPE_DETACH. Image must already exist.

set -euo pipefail

export DOCKPIPE_CONTAINER_WORKDIR="${DOCKPIPE_CONTAINER_WORKDIR:-/work}"

# ------------------------------------------------------------------------------
# Banner and spinner (stderr only; used when attached and stdout is a TTY).
# ------------------------------------------------------------------------------
dockpipe_banner() {
  cat << 'BANNER' >&2

    ██████╗  ██████╗ ██████╗██╗  ██╗██████╗ ██╗██████╗ ███████╗
    ██╔══██╗██╔═══██╗██╔═══╝██║ ██╔╝██╔══██╗██║██╔══██╗██╔════╝
    ██║  ██║██║   ██║██║    █████╔╝ ██████╔╝██║██████╔╝█████╗
    ██║  ██║██║   ██║██║    ██╔═██╗ ██╔═══╝ ██║██╔═══╝ ██╔══╝
    ██████╔╝╚██████╔╝██████╗██║  ██╗██║     ██║██║     ███████╗
    ╚═════╝  ╚═════╝ ╚═════╝╚═╝  ╚═╝╚═╝     ╚═╝╚═╝     ╚══════╝
                      Run  →  Isolate  →  Act

BANNER
}

# Spinner: show "Launching container..." with rotating chars for ~0.6s, then clear line and return.
dockpipe_launch_spinner() {
  local chars='|/-\'
  local i=0
  while [[ $i -lt 8 ]]; do
    printf '\r  Launching container... %s  ' "${chars:i%4:1}" >&2
    sleep 0.08
    ((i++)) || true
  done
  printf '\r  %*s\r' 40 ' ' >&2
}

# Run the commit-worktree logic on the host (so the AI never has git access).
# Call with: workdir_host (path on host). Uses DOCKPIPE_COMMIT_MESSAGE, DOCKPIPE_WORK_BRANCH, DOCKPIPE_BUNDLE_OUT.
dockpipe_commit_on_host() {
  local workdir_host="${1:?}"
  local msg="${DOCKPIPE_COMMIT_MESSAGE:-dockpipe: automated commit}"
  if ! git -C "${workdir_host}" rev-parse --is-inside-work-tree &>/dev/null; then
    echo "[dockpipe] Not a git repo; skipping commit." >&2
    return 0
  fi
  if git -C "${workdir_host}" diff --quiet HEAD 2>/dev/null && [[ -z "$(git -C "${workdir_host}" status --porcelain)" ]]; then
    echo "[dockpipe] No changes to commit." >&2
    return 0
  fi
  local current_branch
  current_branch="$(git -C "${workdir_host}" branch --show-current)"
  echo "[dockpipe] Committing on branch: ${current_branch}" >&2
  git -C "${workdir_host}" add -A
  git -C "${workdir_host}" commit -m "${msg}"
  if [[ -n "${DOCKPIPE_BUNDLE_OUT:-}" ]]; then
    # Default: only the branch that was committed (smaller bundle). Set DOCKPIPE_BUNDLE_ALL=1 for git's --all.
    if [[ "${DOCKPIPE_BUNDLE_ALL:-}" == "1" ]]; then
      if git -C "${workdir_host}" bundle create "${DOCKPIPE_BUNDLE_OUT}" --all; then
        echo "[dockpipe] Bundle written (--all): ${DOCKPIPE_BUNDLE_OUT}" >&2
      else
        echo "[dockpipe] Failed to write bundle: ${DOCKPIPE_BUNDLE_OUT}" >&2
      fi
    elif [[ -n "${current_branch}" ]]; then
      if git -C "${workdir_host}" bundle create "${DOCKPIPE_BUNDLE_OUT}" "refs/heads/${current_branch}"; then
        echo "[dockpipe] Bundle written (branch ${current_branch}): ${DOCKPIPE_BUNDLE_OUT}" >&2
      else
        echo "[dockpipe] Failed to write bundle: ${DOCKPIPE_BUNDLE_OUT}" >&2
      fi
    elif git -C "${workdir_host}" bundle create "${DOCKPIPE_BUNDLE_OUT}" HEAD; then
      echo "[dockpipe] Bundle written (HEAD): ${DOCKPIPE_BUNDLE_OUT}" >&2
    else
      echo "[dockpipe] Failed to write bundle: ${DOCKPIPE_BUNDLE_OUT}" >&2
    fi
  fi
}

# ------------------------------------------------------------------------------
# dockpipe_run [command argv...] — run the container with the current env config.
# ------------------------------------------------------------------------------
dockpipe_run() {
  local img="${DOCKPIPE_IMAGE:?DOCKPIPE_IMAGE is required}"
  local workdir_host="${DOCKPIPE_WORKDIR:-$(pwd)}"
  local action_path="${DOCKPIPE_ACTION:-}"
  local extra_mounts="${DOCKPIPE_EXTRA_MOUNTS:-}"
  local extra_env="${DOCKPIPE_EXTRA_ENV:-}"

  if [[ -z "${DOCKPIPE_DETACH:-}" ]] && [[ -t 1 ]]; then
    dockpipe_banner
    dockpipe_launch_spinner
  fi

  local container_cwd="${DOCKPIPE_CONTAINER_WORKDIR}"
  if [[ -n "${DOCKPIPE_WORK_PATH:-}" ]]; then
    local work_path="${DOCKPIPE_WORK_PATH#/}"
    container_cwd="${DOCKPIPE_CONTAINER_WORKDIR}/${work_path}"
  fi
  local -a run_args=(
    --init
    --hostname dockpipe
    -u "$(id -u):$(id -g)"
    -v "${workdir_host}:${DOCKPIPE_CONTAINER_WORKDIR}"
    -w "${container_cwd}"
    -e "DOCKPIPE_CONTAINER_WORKDIR=${DOCKPIPE_CONTAINER_WORKDIR}"
  )

  if [[ -n "${action_path}" ]] && [[ -f "${action_path}" ]]; then
    local action_name
    action_name="$(basename "${action_path}" .sh)"
    run_args+=(
      -v "$(realpath "${action_path}"):/dockpipe-action.sh:ro"
      -e "DOCKPIPE_ACTION=/dockpipe-action.sh"
    )
  fi

  # --reinit: remove the named data volume (prompt unless -f).
  if [[ -n "${DOCKPIPE_REINIT:-}" ]] && [[ -n "${DOCKPIPE_DATA_VOLUME:-}" ]]; then
    cat << WARN >&2
  ⚠  REINIT: This will permanently delete all data in volume '${DOCKPIPE_DATA_VOLUME}' (login, cache, repos).
WARN
    if [[ -z "${DOCKPIPE_FORCE:-}" ]]; then
      if [[ ! -t 0 ]]; then
        echo "  No TTY. Use -f to reinit non-interactively." >&2
        exit 1
      fi
      printf '  Continue? [y/N] ' >&2
      read -r resp
      case "${resp:-n}" in
        [yY]|[yY][eE][sS]) ;;
        *) echo "Aborted." >&2; exit 1 ;;
      esac
    fi
    echo "  Removing volume '${DOCKPIPE_DATA_VOLUME}'..." >&2
    docker volume rm "${DOCKPIPE_DATA_VOLUME}" 2>/dev/null || true
    echo "  Done. Starting with a fresh volume." >&2
  fi

  # Data volume: bind mount (--data-dir) or named volume (--data-vol). HOME=/dockpipe-data so tool state persists.
  # One-off chown so the mounted dir is writable by the container user (id -u / id -g).
  if [[ -n "${DOCKPIPE_DATA_DIR:-}" ]]; then
    mkdir -p "${DOCKPIPE_DATA_DIR}"
    run_args+=(-v "${DOCKPIPE_DATA_DIR}:/dockpipe-data")
    run_args+=(-e "DOCKPIPE_DATA=/dockpipe-data" -e "HOME=/dockpipe-data")
    docker run --rm -v "${DOCKPIPE_DATA_DIR}:/dockpipe-data" -u 0 "${img}" sh -c "chown -R $(id -u):$(id -g) /dockpipe-data 2>/dev/null || true"
  elif [[ -n "${DOCKPIPE_DATA_VOLUME:-}" ]]; then
    run_args+=(-v "${DOCKPIPE_DATA_VOLUME}:/dockpipe-data")
    run_args+=(-e "DOCKPIPE_DATA=/dockpipe-data" -e "HOME=/dockpipe-data")
    docker run --rm -v "${DOCKPIPE_DATA_VOLUME}:/dockpipe-data" -u 0 "${img}" sh -c "chown -R $(id -u):$(id -g) /dockpipe-data 2>/dev/null || true"
  fi

  local m
  for m in ${extra_mounts}; do
    [[ -z "$m" ]] && continue
    run_args+=(-v "$m")
  done

  local e
  while IFS= read -r e; do
    [[ -z "$e" ]] && continue
    run_args+=(-e "$e")
  done <<< "${extra_env}"

  if [[ -n "${DOCKPIPE_DETACH:-}" ]]; then
    run_args+=(-d)
  else
    if [[ -t 0 ]]; then
      run_args+=(-it)
    else
      run_args+=(-i)
    fi
  fi

  if [[ -z "${DOCKPIPE_DETACH:-}" ]]; then
    local cid="dockpipe-$$-${RANDOM}"
    run_args+=(--name "$cid")
    local start_sec
    start_sec=$(date +%s)
    docker run "${run_args[@]}" "${img}" "$@" || true
    local rc=$?
    local elapsed=$(($(date +%s) - start_sec))
    if [[ $rc -ne 0 ]] || [[ $elapsed -lt 3 ]]; then
      echo "" >&2
      if [[ $rc -ne 0 ]]; then
        echo "  Container exited with code ${rc}. Full container output:" >&2
      else
        echo "  Container exited quickly (${elapsed}s). Full container output:" >&2
      fi
      echo "  ---" >&2
      docker logs "$cid" 2>&1 | sed 's/^/  /' >&2
      echo "  ---" >&2
    fi
    docker rm "$cid" 2>/dev/null || true
    if [[ -n "${DOCKPIPE_COMMIT_ON_HOST:-}" ]]; then
      dockpipe_commit_on_host "${workdir_host}"
    fi
    return "$rc"
  fi

  run_args+=(--rm)
  exec docker run "${run_args[@]}" "${img}" "$@"
}
