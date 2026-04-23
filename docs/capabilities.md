# Capabilities and resolvers

This is primarily a **package/discovery** concept, not the first thing a normal
workflow author needs to learn. If you are just writing a workflow, start with
**runtime** and **resolver**. Come back here when you need resolver inference,
catalog metadata, or package-level dependency matching.

DockPipe separates **what you need** (abstract) from **how it is satisfied** (concrete packages and profiles).

## Terms

| Term | Meaning |
|------|---------|
| **Capability** | An **abstract** need — a stable dotted id (e.g. **`cli.codex`**, **`app.vscode`**, **`blob.storage`**). Names are **categories** + **specifics**: tool class (`cli`, `app`) or infrastructure (`blob`, `secrets`, `cache`) plus a short label. |
| **Resolver** | A **concrete** package or profile under **`templates/core/resolvers/<name>/`** (or an installed resolver **package**) that **implements** a capability: env, images, delegate workflows, tool wiring. **A resolver satisfies a capability.** |
| **Runtime** | **Where** execution runs — **`dockerimage`**, **`dockerfile`**, **`package`** (nesting); legacy **`cli`** / **`powershell`** / **`cmd`** normalize to **`dockerimage`** — **orthogonal** to capability and resolver. |
| **Package** | **Workflow**, **resolver**, **core**, **bundle**, or **assets** — installable units with **`package.yml`**. **Resolvers are packages**; **workflows are packages** too when compiled or published. Same packaging story (`dockpipe package compile`, store tarballs). |

## DockPipe-owned namespacing (`dockpipe.*`)

**First-party** capabilities, runtime-scoped resolver groupings, and other identifiers **authored as part of DockPipe** should use the **`dockpipe.`** prefix so they stay distinct from vendor ids, community packs, and downstream projects.

Examples (illustrative):

| Id | Role |
|----|------|
| **`dockpipe.cli`** | Baseline host/shell execution (project-defined). |
| **`dockpipe.docker`** | Local container isolation substrate as a named capability (when you model substrate this way). |
| **`dockpipe.cloud.aws.ec2`** | Cloud/runtime-specific resolver namespace — provider-shaped grouping under **`cloud`**. |

Existing ecosystem-style ids (e.g. **`cli.codex`**) remain valid; **new** DockPipe-first-party surface area should prefer **`dockpipe.*`** unless you are deliberately aligning with an external convention.

## Rules

1. **Runtime and resolver stay separate** — see **[architecture-model.md](architecture-model.md)**. **Capability** is a third **abstract** axis, not a replacement for runtime.
2. **`package.yml`** — **`kind: resolver`** packages set **`capability:`** to the dotted id this resolver provides. **`kind: workflow`** packages set **`requires_capabilities:`** (and **`requires_resolvers:`** for profile names when needed).
3. **Workflow YAML** — **`capability:`** is optional. It identifies the workflow at the package/discovery layer and may be used for resolver inference when **`resolver:`** is omitted. If **`resolver:`** / **`default_resolver:`** are omitted, the runner looks up **`templates/core/resolvers/<name>`** from resolver packages whose **`package.yml`** declares the same **`capability:`**. **`dockpipe.*`** capabilities can imply a default **`runtime:`** substrate name (e.g. **`dockpipe.docker`** → **`dockerimage`**) when **`runtime:`** is unset — still a **core** profile name, not a new substrate. Explicit **`runtime:`** / **`resolver:`** **take precedence** over those defaults.

**Deprecated YAML (still accepted):** `primitive:` and `requires_primitives:` — same meaning as **`capability:`** and **`requires_capabilities:`**.

## Examples

**Resolver package** (`package.yml`):

```yaml
kind: resolver
name: codex
capability: cli.codex
# ...
```

**Workflow package** (`package.yml`):

```yaml
kind: workflow
name: my-ci
requires_capabilities: [cli.codex]
requires_resolvers: [codex]
# ...
```

**Workflow** (`config.yml`) — advanced form where resolver is inferred from capability (same as explicit `resolver: codex` when `package.yml` declares `capability: cli.codex`):

```yaml
name: my-flow
capability: cli.codex
runtime: dockerimage
```

## Wired behavior (tooling)

- **`dockpipe package list`** — tab-separated columns include **`provider`**, **`capability`**, **`requires_capabilities`** (comma-separated when multiple), **`description`**.
- **`packages-store-manifest.json`** (from **`dockpipe package build store`**) — each artifact includes **`provider`**, **`capability`**, and for workflow packages **`requires_capabilities`** when set in **`package.yml`**.

## See also

- **[architecture-model.md](architecture-model.md)** — normative runtime / resolver / workflow definitions  
- **[package-model.md](package-model.md)** — **`package.yml`** fields  
- **[workflow-yaml.md](workflow-yaml.md)** — workflow keys (default vs per-step **`resolver`** / **`runtime`**)
