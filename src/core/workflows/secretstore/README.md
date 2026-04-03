# secretstore workflow

**`workflow_type: secretstore`** marks this as a secret-injection wrapper. DockPipe does not treat it specially at runtime; the type is for classification, generators, and your own tooling.

**Model:** **`runtime: dockerimage`** with **`skip_container: true`** on the step (host). **`resolver: dotenv`** documents **`SECRETSTORE_ENV_FILE`** / **`SECRETSTORE_COMMAND`**; the bundled script **`scripts/dockpipe/secretstore-exec.sh`** loads a **dotenv-style** file (POSIX **`set -a`** + **`source`**) — no third-party CLI.

## Default: plain env file

1. Copy **`.env.secretstore.example`** → **`.env.secretstore`** in your project (gitignored; use real values locally).
2. Set **`SECRETSTORE_COMMAND`** to the shell command to run with those variables (often another `dockpipe` invocation).

Example:

```bash
export SECRETSTORE_COMMAND='./src/bin/dockpipe --workflow mywf --workdir . --'
dockpipe --workflow secretstore --workdir . --
```

Or **`--var SECRETSTORE_COMMAND=...`**.

## 1Password CLI (`op`)

Not bundled in core. In this repository use **`--workflow secretstore-onepassword`** (see **`.staging/workflows/dockpipe/packages/secrets/resolvers/onepassword/secretstore-onepassword/README.md`**) and copy **`.staging/workflows/dockpipe/packages/secrets/resolvers/onepassword/.env.op.template.example`** to **`.env.op.template`**.

## Adding another vault

Add a **resolver profile** under **`.staging/workflows/…`** or **`templates/core/resolvers/`**, and a **host script** beside it or under **`assets/scripts/`** — keep **`workflow_type: secretstore`** in YAML so downstream tools stay generic.
