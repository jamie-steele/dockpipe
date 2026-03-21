#!/usr/bin/env bash
# Host-only follow-up (invoked from a skip_container step). Not a Cursor product integration.
set -euo pipefail

W="${DOCKPIPE_WORKDIR:-$PWD}"
W="$(cd "$W" && pwd)"

printf '\n[cursor-dev] Next step on the host:\n' >&2
printf '  Open the Cursor app → File → Open Folder →\n' >&2
printf '  %s\n' "$W" >&2
printf '\nRemote SSH / Dev Containers are separate setups; this template only prepares the repo.\n' >&2
