# CLI reference

**Run → isolate → act.** Overrides use the same names as workflow YAML. Precedence: **CLI** > config > environment.

**Prerequisites:** **Docker** and **`bash` on the host** (dockpipe always invokes bash). **`git` on the host** for **`--repo`**, worktrees, and commit-on-host. See **[install.md](install.md)**.

**Workflow YAML (`config.yml`):** single-command layout (`run` / `isolate` / `act`) or multi-step **`steps:`** (ordering, **`outputs:`**, **`is_blocking`**, optional **`group.mode: async`**). Full reference: **[workflow-yaml.md](workflow-yaml.md)**.

**Subcommands:** `dockpipe init`, `dockpipe workflow validate [path]`, `dockpipe action init`, `dockpipe pre init`, `dockpipe template init`, `dockpipe doctor` (verify **bash**, **Docker**, bundled assets), `dockpipe windows setup|doctor` — not covered in the flag table below; see **[README.md](../README.md)** and **[install.md](install.md)**.

## `dockpipe init`

Local project setup only: **no `git clone`**, no treating **`init`** as a remote bootstrap. Everything runs in the **current working directory**.

| Command | Purpose |
|---------|---------|
| `dockpipe init` | Create **`scripts/`**, **`images/`**, **`templates/`** if needed; merge bundled **`templates/core/`** (**`runtimes/`**, **`resolvers/`**, **`strategies/`**, **`assets/`** — scripts, images, compose); add **`README.md`** and **`dockpipe.yml`** when missing. |
| `dockpipe init <name>` | Create **`templates/<name>/`** using the bundled **`init`** template by default. |
| `dockpipe init <name> --from <source>` | **`--from`** selects **`blank`**, a **bundled template** under **`templates/`** (e.g. **`run`**, **`run-apply`**, **`run-apply-validate`**), a **filesystem path** to any workflow directory (including **`shipyard/workflows/…`** in a dockpipe **source checkout**), or a path under **`src/templates/<name>/`** or **`templates/<name>/`** resolved from **`DOCKPIPE_REPO_ROOT`**. Not a Git URL. |
| `dockpipe init <name> --resolver <n> --runtime <n> --strategy <n>` | Optional; written into the new **`config.yml`** as **`resolver:`** / **`default_resolver:`**, **`runtime:`** / **`default_runtime:`**, **`strategy:`** (same meaning as **`dockpipe run`**). Example: **`dockpipe init my-pipeline --from run-apply --resolver codex --runtime docker`**. |
| `dockpipe init --gitignore` | **Opt-in** only: append a marked block to **`.gitignore`** at the **git repository root** (`.dockpipe/`, `.dorkpipe/`, Go caches, `tmp/`). Idempotent if the block is already present. Requires a **git working tree** (run from inside a repo). |

On a source checkout, **`--workflow`** resolves **`shipyard/workflows/<name>/config.yml`** before **`src/templates/<name>/config.yml`** (this repo) or **`templates/<name>/config.yml`** (typical project). The dockpipe project keeps its **own** CI and demo workflows under **`shipyard/workflows/`** in the tree (see **[AGENTS.md](../AGENTS.md)**); there is **no** `init` flag for that — copy directories by hand or point **`--from`** at such a path when running **`dockpipe init`** with a workflow name.

**`dockpipe init`** also creates **`dockpipe/README.md`** and an empty **`shipyard/workflows/`** tree when missing.

## Workflow variables (`--workflow`)

When you use `--workflow <name>`, the resolved **`config.yml`** (see **[workflow-yaml.md](workflow-yaml.md)** for lookup order) may include a top-level **`vars:`** block: indented `KEY: value` lines. Those names are **exported** before run scripts, **only if** they are not already set in your shell (so your environment and CI secrets win).

Additional sources (each only sets **unset** variables, except `--var`):

