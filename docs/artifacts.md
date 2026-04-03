# Maintainer artifacts (`.dockpipe/` / `.dorkpipe/`)

Generated files used as **context** for humans and AI — not a second orchestrator. Refresh via workflows / **`make ci`** / **`make self-analysis*`** when you need current data.

## Governance and security questions

When asked *“compliance issues?”* or *“security posture?”*:

1. Read **`AGENTS.md`**.
2. If present, load **`.dockpipe/ci-analysis/findings.json`**, **`SUMMARY.md`**, **`.dorkpipe/self-analysis/`**, **`.dorkpipe/run.json`**. Do **not** invent scan output.
3. If artifacts are missing or stale vs `HEAD`, say so — do **not** claim “clean” without evidence.
4. This is **not** a certified compliance verdict (SOC2, ISO, etc.).

**Local summary:** `make compliance-handoff` or `./src/bin/dockpipe --workflow compliance-handoff --workdir . --`

## CI scan bundle

**govulncheck** / **gosec** feed **`.dockpipe/ci-raw/`** → normalized **`.dockpipe/ci-analysis/findings.json`** (schema: **`src/schemas/dockpipe-ci-findings.schema.json`**). **`normalize-ci-scans.sh`** + **`jq/merge-ci-findings.jq`** build the bundle; **`bash scripts/ci-local.sh`** reproduces locally.

| Path | Role |
|------|------|
| **`.dockpipe/ci-analysis/findings.json`** | Canonical merged findings |
| **`.dockpipe/ci-analysis/SUMMARY.md`** | Short counts / provenance |
| **`.dockpipe/ci-raw/*.json`** | Original tool JSON |

## User insight queue

Structured human guidance → **`.dockpipe/analysis/`** (`queue.json`, `insights.json`, …). Schemas under **`src/schemas/dockpipe-user-insight-*.schema.json`**.

**Workflow:** **`user-insight-process`** in the maintainer **`dorkpipe`** package — see **`resolvers/user-insight-process/README.md`** there.
