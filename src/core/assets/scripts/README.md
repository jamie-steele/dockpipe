# Scripts

**Agnostic** run/act helpers live **here** (root of this folder): clone/commit worktree, export patch, print summary, examples.

**Shared script SDK libs** live under **`lib/`** in this folder. The canonical cross-language DockPipe SDK support now lives in:

- **`src/core/assets/scripts/lib/dockpipe-sdk.sh`**
- **`src/core/assets/scripts/lib/repo-tools.sh`**
- **`src/core/assets/scripts/lib/repo-tools.ps1`**
- **`src/core/assets/scripts/lib/repo_tools.py`**
- **`src/core/assets/scripts/lib/repotools/repotools.go`**

For shell, prefer direct CLI getters for plain context reads:

```bash
dockpipe get workdir
```

## Injected script context

When a DockPipe-managed host script is launched, the runtime injects generic package script
context into the environment. The backing env vars are:

- `DOCKPIPE_WORKDIR`
- `DOCKPIPE_DOCKPIPE_BIN`
- `DOCKPIPE_WORKFLOW_NAME`
- `DOCKPIPE_SCRIPT_DIR`
- `DOCKPIPE_PACKAGE_ROOT`
- `DOCKPIPE_ASSETS_DIR`

The public shell authoring API for reading those values is `dockpipe get ...`:

```bash
dockpipe get workdir
dockpipe get workflow_name
dockpipe get script_dir
dockpipe get package_root
dockpipe get assets_dir
dockpipe get dockpipe_bin
```

Recommended usage:

- Package scripts and examples: use `dockpipe get ...`
- Shell-only behavior that must mutate the current shell: use `eval "$(dockpipe sdk)"` and `dockpipe_sdk ...`
- Low-level implementation helpers may read the backing env vars directly when they are intentionally
  avoiding subprocess overhead, but that is an internal optimization, not the default authoring style

The important mapping is:

- `dockpipe get script_dir` reads injected `DOCKPIPE_SCRIPT_DIR`
- `dockpipe get package_root` reads injected `DOCKPIPE_PACKAGE_ROOT`
- `dockpipe get assets_dir` reads injected `DOCKPIPE_ASSETS_DIR`
- `dockpipe get workflow_name` reads injected `DOCKPIPE_WORKFLOW_NAME`
- `dockpipe get workdir` prefers injected `DOCKPIPE_WORKDIR` and otherwise falls back to the current directory
- `dockpipe get dockpipe_bin` resolves the active DockPipe binary for the current workdir

For direct test/manual calls that bypass the normal workflow boundary, callers may export the same
backing env vars explicitly before invoking a package script.

For shell-specific actions that need to mutate the current shell, bootstrap the shell SDK:

```bash
eval "$(dockpipe sdk)"
dockpipe_sdk init-script
```

The SDK prompt contract is shared across shell, PowerShell, Python, and Go helpers. Shell exposes it
through `dockpipe_sdk ...`, while the other helper libraries expose matching prompt functions.

The shell SDK exposes a small prompt primitive for package-owned setup and remediation flows:

```bash
answer="$(dockpipe_sdk prompt confirm \
  --id gpu_setup \
  --title "Enable Docker GPU support?" \
  --message "Ollama found an NVIDIA GPU, but Docker is not configured for GPU containers yet." \
  --default no)"
```

Prompt behavior:

- In a normal terminal, `dockpipe_sdk prompt ...` renders an interactive CLI prompt and returns the answer on stdout.
- Under the DockPipe Launcher, the shell SDK emits a structured prompt event (`DOCKPIPE_SDK_PROMPT_MODE=json`), the launcher renders native UI, and the selected answer is written back to the running workflow.
- Supported prompt kinds today: `confirm`, `choice`, `input`.
- Prompt metadata can classify author intent with options like `--intent`, `--automation-group`,
  `--allow-auto-approve`, and `--auto-approve-value`.
- Automation can bypass prompts that explicitly opt in to auto-approval with `dockpipe --yes` or
  `DOCKPIPE_APPROVE_PROMPTS=1`.

The getter path avoids hard-coding source paths or manual root resolution in package scripts. The command prefers `DOCKPIPE_WORKDIR` when it is already set by a workflow and otherwise falls back to the current directory. The shared core SDK is the source of truth for authoring support in shell/PowerShell/Python/Go.

**Resolver-specific** host scripts live **only** under **`templates/core/resolvers/<name>/`** (next to **`config.yml`**). **`ResolveWorkflowScript`** maps **`scripts/cursor-dev/…`** and **`scripts/vscode/…`** to those paths — nothing duplicate under **`assets/scripts/`** for those names.

**Domain assets** (**DorkPipe**, Pipeon, …) live under **`templates/core/bundles/<domain>/`** — see **`../bundles/README.md`**. They are **merged** with **`dockpipe init`** / the materialized **`templates/core`** bundle and referenced as **`scripts/<domain>/…`** in YAML. In **this** repository, review prep lives under **`workflows/review-pipeline/`** (same **`scripts/review-pipeline/…`** path in YAML). Do not park domain scripts next to **`resolvers/`** unless they are true **`--resolver`** profiles.

