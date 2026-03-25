# Pipeon (local-first assistant)

**Pipeon** is a small shell layer that:

1. Builds **`.dockpipe/pipeon-context.md`** from CI signals, user insights, DorkPipe metadata, and self-analysis pointers.
2. Sends **one** chat turn to **local Ollama** with a fixed system prompt + that bundle.

**No cloud LLM required.** Default model: `llama3.2` (override with `PIPEON_OLLAMA_MODEL` or `DOCKPIPE_OLLAMA_MODEL`).

## Feature flags (release 0.6.5)

| Variable | Meaning |
|----------|---------|
| **`DOCKPIPE_PIPEON=1`** | **Required** to run `bundle` or `chat`. |
| **`DOCKPIPE_PIPEON_MIN_VERSION`** | Default **`0.6.5`**. Pipeon refuses if repo `VERSION` is lower, unless prerelease override is set. |
| **`DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1`** | Allow Pipeon **before** `MIN_VERSION` (developers / CI). |

Until **0.6.5** ships, use **`DOCKPIPE_PIPEON=1`** and **`DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1`** to try.

## Commands

```bash
export DOCKPIPE_PIPEON=1
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1   # until VERSION >= 0.6.5
./src/bin/pipeon status
./src/bin/pipeon bundle
./src/bin/pipeon chat "Summarize security posture from available signals."
```

## Shortcuts

See **`src/apps/pipeon/docs/pipeon-shortcuts.md`** (VS Code/Cursor tasks, shell aliases, optional desktop).

## Docs

- **`pipeon/docs/pipeon-ide-experience.md`** — UX and tone
- **`pipeon/docs/pipeon-shortcuts.md`** — keyboard and task shortcuts
