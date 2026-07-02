#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fake_dockpipe="$tmp/dockpipe"
fake_docker="$tmp/docker"
fake_codex="$tmp/codex"
fake_claude="$tmp/claude"
fake_skills_render="$tmp/skills-render"
fake_orchestrate_helper="$tmp/orchestrate-helper"
fake_args="$tmp/args.txt"
fake_docker_args="$tmp/docker-args.txt"
fake_login_args="$tmp/login-args.txt"
fake_lease_state="$tmp/lease-state.txt"
cat >"$fake_dockpipe" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "scope" && "${2:-}" == "resolver" ]]; then
  case "${3:-}:${4:-}" in
    codex:auth-dir) printf '%s\n' "${CODEX_HOME:-$HOME/.codex}" ;;
    codex:container-auth-dir) printf '%s\n' "/home/node/.codex" ;;
    codex:auth-mount-mode) printf '%s\n' "ro" ;;
    codex:config-file|codex:container-config-file) printf '\n' ;;
    claude:auth-dir) printf '%s\n' "${CLAUDE_HOME:-$HOME/.claude}" ;;
    claude:container-auth-dir) printf '%s\n' "/home/node/.claude" ;;
    claude:auth-mount-mode) printf '%s\n' "ro" ;;
    claude:config-file) printf '%s\n' "${CLAUDE_CONFIG_HOME:-$HOME/.claude.json}" ;;
    claude:container-config-file) printf '%s\n' "/home/node/.claude.json" ;;
    *) exit 1 ;;
  esac
  exit 0
fi
if [[ "${1:-}" == "session" && "${2:-}" == "worker-acquire" ]]; then
  mode="serialized"
  worker=""
  state_file="${FAKE_LEASE_STATE:-}"
  while (($#)); do
    case "${1:-}" in
      --mode) mode="${2:-}"; shift 2 ;;
      --worker) worker="${2:-}"; shift 2 ;;
      *) shift ;;
    esac
  done
  if [[ "${mode}" == "serialized" && -n "${FAKE_LEASE_FAIL_ONCE_WORKER:-}" && "${worker}" == "${FAKE_LEASE_FAIL_ONCE_WORKER}" && -n "${state_file}" ]]; then
    if [[ ! -f "${state_file}" ]]; then
      printf 'session "test" already has an active worker lease for "other" (serialized); wait for that writer to finish before requesting serialized mode\n' >&2
      : > "${state_file}"
      exit 1
    fi
  fi
  volume="dockpipe-session-volume"
  base_volume="dockpipe-session-volume"
  if [[ "${mode}" == "split-volume" ]]; then
    volume="dockpipe-session-volume-worker-${worker}"
  fi
  cat <<EOF
{
  "lease_id": "lease-${worker}",
  "worker_id": "${worker}",
  "mode": "${mode}",
  "status": "active",
  "volume": "${volume}",
  "base_volume": "${base_volume}"
}
EOF
  exit 0
fi
if [[ "${1:-}" == "session" && "${2:-}" == "worker-release" ]]; then
  exit 0
fi
printf '%s\n' "$@" > "${FAKE_DOCKPIPE_ARGS:?}"
printf 'container worker ok\n'
SH
chmod +x "$fake_dockpipe"
cat >"$fake_docker" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" >> "${FAKE_DOCKER_ARGS:?}"
case "${1:-}:${2:-}" in
  image:inspect)
    case "${3:-}" in
      dockpipe-*-tools:*) exit 1 ;;
    esac
    exit 0
    ;;
  build:*)
    exit 0
    ;;
esac
exit 0
SH
chmod +x "$fake_docker"
cat >"$fake_codex" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf 'codex:%s\n' "$*" >> "${FAKE_LOGIN_ARGS:?}"
if [[ "${1:-}" == "login" ]]; then
  mkdir -p "$HOME/.codex"
  printf '{"auth_mode":"chatgpt"}\n' > "$HOME/.codex/auth.json"
  exit 0
fi
exit 1
SH
chmod +x "$fake_codex"
cat >"$fake_claude" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf 'claude:%s\n' "$*" >> "${FAKE_LOGIN_ARGS:?}"
if [[ "${1:-}" == "login" ]]; then
  mkdir -p "$HOME/.claude"
  printf '{"claudeAiOauth":{}}\n' > "$HOME/.claude/.credentials.json"
  printf '{"oauthAccount":{}}\n' > "$HOME/.claude.json"
  exit 0
fi
exit 1
SH
chmod +x "$fake_claude"
cat >"$fake_skills_render" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
target=""
output=""
while (($#)); do
  case "${1:-}" in
    --target)
      target="${2:-}"
      shift 2
      ;;
    --output)
      output="${2:-}"
      shift 2
      ;;
    --force)
      shift
      ;;
    *)
      shift
      ;;
  esac
