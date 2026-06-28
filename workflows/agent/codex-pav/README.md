# `codex-pav`

**Purpose:** Three-step **plan → apply → validate** pipeline with **`runtime: dockerimage`** and **`resolver: codex`** (Codex stays a resolver, not a runtime). Replace each step’s **`cmd`** with your real commands (e.g. **`codex`** invocations or scripts).

This example uses the bundled **`dev`** isolate as a placeholder container. If you want a dedicated Codex image, point **`isolate:`** at a real image/template available in your checkout or install.

**Prerequisites:** Docker; **`OPENAI_API_KEY`** (or Codex env) for steps that run the Codex image.

```bash
dockpipe --workflow codex-pav --resolver codex --runtime docker --
```

This workflow ships with the dockpipe **source tree** under **`workflows/agent/codex-pav/`**. To reuse it elsewhere, copy the directory or run **`dockpipe init yourflow --from /path/to/workflows/agent/codex-pav`**. See **[AGENTS.md](../../../AGENTS.md)**.
