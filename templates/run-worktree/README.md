# run-worktree

Prepare a worktree on the **host**, run your tool in a **container**, commit on the **host**. The template sets **`strategy: git-worktree`** so **`templates/core/strategies/git-worktree`** runs **`clone-worktree.sh`** before and **`commit-worktree.sh`** after success (same end result as the older **`run` / `act`** YAML layout).

The **container image** (or **bundled workflow** for IDE-style resolvers) is chosen by **`--resolver`** via **`resolvers/<name>`** → **`DOCKPIPE_RESOLVER_TEMPLATE`** or **`DOCKPIPE_RESOLVER_WORKFLOW`** (not by hardcoding **`isolate:`** in **`config.yml`**).

## Try it

```bash
dockpipe --workflow run-worktree --resolver claude --repo https://github.com/you/repo.git -- claude -p "Your prompt"
dockpipe --workflow run-worktree --resolver codex --repo https://github.com/you/repo.git -- codex "Your prompt"
# Host IDE workflows (same as templates/cursor-dev / vscode) — use -- with nothing after, or a dummy:
dockpipe --workflow run-worktree --resolver cursor --repo https://github.com/you/repo.git --
dockpipe --workflow run-worktree --resolver vscode --repo https://github.com/you/repo.git --
# Command inside the code-server *image* (not the browser workflow):
dockpipe --workflow run-worktree --resolver code-server --repo https://github.com/you/repo.git -- sh -c 'echo ok'
```

Default resolver is **`claude`** (`default_resolver` in **`config.yml`**) if you omit **`--resolver`**.

If your cwd is a clone with **`origin`** set, you can omit **`--repo`**.

## Prerequisites

- **Docker**, **bash**, **git** on the host.
- API keys as required by the resolver (e.g. **`ANTHROPIC_API_KEY`**, **`OPENAI_API_KEY`**).
- **`claude`** / **`codex`** / **`code-server`** resolvers set **`DOCKPIPE_RESOLVER_WORKFLOW`** to bundled **`templates/claude`**, **`templates/codex`**, **`templates/code-server`** (single-step isolate), then **commit** on the branch.
- **`cursor`** / **`vscode`** resolvers set **`DOCKPIPE_RESOLVER_WORKFLOW`** to **`cursor-dev`** / **`vscode`** — the same **`templates/*/config.yml`** as **`dockpipe --workflow cursor-dev`** / **`vscode`** (host / skip_container flows), then **commit**.

## Layout

| Path | Role |
|------|------|
| `config.yml` | **`strategy`** (e.g. **`git-worktree`**), **`default_resolver`**, listed **`strategies`** / **`resolvers`** |
| Shared resolvers | Bundled **`templates/core/resolvers/<name>`** — **Image** (`DOCKPIPE_RESOLVER_TEMPLATE`), **workflow** (`DOCKPIPE_RESOLVER_WORKFLOW`), or **host script** (`DOCKPIPE_RESOLVER_HOST_ISOLATE`). Override with **`templates/run-worktree/resolvers/<name>`** in a fork if needed. |

**Resolver contract:** **[resolvers/README.md](resolvers/README.md)** · **Workflow YAML:** **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** · **Onboarding:** **[docs/onboarding.md](../../docs/onboarding.md)**
