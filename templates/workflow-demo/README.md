# workflow-demo

Small **example template** for **`docs/workflow-yaml.md`**: blocking step → **async group** (two parallel alpine containers writing different **`outputs:`** files) → **join** step that sees merged env (**last declarer wins** on `DEMO_BRANCH` → `b`).

## Run

From the **dockpipe repo root** (so `templates/workflow-demo/` exists):

```bash
dockpipe --workflow workflow-demo
```

Requires **Docker** and a pullable **`alpine`** image. No extra scripts or resolvers.

## Copy into your workspace

```bash
dockpipe template init my-demo --from workflow-demo
dockpipe --workflow my-demo
```

## Simpler two-step chain

For a minimal sequential **outputs** handoff only, see **`templates/chain-test/`**.
