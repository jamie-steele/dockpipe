# package-store-publish

Repo-local maintainer workflow for the self-hosted package mirror.

What it does:

1. builds the repo-local `dockpipe` binary
2. runs `dockpipe build --no-images`
3. writes `release/artifacts/install-manifest.json` plus `templates-core-<version>.tar.gz`
4. writes `release/artifacts/packages-store-manifest.json` plus one tarball per compiled workflow and resolver
5. uploads each artifact file individually with `dockpipe release upload`

This is the correct mirror shape for `dockpipe install core` and store-backed workflow/resolver pulls. It does **not** tar the whole `release/artifacts/` directory into one object.

## Secrets

The workflow sets `vault: op`, so DockPipe runs `op inject` before the host steps.

Typical `.env.vault.template` entries:

```dotenv
R2_BUCKET=dockpipe
CLOUDFLARE_ACCOUNT_ID=op://DockPipe/CLOUDFLARE/url
AWS_ACCESS_KEY_ID=op://DockPipe/CLOUDFLARE/accesskeyid
AWS_SECRET_ACCESS_KEY=op://DockPipe/CLOUDFLARE/secretaccesskey
```

Optional:

```dotenv
R2_PREFIX=packages/
R2_ENDPOINT_URL=op://DockPipe/CLOUDFLARE/r2endpoint
AWS_REGION=auto
```

## Run

Dry-run is the default:

```bash
./src/bin/dockpipe --workflow package-store-publish --workdir . --
```

On a machine without the AWS CLI, DockPipe will prompt to install it from the workflow dependency definition before continuing.

Real upload:

```bash
./src/bin/dockpipe --workflow package-store-publish --workdir . --var R2_PUBLISH_DRY_RUN=0 --
```

Build only:

```bash
./src/bin/dockpipe --workflow package-store-publish --workdir . --var PACKAGE_RELEASE_SKIP_UPLOAD=1 --
```

If you still need bucket/domain provisioning, run `package-store-infra` separately first.
