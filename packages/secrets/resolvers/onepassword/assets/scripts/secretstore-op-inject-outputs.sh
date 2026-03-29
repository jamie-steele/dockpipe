#!/usr/bin/env bash
# Step 1 helper: op inject → file consumed by DockPipe outputs merge (next step sees KEY=VAL in env).
# Maintainer staging — not bundled core. Requires: op, OP_ENV_FILE (default .env.op.template), SECRET_ENV_OUT (must match step outputs: path).
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$(pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
cd "$ROOT"

die() { echo "secretstore-op-inject: $*" >&2; exit 1; }

command -v op >/dev/null 2>&1 || die "install 1Password CLI (https://developer.1password.com/docs/cli/)"

OP_SRC="${OP_ENV_FILE:-.env.op.template}"
[[ -f "$OP_SRC" ]] || die "missing $OP_SRC (op:// template; copy from .staging/workflows/dockpipe/packages/secrets/resolvers/onepassword/.env.op.template.example)"

OUT_REL="${SECRET_ENV_OUT:-}"
[[ -n "$OUT_REL" ]] || die "set SECRET_ENV_OUT to the same path as this step's outputs: in the workflow YAML"

# Single file: op inject overwrites
mkdir -p "$(dirname "$OUT_REL")"
op inject -i "$OP_SRC" -o "$OUT_REL"

# Ensure merge runs even if template only had comments (op can produce empty parse)
if ! grep -qE '^[[:space:]]*[^#[:space:]][^=]*=' "$OUT_REL" 2>/dev/null; then
  printf 'DOCKPIPE_SECRETSTORE_INJECT_PLACEHOLDER=1\n' >> "$OUT_REL"
  echo "secretstore-op-inject: warning: no KEY= lines from op inject; appended placeholder (remove from template when you add real op:// lines)" >&2
fi

echo "secretstore-op-inject: wrote $OUT_REL for next-step env merge"
