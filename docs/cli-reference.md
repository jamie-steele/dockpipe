# CLI reference

**Run → isolate → act.** Overrides use the same names as workflow YAML. Precedence: **CLI** > config > environment.

**Prerequisites:** **Docker** and **`bash` on the host** (dockpipe always invokes bash). **`git` on the host** for **`--repo`**, worktrees, and commit-on-host. See **[install.md](install.md)**.

**Workflow YAML (`config.yml`):** single-command layout (`run` / `isolate` / `act`) or multi-step **`steps:`** (ordering, **`outputs:`**, **`is_blocking`**, optional **`group.mode: async`**). Full reference: **[workflow-yaml.md](workflow-yaml.md)**.

**Subcommands:** `dockpipe init`, `dockpipe action init`, `dockpipe pre init`, `dockpipe template init`, `dockpipe windows setup|doctor` — not covered in the flag table below; see **[README.md](../README.md)** and **[install.md](install.md)**.

## Workflow variables (`--workflow`)

When you use `--workflow <name>`, `templates/<name>/config.yml` may include a top-level **`vars:`** block: indented `KEY: value` lines. Those names are **exported** before run scripts, **only if** they are not already set in your shell (so your environment and CI secrets win).

Additional sources (each only sets **unset** variables, except `--var`):

1. `vars:` defaults in `config.yml`
2. `templates/<name>/.env`
3. `.env` at `DOCKPIPE_REPO_ROOT` (your workspace / install tree)
4. Each `--env-file <path>` (in order)
5. Path in **`DOCKPIPE_ENV_FILE`** if set
6. **`--var KEY=VAL`** (always exported; overrides yml and `.env`)

Use **`--var`** for one-off overrides; use **`.env`** files for local secrets and team defaults.

## Flags

All options must appear **before** a standalone **`--`**. The command and its arguments follow **`--`**.

| Flag | Aliases | Purpose |
|------|---------|---------|
| `--workflow <name>` | | Load `templates/<name>/config.yml`. With **`steps:`**, a final **`--`** is optional (see **[workflow-yaml.md](workflow-yaml.md)**). |
| `--run <path>` | `--pre-script` | **Run:** script(s) on the host before the container. Repeatable. |
| `--isolate <name>` | `--template`, `--image` | **Isolate:** image or template (`base-dev`, `dev`, `agent-dev`, `claude`, `codex`, …). Builds if the value is a template. |
| `--act <path>` | `--action` | **Act:** script after the container command (e.g. commit-worktree). |
| `--resolver <name>` | | Resolver (`claude`, `codex`, …); often used with **`--repo`** / worktree flows. |
| `--repo <url>` | | Clone URL for worktree flows. If omitted and the workflow uses **`clone-worktree.sh`**, dockpipe sets the URL from **`git remote get-url origin`** in the current dir (or **`--workdir`**). When that URL matches your **`origin`**, the worktree can be created from **your local checkout** (not only a fresh remote default). |
| `--branch <name>` | | Explicit work branch for **`--repo`** (optional). |
| `--work-branch <name>` | | When **`--repo`** is set and **`--branch`** is **omitted**, use this as the **full** branch name. If both are omitted, dockpipe generates **`prefix/<adj>-<noun>-<adj>-<noun>`** (random hyphenated slug; **`prefix`** from resolver / template — see **`domain.RandomWorkBranchSlug`** in **[branchslug.go](../lib/dockpipe/domain/branchslug.go)**). If **`--branch`** is set, it takes precedence over **`--work-branch`**. |
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
| `--build <path>` | | Parsed (relative to repo root or POSIX absolute). **Not currently consumed** by the Go runner — image build dirs come from **`--isolate`** / template resolution. Kept for CLI compatibility; see **`CliOpts.BuildPath`** in **[flags.go](../lib/dockpipe/application/flags.go)**. |
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
| `dockpipe windows setup --install-command "<cmd>"` | Run a command inside the selected distro during setup. |
| `dockpipe windows doctor` | List WSL distros and the configured default (if any). |

### Windows host: native vs WSL bridge

**Default:** **`dockpipe.exe`** runs on **Windows**. **Docker Desktop** supplies **`docker`**; **`bash`** and **`git`** must be on **`PATH`** separately (e.g. **Git for Windows**) — Docker Desktop does not ship them.

**WSL bridge (opt-in):** set **`DOCKPIPE_USE_WSL_BRIDGE=1`** so every command **except** `dockpipe windows …` runs inside WSL via **`wsl.exe -d <distro>`**. The current Windows working directory is mapped with **`wslpath`**.

Environment: **`DOCKPIPE_WINDOWS_BRIDGE=1`** is set for the **inner** process when the bridge is used. **Windows-style paths** in path-like flags (`--workdir`, `--work-path`, `--data-dir`, `--mount`, `--build`, `--env-file`, `--run` / `--act`, `--isolate` / `--template` / `--image` when the value is a host path, `--env` / `--var` when the value is a path, etc.) and in **`init` / `action init` / `pre init` / `template init`** destination args are translated to WSL paths: **`wslpath -u`** first, then a **drive-letter** fallback (`C:\…` → `/mnt/c/…`). **UNC** paths (`\\server\share\…`) use a fallback that avoids breaking **`filepath.Clean`**. Everything after a standalone **`--`** is passed through unchanged.

## Examples

**Minimal (workflow does the rest):**

```bash
dockpipe --workflow llm-worktree --repo https://github.com/you/repo.git -- claude -p "Fix the bug"
```

**Override branch, add env:**

```bash
dockpipe --workflow llm-worktree --repo https://github.com/you/repo.git --branch my-feature \
  --env "GIT_PAT=$GIT_PAT" -- claude -p "Implement the plan"
```

**No workflow — override run/isolate/act:**

```bash
dockpipe --resolver claude --repo https://github.com/you/repo.git \
  --run ./my-run.sh --act ./my-act.sh -- claude -p "Task"
```