done
[[ -n "${target}" && -n "${output}" ]] || exit 1
mkdir -p "${output}/dorkpipe-core-review"
printf '# curated %s skill\n' "${target}" > "${output}/dorkpipe-core-review/SKILL.md"
SH
chmod +x "$fake_skills_render"
cat >"$fake_orchestrate_helper" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "worker-lease-env" ]]; then
  file="${2:?lease json}"
  json="$(tr -d '\r\n' < "${file}")"
  extract() {
    local key="${1:?key}"
    printf '%s' "${json}" | sed -n "s/.*\"${key}\": \"\\([^\"]*\\)\".*/\\1/p"
  }
  printf "LEASE_BASE_VOLUME='%s'\n" "$(extract base_volume)"
  printf "LEASE_ID='%s'\n" "$(extract lease_id)"
  printf "LEASE_MODE='%s'\n" "$(extract mode)"
  printf "LEASE_STATUS='%s'\n" "$(extract status)"
  printf "LEASE_VOLUME='%s'\n" "$(extract volume)"
  printf "LEASE_WORKER_ID='%s'\n" "$(extract worker_id)"
  exit 0
fi
exit 1
SH
chmod +x "$fake_orchestrate_helper"

dockpipe_sdk() {
  if [[ "${1:-}" == "require" && "${2:-}" == "dockpipe-bin" ]]; then
    printf '%s\n' "$fake_dockpipe"
    return 0
  fi
  return 1
}

saved_os="${OS:-}"
saved_ostype="${OSTYPE:-}"
saved_msystem="${MSYSTEM:-}"
OS=Windows_NT
OSTYPE=msys
MSYSTEM=MINGW64
converted="$(dorkpipe_orchestrate_cli_mount_host_path 'C:\Users\Jamie\.codex')"
if [[ "${converted}" != 'C:\Users\Jamie\.codex' ]]; then
  echo "expected Windows auth mount host path to stay native when MSYS conversion is disabled, got: ${converted}" >&2
  exit 1
fi
export DORKPIPE_ORCH_WORKER_CWD='C:/Program Files/Git/UniteHere'
converted="$(dorkpipe_orchestrate_worker_cwd codex)"
if [[ "${converted}" != "/UniteHere" ]]; then
  echo "expected Git Bash converted worker cwd to normalize to /UniteHere, got: ${converted}" >&2
  exit 1
fi
unset DORKPIPE_ORCH_WORKER_CWD
export DORKPIPE_ORCH_WORKER_CWD="/UniteHere"
export DORKPIPE_ORCH_CLAUDE_WORKER_CWD="/work"
converted="$(dorkpipe_orchestrate_worker_cwd claude)"
if [[ "${converted}" != "/work" ]]; then
  echo "expected provider-specific Claude worker cwd override, got: ${converted}" >&2
  exit 1
fi
unset DORKPIPE_ORCH_WORKER_CWD
unset DORKPIPE_ORCH_CLAUDE_WORKER_CWD
OS="${saved_os}"
OSTYPE="${saved_ostype}"
MSYSTEM="${saved_msystem}"

export ROOT
export HOME="$tmp/home"
export USERPROFILE="$HOME"
export FAKE_DOCKPIPE_ARGS="$fake_args"
export FAKE_DOCKER_ARGS="$fake_docker_args"
export FAKE_LOGIN_ARGS="$fake_login_args"
export FAKE_LEASE_STATE="$fake_lease_state"
export DORKPIPE_ORCH_ROOT="$tmp/orchestrate"
export DOCKPIPE_CONTAINER_MOUNTS=$'C:\\Source\\UniteHere:/UniteHere:ro\nC:\\docs\\UniteHere\\Design Notes:/DesignNotes:ro'
export DORKPIPE_ORCH_WORKER_CWD="/UniteHere"
export DORKPIPE_ORCH_CONTAINER_IMAGE_PACKAGES="make cmake"
export DOCKPIPE_SKILLS_RENDER_BIN="$fake_skills_render"
export DORKPIPE_ORCH_HELPER_BIN="$fake_orchestrate_helper"
export PATH="$tmp:$PATH"
mkdir -p "$DORKPIPE_ORCH_ROOT"
mkdir -p "$HOME/.codex"
mkdir -p "$HOME/.codex/skills/dorkpipe-package-authoring"
mkdir -p "$HOME/.claude"
mkdir -p "$HOME/.claude/skills/dorkpipe-package-authoring"
export DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING="never"
if dorkpipe_orchestrate_auth_preflight codex 2>"$tmp/codex-auth-never.err"; then
  echo "expected codex auth preflight to fail when login is disabled and auth is missing" >&2
  exit 1
