# Scripts

Run and act scripts in **one folder**. Mix and match with any workflow.

| Script | Type | What it does |
|--------|------|--------------|
| `clone-worktree.sh` | pre | Create worktree and export `DOCKPIPE_WORKDIR` + `DOCKPIPE_COMMIT_ON_HOST`. If `DOCKPIPE_USER_REPO_ROOT` is set (same `origin` as `DOCKPIPE_REPO_URL`), uses **`git worktree add` from that checkout** (new branch from **your current HEAD**). Uncommitted work is **copied** into the worktree (`git diff` + apply + untracked files); your main checkout is unchanged. **Gitignored** local files can be listed in **`.dockpipe-worktreeinclude`** or **`.worktreeinclude`** (see **[docs/worktree-include.md](../../../docs/worktree-include.md)**). Set `DOCKPIPE_STASH_UNCOMMITTED=1` for the legacy **git stash** flow instead. Otherwise clones/fetches into `DOCKPIPE_DATA_DIR` and bases the worktree on **origin/HEAD** (mirror mode). |
| `commit-worktree.sh` | action | Triggers commit-on-host after container exit (commit runs on host). |
| `export-patch.sh` | action | Write uncommitted changes to a patch file. |
| `print-summary.sh` | action | Print exit code and git status summary. |

Use with `--run scripts/clone-worktree.sh`, `--act scripts/commit-worktree.sh`, or set `run:` and `act:` in a workflow config. Framework Dockerfiles live under **`templates/core/assets/images/`** (see **[docs/templates-core-assets.md](../../../../docs/templates-core-assets.md)**).
