# Resolver profile: `onepassword`

Host-only workflows that use the **1Password CLI** (`op`). Pair with **`runtime: keystore`** (preferred) or **`runtime: cli`** and **`skip_container: true`**.

This resolver does **not** run an isolate image; it documents **`DOCKPIPE_RESOLVER_CMD`** and **`DOCKPIPE_RESOLVER_ENV`** hints for keys you inject via `op inject` / `op run`. See **`src/core/workflows/secretstore/`** and **`.staging/workflows/secretstore-r2-publish-test/`** in this repo.

Fuller notes also live under **`.staging/resolvers/onepassword/`** when present; the runner checks staging first, then this lean tree.
