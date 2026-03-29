# package-store-infra

Host-only workflow: **`dockpipe build`** → **`dockpipe package build store`** → print **`release/artifacts/`** so you can inspect the same tarball + manifest layout you would publish for a self-hosted package origin (e.g. Cloudflare R2 behind HTTPS).

## Run

```bash
./src/bin/dockpipe --workflow package-store-infra --workdir . --
```

Optional:

- **`PACKAGE_STORE_OUT`** — override output directory (default `release/artifacts`).
- **`DOCKPIPE_BIN`** — path to `dockpipe` if not using `./src/bin/dockpipe` or `PATH`.

## Next step

Upload in two steps: **`dockpipe.cloudflare.r2infra`** (Terraform) then **`dockpipe.cloudflare.r2upload`** (objects). See `packages/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2publish/README.md` (Terraform module path keeps the historical folder name).

Host script for this workflow: **`workflows/package-store-infra/package-store-setup.sh`** (next to **`config.yml`** — not under repo-root **`scripts/dockpipe/`**).
