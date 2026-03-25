# `codex-pav`

**Purpose:** Three-step **plan → apply → validate** pipeline with **`runtime: docker`** and **`resolver: codex`** (Codex stays a resolver, not a runtime). Replace each step’s **`cmd`** with your real commands (e.g. **`codex`** invocations or scripts).

**Prerequisites:** Docker; **`OPENAI_API_KEY`** (or Codex env) for steps that run the Codex image.

```bash
dockpipe --workflow codex-pav --resolver codex --runtime docker --
```

This workflow ships with the dockpipe **source tree** under **`workflows/codex-pav/`**. To reuse it elsewhere, copy the directory or run **`dockpipe init yourflow --from /path/to/workflows/codex-pav`**. See **[AGENTS.md](../../AGENTS.md)**.
