# code-server

Bundled **workflow** for the **`dockpipe-vscode`** image (code-server stack). This is the **container isolate** path for **`resolver code-server`**.

- **Browser IDE on the host** (port publish, etc.) is **`--resolver vscode`** (delegate YAML under **`templates/core/resolvers/vscode/`**).
- **Run a command inside the code-server image** in a worktree: **`worktree`** strategy sample + **`resolver code-server`** → **`DOCKPIPE_RESOLVER_WORKFLOW=code-server`** → this delegate.

See also: **[workflow YAML — strategies](../../../../docs/workflow-yaml.md#named-strategies)** · **[vscode resolver](../vscode/README.md)**.
