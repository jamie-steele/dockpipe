# Tests

- **unit-tests/** — CLI, runner, repo-root resolution, smoke, .deb install. No Docker required for most; smoke and test_deb_install need Docker. Run: `bash tests/run_tests.sh` (from repo root).
- **integration-tests/** — Full flow with Docker: templates, actions, mounts, env, detach, agent-dev image. Run: `bash tests/integration-tests/run.sh` (from repo root). See [integration-tests/README.md](integration-tests/README.md) for details.
