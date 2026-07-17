#!/usr/bin/env bash
set -euo pipefail
dockpipe_cmd="${DOCKPIPE_BIN:-dockpipe}"
eval "$("$dockpipe_cmd" sdk)"
dockpipe_sdk init-script
if [[ -z "${CODEX_APP_SERVER_BIN:-}" ]]; then echo "cas01: CODEX_APP_SERVER_BIN must be an absolute user-installed Codex path" >&2; exit 2; fi
if [[ "$CODEX_APP_SERVER_BIN" != /* && ! "$CODEX_APP_SERVER_BIN" =~ ^[A-Za-z]:[/\\] ]]; then echo "cas01: CODEX_APP_SERVER_BIN must be absolute; PATH lookup is disabled" >&2; exit 2; fi
HARNESS="${DOCKPIPE_ASSETS_DIR:?DOCKPIPE_ASSETS_DIR is required}/cas01/app_server.go"
if [[ "${1:-}" == "--fixtures" ]]; then exec go run "$HARNESS" --mode fixtures --fixtures "${DOCKPIPE_ASSETS_DIR}/cas01/fixtures/fixtures.json"; fi
exec go run "$HARNESS" --mode live --artifacts "$(dockpipe scope artifacts)" --codex "$CODEX_APP_SERVER_BIN" --workspace "${DOCKPIPE_WORKDIR:-$ROOT}" "$@"