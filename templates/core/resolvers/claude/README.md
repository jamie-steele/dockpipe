# claude

Bundled **workflow** for the **Claude Code** stack (`dockpipe-claude` image).

- **Primary use:** **`worktree`** **strategy** sample + resolver **`claude`** sets **`DOCKPIPE_RESOLVER_WORKFLOW=claude`**, so after **`clone-worktree`** the runner executes this delegate (same pattern as **`cursor-dev`**, **`vscode`**).
- **Isolate step:** one container step; your command after **`--`** is passed to the last step (e.g. `claude -p "…"`).

**Standalone:** `dockpipe --workflow claude -- …` only makes sense if **`/work`** is already the worktree you want (e.g. after a clone). For full clone + commit automation, use **`strategy: worktree`** in your workflow YAML with **`--resolver claude`** — **[docs/workflow-yaml.md](../../../../docs/workflow-yaml.md#named-strategies)**.
