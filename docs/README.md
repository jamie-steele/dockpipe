# Documentation

DockPipe has a small front door and deeper reference material behind it. Start
with the task you are doing, then drop into reference docs only when needed.

## Start Here

| Goal | Doc |
|------|-----|
| Install and run the first command | [install.md](install.md), then [onboarding.md](onboarding.md) |
| Write a workflow | [workflow-authoring.md](workflow-authoring.md) |
| Look up every YAML key | [workflow-yaml.md](workflow-yaml.md) |
| Compile/package reusable artifacts | [package-quickstart.md](package-quickstart.md) |
| Inspect CLI flags and subcommands | [cli-reference.md](cli-reference.md) |

## Core Concepts

| Topic | Doc |
|-------|-----|
| Terms: workflow, runtime, resolver, strategy | [architecture-model.md](architecture-model.md) |
| Isolation layer and profile files | [isolation-layer.md](isolation-layer.md) |
| Capability ids and resolver packages | [capabilities.md](capabilities.md) |
| Optional typed authoring layer | [pipelang.md](pipelang.md) |

## Packages, Security, And Images

| Topic | Doc |
|-------|-----|
| Package/store model | [package-model.md](package-model.md) |
| Security policy | [security-policy.md](security-policy.md) |
| Docker image artifacts | [image-artifacts.md](image-artifacts.md) |
| Combined design notes / history | [docker-security-images.md](docker-security-images.md) |

## Advanced / Maintainer

| Topic | Doc |
|-------|-----|
| Engine data flow | [architecture.md](architecture.md) |
| DockPipe vs first-party maintainer packages | [core-tools.md](core-tools.md) |
| Generated maintainer artifacts | [artifacts.md](artifacts.md) |
| Core-vs-packages audit | [core-vs-packages-audit.md](core-vs-packages-audit.md) |
| Manual QA | [qa/manual-qa.md](qa/manual-qa.md) |
| Messaging / about copy | [messaging.md](messaging.md) |

MCP (`mcpd`) is DorkPipe-owned control-plane tooling; see the package README
under `packages/dorkpipe-mcp/`.
