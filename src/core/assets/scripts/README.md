# Scripts

**Agnostic** run/act helpers live **here** (root of this folder): clone/commit worktree, export patch, print summary, examples.

**Resolver-specific** host scripts live **only** under **`templates/core/resolvers/<name>/`** (next to **`config.yml`**). **`ResolveWorkflowScript`** maps **`scripts/cursor-dev/…`** and **`scripts/vscode/…`** to those paths — nothing duplicate under **`assets/scripts/`** for those names.

**Domain assets** (**DorkPipe**, Pipeon, …) live under **`templates/core/bundles/<domain>/`** — see **`../bundles/README.md`**. They are **merged** with **`dockpipe init`** / the materialized **`templates/core`** bundle and referenced as **`scripts/<domain>/…`** in YAML. In **this** repository, review prep lives under **`workflows/review-pipeline/`** (same **`scripts/review-pipeline/…`** path in YAML). Do not park domain scripts next to **`resolvers/`** unless they are true **`--resolver`** profiles.

| Script | Type | What it does |
|--------|------|--------------|
| **`terraform-pipeline.sh`** | library | **Source** (not execute): `dockpipe_tf_run_pipeline` — see **Terraform** below. |
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
for candidate in "$ROOT/templates/core/assets/scripts/terraform-pipeline.sh" "$ROOT/src/core/assets/scripts/terraform-pipeline.sh"; do
  [[ -f "$candidate" ]] && { source "$candidate"; break; }
done
```

### `DOCKPIPE_TF_*`

| Variable | Meaning |
|----------|---------|
| `DOCKPIPE_TF_COMMANDS` | Comma-separated: `init`, `plan`, `apply`, `validate`, `fmt`, `import`. |
| `DOCKPIPE_TF_SKIP_INIT` | `1` skips auto-`init`. |
| `DOCKPIPE_TF_BACKEND` | `local` or `remote` (R2/S3-style state). |
| `DOCKPIPE_TF_STATE_BUCKET` / `DOCKPIPE_TF_STATE_KEY` | Remote state object. |
| `DOCKPIPE_TF_STATE_ACCESS_KEY_ID` / `DOCKPIPE_TF_STATE_SECRET_ACCESS_KEY` | State credentials. |
| `DOCKPIPE_TF_CLOUDFLARE_ACCOUNT_ID` | R2 endpoint in generated backend HCL. |
| `DOCKPIPE_TF_WORKSPACE` | After `init`, `workspace select` or `new`. |
| `DOCKPIPE_TF_*_ARGS` | Extra args per subcommand (`INIT_ARGS`, `PLAN_ARGS`, …). |
| `DOCKPIPE_TF_APPLY_AUTO_APPROVE` | Default `1`. |
| `DOCKPIPE_TF_IMPORT_ARGS` / `DOCKPIPE_TF_IMPORT_FILE` | Import step. |
| `DOCKPIPE_TF_DRY_RUN` | `1` prints only. |

**R2 publish workflow** maps legacy `R2_TERRAFORM_*` / `R2_TF_*` into `DOCKPIPE_TF_*` — full ops doc: **cloud** maintainer pack **`dockpipe.cloudflare.r2publish`** resolver **README**.
