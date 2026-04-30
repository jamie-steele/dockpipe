# Repo-local workflows

This directory is the **canonical** place for **this repository’s** DockPipe workflows: CI pipelines, Codex demos, R2 publish, self-analysis stacks, sandbox experiments, and host-only helpers.

| Workflow | Role |
|----------|------|
| **`test`** | Multi-step Docker chain: go test → vet → govulncheck → gosec → security brief (mirrors the spirit of `.github/workflows/ci.yml`’s DockPipe workflow step). |
| **`ci-emulate`** | Host-only local mirror of the Linux GitHub CI test job; wraps **`src/scripts/ci-local.sh`** so **`dockpipe --workflow ci-emulate`**, **`make ci`**, and the script stay aligned. |
| **`dockpipe-repo-quality`** | Host-only: lists **`.dockpipe/ci-analysis/`** after you run **`bash src/scripts/ci-local.sh`** (or the govulncheck + gosec + normalize steps from CI). |
| **`codex-pav`** / **`codex-security`** | Optional Codex resolver demos (`OPENAI_API_KEY`; CI gated by `DOCKPIPE_CI_CODEX`). |
| **`review-pipeline`** | Review prep scripts only (`steps: []`); other workflows reference **`scripts/review-pipeline/…`**. Used by demo / **`test-demo`**-style flows (`make demo-record`). |
| **`package-store-infra`** | Thin composer: shared **`vars`** + nested packaged workflow **`dockpipe.cloudflare.r2infra`** (`workflow:` + `package:`); optional **`--tf`**. Store tarballs: run **`dockpipe package build store`** separately when needed. |

**Suggested local “full stack”:** run **`make ci`** or **`./src/bin/dockpipe --workflow ci-emulate --workdir . --`** for the best local mirror of the Linux CI job. Optionally follow with **`./src/bin/dockpipe --workflow dockpipe-repo-quality --workdir . --`** to inspect normalized findings. For Cloudflare R2 infra with repo **`vars`**: **`./src/bin/dockpipe --workflow package-store-infra --workdir . --`**. For store tarballs: **`dockpipe package build store`** after compile.

**`.staging/workflows/`** mirrors this tree for packaging experiments (edit in-tree; no separate sync step).
