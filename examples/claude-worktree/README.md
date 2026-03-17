# Claude worktree example

This example reproduces the original dockpipe flow: clone a repo, create or reuse a worktree, run Claude Code, and commit the result. Use it as a template for AI-assisted workflows.

## Prerequisites

- Docker
- [dockpipe](../../README.md) (add `bin/` to PATH or use full path)
- `ANTHROPIC_API_KEY` or a Claude login at `~/.claude`
- For private repos: `GIT_PAT` (HTTPS personal access token)

## Quick run (full flow)

1. Set a repos directory (clone and worktrees live here):

   ```bash
   export REPOS_DIR="${REPOS_DIR:-$HOME/.dockpipe/repos}"
   mkdir -p "$REPOS_DIR"
   ```

2. From the **dockpipe repo root**, run with your repo, branch, and prompt:

   ```bash
   REPO_URL="https://github.com/you/your-repo.git"
   BRANCH="claude/task-name"
   PROMPT="Fix the login bug in auth.js"
   REPO_NAME="$(basename "$REPO_URL" .git)"

   dockpipe --template claude \
     --mount "$REPOS_DIR:/repos" \
     --mount "$HOME/.claude:/claude-home/.claude" \
     --mount "$HOME/.claude.json:/claude-home/.claude.json" \
     --env "HOME=/claude-home" \
     --env "REPO_URL=$REPO_URL" \
     --env "REPO_NAME=$REPO_NAME" \
     --env "BRANCH=$BRANCH" \
     --env "PROMPT=$PROMPT" \
     --env "GIT_PAT=${GIT_PAT:-}" \
     --env "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}" \
     -- ./examples/claude-worktree/setup-and-claude.sh
   ```

3. Apply the commit from the worktree:

   ```bash
   WORKTREE="$REPOS_DIR/$REPO_NAME/worktrees/${BRANCH//\//-}"
   git cherry-pick $(git -C "$WORKTREE" log -1 --format=%H)
   ```

## Piping a prompt

```bash
echo "Add unit tests for UserService" | dockpipe --template claude \
  --mount "$REPOS_DIR:/repos" \
  --mount "$HOME/.claude:/claude-home/.claude" \
  --env "REPO_URL=$REPO_URL" \
  --env "REPO_NAME=$REPO_NAME" \
  --env "BRANCH=$BRANCH" \
  --env "GIT_PAT=$GIT_PAT" \
  --env "PROMPT=$(cat)" \
  -- ... setup-and-claude.sh
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

The commit-worktree action will commit all changes in `/work` (your repo) after Claude exits.

## Files

- **setup-and-claude.sh** — Clone, worktree, Claude, commit. Use with `--mount .../repos:/repos` and the env vars above.
- **README.md** — This file.
