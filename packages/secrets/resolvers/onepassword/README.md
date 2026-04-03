# Resolver profile: `onepassword`

Host-only workflows that use the **1Password CLI** (`op`). Pair with **`runtime: dockerimage`** and **`skip_container: true`**.

This resolver is **not** part of the lean **`src/core/resolvers/`** bundle — it lives under **`.staging/workflows/dockpipe/packages/secrets/resolvers/onepassword/`** (this **`profile`** file) for maintainer authoring and is consumed via **`dockpipe package compile resolvers`** (→ **`packages/resolvers/onepassword/`**) or resolved from disk in a dockpipe checkout (**staging** is searched before lean core).

It documents **`DOCKPIPE_RESOLVER_CMD`** and **`DOCKPIPE_RESOLVER_ENV`** for keys you inject via **`op inject`** / **`op run`**. See **`secretstore-onepassword/`** (workflow), **`.env.op.template.example`** (vault mapping template), and **`.staging/workflows/dockpipe/storage/cloudflare/r2/secretstore-r2-publish-test/`**.
