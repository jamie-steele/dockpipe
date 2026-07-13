#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SCRIPT="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/codex-cli-update.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

cat >"$TMP/npm" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$DORKPIPE_TEST_LOG"
EOF
cat >"$TMP/codex" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == "--version" ]]; then
  printf 'codex-cli 9.9.9\n'
  exit 0
fi
if [[ "$1" == "app-server" && "$2" == "generate-json-schema" && "$3" == "--out" ]]; then
  mkdir -p "$4"
  : >"$4/schema.json"
  exit 0
fi
exit 1
EOF
chmod +x "$TMP/npm" "$TMP/codex"

export DORKPIPE_TEST_LOG="$TMP/calls.log"
export DORKPIPE_NPM_BIN="$TMP/npm"
export DORKPIPE_CODEX_BIN="$TMP/codex"
export DOCKPIPE_BIN="$ROOT/src/bin/dockpipe"
export DOCKPIPE_APPROVE_PROMPTS=1
export DOCKPIPE_WORKDIR="$ROOT"
export DOCKPIPE_WORKFLOW_NAME="codex.cli.update"
export DOCKPIPE_WORKFLOW_CONFIG="$ROOT/packages/dorkpipe/workflows/codex.cli.update/config.yml"
export DOCKPIPE_SCRIPT_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
export DOCKPIPE_PACKAGE_ROOT="$ROOT/packages/dorkpipe"
export DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
bash "$SCRIPT"

grep -Fx 'install --global @openai/codex@latest' "$DORKPIPE_TEST_LOG"
: >"$DORKPIPE_TEST_LOG"
if printf '\n' | env -u DOCKPIPE_APPROVE_PROMPTS bash "$SCRIPT"; then
  echo "expected the unapproved Codex CLI update to stop" >&2
  exit 1
fi
if [[ -s "$DORKPIPE_TEST_LOG" ]]; then
  echo "unapproved Codex CLI update invoked npm" >&2
  exit 1
fi
echo "test_codex_cli_update.sh OK"
