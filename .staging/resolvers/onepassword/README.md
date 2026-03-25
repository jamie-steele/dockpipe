# Resolver profile: `onepassword`

Host-only workflows that use the **1Password CLI** (`op`). Pair with **`runtime: cli`** and **`skip_container: true`**.

This resolver does **not** run an isolate image; it documents **`DOCKPIPE_RESOLVER_ENV`** hints for tokens you typically inject via `op inject` (see **`.staging/workflows/secretstore-r2-publish-test`** in a dockpipe checkout).
