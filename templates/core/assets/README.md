# Core assets (`templates/core/assets/`)

Reusable **support files** for workflows, resolvers, runtimes, and strategies — **not** new architectural primitives.

| Subfolder | Contents |
|-----------|----------|
| **`scripts/`** | **Agnostic** helpers at this folder root only. **Domain** bundles (**`dorkpipe/`**, **`pipeon/`**, **`review-pipeline/`**) live under **`../bundles/`**. Resolver-only host scripts live under **`../resolvers/<name>/`**. See **`scripts/README.md`**, **`bundles/README.md`**. |
| **`images/`** | Dockerfiles for **`TemplateBuild`** / **`--isolate`**. |
| **`compose/`** | Optional **Compose** examples for richer multi-service setups (see **`compose/README.md`**). |

Merged into user projects by **`dockpipe init`**. Policy: **[docs/templates-core-assets.md](../../../docs/templates-core-assets.md)**. **Compose** is optional example assets only — not a runtime or resolver.
