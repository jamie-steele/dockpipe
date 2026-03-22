# CI scans → DorkPipe signals

Traditional tools (**govulncheck**, **gosec**, and future linters) stay **first-class** in **`.github/workflows/ci.yml`** and **`scripts/ci-local.sh`**. They are **not** replaced. A **post-processing** step turns their **JSON** outputs into a **normalized bundle** that DorkPipe (or any orchestrator) can load for classification, correlation with repo analysis, prioritization, and suggested next steps.

## Artifact layout

| Path | Role |
|------|------|
| **`.dockpipe/ci-raw/gosec.json`** | Original **gosec** JSON (`-fmt json`). |
| **`.dockpipe/ci-raw/govulncheck.json`** | Original **govulncheck** JSON (`-format json`). |
| **`.dockpipe/ci-analysis/findings.json`** | **Canonical** normalized bundle (schema below). |
| **`.dockpipe/ci-analysis/raw/`** | Copies of the two raw files for traceability. |
| **`.dockpipe/ci-analysis/SUMMARY.md`** | Short human summary (counts, commit, run id). |

**GitHub Actions** uploads **`.dockpipe/ci-analysis/`** and **`.dockpipe/ci-raw/`** as workflow artifacts (job `Upload DorkPipe CI analysis bundle`). Locally, these paths are **gitignored**; run **`bash scripts/ci-local.sh`** or the **`govulncheck + gosec`** step by hand to generate them.

## Normalized schema (`findings.json`)

- **`schema_version`**: `"1.0"` — bump when field semantics change.
- **`provenance`**: commit, branch/ref, workflow run id/attempt, repository, UTC timestamp, `source` (`ci` \| `local`), tool versions when known.
- **`findings`**: array of objects with stable **`id`** = `tool|rule_id|file|line` (paths as reported by the tool), plus **`tool`**, **`rule_id`**, **`title`**, **`file`**, **`line`**, **`column`**, **`severity`**, **`confidence`**, **`category`**, **`message`**, **`remediation`**, and **`raw`** (tool-specific object for deep correlation).
- **`raw_paths`**: relative paths under **`ci-analysis/`** to raw JSON copies.

Formal JSON Schema: **`schemas/dockpipe-ci-findings.schema.json`**.

## Handoff contract for DorkPipe

1. **Load** **`.dockpipe/ci-analysis/findings.json`** (or download the CI artifact and point **`DOCKPIPE_WORKDIR`** at the tree).
2. **Join** with repo analysis under **`.dorkpipe/`** / **`.dockpipe/`** using **`provenance.commit`** (and optionally **`.dorkpipe/run.json`**) to align signals with orchestrator state.
3. **Diff** **`findings[].id`** across runs to detect **new**, **resolved**, or **severity-changed** items (stable id is per tool + rule + location).
4. **Prefer** normalized fields for reasoning; use **`raw`** only when you need tool-specific traces (e.g. govulncheck module paths).

## Noise control

- Host steps still **fail the job** on **govulncheck** / **gosec** non-zero exit (real issues).
- **SUMMARY.md** stays short; full detail lives in **JSON** and **raw/**.
- Extend **`merge-ci-findings.jq`** when adding tools; keep one **findings** array.

## Scripts

- **`scripts/dorkpipe/normalize-ci-scans.sh`** — reads **`.dockpipe/ci-raw/*.json`**, writes **`.dockpipe/ci-analysis/`**.
- **`scripts/dorkpipe/jq/merge-ci-findings.jq`** — maps **gosec** `Issues` and **govulncheck** `vulns` into shared finding shape.

**Requires:** **`jq`** (CI runners include it; install locally for **`ci-local.sh`**).
