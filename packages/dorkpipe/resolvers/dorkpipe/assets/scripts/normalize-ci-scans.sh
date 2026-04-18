#!/usr/bin/env bash
# Normalize gosec + govulncheck JSON into bin/.dockpipe/ci-analysis/ for DorkPipe downstream reasoning.
# Prerequisites: jq. Raw inputs: bin/.dockpipe/ci-raw/gosec.json and govulncheck.json (objects or empty {}).
set -euo pipefail

eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"
ROOT="$(dockpipe_sdk workdir)"
RAW="$ROOT/bin/.dockpipe/ci-raw"
OUT="$ROOT/bin/.dockpipe/ci-analysis"
SCRIPT_DIR="$(dockpipe_sdk script-dir)"
MERGE_JQ="$SCRIPT_DIR/jq/merge-ci-findings.jq"

if ! command -v jq >/dev/null 2>&1; then
	echo "normalize-ci-scans: jq is required" >&2
	exit 1
fi

rm -rf "$OUT"
mkdir -p "$RAW" "$OUT/raw"
[[ -f "$RAW/gosec.json" ]] || echo '{}' >"$RAW/gosec.json"
[[ -f "$RAW/govulncheck.json" ]] || echo '{}' >"$RAW/govulncheck.json"

cp -f "$RAW/gosec.json" "$OUT/raw/gosec.json"
cp -f "$RAW/govulncheck.json" "$OUT/raw/govulncheck.json"

COMMIT="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
BRANCH="${GITHUB_REF_NAME:-${DOCKPIPE_GIT_BRANCH:-}}"
if [[ -z "$BRANCH" ]]; then
	BRANCH="$(git -C "$ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
fi
RUN_ID="${GITHUB_RUN_ID:-local}"
RUN_ATT="${GITHUB_RUN_ATTEMPT:-1}"
WF_NAME="${GITHUB_WORKFLOW:-$(dockpipe_sdk workflow-name 2>/dev/null || printf 'local')}"
REPO="${GITHUB_REPOSITORY:-unknown}"
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
SOURCE="local"
[[ -n "${GITHUB_ACTIONS:-}" ]] && SOURCE="ci"

GOSEC_VER="$(jq -r '.GosecVersion // "unknown"' "$RAW/gosec.json" 2>/dev/null || echo unknown)"
GOV_VER="$(jq -r '.config.scanner_version // .ScannerVersion // "unknown"' "$RAW/govulncheck.json" 2>/dev/null || echo unknown)"

# Flat list with stable ids for diff across runs
FINDINGS_JSON="$(
	jq -n --slurpfile gosec "$RAW/gosec.json" --slurpfile gv "$RAW/govulncheck.json" -f "$MERGE_JQ" |
		jq -c 'map(. + {id: "\(.tool)|\(.rule_id)|\(.file)|\(.line)"})'
)"

jq -n \
	--arg sv "1.0" \
	--arg commit "$COMMIT" \
	--arg branch "$BRANCH" \
	--arg run_id "$RUN_ID" \
	--arg run_att "$RUN_ATT" \
	--arg wf "$WF_NAME" \
	--arg repo "$REPO" \
	--arg ts "$TS" \
	--arg src "$SOURCE" \
	--arg gosec_v "$GOSEC_VER" \
	--arg gov_v "$GOV_VER" \
	--argjson findings "$FINDINGS_JSON" \
	'{
		schema_version: $sv,
		provenance: {
			commit: $commit,
			branch_or_ref: $branch,
			workflow_run_id: $run_id,
			workflow_run_attempt: $run_att,
			workflow_name: $wf,
			repository: $repo,
			timestamp_utc: $ts,
			source: $src,
			tools: { gosec: $gosec_v, govulncheck: $gov_v }
		},
		findings: $findings,
		raw_paths: { gosec: "raw/gosec.json", govulncheck: "raw/govulncheck.json" }
	}' >"$OUT/findings.json"

COUNT="$(jq '.findings | length' "$OUT/findings.json")"
GOSEC_N="$(jq '(.Issues // []) | length' "$RAW/gosec.json" 2>/dev/null || echo 0)"
GOV_N="$(jq '(.vulns // .Vulns // []) | length' "$RAW/govulncheck.json" 2>/dev/null || echo 0)"

cat >"$OUT/SUMMARY.md" <<EOF
# CI scan summary (DorkPipe signal bundle)

- **Schema:** \`1.0\` — see \`src/schemas/dockpipe-ci-findings.schema.json\`
- **Commit:** \`$COMMIT\` · **ref:** \`$BRANCH\` · **time (UTC):** $TS
- **Run:** \`$RUN_ID\` attempt \`$RUN_ATT\` · **workflow:** \`$WF_NAME\`
- **Normalized findings:** **$COUNT** (gosec issues in raw: ~$GOSEC_N · govulncheck vulns in raw: ~$GOV_N)

**Artifacts:** \`findings.json\` (machine-readable), \`raw/\` (original tool JSON), this file (human).

**DorkPipe:** load \`bin/.dockpipe/ci-analysis/findings.json\` to classify, correlate with repo analysis, prioritize, and suggest fixes. Compare \`findings[].id\` across runs for new/resolved/changed severity.

See **docs/artifacts.md** (CI bundle).
EOF

echo "normalize-ci-scans: wrote $OUT/findings.json ($COUNT findings) and SUMMARY.md"
