# dockpipe

**Run, isolate, and act — pipe commands into disposable containers and act on the results.**

---

## The model

dockpipe is one primitive:

**spawn → run → act**

1. **Spawn** — Start a container from an image.
2. **Run** — Execute your command or script inside it (your directory is mounted at `/work`).
3. **Act** — Optionally run an action script on the result (e.g. commit, export patch, print summary).

- **Spawn** gives you an isolated environment. **Run** is whatever you pass in — a one-liner, a script, or an AI tool. **Act** is pluggable: you choose the script that runs after the command (or none). Commands can themselves be scripts; chaining and composition are the point.

dockpipe is **not** an AI framework, a Claude wrapper, or a workflow engine. It is a **primitive** for running commands in disposable containers, a **pipe-friendly execution boundary**, and something you **compose** into larger workflows. AI is one use case; the core is command-agnostic. Images use a single entrypoint: run your command, then the action (if any). No vendor-specific logic in the core.

---

## One example

Pipe in a task, run it in a container, act on the result:

```bash
echo "refactor the auth module" \
  | dockpipe --template claude --action examples/actions/commit-worktree.sh \
  -- claude --dangerously-skip-permissions -p "$(cat)"
```

Stdin → spawn container → run Claude with that prompt → action commits the changes. Same primitive works for any command (e.g. `npm test`, `make build`, your own script).

---

## What problem it solves

- **Isolation** — Run arbitrary commands in a clean container without polluting your host.
- **Composition** — Same flow for any command; the action is optional and pluggable.
- **Reusability** — Base images and templates give you a consistent environment; actions and scripts are separate so you can mix and match.

---

## Install

