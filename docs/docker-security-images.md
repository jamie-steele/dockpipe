# Docker Security Policy and Image Artifacts

Status: proposed maintainer design for DockPipe engine work.

This document describes two linked enhancements:

1. A compiled Docker/container security policy model declared in workflow YAML and enforced by DockPipe.
2. A compiled Docker image artifact model so `run` can reuse valid images instead of rebuilding on every execution.

The goal is to keep DockPipe as the central enforcement and artifact-resolution layer. Higher-level tools such as DorkPipe and Pipeon should inherit the same behavior by consuming DockPipe manifests and run records rather than reimplementing their own container policy logic.

## Why these features belong together

Both concerns live at the same boundary:

- DockPipe already owns compile/materialization.
- DockPipe already owns `docker build` / `docker run`.
- DockPipe already has a package/artifact model under `.dockpipe/internal/packages/`.

So the right shape is:

1. Author a higher-level intent in workflow YAML.
2. Compile that intent into an effective runtime/security manifest and an image artifact manifest.
3. Execute strictly from those compiled manifests.

That keeps the system inspectable, cacheable, and explainable.

## Design principles

- Secure defaults first.
- Workflow authors express intent, not raw Docker flags.
- Compile resolves defaults, presets, and overrides into one effective manifest.
- Run consumes the compiled manifest and does not reinterpret policy ad hoc.
- Image validity is based on fingerprints and digests, not tags alone.
- Rebuild only when inputs actually changed.
- If a rule is advisory or partially enforced, DockPipe must say so clearly.

## High-level architecture

### Authoring layer

Workflows gain a higher-level `runtime.security` and `runtime.image` section.

The public surface stays product-shaped:

- `security.profile: secure-default | internet-client | build-online | sidecar-client`
- `network.mode: offline | allowlist | restricted | internet`
- `filesystem.root: readonly | writable`
- `process.user: auto | non-root | root`
- `image.source: auto | build | registry`

DockPipe maps those settings to Docker-specific flags internally.

### Compile layer

Compile merges:

1. Engine defaults
2. Named preset/profile
3. Workflow-level overrides
4. Step-level overrides

Compile then:

- validates contradictions
- normalizes policy
- expands and stores domain/path patterns
- computes fingerprints
- emits inspectable manifests

Suggested outputs per compiled workflow/package:

- `runtime.effective.json`
- `image-artifact.json`
- `steps/<step-id>.runtime.effective.json`
- `steps/<step-id>.image-artifact.json`

These should live with other compiled workflow material under `.dockpipe/internal/`.

### Run layer

Run loads the compiled manifests and decides:

- is the image present?
- is the image still valid for this manifest?
- do we need to build or pull?
- which exact Docker restrictions must be applied?

That makes the fast path cheap:

- valid compiled manifest
- valid image artifact
- no rebuild
- launch immediately

Workflow package metadata may also point at a normal OCI image reference:

```yaml
image:
  source: registry
  ref: ghcr.io/acme/tool@sha256:...
  pull_policy: if-missing
```

Compile folds that package metadata into the effective image artifact manifest. Run then decides whether to reuse a valid local image, pull it when the compiled pull policy explicitly allows `if-missing`, or fail clearly when the image is missing and pulling is not allowed.

## YAML schema direction

This is intentionally higher-level than raw Docker flags:

```yaml
security:
  profile: sidecar-client
  network:
    mode: allowlist
    allow:
      - api.openai.com
      - "*.anthropic.com"
    block:
      - "*.facebook.com"
```

DockPipe compiles this into the effective runtime manifest. Public YAML selects the policy profile and desired restrictions; the compiled manifest records the actual enforcement mode (`native`, `proxy`, or `advisory`).

## Security policy model

### Network modes

- `offline`
  No outbound internet. Preserve Docker-internal behavior needed for container startup and local resolution.
- `allowlist`
  Only declared destinations may be reached.
- `restricted`
  Baseline-deny posture with curated allowances, suitable for common package/tool traffic.
- `internet`
  Full outbound access, still with filesystem/process hardening.

### Filesystem policy

- read-only root filesystem by default
- declared writable paths only
- workspace-only writes as a first-class mode
- explicit temp paths

### Process/runtime policy

- `no-new-privileges`
- drop capabilities by default
- PID and resource limits
- default non-root when compatible

### Explainability

Every compiled rule should have a stable rule id so run records can later answer:

- which rule was enforced
- what decision was made
- why an action was blocked

### Selective proxy path

DockPipe should not force every container onto a sidecar/proxy path.

- `offline` uses native Docker `--network none`
- `internet` uses normal Docker networking
- `allowlist` / `restricted` may compile as `advisory` or `proxy`

