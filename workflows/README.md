# Repo-local workflows

This directory is the **canonical** place for **this repository’s** DockPipe workflows: CI pipelines, Codex demos, R2 publish, self-analysis stacks, sandbox experiments, and host-only helpers.

| Workflow | Role |
|----------|------|
| **`test`** | Multi-step Docker chain: go test → vet → govulncheck → gosec → security brief (mirrors the spirit of `.github/workflows/ci.yml`’s DockPipe workflow step). |
| **`dockpipe-repo-quality`** | Host-only: lists **`.dockpipe/ci-analysis/`** after you run **`bash src/scripts/ci-local.sh`** (or the govulncheck + gosec + normalize steps from CI). |
| **`codex-pav`** / **`codex-security`** | Optional Codex resolver demos (`OPENAI_API_KEY`; CI gated by `DOCKPIPE_CI_CODEX`). |
| **`review-pipeline`** | Review prep scripts only (`steps: []`); other workflows reference **`scripts/review-pipeline/…`**. Used by demo / **`test-demo`**-style flows (`make demo-record`). |
| **`package-store-infra`** | Host-only: **`dockpipe build`** then **`dockpipe package build store`** → **`release/artifacts/`** + **`packages-store-manifest.json`** preview (package-manager layout before R2/upload). |

**Suggested local “full stack”:** run **`make`** / **`go test`**, then **`bash src/scripts/ci-local.sh`** (host scans + **`dockpipe --workflow test`**), optionally **`./src/bin/dockpipe --workflow dockpipe-repo-quality --workdir . --`**. To preview the packaged store layout: **`./src/bin/dockpipe --workflow package-store-infra --workdir . --`**.

**`.staging/workflows/`** mirrors this tree for packaging experiments (edit in-tree; no separate sync step).
