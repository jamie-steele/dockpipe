# DorkPipe resolver package — `assets/scripts/`

**Canonical authoring path** for package-owned shell in this checkout:

**`packages/dorkpipe/resolvers/dorkpipe/assets/scripts/`**

Workflow YAML still uses the **logical** path **`scripts/dorkpipe/<file>`** (same as downstream projects). **`paths.go`** resolves that to **this** resolver **`assets/scripts/`** directory (after **`repo/scripts/`** overrides only — there is **no** **`src/scripts/dockpipe/`**). There is **no** repo-root **`scripts/dockpipe`** tree — do not add a symlink; the engine maps **`scripts/dorkpipe/…`** to this package.

**Bundled** copies for **`dockpipe init`** may also live under **`templates/core/bundles/dorkpipe/`**.

Notable entrypoints:

- **`dev-stack.sh`** — local Postgres + Ollama helper; defaults to packaged consumer assets and supports explicit `DORKPIPE_DEV_STACK_BUNDLE_MODE=checkout` for maintainer binaries
- **`dev-stack-lib.sh`** — shared helper functions for GPU policy, prompt/remediation flow, service readiness checks, and compose override wiring
