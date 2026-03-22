# `dogfood-codex-security`

**Purpose:** Two-step workflow — **tests** in a generic isolated container (**`alpine`** placeholder), then a **Codex** step for security-oriented review. **`resolver: codex`** applies only to the second step; first step uses Docker only.

**Prerequisites:** Docker; **`OPENAI_API_KEY`** for the Codex step.

```bash
dockpipe --workflow dogfood-codex-security --resolver codex --runtime docker --
```

Edit **`cmd`** lines in **`config.yml`**: point the first step at your real test command; point the second at a **`codex`** (or scripted) security prompt.

Ships in the dockpipe repo under **`dockpipe/workflows/`**; copy the tree or use **`dockpipe init`** with **`--from`** pointing at it. See **[AGENTS.md](../../AGENTS.md)**.
