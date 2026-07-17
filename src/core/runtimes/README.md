# Runtime profiles (`templates/core/runtimes/`)

**Runtime** = **where** execution runs (substrate). Profiles are **`KEY=value`** files at **`runtimes/<name>`** or **`runtimes/<name>/profile`**. Only **`DOCKPIPE_RUNTIME_*`** keys belong here; **`DOCKPIPE_RESOLVER_*`** lives under **`../resolvers/`**.

## Bundled substrates

| Name | Role |
|------|------|
| **`dockerimage`** | Container from a **pre-built image**. **Host-only** steps use the same profile with **`kind: host`** — there is no separate “CLI” runtime directory. |
| **`dockerfile`** | Container **built from a Dockerfile** in the repo. |
| **`vm`** | Authored VM runtime profile for virtual-machine substrates. The bundled profile delegates to the internal **`vmimage`** host runner, while concrete VM products come from resolvers such as packaged QEMU profiles. |
| **`vmimage`** | Internal VM image substrate launched on the host via a backend VMM. The bundled profile delegates to a host script; guest disk / firmware / license media are user-supplied, and the runtime prompts before using installer media, persistent disk writes, or explicit host port exposure. |

Shipped stubs set **`DOCKPIPE_RUNTIME_SUBSTRATE`** to **`dockerimage`**, **`dockerfile`**, or **`vmimage`**. Other optional **`DOCKPIPE_RUNTIME_*`** keys (e.g. **`DOCKPIPE_RUNTIME_TYPE`**) are documented with resolver/runtime fields in **`src/lib/domain/resolver.go`**.

## Legacy `runtime:` names in YAML

Older workflows may still use **`docker`**, **`cli`**, **`cmd`**, **`powershell`**, **`kube-pod`**, **`kubepod`**, **`cloud`**, or **`keystore`**. Those normalize to **`dockerimage`** when the workflow is loaded. Current authored runtime substrates are **`dockerimage`**, **`dockerfile`**, and **`vm`**; **`vmimage`** remains the internal VM substrate label carried by the runtime profile.

Overview: **[docs/concepts/architecture-model.md](../../../docs/concepts/architecture-model.md)**.

The runner merges **`runtimes/<name>`** with **`resolvers/<name>`** when both exist (or with an explicit **`--resolver`**).
