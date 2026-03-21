# claude

Bundled **workflow** for the **Claude Code** stack (`dockpipe-claude` image).

- **Primary use:** **`run-worktree`** resolver **`claude`** sets **`DOCKPIPE_RESOLVER_WORKFLOW=claude`**, so after **`clone-worktree`** the runner executes this workflow (same pattern as **`cursor`** → **`cursor-dev`**, **`vscode`** → **`vscode`**).
- **Isolate step:** one container step; your command after **`--`** is passed to the last step (e.g. `claude -p "…"`).

**Standalone:** `dockpipe --workflow claude -- …` only makes sense if **`/work`** is already the worktree you want (e.g. after a clone). For full clone + commit automation, use **`--workflow run-worktree --resolver claude`**.

See also: **[templates/run-worktree/README.md](../run-worktree/README.md)**.
