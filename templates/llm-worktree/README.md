# llm-worktree

Prepare a worktree on the **host**, run your tool in a **container**, commit on the **host** — **run → isolate → act**.

The **container image** is chosen by **`--resolver`** via **`resolvers/<name>`** → **`DOCKPIPE_RESOLVER_TEMPLATE`** (not by hardcoding **`isolate:`** in **`config.yml`**).

## Try it

```bash
dockpipe --workflow llm-worktree --resolver claude --repo https://github.com/you/repo.git -- claude -p "Your prompt"
dockpipe --workflow llm-worktree --resolver codex --repo https://github.com/you/repo.git -- codex "Your prompt"
dockpipe --workflow llm-worktree --resolver vscode --repo https://github.com/you/repo.git -- your-command
```

Default resolver is **`claude`** (`default_resolver` in **`config.yml`**) if you omit **`--resolver`**.

If your cwd is a clone with **`origin`** set, you can omit **`--repo`**.

## Prerequisites

- **Docker**, **bash**, **git** on the host.
- API keys as required by the resolver (e.g. **`ANTHROPIC_API_KEY`**, **`OPENAI_API_KEY`**).
- **`cursor`** resolver is **experimental** — see **`resolvers/cursor`** and **`resolvers/README.md`**.
- **`vscode`** resolver uses the **code-server** image (`dockpipe-vscode`). For a **browser IDE** with host port publish, use **`--workflow vscode`** instead; the resolver is for running commands in that image during **llm-worktree** flows.

## Layout

| Path | Role |
|------|------|
| `config.yml` | **`run`** / **`act`**, **`default_resolver`**, listed **`resolvers`** |
| `resolvers/<name>` | **Image** (`DOCKPIPE_RESOLVER_TEMPLATE`) + optional metadata |

**Resolver contract:** **[resolvers/README.md](resolvers/README.md)** · **Workflow YAML:** **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** · **Onboarding:** **[docs/onboarding.md](../../docs/onboarding.md)**
