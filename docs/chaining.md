# Chaining

Run multiple dockpipe steps in sequence. Each step runs in a fresh container; use the same `--workdir` so the next step sees the previous step's output. Same primitive whether the commands are `make lint`, `npm test`, or an AI tool.

---

## Lint → test → build

Use a directory with a Makefile (or similar) that has `lint`, `test`, `build` targets. Run each in its own container, shared workdir:

```bash
WORKDIR="/path/to/your/project"

dockpipe --workdir "$WORKDIR" -- make lint && \
dockpipe --workdir "$WORKDIR" -- make test && \
dockpipe --workdir "$WORKDIR" -- make build
```

**Pipe output between containers:**

```bash
dockpipe --workdir "$WORKDIR" -- ./scripts/generate-config.sh \
  | dockpipe --workdir "$WORKDIR" -- ./scripts/validate-config.sh
```

---

## Plan → implement → review (with AI)

Same pattern: one container writes a plan to `plan.md`, the next implements from it (optionally with commit-worktree), the next runs tests or another review step.

```bash
REPO="/path/to/your/repo"
TASK="Add a simple health check endpoint"

# Step 1: plan (writes plan.md)
echo "$TASK" | dockpipe --isolate claude --workdir "$REPO" -- claude -p "Create a short plan; write it to plan.md. Task: $(cat)"

# Step 2: implement (optionally with --act commit-worktree)
dockpipe --isolate claude --workdir "$REPO" --act scripts/commit-worktree.sh \
  -- claude -p "Implement the steps in plan.md"

# Step 3: review (e.g. run tests)
dockpipe --workdir "$REPO" -- make test
```

Use your own repo path and commands. Compose in your shell or Makefile.

**Future:** First-class **multi-step pipelines** where each step declares **outputs** that become **variables for the next step** (see **`docs/future-updates.md`** — *Multi-step pipelines* and *Terraform*).
