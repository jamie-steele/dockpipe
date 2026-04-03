# Init template

Copied to **`workflows/<name>/`** when you run **`dockpipe init <name> --from init`**. Bare **`dockpipe init`** also seeds **`workflows/example/`** from this starter when the project has no DockPipe workflows yet.

Edit **`config.yml`** to match your workflow. This starter intentionally uses the current **multi-step pipeline** style with:

- **`types:`** pointing at a tiny typed contract under **`models/`**
- **`vars:`** for user-tunable inputs
- a host **`prepare`** step with **`outputs:`**
- an isolated container **`run`** step
- a host **`report`** step

Use it as a sketch, then replace the commands, images, and env names with your real pipeline. **Learning path:** **[docs/onboarding.md](../../docs/onboarding.md)** · **[docs/architecture-model.md](../../docs/architecture-model.md)**.
