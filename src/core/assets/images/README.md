# Framework images (`templates/core/assets/images/`)

**Agnostic** Dockerfiles used as **defaults** for **`--isolate`** when no resolver/bundle override exists: **`base-dev/`**, **`dev/`**, **`example/`**, **`minimal/`**.

**Domain-specific** images live next to their owner (search order in **`DockerfileDir`** / **`TemplateBuild`**):

1. **`templates/core/resolvers/<name>/assets/images/<name>/`** — e.g. **claude**, **codex**, **vscode**, **code-server**, **ollama**
2. **`templates/core/bundles/<domain>/assets/images/<domain>/`** — per-domain Dockerfiles
3. Fallback: **`templates/core/assets/images/<name>/`** (this directory)

Build context is always the **repository root** (for **`COPY assets/entrypoint.sh`** where used).

**Bundling and licensing:** **[docs/packages/templates-core-assets.md](../../../../docs/packages/templates-core-assets.md)**.