When a workflow compiles under a profile such as `sidecar-client`, DockPipe may derive `proxy` enforcement for `allowlist` / `restricted` modes and expect a proxy-backed egress layer at run time. This keeps the stronger path selective and lets higher-level tools such as DorkPipe reuse their existing sidecar/proxy patterns without making them part of every workflow.

Compose-managed stacks can feed this path cleanly through DockPipe-owned workflow env. For example, a prior `compose_up` step may export `DOCKPIPE_POLICY_PROXY_URL` via `compose.exports`, and the later container step will consume that run-local setting when applying the compiled runtime policy.

In the richer proxy-backed path, DockPipe turns that base proxy URL into a per-step effective proxy URL that carries the compiled network policy token. That keeps the public YAML high-level while still letting a DockPipe-aware proxy enforce the actual per-step `allow` / `block` rules instead of a static sidecar-wide allowlist.

First package consumer in this repository:

- `packages/dorkpipe/resolvers/dorkpipe-self-analysis-stack-proxy/`
- compose-managed support services remain package-owned
- DockPipe still owns lifecycle, env export, manifest compilation, and proxy env injection

## Image artifact model

Docker images should be treated as compiled artifacts, not just side effects of `run`.

Suggested manifest fields:

```json
{
  "schema": 1,
  "kind": "docker-image-artifact",
  "workflow_name": "codex-pav",
  "package_name": "dockpipe.workflow.codex-pav",
  "image_key": "resolver.codex",
  "source": "build",
  "fingerprint": "sha256:...",
  "image_ref": "dockpipe-codex:compiled-abc123",
  "image_id": "sha256:...",
  "repo_digest": "sha256:...",
  "security_manifest_fingerprint": "sha256:..."
}
```

## What contributes to image validity

Image validity should include more than the tag:

- Dockerfile content
- build context content set
- build args
- target stage
- platform
- base image digest when pinned or inspected
- builder schema version
- any security/runtime setting that materially changes the build result

Pure runtime-only restrictions should not force a rebuild unless they affect how the image itself is built or selected.

## Rebuild decision

Rebuild when:

- no artifact record exists
- artifact fingerprint differs
- recorded local image id is missing
- configured image source changed
- required digest or reference no longer matches policy

Do not rebuild merely because the tag is unqualified or old-looking.

## Local and remote coexistence

The model should support both:

- locally built images from workflow/resolver Dockerfiles
- registry-backed images referenced by digest
- workflow package metadata that points at a normal OCI image ref

Suggested rule:

- `source: build` means DockPipe owns the build provenance
- `source: registry` means DockPipe verifies and consumes a pulled image
- `source: auto` allows a future policy such as “prefer local artifact, otherwise pull/build”

## Logging and user-facing summaries

Compile examples:

- `compiled security preset secure-default -> readonly root, non-root, no-new-privileges, cap-drop ALL`
- `image artifact resolver.codex is stale: Dockerfile fingerprint changed`

Run examples:

- `runtime policy: network=restricted, root=readonly, tmpfs=/tmp, no-new-privileges, cap-drop=ALL, pids=256`
- `policy enforcement: network restricted is advisory in this build; full egress filtering is not active yet`
- `policy coverage: domain allow/block rules are compiled for inspection but are not enforced natively by Docker`
- `using cached image artifact resolver.codex`
- `rebuilding image artifact resolver.codex: local image missing`
- `blocked outbound request to example.com by network.allowlist rule network.allow[0]`

## Risks and edge cases

The two places most likely to get messy are:

1. Domain-level network enforcement
2. Over-eager image invalidation

Notes:

- Docker does not natively provide clean domain allow/block enforcement on its own.
- DockPipe must distinguish between native, proxy-backed, and advisory enforcement.
- The UI and logs should show the effective enforcement mode, not just the desired policy.
- Security settings that do not affect the image build should not poison the image cache key.

## Incremental implementation plan

1. Add typed internal manifest models for compiled runtime/security/image artifacts.
2. Compile effective manifests without changing runtime behavior yet.
3. Add Docker-native enforcement for filesystem and process restrictions.
4. Add image artifact fingerprinting and `build` support for first-party Dockerfile-backed profiles.
5. Teach `run` to reuse valid image artifacts and rebuild only when stale.
6. Add registry-backed image manifests and digest verification.
7. Add richer policy explanation and UI-facing summaries.

## Internal vs public surface

Public:

- presets
- network mode
- allow/block domain intent
- writable paths
- non-root / no-new-privileges / capability and resource limits
- image source and build intent

Internal:

- exact Docker flags
- low-level network plumbing
- cache key layout
- local tag naming conventions
- manifest storage layout inside `.dockpipe/internal/`

## Recommended first step

Start by compiling and logging effective manifests before enforcing every rule. That gives DockPipe:

- visibility
- testability
- future UI surfaces
- an incremental rollout path

without pretending enforcement exists before it actually does.
