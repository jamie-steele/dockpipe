# Maintainer Artifacts

Generated files used as **context** for humans and AI — not a second orchestrator. Refresh via workflows / **`make ci`** / **`make self-analysis*`** when you need current data.

## Governance and security questions

When asked *“compliance issues?”* or *“security posture?”*:

1. Read **`AGENTS.md`**.
2. If present, load CI findings from `dockpipe scope workflow ci ci-analysis/findings.json` and DorkPipe run metadata from `dockpipe scope --package dorkpipe run.json`. Do **not** invent scan output.
3. If artifacts are missing or stale vs `HEAD`, say so — do **not** claim “clean” without evidence.
4. This is **not** a certified compliance verdict (SOC2, ISO, etc.).

**Local summary:** `make compliance-handoff` or `./src/bin/dockpipe --workflow compliance-handoff --workdir . --`

## CI scan bundle

**govulncheck** / **gosec** feed workflow-scoped `ci-raw/` artifacts → normalized `ci-analysis/findings.json` (schema: **`src/schemas/dockpipe-ci-findings.schema.json`**). **`dorkpipe ci normalize-scans`** is the canonical implementation; **`packages/dorkpipe/resolvers/dorkpipe/assets/scripts/normalize-ci-scans.sh`** is the compatibility wrapper. **`bash src/scripts/ci-local.sh`** reproduces locally.

| Path | Role |
|------|------|
| `dockpipe scope workflow ci ci-analysis/findings.json` | Canonical merged findings |
| `dockpipe scope workflow ci ci-analysis/SUMMARY.md` | Short counts / provenance |
| `dockpipe scope workflow ci ci-raw` | Original tool JSON directory |

## User insight queue

Structured human guidance → **`dockpipe scope --package dorkpipe analysis`** (`queue.json`, `insights.json`, …). Schemas under **`src/schemas/dockpipe-user-insight-*.schema.json`**.

**Canonical implementation:** **`dorkpipe insight ...`** in the maintainer **`dorkpipe`** package; the workflow **`user-insight-process`** is a host-side entrypoint that uses the same CLI surface. See **`resolvers/user-insight-process/README.md`** there.
