# codex

Bundled **workflow** for the **OpenAI Codex CLI** stack (`dockpipe-codex` image).

- **Primary use:** **`worktree`** **strategy** sample + resolver **`codex`** sets **`DOCKPIPE_RESOLVER_WORKFLOW=codex`**, so after **`clone-worktree`** the runner executes this delegate.
- **Isolate step:** one container step; your command after **`--`** is passed to the last step.

**Standalone:** `dockpipe --workflow codex -- …` only makes sense if **`/work`** is already the worktree you want. For full clone + commit automation, use **`strategy: worktree`** with **`--resolver codex`** — **[docs/workflow-yaml.md](../../../../docs/workflow-yaml.md#named-strategies)**.
