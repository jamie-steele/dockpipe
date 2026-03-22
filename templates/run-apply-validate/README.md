# run-apply-validate

**Purpose:** A **three-step** pipeline named **run → apply → validate** (infrastructure / GitOps style). For **run → apply** only (no validate step), use **`run-apply`**. Each step runs in a disposable **alpine** container by default — replace **`cmd:`** with your real commands (e.g. **`terraform plan`**, **`terraform apply`**, **`terraform validate`**).

```bash
dockpipe --workflow run-apply-validate
```

Needs Docker + **`alpine`**. See **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** for **`outputs:`**, **`vars:`**, and **`imports:`** if phases need to share state.