fi
grep -q -- "codex auth preflight failed" "$tmp/codex-auth-never.err"
export DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING="always"
dorkpipe_orchestrate_auth_preflight codex
grep -qx -- "codex:login" "$fake_login_args"
rm -f "$fake_login_args"
dorkpipe_orchestrate_auth_preflight claude
grep -qx -- "claude:login" "$fake_login_args"
unset DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING
printf '{"auth_mode":"chatgpt"}\n' > "$HOME/.codex/auth.json"
printf '# skill\n' > "$HOME/.codex/skills/dorkpipe-package-authoring/SKILL.md"
printf '{"claudeAiOauth":{}}\n' > "$HOME/.claude/.credentials.json"
printf '{"oauthAccount":{}}\n' > "$HOME/.claude.json"
printf '# skill\n' > "$HOME/.claude/skills/dorkpipe-package-authoring/SKILL.md"
export CLAUDE_HOME="/home/node/.claude"
export CODEX_HOME="/home/node/.codex"
dorkpipe_orchestrate_auth_preflight codex
dorkpipe_orchestrate_auth_preflight claude
unset CLAUDE_HOME
unset CODEX_HOME

prompt="$tmp/prompt.md"
response="$tmp/response.md"
printf 'hello from prompt\n' > "$prompt"

if [[ "$(dorkpipe_orchestrate_work_mode)" != "artifact" ]]; then
  echo "expected default orchestration work mode to be artifact" >&2
  exit 1
fi
dorkpipe_orchestrate_append_work_mode_prompt "$prompt"
grep -q -- "DorkPipe Work Mode: artifact" "$prompt"
grep -q -- "readonly artifact-gathering mode" "$prompt"
grep -q -- "Do not use apply_patch" "$prompt"

dorkpipe_orchestrate_run_container_worker codex "$prompt" "$response"

codex_stage_dir="$DORKPIPE_ORCH_ROOT/skills/codex"
[[ -f "$codex_stage_dir/dorkpipe-package-authoring/SKILL.md" ]]
[[ -f "$codex_stage_dir/dorkpipe-core-review/SKILL.md" ]]

grep -qx -- "--resolver" "$fake_args"
grep -qx -- "codex" "$fake_args"
grep -qx -- "--no-data" "$fake_args"
grep -Fqx -- "DORKPIPE_ORCH_WORK_MODE=artifact" "$fake_args"
grep -Fqx -- "DOCKPIPE_DOCKER_NETWORK=bridge" "$fake_args"
grep -qx -- "--isolate" "$fake_args"
grep -Eqx -- "dockpipe-codex-tools:[0-9a-f]{12}" "$fake_args"
grep -q -- "build" "$fake_docker_args"
grep -q -- "dockpipe-codex-tools:" "$fake_docker_args"
grep -qx -- "--mount" "$fake_args"
grep -qx -- "$HOME/.codex:/dockpipe-auth/codex:ro" "$fake_args"
grep -qx -- "$codex_stage_dir:/dockpipe-auth/codex-skills:ro" "$fake_args"
grep -Fqx -- "C:\\Source\\UniteHere:/UniteHere:ro" "$fake_args"
grep -Fqx -- "C:\\docs\\UniteHere\\Design Notes:/DesignNotes:ro" "$fake_args"
grep -qx -- "/UniteHere" "$fake_args"
grep -qx -- "codex" "$fake_args"
grep -q -- "codex exec" "$fake_args"
grep -q -- "--dangerously-bypass-approvals-and-sandbox" "$fake_args"
grep -qx -- "container worker ok" "$response"

export DORKPIPE_ORCH_CONTAINER_SKILLS="never"
rm -f "$fake_args" "$response"
dorkpipe_orchestrate_run_container_worker codex "$prompt" "$response"
if grep -qx -- "$codex_stage_dir:/dockpipe-auth/codex-skills:ro" "$fake_args"; then
  echo "expected DORKPIPE_ORCH_CONTAINER_SKILLS=never to skip skills mount" >&2
  exit 1
fi
unset DORKPIPE_ORCH_CONTAINER_SKILLS

