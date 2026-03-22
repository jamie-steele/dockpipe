#!/usr/bin/env bash
# Used by DockPipe workflow dorkpipe-self-analysis-stack — compose down unless skipped.
# Set DORKPIPE_DEV_STACK_AUTODOWN=0 to keep Postgres+Ollama running after the workflow.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ "${DORKPIPE_DEV_STACK_AUTODOWN:-1}" == "0" ]]; then
	echo "dev-stack-maybe-down: DORKPIPE_DEV_STACK_AUTODOWN=0 — leaving sidecars up"
	exit 0
fi
exec "$SCRIPT_DIR/dev-stack.sh" down
