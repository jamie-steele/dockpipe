# Codex worktree example

This example mirrors the Claude worktree flow for OpenAI Codex: clone a repo, create or reuse a worktree, run Codex, and commit the result in that branch (never on main).

**Data volume:** By default dockpipe mounts a named volume `dockpipe-data` at `/dockpipe-data` and sets `HOME` there; repos and worktrees live under `$DOCKPIPE_DATA/repos`. Use `--data-dir $HOME/.dockpipe` to bind mount a host path. Use `--data-vol <name>` or `--no-data` to change or disable.

## Prerequisites

- Docker
- [dockpipe](../../README.md) (add `bin/` to PATH or use full path)
- `OPENAI_API_KEY` or Codex login (first run in the container; saved in the data volume)
- For private repos: `GIT_PAT` (HTTPS personal access token)

## Quick run (full flow)

1. From the **dockpipe repo root**, run with your repo, branch, and optional prompt:

   ```bash
   REPO_URL="https://github.com/you/your-repo.git"
   BRANCH="codex/task-name"
   PROMPT="Fix the login bug in auth.js"
   REPO_NAME="$(basename "$REPO_URL" .git)"

   dockpipe --template codex \
     --env "REPO_URL=$REPO_URL" \
     --env "REPO_NAME=$REPO_NAME" \
     --env "BRANCH=$BRANCH" \
     --env "PROMPT=$PROMPT" \
     --env "GIT_PAT=${GIT_PAT:-}" \
     --env "OPENAI_API_KEY=${OPENAI_API_KEY:-}" \
     -- ./examples/codex-worktree/setup-and-codex.sh
   ```

2. Apply the commit from the worktree (if you used `--data-dir $HOME/.dockpipe`):

   ```bash
   WORKTREE="$HOME/.dockpipe/repos/$REPO_NAME/worktrees/${BRANCH//\//-}"
   git cherry-pick $(git -C "$WORKTREE" log -1 --format=%H)
   ```

## Piping a prompt

```bash
REPO_NAME="$(basename "$REPO_URL" .git)"
echo "Add unit tests for UserService" | dockpipe --template codex \
  --env "REPO_URL=$REPO_URL" \
  --env "REPO_NAME=$REPO_NAME" \
  --env "BRANCH=$BRANCH" \
  --env "GIT_PAT=$GIT_PAT" \
  --env "PROMPT=$(cat)" \
  -- ./examples/codex-worktree/setup-and-codex.sh
```

## Simpler flow (current directory as repo)

If you already have a repo checked out and want to run Codex in it and commit with dockpipe’s action (which creates a branch if you’re on main):

```bash
cd /path/to/your/repo
dockpipe --template codex \
  --action examples/actions/commit-worktree.sh \
  --env "DOCKPIPE_COMMIT_MESSAGE=codex: my task" \
  -- codex exec "Your prompt"
```

The commit-worktree action will commit all changes; if you’re on main or master, it creates a new branch (e.g. `dockpipe/agent-<timestamp>`) and commits there.

## Files

- **setup-and-codex.sh** — Clone, worktree, Codex, commit. Uses `DOCKPIPE_DATA` (default `/dockpipe-data`) for repos; pass env vars above.
- **README.md** — This file.
