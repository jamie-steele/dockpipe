# run-apply

**Purpose:** A **two-step** pipeline **run → apply** (plan/apply style without a separate validate step). Each step runs in a disposable **alpine** container by default — replace **`cmd:`** with your real commands (e.g. **`terraform plan`**, **`terraform apply`**). For **run → apply → validate**, use **`run-apply-validate`** instead.

```bash
dockpipe --workflow run-apply
```

Needs Docker + **`alpine`**. See **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** for **`outputs:`**, **`vars:`**, and **`imports:`** if phases need to share state.
