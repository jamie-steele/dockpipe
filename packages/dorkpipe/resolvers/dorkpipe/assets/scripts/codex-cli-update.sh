#!/usr/bin/env bash
set -euo pipefail

dockpipe_cmd="${DOCKPIPE_BIN:-dockpipe}"
if ! command -v "$dockpipe_cmd" >/dev/null 2>&1 && command -v dockpipe >/dev/null 2>&1; then
  dockpipe_cmd="dockpipe"
fi

eval "$("$dockpipe_cmd" sdk)"
dockpipe_sdk init-script

approval="$(dockpipe_sdk prompt confirm \
  --id codex-cli-update \
  --title "Update Codex CLI" \
  --message "Update the host Codex CLI through npm and verify its App Server schema?" \
  --default no \
  --intent system_change \
  --allow-auto-approve \
  --auto-approve-value yes)"
case "${approval,,}" in
  yes|y)
    ;;
  *)
    echo "Codex CLI update was not approved." >&2
    exit 1
    ;;
esac

npm_cmd="${DORKPIPE_NPM_BIN:-npm}"
codex_cmd="${DORKPIPE_CODEX_BIN:-codex}"
command -v "$npm_cmd" >/dev/null 2>&1 || dockpipe_sdk die "npm is required to update the Codex CLI"

"$npm_cmd" install --global @openai/codex@latest
hash -r
command -v "$codex_cmd" >/dev/null 2>&1 || dockpipe_sdk die "updated Codex CLI is not discoverable"
"$codex_cmd" --version

schema_dir="$(mktemp -d "${TMPDIR:-/tmp}/dockpipe-codex-schema.XXXXXX")"
trap 'rm -rf "$schema_dir"' EXIT
"$codex_cmd" app-server generate-json-schema --out "$schema_dir" >/dev/null

echo "Codex CLI updated and App Server schema verified."
