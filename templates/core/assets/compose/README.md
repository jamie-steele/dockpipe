# Compose assets (`templates/core/assets/compose/`)

**Compose is a reusable asset**, not a runtime, resolver, or strategy. Dockpipe does **not** run these files automatically; they document optional **`docker compose`** stacks for richer local setups (sidecars, multi-service dev environments).

Use from the **project root** (where **`templates/core/`** was merged by **`dockpipe init`**):

```bash
docker compose -f templates/core/assets/compose/<example>/docker-compose.yml up
```

Build template images first (e.g. **`docker build -t dockpipe-claude:latest -f templates/core/assets/images/claude/Dockerfile .`** from repo root).

| Directory | Illustrates |
|-----------|----------------|
| **`claude/`** | Optional stack around the **`claude`** resolver image. |
| **`codex/`** | Optional stack around **`codex`**. |
| **`code-server/`** | **`code-server`**-oriented example. |
| **`cursor-dev/`** | **`cursor-dev`**-oriented example. |
| **`vscode/`** | **`vscode`** / browser IDE sidecar patterns. |

Resolvers that only need a single container can ignore Compose entirely.
