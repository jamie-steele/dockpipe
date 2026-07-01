#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fake_dockpipe="$tmp/dockpipe"
fake_args="$tmp/args.txt"
cat >"$fake_dockpipe" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "scope" && "${2:-}" == "resolver" && "${3:-}" == "codex" ]]; then
  case "${4:-}" in
    auth-dir) printf '%s\n' "$HOME/.codex" ;;
    container-auth-dir) printf '%s\n' "/home/node/.codex" ;;
    auth-mount-mode) printf '%s\n' "ro" ;;
    config-file|container-config-file) printf '\n' ;;
    *) exit 1 ;;
  esac
  exit 0
fi
printf '%s\n' "$@" > "${FAKE_DOCKPIPE_ARGS:?}"
printf 'container worker ok\n'
SH
chmod +x "$fake_dockpipe"

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
if [[ "${converted}" != "/c/Users/Jamie/.codex" ]]; then
  echo "expected Windows auth mount host path to convert for Bash CLI launch, got: ${converted}" >&2
  exit 1
fi
export DORKPIPE_ORCH_WORKER_CWD='C:/Program Files/Git/UniteHere'
converted="$(dorkpipe_orchestrate_worker_cwd)"
if [[ "${converted}" != "/UniteHere" ]]; then
  echo "expected Git Bash converted worker cwd to normalize to /UniteHere, got: ${converted}" >&2
  exit 1
fi
unset DORKPIPE_ORCH_WORKER_CWD
OS="${saved_os}"
OSTYPE="${saved_ostype}"
MSYSTEM="${saved_msystem}"

export ROOT
export HOME="$tmp/home"
export FAKE_DOCKPIPE_ARGS="$fake_args"
export DOCKPIPE_CONTAINER_MOUNTS=$'C:\\Source\\UniteHere:/UniteHere:ro\nC:\\docs\\UniteHere\\Design Notes:/DesignNotes:ro'
export DORKPIPE_ORCH_WORKER_CWD="/UniteHere"
export PATH="$tmp:$PATH"
mkdir -p "$HOME/.codex"
mkdir -p "$HOME/.codex/skills/dorkpipe-package-authoring"
printf '{"auth_mode":"chatgpt"}\n' > "$HOME/.codex/auth.json"
printf '# skill\n' > "$HOME/.codex/skills/dorkpipe-package-authoring/SKILL.md"

prompt="$tmp/prompt.md"
response="$tmp/response.md"
printf 'hello from prompt\n' > "$prompt"

dorkpipe_orchestrate_run_container_worker codex "$prompt" "$response"

grep -qx -- "--resolver" "$fake_args"
grep -qx -- "codex" "$fake_args"
grep -qx -- "--no-data" "$fake_args"
grep -qx -- "--mount" "$fake_args"
grep -qx -- "$HOME/.codex:/dockpipe-auth/codex:ro" "$fake_args"
grep -qx -- "$HOME/.codex/skills:/dockpipe-auth/codex-skills:ro" "$fake_args"
grep -qx -- "/c/Source/UniteHere:/UniteHere:ro" "$fake_args"
grep -qx -- "/c/docs/UniteHere/Design Notes:/DesignNotes:ro" "$fake_args"
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

echo "test_orchestration_container_auth OK"