| Script | Type | What it does |
|--------|------|--------------|
| **`terraform-pipeline.sh` / `terraform-run.sh`** | (moved) | **Provider-agnostic Terraform** ships with **`packages/terraform/resolvers/terraform-core/`** (`dockpipe.terraform.core`), not this folder. Workflow YAML still uses **`scripts/core.assets.scripts.terraform-run.sh`** — resolution finds the package first. |
| **`packages/cloud/storage/.../terraform-cloudflare-r2-run.sh`** | host | Cloudflare R2 + bundled infra-local Terraform module: **`scripts/dockpipe.cloudflare.r2infra/terraform-cloudflare-r2-run.sh`** (resolver **`dockpipe.cloudflare.r2infra`**). Used by **`r2-publish.sh`** too (not **`terraform-run.sh`**). |
| `clone-worktree.sh` | pre | Create worktree and export `DOCKPIPE_WORKDIR` + `DOCKPIPE_COMMIT_ON_HOST`. If `DOCKPIPE_USER_REPO_ROOT` is set (same `origin` as `DOCKPIPE_REPO_URL`), uses **`git worktree add` from that checkout** (new branch from **your current HEAD**). Uncommitted work is **copied** into the worktree (`git diff` + apply + untracked files); your main checkout is unchanged. **Gitignored** local files can be listed in **`.dockpipe-worktreeinclude`** or **`.worktreeinclude`** (see **[docs/worktree-include.md](../../../docs/worktree-include.md)**). Set `DOCKPIPE_STASH_UNCOMMITTED=1` for the **git stash** flow instead. Otherwise clones/fetches into `DOCKPIPE_DATA_DIR` and bases the worktree on **origin/HEAD** (mirror mode). |
| `commit-worktree.sh` | action | Triggers commit-on-host after container exit (commit runs on host). |
| `export-patch.sh` | action | Write uncommitted changes to a patch file. |
| `print-summary.sh` | action | Print exit code and git status summary. |

Use with `--run scripts/clone-worktree.sh`, `--act scripts/commit-worktree.sh`, or set `run:` and `act:` in a workflow config. Framework Dockerfiles are resolved by **`DockerfileDir`** / **`TemplateBuild`** (see **[docs/templates-core-assets.md](../../../../docs/templates-core-assets.md)**).

---

## `terraform-pipeline.sh`

**CLI:** `dockpipe terraform pipeline-path` (or `dockpipe core script-path …`) prints the resolved path — same rules as workflow **`scripts/`** paths.

**Source** from a workflow script (example):

```bash
for candidate in \
  "$ROOT/packages/terraform/resolvers/terraform-core/assets/scripts/terraform-pipeline.sh" \
  "$ROOT/templates/core/assets/scripts/terraform-pipeline.sh" \
  "$ROOT/src/core/assets/scripts/terraform-pipeline.sh"
do
  [[ -f "$candidate" ]] && { source "$candidate"; break; }
done
```

### `DOCKPIPE_TF_*`

| Variable | Meaning |
|----------|---------|
| `DOCKPIPE_TF_COMMANDS` | Comma-separated: `init`, `plan`, `apply`, `validate`, `fmt`, `import`. |
| `DOCKPIPE_TF_SKIP_INIT` | `1` skips auto-`init`. |
| `DOCKPIPE_TF_BACKEND` | `local` or `remote` (R2/S3-style state). |
| `DOCKPIPE_TF_REMOTE_BACKEND_FILE` | If set, `terraform init` uses this file as `-backend-config=` (skip generated R2 `backend` block). |
| `dockpipe_tf_map_generic_env` (function) | Call after sourcing the library for **`dockpipe.terraform.core`**: default `DOCKPIPE_TF_COMMANDS=plan`, `DOCKPIPE_TF_BACKEND=local`. |
| `DOCKPIPE_TF_STATE_BUCKET` / `DOCKPIPE_TF_STATE_KEY` | Remote state object. |
| `DOCKPIPE_TF_STATE_ACCESS_KEY_ID` / `DOCKPIPE_TF_STATE_SECRET_ACCESS_KEY` | State credentials. |
| `DOCKPIPE_TF_CLOUDFLARE_ACCOUNT_ID` | R2 endpoint in generated backend HCL. |
| `DOCKPIPE_TF_WORKSPACE` | After `init`, `workspace select` or `new`. |
| `DOCKPIPE_TF_*_ARGS` | Extra args per subcommand (`INIT_ARGS`, `PLAN_ARGS`, …). |
| `DOCKPIPE_TF_APPLY_AUTO_APPROVE` | Default `1`. |
| `DOCKPIPE_TF_IMPORT_ARGS` / `DOCKPIPE_TF_IMPORT_FILE` | Import step. |
| `DOCKPIPE_TF_DRY_RUN` | `1` prints only. |

**R2 publish workflow** maps legacy `R2_TERRAFORM_*` / `R2_TF_*` into `DOCKPIPE_TF_*` — full ops doc: **cloud** maintainer pack **`dockpipe.cloudflare.r2publish`** resolver **README**.