**Package** — Download the latest [.deb from Releases](https://github.com/jamie-steele/dockpipe/releases) and run:

```bash
sudo dpkg -i dockpipe_*_all.deb
```

**From source** — Clone and add `bin` to your `PATH`:

```bash
git clone https://github.com/jamie-steele/dockpipe.git
export PATH="$PATH:$(pwd)/dockpipe/bin"
```

**Requirements:** Bash, Docker. See [docs/install.md](docs/install.md) for more.

---

## Usage

```text
dockpipe [options] -- <command> [args...]
<stdin> | dockpipe [options] -- <command> [args...]
```

| Option | Description |
|--------|-------------|
| `--image <name>` | Use this Docker image (default: `dockpipe-base-dev`, built if missing). |
| `--template <name>` | Preset: `base-dev`, `dev`, or `claude`. Builds the image if needed. |
| `--action <script>` | Script run inside the container after the command (e.g. commit, export patch). |
| `--workdir <path>` | Host path to mount at `/work` (default: current directory). |
| `--mount <host:container>` | Extra volume; can be repeated. |
| `--env <KEY=VAL>` | Pass env var into container; can be repeated. |
| `--build <path>` | Build image from path and use as `--image`. |
| `--help` | Show help. |

---

## Examples

**Generic command (default base-dev image):**

```bash
dockpipe -- ls -la
dockpipe -- bash -c "npm test"
dockpipe --image alpine -- sh -c "echo hello"
```

**Using a template:**

```bash
dockpipe --template dev -- make test
dockpipe --template claude -- claude --dangerously-skip-permissions -p "Explain this function"
```

**Piping stdin (e.g. prompt):**

```bash
echo "fix the auth bug" | dockpipe --template claude -- claude --dangerously-skip-permissions -p "$(cat)"
```

**Custom image and workdir:**

```bash
dockpipe --image my-dev --workdir /path/to/repo -- bash -c "npm ci && npm test"
```

**Run a script and then commit (action):**

```bash
dockpipe --action examples/actions/commit-worktree.sh -- ./my-script.sh
```

**Claude + commit in current repo:**

```bash
cd /path/to/your/repo
dockpipe --template claude --action examples/actions/commit-worktree.sh \
  --env "DOCKPIPE_COMMIT_MESSAGE=claude: my task" \
  -- claude --dangerously-skip-permissions -p "Your prompt"
```

**Chained workflow (non-AI):** each step runs in a fresh container; same directory is mounted so the next step sees the previous step’s output.

```bash
# Lint → test → build, each in an isolated environment
dockpipe -- make lint && \
dockpipe -- make test && \
dockpipe -- make build
```

Or pipe output from one step into the next:

```bash
# Generate a config, then validate it in another container
./scripts/generate-config.sh | dockpipe -- ./scripts/validate-config.sh
```

**Chained multi-AI workflow:** run multiple AI steps in sequence, each in its own container. Step 1 writes a plan; step 2 reads it and implements; step 3 reviews. Use a shared workdir so each step sees the repo.

```bash
REPO="$(pwd)"
# Step 1: generate plan (output to repo or stdout)
echo "Add auth to the API" | dockpipe --template claude --workdir "$REPO" \
  -- claude --dangerously-skip-permissions -p "Create a short implementation plan. Write it to plan.md. $(cat)"

# Step 2: implement from plan (same repo)
dockpipe --template claude --workdir "$REPO" --action examples/actions/commit-worktree.sh \
  --env "DOCKPIPE_COMMIT_MESSAGE=impl: auth from plan" \
  -- claude --dangerously-skip-permissions -p "Implement the steps in plan.md"

# Step 3: review (e.g. run tests or another AI pass)
dockpipe --workdir "$REPO" -- bash -c "npm test"
```

**Full examples (runnable from the repo):**

- [Chained non-AI](examples/chained-non-ai/README.md) — Lint → test → build, or generate → validate (Makefile + scripts).
- [Chained multi-AI](examples/chained-multi-ai/README.md) — Plan → implement → review with Claude (shared workdir).
- [Claude worktree](examples/claude-worktree/README.md) — Clone, worktree, Claude, commit.

---

## Templates and images

| Template | Image | Description |
|----------|--------|-------------|
| `base-dev` | `dockpipe-base-dev` | Light: git, curl, bash, ripgrep, jq, ca-certificates. |
| `dev` | `dockpipe-dev` | Heavier: base-dev + build-essential, ssh, less, man, zip, etc. |
| `claude` | `dockpipe-claude` | Node 20 + Claude Code; for AI-assisted workflows. |

Images are built from the repo when you use `--template` (or default) and the image is missing. Build from repo root so `COPY lib/entrypoint.sh` works:

```bash
docker build -t dockpipe-base-dev -f images/base-dev/Dockerfile .
docker build -t dockpipe-dev -f images/dev/Dockerfile .      # after base-dev
docker build -t dockpipe-claude -f images/claude/Dockerfile .
```

---

## Actions

Actions are scripts that run **inside** the container after your command. They receive:

- `DOCKPIPE_EXIT_CODE` — Exit code of the command that just ran.
- `DOCKPIPE_CONTAINER_WORKDIR` — Work dir in the container (default `/work`).

Bundled examples:

- **examples/actions/commit-worktree.sh** — `git add -A && git commit` in the current dir (optional `DOCKPIPE_COMMIT_MESSAGE`).
- **examples/actions/export-patch.sh** — Write uncommitted changes to a patch file (e.g. `DOCKPIPE_PATCH_PATH`).
- **examples/actions/print-summary.sh** — Print exit code and git summary.

Use `--action /path/to/script.sh` or a path relative to the dockpipe repo (e.g. `examples/actions/commit-worktree.sh` when run from the dockpipe CLI).

---

## Repo layout

```text
bin/dockpipe          # CLI entrypoint
lib/
  runner.sh           # Core runner (sourced by CLI)
  entrypoint.sh       # Container entrypoint (copied into images)
images/
  base-dev/           # Light dev Dockerfile
  dev/                # Heavier dev Dockerfile
  claude/             # Claude Code Dockerfile
examples/
  actions/            # Example action scripts
  claude-worktree/    # Full Claude + worktree example
docs/                 # Additional documentation
tests/                # Tests
```

---

## Tests

From the repo root:

```bash
bash tests/run_tests.sh
```

Runs CLI and runner unit tests; optionally runs a smoke test (requires Docker).

---

## Documentation

- [Blog post: Run, Isolate, and Act](https://dev.to/jamie-steele/run-isolate-and-act-a-minimal-primitive-for-container-workflows-553m) — Intro, benefits, and examples on DEV Community.
- [Architecture](docs/architecture.md) — Core primitive, data flow, extension points.
- [Examples](examples/) — Actions and the [Claude worktree example](examples/claude-worktree/README.md).
- [AGENTS.md](AGENTS.md) — For maintainers and AI agents: repo purpose, standards, how to add templates/actions.

---

## License

Apache-2.0. See [LICENSE](LICENSE).
