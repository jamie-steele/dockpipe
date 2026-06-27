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

export ROOT
export HOME="$tmp/home"
export FAKE_DOCKPIPE_ARGS="$fake_args"
export DORKPIPE_ORCH_AUTH_MOUNT_MODE=ro
mkdir -p "$HOME/.codex"

prompt="$tmp/prompt.md"
response="$tmp/response.md"
printf 'hello from prompt\n' > "$prompt"

dorkpipe_orchestrate_run_container_worker codex "$prompt" "$response"

grep -qx -- "--resolver" "$fake_args"
grep -qx -- "codex" "$fake_args"
grep -qx -- "--no-data" "$fake_args"
grep -qx -- "--mount" "$fake_args"
grep -qx -- "$HOME/.codex:/home/node/.codex:ro" "$fake_args"
grep -qx -- "codex" "$fake_args"
grep -qx -- "exec" "$fake_args"
grep -qx -- "--dangerously-bypass-approvals-and-sandbox" "$fake_args"
grep -qx -- "container worker ok" "$response"

echo "test_orchestration_container_auth OK"
