# Core resolvers (lean)

**`templates/core/resolvers/`** in the shipped tree holds **`example/`** and **`dotenv/`** (vendor-neutral **`workflow_type: secretstore`**). **Vendor-specific profiles** (e.g. **`onepassword`**) are **maintainer-authored** under **`.staging/workflows/dockpipe/packages/*/resolvers/<name>/`** in this repo (or materialized by **`dockpipe package compile resolvers`**). The runner resolves **`--resolver`** and **`scripts/…`** against **staging workflows** first, then lean **`templates/core/resolvers/`**.

See **`docs/architecture-model.md`** and **`docs/templates-core-assets.md`**.
