# Shared resolver profiles (`templates/core/resolvers/`)

**Normative architecture** (workflow, runtime, resolver, strategy, **`runtime.type`**, invariants): **[docs/architecture-model.md](../../docs/architecture-model.md)**. This README is **mechanical**: file format and keys the Go runner reads. It does **not** redefine those concepts.

Each profile is **`templates/core/resolvers/<name>`** (flat **`KEY=value`** file) or **`templates/core/resolvers/<name>/profile`**, optionally with **`config.yml`** (delegate workflow for **`DOCKPIPE_*_WORKFLOW`**). Selected by resolver profile **name** (CLI **`--runtime`** / **`--resolver`**). The runner merges **`DOCKPIPE_RUNTIME_*`** then **`DOCKPIPE_RESOLVER_*`** (see **`domain.FromResolverMap`**); other lines are comments (`#`). See **[docs/isolation-layer.md](../../docs/isolation-layer.md)** and **[docs/runtime-architecture.md](../../docs/runtime-architecture.md)**.

## Contract (runner)

| Key | Required | Meaning |
|-----|----------|---------|
| **`DOCKPIPE_RUNTIME_TYPE`** | Recommended | **`runtime.type`**: **`execution`** \| **`ide`** \| **`agent`** — classification only (see **`domain/runtime_kind.go`**). |
| **`DOCKPIPE_RESOLVER_TEMPLATE`** | Usually yes | Built-in template name passed to **`TemplateBuild`** → Docker image. **Omit** when **`DOCKPIPE_RESOLVER_WORKFLOW`** or **`DOCKPIPE_RESOLVER_HOST_ISOLATE`** is set. |
| **`DOCKPIPE_RESOLVER_WORKFLOW`** | no | Bundled delegate YAML under **`templates/core/resolvers/<name>/config.yml`** (or **`dockpipe/core/resolvers/<name>/config.yml`** when materialized). **Mutually exclusive** with **`DOCKPIPE_RESOLVER_HOST_ISOLATE`**. |
| **`DOCKPIPE_RESOLVER_HOST_ISOLATE`** | no | Repo-relative host script instead of **`docker run`**. |
| **`DOCKPIPE_RESOLVER_PRE_SCRIPT`** | no | Host script when using **`--resolver`** *without* **`--workflow`** (defaults from workflow **`run`** otherwise). |
| **`DOCKPIPE_RESOLVER_ACTION`** | no | Act script for resolver-only runs; **`--workflow`** uses **`config.yml`** **`act`**. |
| **`DOCKPIPE_RESOLVER_CMD`** | no | Default CLI name for documentation; **not** executed by dockpipe. |
| **`DOCKPIPE_RESOLVER_ENV`** | no | Comma-separated env var names (documentation). |
| **`DOCKPIPE_RESOLVER_EXPERIMENTAL`** | no | Set to **`1`** for experimental warning on stderr. |

**Lifecycle:** **`run`** → **isolate** (per keys above) → **`act`**.

Adding a profile = add **`templates/core/resolvers/<name>`** (file or **`profile`**) and optionally list the name under **`runtimes:`** in workflow **`config.yml`** as an allowlist.

**Multi-step workflows** can set **`runtime:`** / **`resolver:`** on **`steps:`**. **`isolate:`** on a step can override the template from the profile. Async steps (`is_blocking: false`) cannot use profiles that define **`DOCKPIPE_RESOLVER_HOST_ISOLATE`**.
