#!/usr/bin/env bash
# Host prep: on-disk hints for Claude Code on the host (CLI; no separate “Claude Code server” in this template).
set -euo pipefail

ROOT="${DOCKPIPE_WORKDIR:-$PWD}"
ROOT="$(cd "$ROOT" && pwd)"
DIR="$ROOT/.dockpipe/claude-code"
mkdir -p "$DIR"

cat > "$DIR/README.txt" <<'EOF'
claude-code template (Dockpipe)

This folder is created by the claude-code workflow. Dockpipe does not install Anthropic’s
Claude Code CLI for you — install it on the host (e.g. npm i -g @anthropic-ai/claude-code)
and run `claude` from your project directory.

When you run isolate steps, your project is mounted at /work inside the container.

For Claude Code inside a container with dockpipe, see the llm-worktree workflow and
`--resolver claude` / `--isolate claude` in the main README.
EOF

printf '[dockpipe] Wrote %s\n' "$DIR/README.txt" >&2
