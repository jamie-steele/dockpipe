# Docker Image Artifacts

DockPipe treats Docker images as build artifacts, not just accidental side
effects of `run`.

The goal is simple: if a valid image already exists, do not rebuild it.

## Lifecycle

| Phase | What happens |
|-------|--------------|
| `dockpipe package compile` | Emits planned image artifact manifests. Does not run Docker builds. |
| `dockpipe build` | Compiles packages, then materializes Dockerfile-backed image artifacts by default. |
| `dockpipe run` | Reuses valid materialized/cached images, rebuilds missing/stale build artifacts, or pulls registry images only when policy allows. |
| `dockpipe package images` | Shows the merged diagnostic view. |

## Artifact States

| State | Meaning |
|-------|---------|
| `planned` | Compile selected a Dockerfile-backed image, but it has not been built/verified. |
| `materialized` | DockPipe built or verified a local Docker image for this artifact. |
| `referenced` | Metadata points at an OCI image ref; layers live in Docker/OCI. |
| `cached` | A registry-backed image has been pulled/verified and recorded locally. |

`dockpipe package images` also reports diagnostic statuses such as `ready`,
`missing`, `stale`, `planned`, `referenced`, and `docker-error`.

## Dockerfile-Backed Images

Runtime/resolver profiles can select Dockerfile-backed images. Compile records
the planned artifact with:

- image ref
- Dockerfile path
- build context path
- source fingerprint
- artifact fingerprint
- security manifest fingerprint
- runtime/resolver/package provenance

`dockpipe build` materializes those artifacts and writes receipts under:

```text
bin/.dockpipe/internal/images/by-fingerprint/
```

`dockpipe run` checks that fingerprint index before falling back to Docker daemon
state. If the receipt is valid and Docker still has the image, run skips build.

## Registry Images

Package metadata may point at a normal OCI image:

```yaml
image:
  source: registry
  ref: ghcr.io/acme/tool@sha256:0123456789abcdef...
  pull_policy: if-missing
```

Compile folds that reference into the effective runtime/image manifests. Run may
pull only when:

- the image is missing locally
- `pull_policy` allows it
- the compiled network policy allows ordinary outbound access

Digest-pinned refs are preferred. Mutable tags are hints, not proof of validity.

## Validity And Staleness

Image validity is not based only on tag names.

For Dockerfile-backed images, the fingerprint should cover:

- Dockerfile content
- build context content
- build args
- target stage
- platform
- base image digest when available
- runtime/resolver/package provenance
- builder schema version when needed

Runtime-only security restrictions should not force an image rebuild unless they
change how the image is built or selected. DockPipe keeps the image fingerprint
and `security_manifest_fingerprint` separate for that reason.

## Inspecting Images

```bash
dockpipe package images
```

Example columns:

```text
fingerprint  status  state  source  image_ref  workflow  package  step_id  image_key
```

Status meanings:

- `ready`: receipt exists and Docker has the image
- `missing`: receipt exists, but Docker image is gone
- `stale`: compiled planned artifact no longer matches the receipt
- `planned`: compile selected the artifact, but it is not materialized
- `referenced`: registry image reference
- `docker-error`: Docker check failed, but inspection remains non-fatal

## What Is Not In V1

Keep these out of the first stable artifact model:

- Docker layer packaging inside DockPipe package tarballs
- custom Docker build DSL
- registry auth manager
- multi-arch build orchestration
- SBOM/attestations
- proxy-mediated `docker pull`
- automatic online base image refresh during normal run

Those are useful later, but they would blur the simple model: install/publish are
the network-facing operations; run consumes local artifacts unless the workflow
itself needs network.
