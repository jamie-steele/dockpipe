# Workflow: `secretstore-onepassword`

Maintainer-only **`workflow_type: secretstore`** flow using the **1Password CLI** (`op run --env-file`). Same shape as bundled **`secretstore`** (dotenv file) but **`resolver: onepassword`** and **`scripts/dockpipe/secretstore-op-exec.sh`** (resolved from **`.staging/workflows/dockpipe/assets/scripts/`**).

1. Install [1Password CLI](https://developer.1password.com/docs/cli/) and sign in.
2. Copy **`.env.op.template.example`** from **`.staging/workflows/dockpipe/packages/secrets/resolvers/onepassword/`** (this resolver tree) to **`.env.op.template`** in your **`--workdir`** and set **`op://`** references.
3. Run with **`SECRETSTORE_COMMAND`** set (or pass after **`--`** per **`secretstore`** docs).

```bash
export SECRETSTORE_COMMAND='./src/bin/dockpipe --workflow dockpipe.cloudflare.r2publish --workdir . --'
dockpipe --workflow secretstore-onepassword --workdir . --
```
