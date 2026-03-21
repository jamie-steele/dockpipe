# Shared template assets (`templates/core/`)

Bundled **shared** pieces used by multiple workflows. Workflow folders under **`templates/<name>/`** typically contain only **`config.yml`**, **`README.md`**, and workflow-specific overrides; they **reference** resolvers and strategies here.

| Subfolder | Role |
|-----------|------|
| **`resolvers/`** | Shared **`KEY=value`** resolver files (e.g. `claude`, `codex`) — same contract as per-workflow `resolvers/`. |
| **`strategies/`** | Shared named strategy files (e.g. **`git-worktree`**, **`git-commit`**). |
| **`scripts/`** | Optional notes or snippets; primary repo scripts stay at repo **`scripts/`** (referenced from strategies and workflows). |
| **`images/`** | Optional shared image notes or assets; most Dockerfiles remain under repo **`images/`**. |

**Resolution:** the runner loads **`templates/<workflow>/resolvers/<name>`** first if present, then **`templates/core/resolvers/<name>`**, then the legacy path **`templates/run-worktree/resolvers/<name>`**. Same idea for strategies: per-workflow **`strategies/<name>`** → **`templates/core/strategies/<name>`** → legacy **`templates/strategies/<name>`**.

**`dockpipe init`** copies **`templates/core/`** into your workspace so local workflows resolve the same way.
