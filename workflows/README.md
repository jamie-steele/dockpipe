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
| **`agent/docs.orchestrate`** / **`agent/docs.optimize-orchestrate`** | DorkPipe docs-orchestration dogfood for this checkout; package docs now also ship a generic consumer-repo brain baseline for repo-native durable guidance. |
| **`ci/test`** | Multi-step Docker chain: go test → vet → govulncheck → gosec → security brief (mirrors the spirit of `.github/workflows/ci.yml`’s DockPipe workflow step). |
| **`ci/ci-emulate`** | Local GitHub Actions emulator. Runs the real **`.github/workflows/ci.yml`** **`test`** job through **`act`** for runner/container parity before pushing. |
| **`ci/dockpipe-repo-quality`** | Host-only: lists the CI analysis artifact directory from `dockpipe scope workflow ci ci-analysis` after **`ci-emulate`** or GitHub CI writes the normalized scan bundle. |
| **`package/package-store-infra`** | Thin composer: shared **`vars`** + nested packaged workflow **`dockpipe.cloudflare.r2infra`** (`workflow:` + `package:`); optional **`--tf`**. Store tarballs: run **`dockpipe package build store`** separately when needed. |

**Suggested local “full stack”:** run **`make ci`** or **`./src/bin/dockpipe --workflow ci-emulate --workdir . --`** to execute the real GitHub Actions test job locally through **`act`**. Optionally follow with **`./src/bin/dockpipe --workflow dockpipe-repo-quality --workdir . --`** to inspect normalized findings. For Cloudflare R2 infra with repo **`vars`**: **`./src/bin/dockpipe --workflow package-store-infra --workdir . --`**. For store tarballs: **`dockpipe package build store`** after compile.
