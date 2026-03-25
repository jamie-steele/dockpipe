# Documentation

**Start:** [onboarding.md](onboarding.md) — first commands, then concepts.

**Canonical terms:** [architecture-model.md](architecture-model.md) — workflow, runtime, resolver, strategy, assets.

**Store packages vs authoring tree:** [package-model.md](package-model.md) — `.dockpipe/internal/packages/`, `package.yml`, R2/S3 direction.

**Bounded contexts (DockPipe vs DorkPipe vs Pipeon):** [core-tools.md](core-tools.md) · Pipeon-only docs: [pipeon.md](pipeon.md) → [`src/pipeon/`](../src/pipeon/README.md) · **MCP bridge (AI/client interface):** [mcp-architecture.md](mcp-architecture.md) · **trust:** [mcp-agent-trust.md](mcp-agent-trust.md) · **host hardening:** [mcp-host-hardening.md](mcp-host-hardening.md) → [`src/lib/mcpbridge/`](../src/lib/mcpbridge/README.md)

| First | Reference |
|-------|-----------|
| [install.md](install.md) | [cli-reference.md](cli-reference.md) |
| [workflow-yaml.md](workflow-yaml.md) | [architecture.md](architecture.md) |
| [isolation-layer.md](isolation-layer.md) | [chaining.md](chaining.md) |

**Bundled paths:** In the materialized cache, **`shipyard/core/`** and **`shipyard/workflows/`** mirror **`src/templates/core/`** and **`src/templates/<workflow>/`**, and embedded repo-root **`workflows/`** lands under **`shipyard/workflows/`** in the cache. This repo’s git checkout uses **`workflows/`** at the project root (or **`templates/`** after **`dockpipe init`**). See [install.md](install.md#bundled-templates-no-extra-install-tree).

**QA:** [qa/manual-qa.md](qa/manual-qa.md)

**Repo copy (About, etc.):** [messaging.md](messaging.md)
