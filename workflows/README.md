# Repo-local workflows

This directory is the **canonical** place for **this repository’s** DockPipe workflows: CI pipelines, Codex demos, R2 publish, self-analysis stacks, sandbox experiments, and host-only helpers.

Workflows are grouped by purpose while retaining their leaf workflow names for `--workflow`:

- `agent/` — Codex demos and DorkPipe agentic docs-orchestration dogfood.
- `ci/` — local CI mirrors and quality summaries.
- `package/` — package/store/infrastructure composers.

| Workflow | Role |
|----------|------|
| **`agent/codex-pav`** | Optional Codex plan/apply/validate resolver demo (`OPENAI_API_KEY`; CI gated by `DOCKPIPE_CI_CODEX`). |
| **`agent/codex-security`** | Optional Codex security-review resolver demo (`OPENAI_API_KEY`; CI gated by `DOCKPIPE_CI_CODEX`). |
| **`agent/docs.orchestrate`** / **`agent/docs.optimize-orchestrate`** | DorkPipe docs-orchestration dogfood for this checkout; keeps repo-specific AI task graphs out of published package examples. |
| **`ci/test`** | Multi-step Docker chain: go test → vet → govulncheck → gosec → security brief (mirrors the spirit of `.github/workflows/ci.yml`’s DockPipe workflow step). |
| **`ci/ci-emulate`** | Host-only local mirror of the Linux GitHub CI test job; wraps **`src/scripts/ci-local.sh`** so **`dockpipe --workflow ci-emulate`**, **`make ci`**, and the script stay aligned. |
| **`ci/dockpipe-repo-quality`** | Host-only: lists **`bin/.dockpipe/workflows/ci/dorkpipe/ci-analysis/`** after you run **`bash src/scripts/ci-local.sh`** (or the govulncheck + gosec + normalize steps from CI). |
| **`package/package-store-infra`** | Thin composer: shared **`vars`** + nested packaged workflow **`dockpipe.cloudflare.r2infra`** (`workflow:` + `package:`); optional **`--tf`**. Store tarballs: run **`dockpipe package build store`** separately when needed. |

**Suggested local “full stack”:** run **`make ci`** or **`./src/bin/dockpipe --workflow ci-emulate --workdir . --`** for the best local mirror of the Linux CI job. Optionally follow with **`./src/bin/dockpipe --workflow dockpipe-repo-quality --workdir . --`** to inspect normalized findings. For Cloudflare R2 infra with repo **`vars`**: **`./src/bin/dockpipe --workflow package-store-infra --workdir . --`**. For store tarballs: **`dockpipe package build store`** after compile.

**`.staging/workflows/`** mirrors this tree for packaging experiments (edit in-tree; no separate sync step).