1. `vars:` defaults in `config.yml`
2. `.env` beside the resolved workflow **`config.yml`** (e.g. `templates/<name>/.env` in a checkout, or `shipyard/workflows/<name>/.env` in the materialized bundle)
3. `.env` at the bundled materialized root (default: user cache; layout under **[install.md](install.md#bundled-templates-no-extra-install-tree)**) or at **`DOCKPIPE_REPO_ROOT`** if you set it
4. Each `--env-file <path>` (in order)
5. Path in **`DOCKPIPE_ENV_FILE`** if set
6. **`--var KEY=VAL`** (always exported; overrides yml and `.env`)

Use **`--var`** for one-off overrides; use **`.env`** files for local secrets and team defaults.

## Flags

All options must appear **before** a standalone **`--`**. The command and its arguments follow **`--`**.

| Flag | Aliases | Purpose |
|------|---------|---------|
| `--workflow <name>` | | Load workflow YAML: **`templates/<name>/config.yml`** (project) or **`shipyard/workflows/<name>/config.yml`** (materialized bundle), then resolver delegate **`templates/core/resolvers/<name>/config.yml`** or **`shipyard/core/resolvers/<name>/config.yml`** (see **`ResolveWorkflowConfigPath`** in **`src/lib/dockpipe/infrastructure/workflow_dirs.go`**). With **`steps:`**, a final **`--`** is optional (see **[workflow-yaml.md](workflow-yaml.md)**). Mutually exclusive with **`--workflow-file`**. |
| `--workflow-file <path>` | | Load workflow YAML from an arbitrary path (same shape as bundled **`config.yml`**). Relative **`run:`** / **`act:`** paths resolve next to that file. **Resolver** profiles load only from **`templates/core/resolvers/`** (or **`shipyard/core/resolvers/`** in the materialized bundle) — not from folders beside the YAML file. Mutually exclusive with **`--workflow`**. |
| `--run <path>` | `--pre-script` | **Run:** script(s) on the host before the container. Repeatable. |
| `--isolate <name>` | `--template`, `--image` | **Isolate:** image or template (`base-dev`, `dev`, `agent-dev`, `claude`, `codex`, …). Builds if the value is a template. |
| `--act <path>` | `--action` | **Act:** script after the container command (e.g. commit-worktree). |
| `--runtime <name>` | | **Runtime** profile — **`templates/core/runtimes/<name>`** or **`shipyard/core/runtimes/<name>`** (environment / **`DOCKPIPE_RUNTIME_*`**). May be combined with **`--resolver`**. |
| `--resolver <name>` | | **Resolver** profile — **`templates/core/resolvers/<name>`** or **`shipyard/core/resolvers/<name>`** (or **`…/profile`** under the same dir) (tool adapter / **`DOCKPIPE_RESOLVER_*`**). Often used with **`--repo`** / worktree flows. Both flags may be set; the runner **merges** the two files. |
| `--strategy <name>` | | Named lifecycle wrapper from **`templates/<workflow>/strategies/<name>`** or **`shipyard/workflows/<workflow>/strategies/<name>`** (optional) or **`templates/core/strategies/<name>`** / **`shipyard/core/strategies/<name>`**: runs **before** / **after** host scripts around the workflow body. Overrides **`strategy:`** in workflow YAML when both are set. |
| `--repo <url>` | | Clone URL for worktree flows. If omitted and the workflow uses **`clone-worktree.sh`**, dockpipe sets the URL from **`git remote get-url origin`** in the current dir (or **`--workdir`**). When that URL matches your **`origin`**, the worktree can be created from **your local checkout** (not only a fresh remote default). |
| `--branch <name>` | | Explicit work branch for **`--repo`** (optional). |
| `--work-branch <name>` | | When **`--repo`** is set and **`--branch`** is **omitted**, use this as the **full** branch name. If both are omitted, dockpipe generates **`prefix/<adj>-<noun>-<adj>-<noun>`** (random hyphenated slug; **`prefix`** from resolver / template — see **`domain.RandomWorkBranchSlug`** in **[branchslug.go](../src/lib/dockpipe/domain/branchslug.go)**). If **`--branch`** is set, it takes precedence over **`--work-branch`**. |
| `--workdir <path>` | | Host path mounted at `/work`. Default: current directory. |
| `--work-path <path>` | | Subdirectory inside the repo used as the container working directory (`/work/<path>`). |
| `--bundle-out <path>` | | After commit-on-host, write a git bundle here (default: **current branch only**). Set **`DOCKPIPE_BUNDLE_ALL=1`** for `git bundle create … --all`. |
| `--env KEY=VAL` | | Pass env into the container. Repeatable. When `/work` is a git worktree, dockpipe also sets **`DOCKPIPE_WORKTREE_BRANCH`**, **`DOCKPIPE_WORKTREE_HEAD`**, or **`DOCKPIPE_WORKTREE_DETACHED=1`** from the **host** `git` view. |
| `--mount <host:container>` | | Extra volume mount. Repeatable. |
| `--data-dir <path>` | | Bind-mount host path for persistent tool data (`HOME` in container). |
| `--data-vol <name>` | `--data-volume` | Named Docker volume for persistent data (default: `dockpipe-data` when neither **`--data-dir`** nor **`--no-data`**). |
| `--no-data` | | Do not mount a data volume / data dir. |
| `--reinit` | | With a data volume: remove that volume before run (fresh HOME). Confirms on TTY; use **`-f`** / **`--force`** to skip confirmation. |
| `-f` / `--force` | | With **`--reinit`**, skip confirmation (warning may still print). |
| `--build <path>` | | Parsed (relative to repo root or POSIX absolute). **Not currently consumed** by the Go runner — image build dirs come from **`--isolate`** / template resolution. Kept for CLI compatibility; see **`CliOpts.BuildPath`** in **[flags.go](../src/lib/dockpipe/application/flags.go)**. |
| `--env-file <path>` | | Merge `KEY=VAL` lines into workflow env (unset keys only unless overridden). Repeatable. |
| `--var KEY=VAL` | | Set workflow var; **overrides** yaml / `.env`. Repeatable. |
| `-d` / `--detach` | | Run the container in the background. |
| `-h` / `--help` | | Print usage. |
| `--version`, `-v`, `-V` | | Print version (handled in **`main`** before **`Run`**). |

## Windows subcommands (`dockpipe windows …`)

**Optional.** For **`DOCKPIPE_USE_WSL_BRIDGE=1`**: install **`dockpipe`** in WSL and run **`dockpipe windows setup`** / **`doctor`**. Native **`dockpipe.exe`** users can ignore this. See **Optional: WSL bridge** in **[install.md](install.md)**.

| Command | Purpose |
|---------|---------|
| `dockpipe windows setup` | Interactive: select WSL distro, bootstrap env, optional **`--install-command`** in the distro. |
| `dockpipe windows setup --distro <name> --non-interactive` | Scripted setup (**`--distro`** required). |
| `dockpipe windows setup --bootstrap-wsl` | If WSL is missing or has no distros, run **`wsl --install`** (default distro **Alpine** when unset; may UAC / reboot), then continue. |
| `dockpipe windows setup --install-dockpipe` | Download latest **`dockpipe_*_linux_*.tar.gz`** from GitHub into **`~/.local/bin`** in WSL (for the bridge). |
| `dockpipe windows setup --install-command "<cmd>"` | Run a command inside the selected distro during setup (overrides **`--install-dockpipe`**). |
| `dockpipe windows doctor` | Print **`wsl --version`**, list distros and configured default, or hints if WSL is missing. |

### Windows host: native vs WSL bridge

**Default:** **`dockpipe.exe`** runs on **Windows**. **Docker Desktop** supplies **`docker`**; **`bash`** and **`git`** must be on **`PATH`** separately (e.g. **Git for Windows**) — Docker Desktop does not ship them.

**Container user:** **Linux/macOS** pass **`-u`** with the host **uid:gid** (overrides **`USER`** in the image). If you run **`dockpipe` as root** (e.g. **`sudo`**), that maps to **`-u 0:0`**, which breaks CLIs that refuse flags like **`--dangerously-skip-permissions`** as root. For **claude / codex / agent-dev** images, dockpipe defaults to **`-u node`** when the host uid is **0** (unless **`DOCKPIPE_FORCE_ROOT_CONTAINER=1`**). Override with **`DOCKPIPE_CONTAINER_USER`**.

**Windows:** host uid is unavailable; dockpipe **does not** pass **`-u`** unless **`DOCKPIPE_WINDOWS_CONTAINER_USER`** is set (image **`USER`** applies — e.g. **`node`** in **claude/codex** images). Defaulting **`-u node`** from the CLI caused bind-mount stalls for some Docker Desktop setups, so use an **explicit** value when needed: **`DOCKPIPE_WINDOWS_CONTAINER_USER=node`** for Claude Code with **`--dangerously-skip-permissions`**, or **`0`** for root.

**Claude Code `--dangerously-skip-permissions`:** The CLI blocks that flag when it thinks you’re root/sudo. Two mechanisms in **`assets/entrypoint.sh`:** (1) **`IS_SANDBOX=1`** is set by default (Claude Code’s supported way to treat the run as sandboxed — see **[anthropics/claude-code#9184](https://github.com/anthropics/claude-code/issues/9184)**). Already set **`IS_SANDBOX`** via **`-e`**? Left as-is. Opt out: **`DOCKPIPE_NO_SANDBOX_ENV=1`**. (2) **root → `node`** via **`runuser`**/**`setpriv`** when the container still starts as uid 0. Opt out: **`DOCKPIPE_SKIP_DROP_TO_NODE=1`**. **`DOCKPIPE_DEBUG=1`** prints **`id`**. Rebuild the image after **`entrypoint.sh`** changes (`docker rmi dockpipe-claude:…`). **`DOCKPIPE_WINDOWS_CONTAINER_USER`** remains available if you want an explicit **`-u`** from the host.

**Design: loose defaults in isolation.** Dockpipe targets **disposable containers**; defaults skew **automation-friendly** (e.g. **`IS_SANDBOX=1`**, host **uid:gid** on Unix). People who want stricter behavior can opt out with the env vars above. **Dockpipe does not append** **`--dangerously-skip-permissions`** to your command — add it after **`--`** if you want that mode. **Security** is mostly **what you mount** (only **`/work`** etc.), **secrets** in env/volumes, and **network** — not “CLI tightness” alone.

**Windows + host `git` / Docker mounts:** Bash scripts may export **`DOCKPIPE_WORKDIR`** as **`/c/Users/...`** (Git Bash). Native **`git.exe`** and **`docker -v`** need normal Windows paths; dockpipe converts MSYS-style paths before **`git -C`**, **`docker run` bind mounts**, **`--data-dir`**, and **`--mount`**. If **`/work`** looks empty in the container, rebuild with a current dockpipe and confirm the printed **Mount /work ← …** path exists on the host.

**Code edits vs host user:** On **Linux/macOS**, the container usually runs as **your host uid:gid**, so files written under **`/work`** are owned by **you** on the host (unless you override **`-u`**). **Claude**’s permission model (prompts vs **`--dangerously-skip-permissions`**) is **application-level**; it does not replace **POSIX** ownership — both apply: the process uid writes files, and Claude may still ask or gate tools unless you pass flags it documents.

**WSL bridge (opt-in):** set **`DOCKPIPE_USE_WSL_BRIDGE=1`** so every command **except** `dockpipe windows …` runs inside WSL via **`wsl.exe -d <distro>`**. The current Windows working directory is mapped with **`wslpath`**.

Environment: **`DOCKPIPE_WINDOWS_BRIDGE=1`** is set for the **inner** process when the bridge is used. **Windows-style paths** in path-like flags (`--workdir`, `--work-path`, `--data-dir`, `--mount`, `--build`, `--env-file`, `--run` / `--act`, `--isolate` / `--template` / `--image` when the value is a host path, `--env` / `--var` when the value is a path, etc.) and in **`init` / `action init` / `pre init` / `template init`** destination args are translated to WSL paths: **`wslpath -u`** first, then a **drive-letter** fallback (`C:\…` → `/mnt/c/…`). **UNC** paths (`\\server\share\…`) use a fallback that avoids breaking **`filepath.Clean`**. Everything after a standalone **`--`** is passed through unchanged.

## Examples

**Minimal (workflow does the rest):**

```bash
dockpipe --workflow my-ai --resolver claude --repo https://github.com/you/repo.git -- claude -p "Fix the bug"
```

**Override branch, add env:**

```bash
dockpipe --workflow my-ai --resolver claude --repo https://github.com/you/repo.git --branch my-feature \
  --env "GIT_PAT=$GIT_PAT" -- claude -p "Implement the plan"
```

**No workflow — override run/isolate/act:**

```bash
dockpipe --resolver claude --repo https://github.com/you/repo.git \
  --run ./my-run.sh --act ./my-act.sh -- claude -p "Task"
```
