#!/usr/bin/env bash
# Lightweight guardrail: fail if docs/templates reintroduce pre-assets paths or removed legacy dirs.
# Run from repo root: bash src/scripts/check-templates-core-paths.sh
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

# Regexes only — do not spell obsolete paths verbatim in comments or this file will self-match.
OBSOLETE_SCRIPTS_IMAGES='(src/core|src/templates|templates)/core/(scripts|images)/'
OBSOLETE_LEGACY_DIRS='(src/core|src/templates|templates)/core/workflows/|(src/templates|templates)/run-worktree/'

fail() {
  echo "check-templates-core-paths: FAIL — $1" >&2
  exit 1
}

# Bundled framework paths use templates/core/assets/{scripts,images,compose}/ only.
if grep -rE "${OBSOLETE_SCRIPTS_IMAGES}" \
    --include='*.md' \
    --include='*.go' \
    docs README.md AGENTS.md CONTRIBUTING.md templates lib cmd 2>/dev/null | grep -q .; then
  grep -rE "${OBSOLETE_SCRIPTS_IMAGES}" \
    --include='*.md' \
    --include='*.go' \
    docs README.md AGENTS.md CONTRIBUTING.md templates lib cmd 2>/dev/null || true
  fail 'found pre-assets templates/core scripts or images path; use templates/core/assets/…'
fi

if grep -rE "${OBSOLETE_LEGACY_DIRS}" \
    --include='*.md' \
    --include='*.go' \
    docs README.md AGENTS.md CONTRIBUTING.md src/core src/lib src/cmd 2>/dev/null | grep -q .; then
  grep -rE "${OBSOLETE_LEGACY_DIRS}" \
    --include='*.md' \
    --include='*.go' \
    docs README.md AGENTS.md CONTRIBUTING.md src/core src/lib src/cmd 2>/dev/null || true
  fail 'found obsolete workflows/ or run-worktree/ under templates; see docs and workflow_dirs.go'
fi

exit 0
