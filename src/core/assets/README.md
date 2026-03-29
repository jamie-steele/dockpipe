# Core assets (`templates/core/assets/`)

Reusable **support files** for workflows, resolvers, runtimes, and strategies — **not** new architectural primitives.

| File / subfolder | Contents |
|------------------|----------|
| **`env.vault.template.example`** | Example **`op://`** env template; **init** also copies it to the project root as **`.env.vault.template.example`**. |
| **`scripts/`** | **Agnostic** helpers at this folder root only. **Domain** bundles (**`dorkpipe/`**, **`pipeon/`**, …) live under **`../bundles/`** (downstream); maintainer DorkPipe/Pipeon workflows live under **`dockpipe.config.json`** compile roots, not under **`src/core/workflows/`**. **This repo** also keeps **`review-pipeline`** under repo-root **`workflows/review-pipeline/`**. Resolver-only host scripts live under **`../resolvers/<name>/`**. See **`scripts/README.md`**, **`bundles/README.md`**. |
| **`images/`** | Dockerfiles for **`TemplateBuild`** / **`--isolate`**. |
| **`compose/`** | Optional **Compose** examples for richer multi-service setups (see **`compose/README.md`**). |

Merged into user projects by **`dockpipe init`**. Policy: **[docs/templates-core-assets.md](../../../docs/templates-core-assets.md)**. **Compose** is optional example assets only — not a runtime or resolver.
