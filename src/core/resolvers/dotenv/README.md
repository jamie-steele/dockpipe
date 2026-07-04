# Resolver profile: `dotenv`

Bundled **vendor-neutral** profile for **`workflow_type: secretstore`**: load variables from a **`SECRETSTORE_ENV_FILE`** (default **`.env.secretstore`**) using POSIX shell **`set -a`** + **`source`**, then run **`SECRETSTORE_COMMAND`**.

For **1Password CLI** (`op run` / `op inject`), use the maintainer workflow **`secretstore-onepassword`** and resolver **`onepassword`** under **`packages/secrets/resolvers/onepassword/`** (this repo) or **`dockpipe package compile resolvers`**.
