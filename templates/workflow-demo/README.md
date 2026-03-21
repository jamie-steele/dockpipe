# workflow-demo

**Purpose:** Show **`steps:`**, an **async** group, and **merged `outputs:`** before a blocking join. Smaller sequential example: **`templates/chain-test/`**.

## Run

```bash
dockpipe --workflow workflow-demo
```

From a checkout that includes **`templates/workflow-demo/`** (e.g. repo root). Needs Docker + **`alpine`**.

## Copy

```bash
dockpipe template init my-demo --from workflow-demo
dockpipe --workflow my-demo
```

**Details:** **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)**
