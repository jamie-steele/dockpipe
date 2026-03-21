# codex

Bundled **workflow** for the **OpenAI Codex CLI** stack (`dockpipe-codex` image).

- **Primary use:** **`run-worktree`** resolver **`codex`** sets **`DOCKPIPE_RESOLVER_WORKFLOW=codex`**, so after **`clone-worktree`** the runner executes this workflow.
- **Isolate step:** one container step; your command after **`--`** is passed to the last step.

**Standalone:** `dockpipe --workflow codex -- …` only makes sense if **`/work`** is already the worktree you want. For full clone + commit automation, use **`--workflow run-worktree --resolver codex`**.

See also: **[templates/run-worktree/README.md](../run-worktree/README.md)**.
