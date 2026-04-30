# DorkPipe resolver package — `assets/scripts/`

**Canonical authoring path** for maintainer shell in this checkout:

**`packages/dorkpipe/resolvers/dorkpipe/assets/scripts/`**

Workflow YAML still uses the **logical** path **`scripts/dorkpipe/<file>`** (same as downstream projects). **`paths.go`** resolves that to **this** resolver **`assets/scripts/`** directory (after **`repo/scripts/`** overrides only — there is **no** **`src/scripts/dockpipe/`**). There is **no** repo-root **`scripts/dockpipe`** tree — do not add a symlink; the engine maps **`scripts/dorkpipe/…`** to this package.

**Bundled** copies for **`dockpipe init`** may also live under **`templates/core/bundles/dorkpipe/`**.

Notable entrypoints:

- **`dev-stack.sh`** — local Postgres + Ollama helper; now GPU-aware for the Ollama container when Docker can expose NVIDIA
- **`dev-stack-lib.sh`** — shared helper functions for GPU detection, prompt/remediation flow, and compose override wiring
