# Runtime profiles (`templates/core/runtimes/`)

**Runtime** = **where** execution runs (substrate). Profiles are **`KEY=value`** files at **`runtimes/<name>`** or **`runtimes/<name>/profile`**. Only **`DOCKPIPE_RUNTIME_*`** keys belong here; **`DOCKPIPE_RESOLVER_*`** lives under **`../resolvers/`**.

## Bundled substrates

| Name | Role |
|------|------|
| **`dockerimage`** | Container from a **pre-built image**. **Host-only** steps use the same profile with **`kind: host`** — there is no separate “CLI” runtime directory. |
| **`dockerfile`** | Container **built from a Dockerfile** in the repo. |

Shipped stubs set **`DOCKPIPE_RUNTIME_SUBSTRATE`** to **`dockerimage`** or **`dockerfile`**. Other optional **`DOCKPIPE_RUNTIME_*`** keys (e.g. **`DOCKPIPE_RUNTIME_TYPE`**) are documented with resolver/runtime fields in **`src/lib/domain/resolver.go`**.

## Legacy `runtime:` names in YAML

Older workflows may still use **`docker`**, **`cli`**, **`cmd`**, **`powershell`**, **`kube-pod`**, **`kubepod`**, **`cloud`**, or **`keystore`**. Those normalize to **`dockerimage`** when the workflow is loaded. Only **`dockerimage`** and **`dockerfile`** are part of the authored runtime story.

Overview: **[docs/architecture-model.md](../../../docs/architecture-model.md)**.

The runner merges **`runtimes/<name>`** with **`resolvers/<name>`** when both exist (or with an explicit **`--resolver`**).
