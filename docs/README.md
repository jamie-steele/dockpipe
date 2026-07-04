# Documentation

DockPipe has a small front door and deeper reference material behind it. Start
with the task you are doing, then drop into reference docs only when needed.

## Start Here

If you are new to DockPipe, stay in this section first. The concept and
maintainer docs below are reference material, not the shortest learning path.

| Goal | Doc |
|------|-----|
| Install and run the first command | [install.md](install.md), then [onboarding.md](onboarding.md) |
| Write a workflow | [workflows/workflow-authoring.md](workflows/workflow-authoring.md) |
| Look up every YAML key | [workflows/workflow-yaml.md](workflows/workflow-yaml.md) |
| Compile/package reusable artifacts | [packages/package-quickstart.md](packages/package-quickstart.md) |
| Inspect CLI flags and subcommands | [cli-reference.md](cli-reference.md) |

## Core Concepts

| Topic | Doc |
|-------|-----|
| Terms: workflow, runtime, resolver, strategy | [concepts/architecture-model.md](concepts/architecture-model.md) |
| Isolation layer and profile files | [concepts/isolation-layer.md](concepts/isolation-layer.md) |
| Capability ids and resolver packages | [concepts/capabilities.md](concepts/capabilities.md) |
| Governed AI/documentation workflows | [workflows/agentic-workflows.md](workflows/agentic-workflows.md) |
| Optional typed authoring layer | [concepts/pipelang.md](concepts/pipelang.md) |

## Packages, Security, And Images

| Topic | Doc |
|-------|-----|
| Package/store model | [packages/package-model.md](packages/package-model.md) |
| Security policy | [security/security-policy.md](security/security-policy.md) |
| Docker image artifacts | [runtime/image-artifacts.md](runtime/image-artifacts.md) |
| Combined design notes / history | [security/docker-security-images.md](security/docker-security-images.md) |

## Advanced / Maintainer

Most users do not need this section on day one. Use it when you are maintaining
DockPipe itself, publishing packages, or debugging compiled/runtime behavior.

| Topic | Doc |
|-------|-----|
| Engine data flow | [concepts/architecture.md](concepts/architecture.md) |
| DockPipe vs first-party maintainer packages | [packages/core-tools.md](packages/core-tools.md) |
| Generated maintainer artifacts | [runtime/artifacts.md](runtime/artifacts.md) |
| Agent routing and focused maintainer rules | [agents/index.yaml](agents/index.yaml), [agents/](agents/) |
| Core-vs-packages audit | [packages/core-vs-packages-audit.md](packages/core-vs-packages-audit.md) |
| Manual QA | [manual-qa.md](manual-qa.md) |
| Messaging / about copy | [concepts/messaging.md](concepts/messaging.md) |

MCP (`mcpd`) is DorkPipe-owned control-plane tooling; see the package README
under `packages/dorkpipe-mcp/`.
