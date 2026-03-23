# Runtime profiles (`templates/core/runtimes/`)

**Runtime** = **where** execution runs (substrate). Use **`DOCKPIPE_RUNTIME_*`** keys only — workflow delegation, host scripts, **`DOCKPIPE_RUNTIME_TYPE`** (**`runtime.type`**). **No** tool-specific **`DOCKPIPE_RESOLVER_*`** keys here; those live under **`../resolvers/`**.

Bundled **substrate** names:

| Name | Role |
|------|------|
| **`cli`** | Host / local shell. |
| **`docker`** | Container-based isolated execution (typical Dockpipe path). |
| **`kube-pod`** | Kubernetes pod/job execution (future-facing placeholder). |

Each may be a flat **`KEY=value`** file or a directory with a **`profile`** file (same resolution as resolvers).

Normative model: **[docs/architecture-model.md](../../../docs/architecture-model.md)**.

The runner merges **`runtimes/<name>`** with **`resolvers/<name>`** (when both exist) or with an explicit **`--resolver`** profile name.
