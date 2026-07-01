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
fake_args="$tmp/args.txt"
fake_docker_args="$tmp/docker-args.txt"
cat >"$fake_dockpipe" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "scope" && "${2:-}" == "resolver" ]]; then
  case "${3:-}:${4:-}" in
    codex:auth-dir) printf '%s\n' "$HOME/.codex" ;;
    codex:container-auth-dir) printf '%s\n' "/home/node/.codex" ;;
    codex:auth-mount-mode) printf '%s\n' "ro" ;;
    codex:config-file|codex:container-config-file) printf '\n' ;;
    claude:auth-dir) printf '%s\n' "$HOME/.claude" ;;
    claude:container-auth-dir) printf '%s\n' "/home/node/.claude" ;;
    claude:auth-mount-mode) printf '%s\n' "ro" ;;
    claude:config-file) printf '%s\n' "$HOME/.claude.json" ;;
    claude:container-config-file) printf '%s\n' "/home/node/.claude.json" ;;
    *) exit 1 ;;
  esac
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
export FAKE_DOCKPIPE_ARGS="$fake_args"
export FAKE_DOCKER_ARGS="$fake_docker_args"
export DORKPIPE_ORCH_ROOT="$tmp/orchestrate"
export DOCKPIPE_CONTAINER_MOUNTS=$'C:\\Source\\UniteHere:/UniteHere:ro\nC:\\docs\\UniteHere\\Design Notes:/DesignNotes:ro'
export DORKPIPE_ORCH_WORKER_CWD="/UniteHere"
export DORKPIPE_ORCH_CONTAINER_IMAGE_PACKAGES="make cmake"
export PATH="$tmp:$PATH"
mkdir -p "$DORKPIPE_ORCH_ROOT"
mkdir -p "$HOME/.codex"
mkdir -p "$HOME/.codex/skills/dorkpipe-package-authoring"
mkdir -p "$HOME/.claude"
mkdir -p "$HOME/.claude/skills/dorkpipe-package-authoring"
printf '{"auth_mode":"chatgpt"}\n' > "$HOME/.codex/auth.json"
printf '# skill\n' > "$HOME/.codex/skills/dorkpipe-package-authoring/SKILL.md"
printf '{"claudeAiOauth":{}}\n' > "$HOME/.claude/.credentials.json"
printf '{"oauthAccount":{}}\n' > "$HOME/.claude.json"
printf '# skill\n' > "$HOME/.claude/skills/dorkpipe-package-authoring/SKILL.md"

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

grep -qx -- "--resolver" "$fake_args"
grep -qx -- "codex" "$fake_args"
grep -qx -- "--no-data" "$fake_args"
grep -Fqx -- "DORKPIPE_ORCH_WORK_MODE=artifact" "$fake_args"
grep -qx -- "--isolate" "$fake_args"
grep -Eqx -- "dockpipe-codex-tools:[0-9a-f]{12}" "$fake_args"
grep -q -- "build" "$fake_docker_args"
grep -q -- "dockpipe-codex-tools:" "$fake_docker_args"
grep -qx -- "--mount" "$fake_args"
grep -qx -- "$HOME/.codex:/dockpipe-auth/codex:ro" "$fake_args"
grep -qx -- "$HOME/.codex/skills:/dockpipe-auth/codex-skills:ro" "$fake_args"
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
if grep -qx -- "$HOME/.codex/skills:/dockpipe-auth/codex-skills:ro" "$fake_args"; then
  echo "expected DORKPIPE_ORCH_CONTAINER_SKILLS=never to skip skills mount" >&2
  exit 1
fi
unset DORKPIPE_ORCH_CONTAINER_SKILLS

export DORKPIPE_ORCH_WORK_MODE="edit"
export DOCKPIPE_CONTAINER_MOUNTS=$'C:\\Source\\UniteHere:/UniteHere:rw\nC:\\docs\\UniteHere\\Design Notes:/DesignNotes:ro'
edit_prompt="$tmp/edit-prompt.md"
printf 'edit prompt\n' > "$edit_prompt"
dorkpipe_orchestrate_append_work_mode_prompt "$edit_prompt"
grep -q -- "DorkPipe Work Mode: edit" "$edit_prompt"
grep -q -- "direct workspace edit mode" "$edit_prompt"
rm -f "$fake_args" "$response"
dorkpipe_orchestrate_run_container_worker codex "$edit_prompt" "$response"
grep -Fqx -- "DORKPIPE_ORCH_WORK_MODE=edit" "$fake_args"
grep -Fqx -- "C:\\Source\\UniteHere:/UniteHere:rw" "$fake_args"
grep -qx -- "container worker ok" "$response"
export DORKPIPE_ORCH_WORK_MODE="artifact"
export DOCKPIPE_CONTAINER_MOUNTS=$'C:\\Source\\UniteHere:/UniteHere:ro\nC:\\docs\\UniteHere\\Design Notes:/DesignNotes:ro'

rm -f "$fake_args" "$response"
dorkpipe_orchestrate_run_container_worker claude "$prompt" "$response"
grep -qx -- "--resolver" "$fake_args"
grep -qx -- "claude" "$fake_args"
grep -qx -- "$HOME/.claude:/dockpipe-auth/claude:ro" "$fake_args"
grep -qx -- "$HOME/.claude.json:/dockpipe-auth/claude-config/.claude.json:ro" "$fake_args"
grep -qx -- "$HOME/.claude/skills:/dockpipe-auth/claude-skills:ro" "$fake_args"
grep -q -- "cp /dockpipe-auth/claude-config/.claude.json /home/node/.claude.json" "$fake_args"
grep -q -- "claude --dangerously-skip-permissions" "$fake_args"
grep -qx -- "container worker ok" "$response"

echo "test_orchestration_container_auth OK"
