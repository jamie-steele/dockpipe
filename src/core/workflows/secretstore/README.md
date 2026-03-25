# secretstore workflow

**`workflow_type: secretstore`** marks this as a secret-injection wrapper. DockPipe does not treat it specially at runtime; the type is for classification, generators, and your own tooling.

**Model:** **`runtime: keystore`** is the substrate (host + secret-store merge). **`resolver: onepassword`** is the 1Password CLI adapter (`DOCKPIPE_RESOLVER_*` in **`templates/core/resolvers/onepassword/profile`**). Other vaults belong in **new resolver profiles**, not new runtimes.

## Default provider: 1Password CLI (`op`)

1. Install [1Password CLI](https://developer.1password.com/docs/cli/) and sign in.
2. Copy **`.env.op.template.example`** → **`.env.op.template`** in your project and replace placeholders with **`op://`** references.
3. Set **`SECRETSTORE_COMMAND`** to the shell command to run with those variables visible (often another `dockpipe` invocation).

Example:

```bash
export SECRETSTORE_COMMAND='./src/bin/dockpipe --workflow r2-publish --workdir . --'
dockpipe --workflow secretstore --workdir . --
```

Or use **`--var SECRETSTORE_COMMAND=...`**.

## Adding a provider

Edit **`scripts/dockpipe/secretstore-exec.sh`**: add a branch for `SECRETSTORE_PROVIDER` (e.g. `vault`, `doppler`). Keep **`workflow_type: secretstore`** in YAML so downstream tools stay generic.
