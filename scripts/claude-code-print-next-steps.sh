#!/usr/bin/env bash
# Host-only follow-up (skip_container step). Not an Anthropic product integration.
# Claude Code is normally run from the repo root: `cd project && claude` (see README).
set -euo pipefail

W="${DOCKPIPE_WORKDIR:-$PWD}"
W="$(cd "$W" && pwd)"

CLAUDE_CODE_SKIP_DOCKER_CHECK="${CLAUDE_CODE_SKIP_DOCKER_CHECK:-0}"
if [[ "${CLAUDE_CODE_SKIP_DOCKER_CHECK}" != "1" ]] && command -v docker >/dev/null 2>&1; then
  if ! docker version >/dev/null 2>&1; then
    printf '[dockpipe] Docker daemon is not reachable — not launching claude.\n' >&2
    printf '  Start Docker Desktop (or Linux: sudo systemctl start docker), then re-run.\n' >&2
    exit 1
  fi
fi

# CLAUDE_CODE_LAUNCH: none = print only; cli = try `claude` on PATH + common npm global paths.
CLAUDE_CODE_LAUNCH="${CLAUDE_CODE_LAUNCH:-cli}"
CLAUDE_CODE_WAIT="${CLAUDE_CODE_WAIT:-0}"
CLAUDE_CODE_CMD="${CLAUDE_CODE_CMD:-}"

LAUNCH_PID=""

# Run Claude Code with project cwd (not “claude /path/to/repo” — CLI expects cwd).
run_claude_in_project() {
  local exe="$1"
  if [[ -f "$exe" ]]; then
    :
  elif command -v "$exe" >/dev/null 2>&1; then
    exe=$(command -v "$exe")
  else
    return 1
  fi
  printf '[dockpipe] Starting Claude Code in %s (%s)\n' "$W" "$exe" >&2
  pushd "$W" >/dev/null || return 1
  "$exe" >/dev/null 2>&1 &
  LAUNCH_PID=$!
  popd >/dev/null || true
  return 0
}

try_launch_claude() {
  LAUNCH_PID=""
  if [[ "${CLAUDE_CODE_LAUNCH}" == "none" ]] || [[ "${CLAUDE_CODE_LAUNCH}" == "0" ]]; then
    return 1
  fi
  if [[ "${CLAUDE_CODE_LAUNCH}" != "cli" ]]; then
    printf '[dockpipe] Unknown CLAUDE_CODE_LAUNCH=%s (use none or cli).\n' "${CLAUDE_CODE_LAUNCH}" >&2
    return 1
  fi

  if [[ -n "${CLAUDE_CODE_CMD:-}" ]]; then
    if run_claude_in_project "${CLAUDE_CODE_CMD}"; then
      return 0
    fi
    printf '[dockpipe] CLAUDE_CODE_CMD not found or not runnable: %s\n' "${CLAUDE_CODE_CMD}" >&2
    return 1
  fi

  if command -v claude >/dev/null 2>&1; then
    run_claude_in_project "$(command -v claude)"
    return 0
  fi

  # npm global (user)
  for c in \
    "${HOME}/.npm-global/bin/claude" \
    "${HOME}/.local/share/npm/bin/claude" \
    /usr/local/bin/claude; do
    if [[ -f "$c" ]] || [[ -x "$c" ]]; then
      run_claude_in_project "$c"
      return 0
    fi
  done

  # Windows: npm puts claude.cmd in Roaming/npm
  if [[ -n "${WINDIR:-}${SYSTEMROOT:-}" ]]; then
    local _ap="${APPDATA:-${USERPROFILE:-$HOME}/AppData/Roaming}"
    for c in \
      "${_ap}/npm/claude.cmd" \
      "${_ap}/npm/claude" \
      "${HOME}/AppData/Roaming/npm/claude.cmd"; do
      if [[ -f "$c" ]]; then
        run_claude_in_project "$c"
        return 0
      fi
    done
  fi

  return 1
}

printf '\n[claude-code] Next step on the host:\n' >&2
printf '  Install Claude Code if needed: npm i -g @anthropic-ai/claude-code\n' >&2
printf '  Then from a terminal, in this folder, run: claude\n' >&2
printf '  Project: %s\n' "$W" >&2

if try_launch_claude; then
  if [[ "${CLAUDE_CODE_WAIT}" == "1" ]] && [[ -n "${LAUNCH_PID:-}" ]]; then
    printf '[dockpipe] Waiting for Claude Code PID %s (Ctrl+C stops dockpipe).\n' "${LAUNCH_PID}" >&2
    wait "${LAUNCH_PID}" || true
  fi
else
  printf '\n[claude-code] `claude` not found on PATH (install @anthropic-ai/claude-code globally).\n' >&2
fi

printf '\nIn-container Claude Code: dockpipe --workflow llm-worktree --resolver claude --repo <url> -- claude -p "…"\n' >&2
printf 'Browser IDE (code-server): dockpipe --workflow vscode\n' >&2
