# Resolver profile: `onepassword`

Host-only workflows that use the **1Password CLI** (`op`). Pair with **`runtime: keystore`** (preferred) or **`runtime: cli`** and **`skip_container: true`**.

This resolver is **not** part of the lean **`src/core/resolvers/`** bundle — it lives under **`.staging/resolvers/onepassword/`** for maintainer authoring and is consumed via **`dockpipe package compile resolvers`** (→ **`.dockpipe/internal/packages/core/resolvers/onepassword/`**) or resolved from disk in a dockpipe checkout (**staging** is searched before lean core).

It documents **`DOCKPIPE_RESOLVER_CMD`** and **`DOCKPIPE_RESOLVER_ENV`** for keys you inject via **`op inject`** / **`op run`**. See **`src/core/workflows/secretstore/`** and **`.staging/workflows/secretstore-r2-publish-test/`**.
