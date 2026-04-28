# Workflow Authoring Quickstart

Use a workflow when a command becomes repeatable. A workflow is a
`workflows/<name>/config.yml` file plus any scripts/assets beside it.

For the complete key reference, see [workflow-yaml.md](workflow-yaml.md).

## Mental Model

| Concept | Meaning |
|---------|---------|
| `workflow` | What happens. |
| `step` | One action in the workflow. |
| `runtime` | Where the action runs. |
| `resolver` | Which tool/profile the action uses. |
| `kind: host` | Run this step on the host instead of in a container. |

Top-level `runtime` and `resolver` set workflow defaults. Step-level `runtime`
and `resolver` override those defaults for one step. Use `isolate` only when you
need to pin a specific image/template.

## Simple Container Workflow

```yaml
name: test
runtime: dockerimage
resolver: codex

steps:
  - id: test
    cmd: npm test
```

Run it:

```bash
dockpipe --workflow test --
```

## Host Step

Use `kind: host` only when the step genuinely needs host access, such as a local
CLI, GUI launcher, or sidecar lifecycle script.

```yaml
name: build-with-setup

steps:
  - id: prepare
    kind: host
    cmd: ./scripts/prepare.sh

  - id: test
    runtime: dockerimage
    resolver: codex
    cmd: npm test
```

Host steps are outside Docker container security policy. Keep container policy on
container steps.

## Resolver-Backed Container Step

Set defaults once, then override only where a step differs:

```yaml
name: review
runtime: dockerimage
resolver: codex

steps:
  - id: lint
    cmd: npm run lint

  - id: security-review
    resolver: claude
    cmd: ./scripts/review.sh
```

## Packaged Child Workflow

Use explicit `workflow` + `package` for nested packaged workflows:

```yaml
steps:
  - workflow: child-name
    package: acme-team
```

Do not model packaged workflow calls as runtimes. The child workflow keeps its
own runtime/resolver/security settings.

## Security Profile Example

```yaml
name: offline-test
runtime: dockerimage
resolver: codex

security:
  profile: secure-default
  network:
    mode: offline

steps:
  - cmd: npm test
```

Security policy is container-only. DockPipe compiles it into an effective runtime
manifest that run consumes. For the security model, see
[security-policy.md](security-policy.md).

## Step Outputs

Steps pass values forward with dotenv-style output files:

```yaml
steps:
  - id: compute
    cmd: sh -c 'echo VERSION=1.2.3 > bin/.dockpipe/version.env'
    outputs: bin/.dockpipe/version.env

  - id: use
    cmd: echo "$VERSION"
```

## Next

- Full YAML reference: [workflow-yaml.md](workflow-yaml.md)
- Package/reuse workflows: [package-quickstart.md](package-quickstart.md)
- Terms and boundaries: [architecture-model.md](architecture-model.md)
