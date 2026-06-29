#!/usr/bin/env bash
# Step 1 helper: op inject → file consumed by DockPipe outputs merge (next step sees KEY=VAL in env).
# Requires: op, OP_ENV_FILE (default .env.op.template), SECRET_ENV_OUT (must match step outputs: path).
set -euo pipefail

die() { echo "secretstore-op-inject: $*" >&2; exit 1; }

command -v op >/dev/null 2>&1 || die "install 1Password CLI (https://developer.1password.com/docs/cli/)"

OP_SRC="${OP_ENV_FILE:-.env.op.template}"
[[ -f "$OP_SRC" ]] || die "missing $OP_SRC (op:// template; copy from packages/secrets/resolvers/onepassword/.env.op.template.example)"

OUT_REL="${SECRET_ENV_OUT:-}"
[[ -n "$OUT_REL" ]] || die "set SECRET_ENV_OUT to the same path as this step's outputs: in the workflow YAML"
case "$OUT_REL" in
  /*) OUT_PATH="$OUT_REL" ;;
  *) OUT_PATH="$(dockpipe scope artifacts "$OUT_REL")" ;;
esac

# Single file: op inject overwrites
mkdir -p "$(dirname "$OUT_PATH")"
op inject -i "$OP_SRC" -o "$OUT_PATH"

# Ensure merge runs even if template only had comments (op can produce empty parse)
if ! grep -qE '^[[:space:]]*[^#[:space:]][^=]*=' "$OUT_PATH" 2>/dev/null; then
  printf 'DOCKPIPE_SECRETSTORE_INJECT_PLACEHOLDER=1\n' >> "$OUT_PATH"
  echo "secretstore-op-inject: warning: no KEY= lines from op inject; appended placeholder (remove from template when you add real op:// lines)" >&2
fi

echo "secretstore-op-inject: wrote $OUT_PATH for next-step env merge"
