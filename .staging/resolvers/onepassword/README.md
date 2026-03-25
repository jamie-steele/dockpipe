# Resolver profile: `onepassword`

Host-only workflows that use the **1Password CLI** (`op`). Pair with **`runtime: keystore`** (preferred for secret-injection flows) or **`runtime: cli`**, and **`skip_container: true`**.

Lean **`templates/core/resolvers/onepassword/profile`** ships with **`dockpipe init`**; this staging copy stays in sync for maintainer workflows.

This resolver does **not** run an isolate image; it documents **`DOCKPIPE_RESOLVER_ENV`** hints for tokens you typically inject via `op inject` (see **`.staging/workflows/secretstore-r2-publish-test`** in a dockpipe checkout).
