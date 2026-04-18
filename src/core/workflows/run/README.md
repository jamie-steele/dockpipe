# run

**Audience:** default-friendly — **current branch only**, no worktrees, no new branches.

| | **run** (this template) | **`worktree`** **strategy** + your own **`templates/<name>/config.yml`** |
|--|------------------------|----------------------------|
| Branch | Commits on **whatever branch you’re on** | Isolated **work branch** + worktree (clone/worktree automation) |
| Git concepts | “Run tool → optional commit” | **`strategy: worktree`**, `--repo`, runtime profiles, branch prefixes |
| Use when | You’re already in a clone and want one commit after a containerized command | AI sessions, parallel work, or remote-first flows |

## Behavior

1. **Isolate:** runs your command after `--` inside the **base-dev** image (override with `--isolate` if needed).
2. **Project:** host directory is mounted at **`/work`** (default: current working directory; set `--workdir` for another path).
3. **Strategy `commit`:** after the container exits **successfully** (exit code 0), the **`templates/core/strategies/commit`** **after** hook runs **`scripts/commit-worktree.sh`** on the host — **one** `git add -A` + `git commit` at that directory — **same branch**, no branch creation (equivalent to the older top-level **`act:`** in **`config.yml`**).
4. **One commit per invocation** when there are staged/uncommitted changes after the run.
5. **No worktrees** and **no** `clone-worktree.sh` — nothing in this workflow creates branches or extra checkouts.

## CLI examples

```bash
# From your repo root; run a command, then commit if files changed
dockpipe --workflow run -- sh -c 'echo hello >> notes.txt'

# Custom commit message
dockpipe --workflow run --var DOCKPIPE_COMMIT_MESSAGE="docs: tweak notes" -- npm run format

# Different project directory
dockpipe --workflow run --workdir /path/to/repo -- ./scripts/task.sh
```

## Logs / messages

- **`[dockpipe] Committing on branch: <name>`** — before commit (from `CommitOnHost`).
- **`[dockpipe] No changes to commit.`** — working tree clean after container; exit **0**.
- **`[dockpipe] Not a git repo; skipping commit.`** — not a git checkout; exit **0** for commit step.
- **`[dockpipe] Skipping host commit (container exited with non-zero code).`** — isolate step failed; **no** commit (avoid surprise partial commits).

## Edge cases

| Situation | Behavior |
|-----------|----------|
| Container exit ≠ 0 | No commit; dockpipe exits with that code. |
| No file changes after success | No commit; stderr says no changes; exit **0**. |
| Detached HEAD | Commit still runs if there are changes (same as `git commit` rules). |
| `DOCKPIPE_COMMIT_MESSAGE` empty | Falls back to a generic automated message (see `CommitOnHost` in code). |

## Resolver images (**codex**, **claude**, …)

If your **`--isolate`** / workflow uses **codex** or **claude** images, read **`templates/core/resolvers/codex/README.md`** and **`templates/core/resolvers/claude/README.md`** for API keys forwarded into Docker and Codex nested-sandbox notes.

## Compared to **`strategy: worktree`**

- **run** does **not** use runtime profiles for clone, `--repo`, or **`strategy: worktree`**.
- Isolated branches + worktrees use the **`worktree`** **strategy** in **your** workflow YAML — see **[docs/workflow-yaml.md](../../../docs/workflow-yaml.md#named-strategies)** (`dockpipe template init my-ai` then edit **`config.yml`**).
