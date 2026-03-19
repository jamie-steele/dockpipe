# CLI reference

**Run → isolate → act.** Overrides use the same names as workflow YAML. Precedence: **CLI** > config > environment.

**Workflow YAML (`config.yml`):** single-command layout (`run` / `isolate` / `act`) or multi-step **`steps:`** (ordering, **`outputs:`**, **`is_blocking`**, optional **`group.mode: async`**). Full reference: **[workflow-yaml.md](workflow-yaml.md)**.

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

| Flag | Purpose |
|------|---------|
| `--workflow <name>` | Use a workflow; config sets run/isolate/act and/or **`steps:`**. Override with below. See **[workflow-yaml.md](workflow-yaml.md)**. |
| `--run <path>` | **Run:** script(s) on host before container. Can be repeated. |
| `--isolate <name>` | **Isolate:** image or template (base-dev, claude, codex, …). Builds if template. |
| `--act <path>` | **Act:** script after command (e.g. commit-worktree). |
| `--resolver <name>` | Resolver (claude, codex); sets isolate + command. Use with `--repo`. |
| `--repo <url>` | Clone and use a worktree; worktree on host, commit on host. |
| `--branch <name>` | Work branch (optional). Omit for new branch each run. |
| `--workdir <path>` | Host path to mount at `/work`. Default: current directory. |
| `--work-path <path>` | Subfolder inside repo as cwd. |
| `--bundle-out <path>` | After commit-on-host, write a git bundle here. |
| `--env KEY=VAL` | Pass env var into container. Can be repeated. |
| `--mount <host:container>` | Extra volume. Can be repeated. |
| `--data-dir <path>` | Bind mount for persistent data. |
| `--no-data` | Do not mount the data volume. |
| `-d` / `--detach` | Run container in background. |

## Windows subcommands

Windows host helpers for WSL2 onboarding and diagnostics:

| Command | Purpose |
|---------|---------|
| `dockpipe windows setup` | Interactive setup: select WSL distro, bootstrap WSL env, optionally run install command in distro. |
| `dockpipe windows setup --distro <name> --non-interactive` | Fully scripted setup (requires explicit distro). |
| `dockpipe windows setup --install-command "<cmd>"` | Run custom install command inside selected distro during setup. |
| `dockpipe windows doctor` | Show discovered WSL distros and currently configured default distro (if set). |

### Windows host: forward runs into WSL

When using **`dockpipe.exe`** on Windows, every command **except** `dockpipe windows …` is executed inside WSL via `wsl.exe -d <distro>`. The **current Windows working directory** is mapped with `wslpath` and used as `cd` before `dockpipe` runs, so a repo opened at `C:\…\project` is used as `/mnt/c/…/project` in WSL.

Environment: `DOCKPIPE_WINDOWS_BRIDGE=1` is set for the inner process. **Windows-style paths** in path-like flags (`--workdir`, `--work-path`, `--data-dir`, `--mount`, `--build`, `--env-file`, `--run` / `--act`, `--isolate` / `--template` / `--image` when the value is a host path, `--env` / `--var` when the value is a path, etc.) and in **`init` / `action init` / `pre init` / `template init`** destination args are translated to WSL paths: **`wslpath -u`** on Windows first, then a **drive-letter mapper** (`C:\…` → `/mnt/c/…`). **UNC** paths (`\\server\share\…`) are normalized without being mangled by Unix `filepath.Clean` in the fallback. Everything after a standalone **`--`** is passed through unchanged.

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
