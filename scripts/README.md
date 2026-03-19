# Scripts

Run and act scripts in **one folder**. Mix and match with any workflow.

| Script | Type | What it does |
|--------|------|--------------|
| `clone-worktree.sh` | pre | Clone/fetch repo, create worktree; export DOCKPIPE_WORKDIR and DOCKPIPE_COMMIT_ON_HOST. |
| `commit-worktree.sh` | action | Triggers commit-on-host after container exit (commit runs on host). |
| `export-patch.sh` | action | Write uncommitted changes to a patch file. |
| `print-summary.sh` | action | Print exit code and git status summary. |

Use with `--run scripts/clone-worktree.sh`, `--act scripts/commit-worktree.sh`, or set `run:` and `act:` in a workflow config. Images live in **images/** at repo root.
