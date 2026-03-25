# Compose (`templates/core/assets/compose/`)

**Compose is a reusable asset**, not a runtime or resolver. Dockpipe does **not** run these files automatically; they document optional **`docker compose`** patterns.

**Where examples live:** domain-specific **Compose** files sit next to the resolver or bundle that owns them, under **`assets/compose/docker-compose.yml`**:

- **`templates/core/bundles/dorkpipe/assets/compose/docker-compose.yml`** — Postgres + Ollama sidecar for DorkPipe local DAG work (**`scripts/dorkpipe/dev-stack.sh`**).
- **`templates/core/resolvers/<name>/assets/compose/docker-compose.yml`** — optional examples for **`claude`**, **`codex`**, **`code-server`**, **`cursor-dev`**, **`vscode`**, etc.

Use from the **project root** (where **`templates/core/`** was merged by **`dockpipe init`**):

```bash
docker compose -f templates/core/bundles/dorkpipe/assets/compose/docker-compose.yml up -d
```

**Agnostic Compose demos** (not tied to a resolver name) live here too:

- **`minimal/docker-compose.yml`** — single service.
- **`multi-service/docker-compose.yml`** — two services on a private network.

Build template images first (e.g. **`docker build -t dockpipe-claude:latest -f templates/core/resolvers/claude/assets/images/claude/Dockerfile .`** from repo root).

Resolvers that only need a single container can ignore Compose entirely.
