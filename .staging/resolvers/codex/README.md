# codex

Bundled **workflow** for the **OpenAI Codex CLI** stack (`dockpipe-codex` image).

- **Primary use:** **`worktree`** **strategy** sample + resolver **`codex`** sets **`DOCKPIPE_RESOLVER_WORKFLOW=codex`**, so after **`clone-worktree`** the runner executes this delegate.
- **Isolate step:** one container step; your command after **`--`** is passed to the last step.

**Standalone:** `dockpipe --workflow codex -- …` only makes sense if **`/work`** is already the worktree you want. For full clone + commit automation, use **`strategy: worktree`** with **`--resolver codex`** — **[docs/workflow-yaml.md](../../../../docs/workflow-yaml.md#named-strategies)**.

## Codex inside DockPipe Docker (no nested sandbox)

DockPipe **runtime `docker`** is the isolation layer for the project at **`/work`**.

Codex’s **`--sandbox workspace-write`** (and similar) runs **bubblewrap** inside the container. That stacks a second Linux sandbox on top of Docker and typically **fails** with user-namespace errors (`bwrap: No permissions to create a new namespace`, etc.) on common kernels/AppArmor setups—**without** any fault in DockPipe.

**Recommended for workflow steps that invoke `codex exec` in this image:**

- Use **`codex exec --dangerously-bypass-approvals-and-sandbox`** (or the documented alias **`--yolo`**) so **command execution is not wrapped in bwrap**; trust **Docker + uid-mapped bind mounts** as the boundary.
- Keep **`OPENAI_API_KEY`** / **`CODEX_API_KEY`** in the environment (resolver **`DOCKPIPE_RESOLVER_ENV`** forwards them into the container).

This is **workflow/resolver configuration**, not a dockpipe core special case. Example: **`shipyard/workflows/test-demo`** review step.
