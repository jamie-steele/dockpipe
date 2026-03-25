# Repo-local workflows

This directory is the **canonical** place for **this repository’s** DockPipe workflows: CI pipelines, Codex demos, R2 publish, self-analysis stacks, sandbox experiments, and host-only helpers.

| Workflow | Role |
|----------|------|
| **`test`** | Multi-step Docker chain: go test → vet → govulncheck → gosec → security brief (mirrors the spirit of `.github/workflows/ci.yml`’s DockPipe workflow step). |
| **`dockpipe-repo-quality`** | Host-only: lists **`.dockpipe/ci-analysis/`** after you run **`bash src/scripts/ci-local.sh`** (or the govulncheck + gosec + normalize steps from CI). |
| **`codex-pav`** / **`codex-security`** | Optional Codex resolver demos (`OPENAI_API_KEY`; CI gated by `DOCKPIPE_CI_CODEX`). |

**Suggested local “full stack”:** run **`make`** / **`go test`**, then **`bash src/scripts/ci-local.sh`** (host scans + **`dockpipe --workflow test`**), optionally **`./src/bin/dockpipe --workflow dockpipe-repo-quality --workdir . --`**.

**`.staging/workflows/`** mirrors this tree for packaging experiments (`scripts/dockpipe/sync-packaging-staging.sh`).
