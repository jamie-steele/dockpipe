# Core resolvers (lean)

**`templates/core/resolvers/`** in the shipped tree holds **only** the **`example/`** reference layout. **Other profiles** (including **`onepassword`** for **`workflow_type: secretstore`** flows) are **maintainer-authored** under **`.staging/resolvers/`** in this repo (or materialized by **`dockpipe package compile resolvers`**). The runner resolves **`--resolver`** and **`scripts/…`** against **staging** first, then lean **`templates/core/resolvers/`**.

See **`docs/architecture-model.md`** and **`docs/templates-core-assets.md`**.
