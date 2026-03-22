#!/usr/bin/env bash
# Host prep: small on-disk hints for Cursor desktop use (no remote Cursor server).
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$PWD}"
ROOT="$(cd "$ROOT" && pwd)"
DIR="$ROOT/.dockpipe/cursor-dev"
mkdir -p "$DIR"

cat > "$DIR/README.txt" <<'EOF'
cursor-dev template (Dockpipe)

This folder is created by the cursor-dev workflow. Dockpipe does not install or run a
"Cursor server". The Cursor editor is a desktop application from Anysphere — use it on
the host by opening this repository folder.

When you run isolate steps, your project is mounted at /work inside the container.
EOF

printf '[dockpipe] Wrote %s\n' "$DIR/README.txt" >&2
