# code-server

Bundled **workflow** for the **`dockpipe-vscode`** image (code-server stack). This is the **container isolate** path for **`resolver code-server`**.

- **Browser IDE on the host** (port publish, etc.) is **`--resolver vscode`** (delegates to **`templates/vscode`**).
- **Run a command inside the code-server image** in a worktree: **`run-worktree`** + **`resolver code-server`** → **`DOCKPIPE_RESOLVER_WORKFLOW=code-server`** → this workflow.

See also: **[templates/run-worktree/README.md](../run-worktree/README.md)** · **[templates/vscode/README.md](../vscode/README.md)**.
