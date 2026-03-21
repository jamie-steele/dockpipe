# llm-worktree

Prepare a worktree on the **host**, run your tool in a **container**, commit on the **host** — **run → isolate → act**.

The **container image** (or **bundled workflow** for IDE-style resolvers) is chosen by **`--resolver`** via **`resolvers/<name>`** → **`DOCKPIPE_RESOLVER_TEMPLATE`** or **`DOCKPIPE_RESOLVER_WORKFLOW`** (not by hardcoding **`isolate:`** in **`config.yml`**).

## Try it

```bash
dockpipe --workflow llm-worktree --resolver claude --repo https://github.com/you/repo.git -- claude -p "Your prompt"
dockpipe --workflow llm-worktree --resolver codex --repo https://github.com/you/repo.git -- codex "Your prompt"
# Host IDE workflows (same as templates/cursor-dev / vscode) — use -- with nothing after, or a dummy:
dockpipe --workflow llm-worktree --resolver cursor --repo https://github.com/you/repo.git --
dockpipe --workflow llm-worktree --resolver vscode --repo https://github.com/you/repo.git --
# Command inside the code-server *image* (not the browser workflow):
dockpipe --workflow llm-worktree --resolver code-server --repo https://github.com/you/repo.git -- sh -c 'echo ok'
```

Default resolver is **`claude`** (`default_resolver` in **`config.yml`**) if you omit **`--resolver`**.

If your cwd is a clone with **`origin`** set, you can omit **`--repo`**.

## Prerequisites

- **Docker**, **bash**, **git** on the host.
- API keys as required by the resolver (e.g. **`ANTHROPIC_API_KEY`**, **`OPENAI_API_KEY`**).
- **`cursor`** / **`vscode`** resolvers set **`DOCKPIPE_RESOLVER_WORKFLOW`** to **`cursor-dev`** / **`vscode`** — the same **`templates/*/config.yml`** as **`dockpipe --workflow cursor-dev`** / **`vscode`** (after clone-worktree; no `docker run` isolate step), then **commit** on the branch.
- **`code-server`** resolver uses the **`dockpipe-vscode`** image for a normal **container** command after `--`.

## Layout

| Path | Role |
|------|------|
| `config.yml` | **`run`** / **`act`**, **`default_resolver`**, listed **`resolvers`** |
| `resolvers/<name>` | **Image** (`DOCKPIPE_RESOLVER_TEMPLATE`), **workflow** (`DOCKPIPE_RESOLVER_WORKFLOW`), or **host script** (`DOCKPIPE_RESOLVER_HOST_ISOLATE`) + optional metadata |

**Resolver contract:** **[resolvers/README.md](resolvers/README.md)** · **Workflow YAML:** **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** · **Onboarding:** **[docs/onboarding.md](../../docs/onboarding.md)**
