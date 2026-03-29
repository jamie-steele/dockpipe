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

Publish the folder (or a tarball of it) to your static origin, then point installs at that base URL. In this repo the packaged R2 flow is **`dockpipe.cloudflare.r2publish`** (see `packages/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2publish/README.md`).
