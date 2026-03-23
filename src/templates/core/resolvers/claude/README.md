# claude

Bundled **workflow** for the **Claude Code** stack (`dockpipe-claude` image).

- **Primary use:** **`worktree`** **strategy** sample + resolver **`claude`** sets **`DOCKPIPE_RESOLVER_WORKFLOW=claude`**, so after **`clone-worktree`** the runner executes this delegate (same pattern as **`cursor-dev`**, **`vscode`**).
- **Isolate step:** one container step; your command after **`--`** is passed to the last step (e.g. `claude -p "…"`).

**Standalone:** `dockpipe --workflow claude -- …` only makes sense if **`/work`** is already the worktree you want (e.g. after a clone). For full clone + commit automation, use **`strategy: worktree`** in your workflow YAML with **`--resolver claude`** — **[docs/workflow-yaml.md](../../../../docs/workflow-yaml.md#named-strategies)**.

## Claude inside DockPipe Docker

**Runtime `docker`** is the isolation layer for the project at **`/work`**.

- **API keys:** set **`ANTHROPIC_API_KEY`** and/or **`CLAUDE_API_KEY`** in the environment or repo-root **`.env`**. Resolver **`DOCKPIPE_RESOLVER_ENV`** lists both; the runner forwards hinted keys into the container when using **`docker run`** (same pattern as **codex** / **`OPENAI_API_KEY`**).
- **Non-interactive** one-shot commands often need **`claude --dangerously-skip-permissions -p "…"`** (or **`$(cat)`** from a pipe). That is a Claude Code permission model, not a second Linux namespace sandbox like Codex’s optional bubblewrap stack — see **codex** resolver README for the **Codex**-specific nested-**bwrap** topic.
