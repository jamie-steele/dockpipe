# Tests

- **unit-tests/** — CLI, runner, repo-root resolution, layout guard, clone-worktree include. Run: `bash tests/run_tests.sh` (from repo root).
- **Maintainer packages** — Shell and Go tests: **`packages/pipeon/tests/`**, **`packages/dorkpipe/tests/`**, **`packages/dockpipe-mcp/tests/`**. `run_tests.sh` calls each package’s **`tests/run.sh`**.
- **integration-tests/** — Full flow with Docker: templates, actions, mounts, env, detach, agent-dev image. Run: `bash tests/integration-tests/run.sh` (from repo root). See [integration-tests/README.md](integration-tests/README.md) for details.

Smoke and **test_deb_install** (Docker + `.deb`) run from **`run_tests.sh`** when available.
