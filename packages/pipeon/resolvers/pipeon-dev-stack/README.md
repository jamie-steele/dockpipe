# `pipeon-dev-stack`

First-party Pipeon product workflow for the local assistant stack.

Normal behavior: **close the Pipeon window and the stack should come down automatically**. The
companion stop workflow exists only as a manual recovery path if a session is left behind.

What it does:

- resolves explicit **`dockpipe`**, **`dorkpipe`**, and **`mcpd`** binaries into the isolated stack runtime env
- brings up an isolated DorkPipe stack container plus internal **Ollama** and **Postgres/pgvector**
- enables NVIDIA GPU access for **Ollama** when the host can see NVIDIA devices, then verifies Docker actually attached them
- exposes only a loopback MCP proxy sidecar on the host; the upstream DorkPipe MCP service stays private to the compose network over local TLS and keeps its auth secret out of the editor container
- refreshes the Pipeon context bundle
- starts the branded Pipeon code-server surface and opens it in the Pipeon desktop shell

Companion workflows:

- **`pipeon-dev-stack-status`** — inspection / debugging
- **`pipeon-dev-stack-stop`** — manual cleanup only when the normal window-close teardown does not fire

Typical use from the repo root:

```bash
make maintainer-tools
make build-pipeon-desktop
PATH="$PWD/packages/dorkpipe/bin:$PATH" \
dockpipe --workflow pipeon-dev-stack --workdir . --
```

The stack now prefers the dedicated Tauri desktop shell at
**`packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop`** instead of opening a normal
browser window.

## Ollama GPU

The stack defaults to **`PIPEON_DEV_STACK_GPU=auto`**. In auto mode, the launch script enables a
small compose override for the `ollama` service only when host **`nvidia-smi -L`** can see a GPU
and Docker reports an **`nvidia`** runtime. If the host GPU exists but Docker is not configured for
NVIDIA passthrough, the stack now prompts to either enable Docker GPU access, continue on CPU for
this launch, or cancel instead of failing during compose startup.
After compose starts in NVIDIA mode, the script verifies that NVIDIA devices are actually attached
to the Ollama container; if not, it fails with a setup hint instead of silently using CPU.

Set **`PIPEON_DEV_STACK_GPU=nvidia`** or **`all`** to force Docker Compose GPU access, or set
**`PIPEON_DEV_STACK_GPU=cpu`** / **`none`** / **`off`** to force CPU. The status workflow prints
the resolved mode and the last container-side verification status.

For the full rebuild / refresh sequence when local changes are not showing up, see
**`../pipeon/assets/docs/pipeon-refresh.md`**.

## Boundary

Pipeon is the client surface. DorkPipe is orchestration and routing. DockPipe remains the mutation
boundary. The dev stack now keeps the DorkPipe control plane inside compose and exposes only MCP back
to Pipeon / VS Code. See **`../pipeon/assets/docs/pipeon-dorkpipe-contract.md`**.
