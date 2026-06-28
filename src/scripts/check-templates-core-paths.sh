#!/usr/bin/env bash
# Lightweight guardrail: fail if docs/templates reintroduce pre-assets paths or removed legacy dirs.
# Run from repo root: bash src/scripts/check-templates-core-paths.sh
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

# Regexes only — do not spell obsolete paths verbatim in comments or this file will self-match.
OBSOLETE_SCRIPTS_IMAGES='(src/core|src/templates|templates)/core/(scripts|images)/'
OBSOLETE_LEGACY_DIRS='(src/core|src/templates|templates)/core/workflows/|(src/templates|templates)/run-worktree/'
GENERATED_STATE_ROOT='bin/[.]dockpipe'
DORKPIPE_CI_STATE_PATHS="${GENERATED_STATE_ROOT}/(workflows/ci/dorkpipe|packages/dorkpipe/ci|ci-analysis|ci-raw)"
DORKPIPE_CI_WORKFLOW_SDK='dockpipe_sdk[[:space:]]+path[[:space:]]+workflow[[:space:]]+ci[[:space:]]+dorkpipe'
DORKPIPE_CI_PACKAGE_SDK='dockpipe_sdk[[:space:]]+path[[:space:]]+package[[:space:]]+dorkpipe[[:space:]]+ci'

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

package_hits="$(
  grep -rInE "${DORKPIPE_CI_STATE_PATHS}|${DORKPIPE_CI_WORKFLOW_SDK}" \
    --include='*.go' \
    --include='*.sh' \
    --include='*.md' \
    --include='*.json' \
    --include='*.yml' \
    --include='*.yaml' \
    --include='*.ts' \
    --include='*.js' \
    packages 2>/dev/null || true
)"
package_sdk_hits="$(
  grep -rInE "${DORKPIPE_CI_PACKAGE_SDK}" \
    --include='*.sh' \
    --include='*.md' \
    packages 2>/dev/null || true
)"
if [[ -n "$package_sdk_hits" ]]; then
  package_hits="${package_hits}${package_hits:+$'\n'}${package_sdk_hits}"
fi
if [[ -n "$package_hits" ]]; then
  printf '%s\n' "$package_hits" >&2
  fail 'package-owned files must not hardcode DorkPipe CI generated-state paths; use injected DOCKPIPE_CI_RAW_DIR / DOCKPIPE_CI_ANALYSIS_DIR, Go statepath helpers, or explicit DOCKPIPE_CI_* env inputs'
fi

exit 0
