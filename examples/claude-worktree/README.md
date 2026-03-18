# Claude worktree example

This example reproduces the original dockpipe flow: clone a repo, create or reuse a worktree, run Claude Code, and commit the result. Use it as a template for AI-assisted workflows.

**Data volume:** By default dockpipe mounts a named volume `dockpipe-data` at `/dockpipe-data` and sets `HOME` there; repos and worktrees live under `$DOCKPIPE_DATA/repos`. Use `--data-dir $HOME/.dockpipe` to bind mount a host path (e.g. for the cherry-pick step). Use `--data-vol <name>` or `--no-data` to change or disable.

## Prerequisites

- Docker
- [dockpipe](../../README.md) (add `bin/` to PATH or use full path)
- `ANTHROPIC_API_KEY` or Claude login (first run in the container; saved in the data volume)
- For private repos: `GIT_PAT` (HTTPS personal access token)

## Quick run (full flow)

1. From the **dockpipe repo root**, run with your repo, branch, and prompt (no extra mounts; data dir is automatic):

   ```bash
   REPO_URL="https://github.com/you/your-repo.git"
   BRANCH="claude/task-name"
   PROMPT="Fix the login bug in auth.js"
   REPO_NAME="$(basename "$REPO_URL" .git)"

   dockpipe --template claude \
     --env "REPO_URL=$REPO_URL" \
     --env "REPO_NAME=$REPO_NAME" \
     --env "BRANCH=$BRANCH" \
     --env "PROMPT=$PROMPT" \
     --env "GIT_PAT=${GIT_PAT:-}" \
     --env "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}" \
     -- ./examples/claude-worktree/setup-and-claude.sh
   ```

2. Apply the commit from the worktree (if you used `--data-dir $HOME/.dockpipe`, repos are on the host):

   ```bash
   WORKTREE="$HOME/.dockpipe/repos/$REPO_NAME/worktrees/${BRANCH//\//-}"
   git cherry-pick $(git -C "$WORKTREE" log -1 --format=%H)
   ```
   With the default named volume, the worktree lives inside the volume; use a container with the volume mounted to push the branch, then pull and merge on the host.

## Piping a prompt

```bash
REPO_NAME="$(basename "$REPO_URL" .git)"
echo "Add unit tests for UserService" | dockpipe --template claude \
  --env "REPO_URL=$REPO_URL" \
  --env "REPO_NAME=$REPO_NAME" \
  --env "BRANCH=$BRANCH" \
  --env "GIT_PAT=$GIT_PAT" \
  --env "PROMPT=$(cat)" \
  -- ./examples/claude-worktree/setup-and-claude.sh
```

## Simpler flow (current directory as repo)

If you already have a repo checked out and want to run Claude in it and commit with dockpipe’s action, run from your **repo root** (that directory is mounted as `/work`):

```bash
cd /path/to/your/repo
dockpipe --template claude \
  --action examples/actions/commit-worktree.sh \
  --env "DOCKPIPE_COMMIT_MESSAGE=claude: my task" \
  -- claude --dangerously-skip-permissions -p "Your prompt"
```

The commit-worktree action will commit all changes in `/work` (your repo) after Claude exits. To customize it, copy first: `dockpipe action init my-commit.sh --from commit-worktree`, then use `--action my-commit.sh`.

## Files

- **setup-and-claude.sh** — Clone, worktree, Claude, commit. Uses `DOCKPIPE_DATA` (default `/dockpipe-data`) for repos; pass env vars above.
- **README.md** — This file.
