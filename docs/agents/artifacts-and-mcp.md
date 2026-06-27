# Artifacts And MCP

Read when using generated context, self-analysis artifacts, or DorkPipe MCP tools.

## Two Channels

| Channel | Use | Rule |
| --- | --- | --- |
| On-disk artifacts | `bin/.dockpipe/`, `.dorkpipe/` handoffs, facts, CI bundles, metrics, insights | Read-only grounding unless user asks to refresh. |
| MCP (`mcpd`) | Bounded tool access with tiered IAM | Respect tier. Do not assume exec tools are available. |

## Artifact Rules

- Say whether artifacts look current vs `HEAD` when relevant.
- If missing or stale, suggest refresh instead of silently regenerating.
- Refresh only when the user asks.
- Pipeon binary: `packages/pipeon/resolvers/pipeon/bin/pipeon`.
- Pipeon host apps: `packages/pipeon/apps/`.

## MCP Tier Model

| Tier | Capability |
| --- | --- |
| `readonly` | list/read only |
| `validate` | list + validate, no run tools |
| `exec` | run tools enabled |

Set with `DOCKPIPE_MCP_TIER`. Default tier is `validate`. `exec` or legacy
`DOCKPIPE_MCP_ALLOW_EXEC=1` is required for run tools.

## Freshness

- If artifacts exist, say whether they look current vs `HEAD`.
- If missing or stale, suggest refresh only when relevant.
- Do not auto-regenerate self-analysis.
- `dockpipe init ... --from dorkpipe-self-analysis` appends a handoff section to `AGENTS.md` in new projects using marker `<!-- dockpipe: self-analysis handoff -->`.

## Docs

- `docs/artifacts.md`
- `packages/dorkpipe-mcp/README.md`
- `packages/dorkpipe-mcp/mcpbridge/README.md`
