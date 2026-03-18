# Chained multi-AI workflow

A full example: **plan → implement → review** using multiple dockpipe runs. Each step runs in its own container; a shared workdir (your repo or a project directory) lets the next step see the previous step’s output.

Run from the **dockpipe repo root**. You need the Claude template and API access.

---

## Prerequisites

- Docker
- dockpipe on PATH
- `ANTHROPIC_API_KEY` or Claude login at `~/.claude`
- For Claude template: mount `~/.claude` and `~/.claude.json` (see below)

---

## Workflow

1. **Plan** — One container: Claude writes an implementation plan to `plan.md` in the workdir.
2. **Implement** — Second container: Claude reads `plan.md` and implements; the commit-worktree action commits the result.
3. **Review** — Third container: run tests (or another AI pass) in the same workdir.

Use a **single workdir** (e.g. your repo or a copy) for all three steps so they share `plan.md` and the edited files.

---

## Example: run in a real repo

Replace `/path/to/your/repo` with your project directory (must be absolute). The plan will be written there as `plan.md`; the implement step will change files and commit.

```bash
REPO="/path/to/your/repo"
TASK="Add a simple health check endpoint to the API"

# Step 1: generate plan (writes plan.md in $REPO)
echo "$TASK" | dockpipe --template claude \
  --workdir "$REPO" \
  --mount "$HOME/.claude:/claude-home/.claude" \
  --mount "$HOME/.claude.json:/claude-home/.claude.json" \
  --env "HOME=/claude-home" \
  --env "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}" \
  -- claude --dangerously-skip-permissions -p "Create a short implementation plan (bullet points). Write it to a file named plan.md in the current directory. Task: $(cat)"

# Step 2: implement from plan (commits with action)
dockpipe --template claude \
  --workdir "$REPO" \
  --action examples/actions/commit-worktree.sh \
  --mount "$HOME/.claude:/claude-home/.claude" \
  --mount "$HOME/.claude.json:/claude-home/.claude.json" \
  --env "HOME=/claude-home" \
  --env "DOCKPIPE_COMMIT_MESSAGE=impl: from plan" \
  --env "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}" \
  -- claude --dangerously-skip-permissions -p "Implement the steps described in plan.md. Make the changes in the current directory."

# Step 3: review (e.g. run tests)
dockpipe --workdir "$REPO" -- make test
# Or: run another Claude pass to review the diff, etc.
```

---

## Example: run in the included project dir

This repo includes a minimal project dir you can use without touching a real codebase. It’s just a placeholder so you can see the chain; step 1 will create `plan.md`, step 2 may add a file or two, step 3 runs a no-op “test.”

```bash
# From dockpipe repo root
REPO="$(pwd)/examples/chained-multi-ai/project"
mkdir -p "$REPO"

# Step 1: plan
echo "Add a README with setup instructions" | dockpipe --template claude \
  --workdir "$REPO" \
  --mount "$HOME/.claude:/claude-home/.claude" \
  --mount "$HOME/.claude.json:/claude-home/.claude.json" \
  --env "HOME=/claude-home" \
  --env "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}" \
  -- claude --dangerously-skip-permissions -p "Create a short plan (bullet points) and write it to plan.md. Task: $(cat)"

# Step 2: implement (no git in project/ by default; action may no-op or you can init git)
dockpipe --template claude \
  --workdir "$REPO" \
  --mount "$HOME/.claude:/claude-home/.claude" \
  --mount "$HOME/.claude.json:/claude-home/.claude.json" \
  --env "HOME=/claude-home" \
  --env "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}" \
  -- claude --dangerously-skip-permissions -p "Implement the steps in plan.md. Write files in the current directory."

# Step 3: review
dockpipe --workdir "$REPO" -- cat plan.md
```

---

## Layout

```
chained-multi-ai/
├── README.md    # This file
└── project/     # Optional empty dir for trying the chain (no git required)
```

Use your own repo as `REPO` for a real plan → implement → commit → test flow. The important part is that every step uses the same `--workdir` so they share state.
