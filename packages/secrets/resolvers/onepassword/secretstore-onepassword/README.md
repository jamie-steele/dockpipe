# Workflow: `secretstore-onepassword`

Maintainer-only **`workflow_type: secretstore`** flow using the **1Password CLI** (`op run --env-file`). Same shape as bundled **`secretstore`** (dotenv file) but **`resolver: onepassword`** and **`scripts/onepassword/secretstore-op-exec.sh`** from this package.

1. Install [1Password CLI](https://developer.1password.com/docs/cli/) and sign in.
2. Copy **`packages/secrets/resolvers/onepassword/.env.op.template.example`** to **`.env.op.template`** in your **`--workdir`** and set **`op://`** references.
3. Run with **`SECRETSTORE_COMMAND`** set (or pass after **`--`** per **`secretstore`** docs).

```bash
export SECRETSTORE_COMMAND='./src/bin/dockpipe --workflow dockpipe.cloudflare.r2publish --workdir . --'
dockpipe --workflow secretstore-onepassword --workdir . --
```
