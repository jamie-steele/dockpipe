# Core resolvers (lean)

**`templates/core/resolvers/`** in the shipped tree holds **`example/`** and **`dotenv/`** (vendor-neutral **`workflow_type: secretstore`**). **Vendor-specific profiles** (e.g. **`onepassword`**) are maintainer-authored in package trees and materialized by **`dockpipe package compile resolvers`**. The runner resolves **`--resolver`** and **`scripts/…`** against compiled/package-provided resolvers first, then lean **`templates/core/resolvers/`**.

See **`docs/concepts/architecture-model.md`** and **`docs/packages/templates-core-assets.md`**.