export DORKPIPE_ORCH_WORK_MODE="edit"
export DOCKPIPE_SESSION_ID="brain-optimize-session"
export DOCKPIPE_SESSION_VOLUME="dockpipe-session-volume"
export DOCKPIPE_CONTAINER_MOUNTS=$'C:\\Source\\UniteHere:/UniteHere:rw\nC:\\docs\\UniteHere\\Design Notes:/DesignNotes:ro'
export FAKE_LEASE_FAIL_ONCE_WORKER="codex-serialized-retry"
export DORKPIPE_ORCH_EDIT_LEASE_RETRY_SECONDS="0.01"
export DORKPIPE_ORCH_EDIT_LEASE_MAX_WAIT_SECONDS="5"
rm -f "$fake_lease_state"
lease_json="$tmp/lease-retry.json"
dorkpipe_orchestrate_edit_worker_acquire "codex-serialized-retry" "$lease_json" 2>"$tmp/lease-retry.err"
[[ -f "$lease_json" ]]
grep -q -- "waiting for serialized edit lease for codex-serialized-retry" "$tmp/lease-retry.err"
unset FAKE_LEASE_FAIL_ONCE_WORKER
unset DORKPIPE_ORCH_EDIT_LEASE_RETRY_SECONDS
unset DORKPIPE_ORCH_EDIT_LEASE_MAX_WAIT_SECONDS
edit_prompt="$tmp/edit-prompt.md"
printf 'edit prompt\n' > "$edit_prompt"
dorkpipe_orchestrate_append_work_mode_prompt "$edit_prompt"
grep -q -- "DorkPipe Work Mode: edit" "$edit_prompt"
grep -q -- "direct workspace edit mode" "$edit_prompt"
rm -f "$fake_args" "$response"
dorkpipe_orchestrate_run_container_worker codex "$edit_prompt" "$response"
grep -Fqx -- "DORKPIPE_ORCH_WORK_MODE=edit" "$fake_args"
grep -Fqx -- "C:\\Source\\UniteHere:/UniteHere:rw" "$fake_args"
grep -Fqx -- "DOCKPIPE_SESSION_VOLUME=dockpipe-session-volume" "$fake_args"
grep -Fqx -- "DOCKPIPE_SESSION_VOLUME_AUTHORITATIVE=1" "$fake_args"
grep -qx -- "container worker ok" "$response"
export DORKPIPE_ORCH_WORK_MODE="artifact"
export DOCKPIPE_CONTAINER_MOUNTS=$'C:\\Source\\UniteHere:/UniteHere:ro\nC:\\docs\\UniteHere\\Design Notes:/DesignNotes:ro'

export DORKPIPE_ORCH_WORK_MODE="edit"
export DORKPIPE_ORCH_EDIT_ISOLATION="split-volume"
export DOCKPIPE_SESSION_ID="brain-optimize-session"
export DOCKPIPE_SESSION_VOLUME="dockpipe-session-volume"
rm -f "$fake_args" "$response"
dorkpipe_orchestrate_run_container_worker codex "$edit_prompt" "$response"
grep -Fqx -- "DOCKPIPE_SESSION_VOLUME=dockpipe-session-volume-worker-worker" "$fake_args"
grep -Fqx -- "DOCKPIPE_SESSION_VOLUME_AUTHORITATIVE=1" "$fake_args"
grep -qx -- "container worker ok" "$response"
unset DORKPIPE_ORCH_EDIT_ISOLATION
unset DOCKPIPE_SESSION_ID
unset DOCKPIPE_SESSION_VOLUME
export DORKPIPE_ORCH_WORK_MODE="artifact"

export CODEX_HOME="/home/node/.codex"
export CLAUDE_HOME="/home/node/.claude"
export CLAUDE_CONFIG_HOME="/home/node/.claude.json"
rm -f "$fake_args" "$response"
dorkpipe_orchestrate_run_container_worker codex "$prompt" "$response"
grep -qx -- "$HOME/.codex:/dockpipe-auth/codex:ro" "$fake_args"
rm -f "$fake_args" "$response"
dorkpipe_orchestrate_run_container_worker claude "$prompt" "$response"
claude_stage_dir="$DORKPIPE_ORCH_ROOT/skills/claude"
[[ -f "$claude_stage_dir/dorkpipe-package-authoring/SKILL.md" ]]
[[ -f "$claude_stage_dir/dorkpipe-core-review/SKILL.md" ]]
grep -qx -- "$HOME/.claude:/dockpipe-auth/claude:ro" "$fake_args"
grep -qx -- "$HOME/.claude.json:/dockpipe-auth/claude-config/.claude.json:ro" "$fake_args"
unset CODEX_HOME
unset CLAUDE_HOME
unset CLAUDE_CONFIG_HOME

rm -f "$fake_args" "$response"
dorkpipe_orchestrate_run_container_worker claude "$prompt" "$response"
grep -qx -- "--resolver" "$fake_args"
grep -qx -- "claude" "$fake_args"
grep -Fqx -- "DOCKPIPE_DOCKER_NETWORK=bridge" "$fake_args"
grep -qx -- "$HOME/.claude:/dockpipe-auth/claude:ro" "$fake_args"
grep -qx -- "$HOME/.claude.json:/dockpipe-auth/claude-config/.claude.json:ro" "$fake_args"
grep -qx -- "$claude_stage_dir:/dockpipe-auth/claude-skills:ro" "$fake_args"
grep -q -- "cp /dockpipe-auth/claude-config/.claude.json /home/node/.claude.json" "$fake_args"
grep -q -- "claude --dangerously-skip-permissions" "$fake_args"
grep -qx -- "container worker ok" "$response"

echo "test_orchestration_container_auth OK"
