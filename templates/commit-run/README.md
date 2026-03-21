# commit-run

**Audience:** default-friendly — **current branch only**, no worktrees, no new branches.

| | **commit-run** (this template) | **llm-worktree** (advanced) |
|--|-------------------------------|----------------------------|
| Branch | Commits on **whatever branch you’re on** | Isolated **work branch** + worktree (clone/worktree automation) |
| Git concepts | “Run tool → optional commit” | `--repo`, resolvers, branch prefixes |
| Use when | You’re already in a clone and want one commit after a containerized command | AI sessions, parallel work, or remote-first flows |

## Behavior

1. **Isolate:** runs your command after `--` inside the **base-dev** image (override with `--isolate` if needed).
2. **Project:** host directory is mounted at **`/work`** (default: current working directory; set `--workdir` for another path).
3. **Act:** after the container exits **successfully** (exit code 0), dockpipe runs **one** `git add -A` + `git commit` on the **host** repo at that directory — **same branch**, no branch creation.
4. **One commit per invocation** when there are staged/uncommitted changes after the run.
5. **No worktrees** and **no** `clone-worktree.sh` — nothing in this workflow creates branches or extra checkouts.

## CLI examples

```bash
# From your repo root; run a command, then commit if files changed
dockpipe --workflow commit-run -- sh -c 'echo hello >> notes.txt'

# Custom commit message
dockpipe --workflow commit-run --var DOCKPIPE_COMMIT_MESSAGE="docs: tweak notes" -- npm run format

# Different project directory
dockpipe --workflow commit-run --workdir /path/to/repo -- ./scripts/task.sh
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

## Compared to `llm-worktree`

- **commit-run** does **not** use `resolvers/`, `--repo`, or worktree scripts.
- **llm-worktree** remains the full **advanced** path for branch isolation and AI backends — see [templates/llm-worktree/README.md](../llm-worktree/README.md).
