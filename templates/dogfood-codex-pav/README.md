# `dogfood-codex-pav`

**Purpose:** Three-step **plan → apply → validate** pipeline with **`runtime: docker`** and **`resolver: codex`** (Codex stays a resolver, not a runtime). Replace each step’s **`cmd`** with your real commands (e.g. **`codex`** invocations or scripts).

**Prerequisites:** Docker; **`OPENAI_API_KEY`** (or Codex env) for steps that run the Codex image.

```bash
dockpipe --workflow dogfood-codex-pav --resolver codex --runtime docker --
```

Install into a project with **`dockpipe init --dogfood-codex-pav`** — copies this preset to **`dockpipe/workflows/dogfood-codex-pav/`** (repo-local). See **[docs/cli-reference.md](../../docs/cli-reference.md)** (`dockpipe init`).
