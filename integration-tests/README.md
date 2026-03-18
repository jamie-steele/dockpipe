# Integration tests

Tests that run dockpipe with Docker: agent-dev template, data volume, and the commit-worktree action against a real git repo. Require Docker to be installed and runnable.

**Run from repo root:**

```bash
bash integration-tests/run.sh
```

Or run a single test:

```bash
bash integration-tests/test_agent_dev_smoke.sh
bash integration-tests/test_commit_action_git_repo.sh
```

| Test | What it does |
|------|----------------|
| `test_agent_dev_smoke.sh` | Agent-dev template runs; default data volume is mounted (`DOCKPIPE_DATA`, `/dockpipe-data`); `--no-data` disables the volume. |
| `test_commit_action_git_repo.sh` | Creates a temp git repo, runs dockpipe with commit-worktree action and a command that adds a file; asserts the action commits it with the given message. |

These are separate from `tests/` (unit and smoke tests). The main suite is `bash tests/run_tests.sh`; use `bash integration-tests/run.sh` when you want to validate the full flow with Docker and the agent-dev image.
