# Package Quickstart

You do not need packages to use DockPipe. Source workflows under `workflows/`
are the low-friction authoring path.

Use package/build commands when you want reusable, inspectable artifacts:

1. **Source mode** — edit `workflows/<name>/config.yml` and run it directly.
2. **Compile** — validate and emit package/runtime/security/image manifests.
3. **Build** — compile, then materialize Dockerfile-backed image artifacts.
4. **Run** — consume local artifacts; rebuild/pull only when policy allows and
   the artifact is missing or stale.

For the full store layout, see [package-model.md](package-model.md).

## Compile A Workflow Package

```bash
dockpipe package compile workflow workflows/test --workdir .
```

Compiled workflow tarballs land under:

```text
bin/.dockpipe/internal/packages/workflows/
```

Compile emits planned runtime/security/image manifests, but it does not run
Docker builds.

## Build Local Artifacts

For contributors working from a source checkout:

```bash
make build
dockpipe package build
dockpipe package test
```

- `make build` rebuilds DockPipe core plus the DockPipe Launcher.
- `dockpipe package build` runs package-owned source builds for packages in this checkout that declare `build.source.script`.
- `dockpipe package test` runs package-owned tests for packages in this checkout that declare `test.script`.

If your shell `dockpipe` points at an older install, contributors can optionally run `make dev-install` to update their local PATH binary. That is a source-checkout convenience, not part of the normal package/user flow.

```bash
dockpipe build
```

`dockpipe build` runs `compile all --force`, then prebuilds Dockerfile-backed
image artifacts by default. When a source package declares `build.source.script`
in `package.yml`, `dockpipe build` also runs that package-owned authoring-tree
build hook before image materialization. Materialized image receipts land under:

```text
bin/.dockpipe/internal/images/by-fingerprint/
```

Use this when you want later runs to skip rebuilding valid images:

```bash
dockpipe --workflow test --
```

If you only want manifests/package materialization:

```bash
dockpipe build --no-images
```

To run only package-owned source builds without compiling images:

```bash
dockpipe package build
```

To run all DockPipe-owned package and workflow tests in the current project:

```bash
dockpipe test
```

Package authors can also run the narrower forms:

```bash
dockpipe package test
dockpipe workflow test
```

For the explicit subtarget form:

```bash
dockpipe package build source
```

## Inspect Packages And Images

```bash
dockpipe package list
dockpipe package images
```

`dockpipe package images` merges planned image artifacts from compiled workflow
tarballs with materialized/cached fingerprint receipts.

Status meanings:

| Status | Meaning |
|--------|---------|
| `ready` | Receipt exists and Docker has the image. |
| `missing` | Receipt exists, but the Docker image is gone. |
| `stale` | Compiled planned artifact no longer matches the receipt. |
| `planned` | Compile selected an image artifact, but it is not materialized. |
| `referenced` | Registry image reference; layers live in Docker/OCI. |
| `docker-error` | Docker check failed; the inspect command stays non-fatal. |

## Install/Publish Network Boundary

Normal runs should not need network after artifacts are local unless the workflow
itself needs network. Network-facing operations should be explicit:

- `dockpipe install core`
- future package install commands
- `dockpipe release upload`
- workflow commands that intentionally call APIs or registries

Package metadata can point at a normal OCI image reference, but compile folds
that into runtime/image manifests. Run then reuses local images, pulls only when
the compiled pull policy and network policy allow it, or fails clearly.

## What Stays Advanced

The package model also covers global installs, published tarballs, namespace
resolution, dependency hints, and exact store layout. Keep those details in
[package-model.md](package-model.md) unless you are building package tooling.

## Package Author Safety

When a package script may change the user’s machine or local tool state, prefer
an explicit DockPipe prompt before doing it.

Examples:

- installing software or host dependencies
- changing Docker or other daemon configuration
- restarting services
- updating WSL / Docker Desktop prerequisites
- writing local credential state such as `docker login`

Use the SDK prompt primitive and classify the prompt with metadata so the
launcher can present it clearly and automation can bypass it deliberately:

```bash
dockpipe_sdk prompt confirm \
  --id enable_gpu_setup \
  --title "Allow Docker GPU Setup?" \
  --message "DockPipe will install GPU container support, update Docker config, and restart Docker. Continue?" \
  --default no \
  --intent host-mutation \
  --automation-group system-changes \
  --allow-auto-approve \
  --auto-approve-value yes
```

Guidance:

- prompt before the mutation, not after detection
- keep the message explicit about what will change
- use `--intent` / `--automation-group` metadata for transparency
- only opt into `--allow-auto-approve` when the workflow is safe to run under explicit automation approval such as `dockpipe --yes`

DockPipe can provide the prompt path and the automation override, but package
authors still need to choose to use it in the flows they own.

## Package Author Test Hooks

If a package needs source-checkout or CI tests, declare them in `package.yml`
instead of teaching repo scripts or CI to know the package path:

```yaml
test:
  script: tests/run.sh
```

Then contributors and CI can run:

```bash
dockpipe package test
```

Use workflow-local `tests/run.sh` when you want a workflow tree under
`workflows/<name>/` to carry its own tests and participate in:

```bash
dockpipe workflow test
dockpipe test
```
