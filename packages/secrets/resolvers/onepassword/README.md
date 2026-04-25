# Resolver profile: `onepassword`

Host-only workflows that use the **1Password CLI** (`op`). Pair with **`runtime: dockerimage`** and **`kind: host`**.

This resolver is **not** part of the lean **`src/core/resolvers/`** bundle. It lives under **`packages/secrets/resolvers/onepassword/`** in this repo and is consumed via **`dockpipe package compile resolvers`** or resolved from declared compile roots in a dockpipe checkout.

It documents **`DOCKPIPE_RESOLVER_CMD`** and **`DOCKPIPE_RESOLVER_ENV`** for keys you inject via **`op inject`** / **`op run`**. See **`secretstore-onepassword/`** (workflow), **`.env.op.template.example`** (vault mapping template), and **`packages/cloud/storage/resolvers/r2/secretstore-r2-publish-test/`**.
