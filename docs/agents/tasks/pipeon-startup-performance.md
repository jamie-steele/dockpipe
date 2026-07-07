# TASK-012 Pipeon Startup And Provisioning Performance

## Goal

Make Pipeon open quickly from a normal developer machine while preserving the current local-first,
offline-capable stack.

Pipeon startup should prefer already-materialized state: repo bind mounts, stable package state under
`bin/.dockpipe`, cached Go/npm/Cargo outputs, existing Docker images, existing containers when safe,
and already-pulled Ollama models.

## Current Startup Cost Centers

- Branded `dockpipe-code-server:latest` image refresh: packaging VSIX files and rebuilding the image
  is correct but expensive when extension inputs changed.
- DorkPipe stack image refresh: Linux `dockpipe`, `dorkpipe`, and `mcpd` binaries are built for the
  container image and copied into a Docker build context.
- Docker image availability: first launch pays base image pulls and layer extraction.
- Ollama model provisioning: model pulls dominate startup when the model is absent or Docker volume
  state was reset.
- Signature checks: recursive source hashing is safer than mtime-only checks, but it is still a
  startup tax on large extension/source trees.
- Host bridge setup: MCP bridge restart is cheap, but stale allowlists must be detected before reuse.
- Code-server container setup: bind mounting is the right workspace model; copying the repo into a
  volume should not be part of normal Pipeon startup.

## Immediate Optimizations

- Skip `ollama pull <model>` when `ollama list` already shows the requested model.
- Keep non-interactive Windows PowerShell calls hidden so startup does not flash transient consoles.
- Reuse the host MCP bridge only when its tool catalog exposes the required tools for the current
  Pipeon build.
- Keep Go/npm/Cargo caches under package/build state, not global `.gocache` or `.gotemp` paths.
- Keep the workspace bind-mounted so repeated launches reuse the same repo and package state.

## Prebuilt Image Strategy

The biggest release-time win is prebuilding common Pipeon images instead of building them on the
developer machine during first launch.

Candidate images:

- `dockpipe-code-server:<version>` with Pipeon and DockPipe language-support VSIX files already
  installed.
- `dockpipe-dorkpipe-stack:<version>-linux-amd64` with Linux `dockpipe`, `dorkpipe`, and `mcpd`
  binaries already present.
- Optional GPU-aware stack variants only if the runtime contract really differs. Prefer one image
  with compose/runtime GPU toggles when possible.
- Optional base/runtime variants for common host configs only after measuring actual demand.

Release flow:

- Build images from exact package inputs and versioned tool binaries.
- Tag by DockPipe version and content digest.
- Publish image metadata with expected package/version/signature.
- Let `pipeon-dev-stack` prefer matching prebuilt images when available.
- Fall back to local source builds when offline, unpublished, or running dirty development inputs.
- Keep local build mode available for open-source/offline users and maintainers.

This likely needs its own small release/helper application or package command that can generate and
publish the image matrix as new versions come out. Treat it like Docker layer caching plus package
release automation, not as ad hoc launch-script logic.

## Measurement Plan

Capture coarse timing around:

- code-server image signature check
- Pipeon VSIX packaging
- code-server image build or reuse
- Linux tool build or reuse
- DorkPipe stack image build or reuse
- Docker compose up
- MCP readiness
- host MCP bridge readiness
- Ollama readiness
- model pull or cached model check
- code-server container readiness
- desktop shell open

The launch script should emit enough timing to explain slow starts without forcing users to inspect
Docker logs manually.

## Open Questions

- Which image variants are worth publishing for the first release: CPU-only, NVIDIA GPU, or one
  runtime-configurable image?
- Should image selection be automatic by version/signature or explicit by `PIPEON_DEV_STACK_IMAGE_*`
  overrides?
- Where should image metadata live: package catalog, release manifest, or generated package state?
- How should offline installs seed images: tarball import, local registry, or documented `docker pull`
  cache warming?
- What is the acceptable first-launch target on Windows with Docker Desktop already running?

## Current Status

Started. Launch now skips cached Ollama model pulls and hides non-interactive Windows PowerShell
startup calls. Prebuilt image generation is documented here as the larger release-engineering item.
