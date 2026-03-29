# Scripts

**Agnostic** run/act helpers live **here** (root of this folder): clone/commit worktree, export patch, print summary, examples.

**Resolver-specific** host scripts live **only** under **`templates/core/resolvers/<name>/`** (next to **`config.yml`**). **`ResolveWorkflowScript`** maps **`scripts/cursor-dev/…`** and **`scripts/vscode/…`** to those paths — nothing duplicate under **`assets/scripts/`** for those names.

**Domain assets** (**DorkPipe**, Pipeon, review-pipeline, …) live under **`templates/core/bundles/<domain>/`** — see **`../bundles/README.md`**. They are **merged** with **`dockpipe init`** / the materialized **`templates/core`** bundle and referenced as **`scripts/<domain>/…`** in YAML. Do not park domain scripts next to **`resolvers/`** unless they are true **`--resolver`** profiles.

| Script | Type | What it does |
|--------|------|--------------|
| **`terraform-pipeline.sh`** | library | **Source** (not execute): `dockpipe_tf_run_pipeline` runs **`terraform`** steps from **`DOCKPIPE_TF_COMMANDS`** (`init`, `plan`, `apply`, `validate`, `fmt`, `import`) with optional R2 remote backend. Used by **`dockpipe.cloudflare.r2publish`**; copy the pattern for other host workflows. See **[docs/terraform-pipeline.md](../../../docs/terraform-pipeline.md)**. |
| `clone-worktree.sh` | pre | Create worktree and export `DOCKPIPE_WORKDIR` + `DOCKPIPE_COMMIT_ON_HOST`. If `DOCKPIPE_USER_REPO_ROOT` is set (same `origin` as `DOCKPIPE_REPO_URL`), uses **`git worktree add` from that checkout** (new branch from **your current HEAD**). Uncommitted work is **copied** into the worktree (`git diff` + apply + untracked files); your main checkout is unchanged. **Gitignored** local files can be listed in **`.dockpipe-worktreeinclude`** or **`.worktreeinclude`** (see **[docs/worktree-include.md](../../../docs/worktree-include.md)**). Set `DOCKPIPE_STASH_UNCOMMITTED=1` for the **git stash** flow instead. Otherwise clones/fetches into `DOCKPIPE_DATA_DIR` and bases the worktree on **origin/HEAD** (mirror mode). |
| `commit-worktree.sh` | action | Triggers commit-on-host after container exit (commit runs on host). |
| `export-patch.sh` | action | Write uncommitted changes to a patch file. |
| `print-summary.sh` | action | Print exit code and git status summary. |

Use with `--run scripts/clone-worktree.sh`, `--act scripts/commit-worktree.sh`, or set `run:` and `act:` in a workflow config. Framework Dockerfiles are resolved by **`DockerfileDir`** / **`TemplateBuild`** (see **[docs/templates-core-assets.md](../../../../docs/templates-core-assets.md)**).
