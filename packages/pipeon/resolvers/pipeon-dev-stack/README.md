# `pipeon-dev-stack`

First-party Pipeon product workflow for the local assistant stack.

Normal behavior: **close the Pipeon window and the stack should come down automatically**. The
companion stop workflow exists only as a manual recovery path if a session is left behind.

What it does:

- ensures **`src/bin/dockpipe`**, **`packages/dorkpipe/bin/dorkpipe`**, and **`packages/dorkpipe-mcp/bin/mcpd`**
- brings up the DorkPipe sidecars (**Ollama** + **Postgres/pgvector**)
- starts **`mcpd`** on loopback HTTP with a generated API key
- refreshes the Pipeon context bundle
- starts the branded Pipeon code-server surface and opens it in the Pipeon desktop shell

Companion workflows:

- **`pipeon-dev-stack-status`** — inspection / debugging
- **`pipeon-dev-stack-stop`** — manual cleanup only when the normal window-close teardown does not fire

Typical use from the repo root:

```bash
make build-pipeon-desktop
./src/bin/dockpipe --workflow pipeon-dev-stack --workdir . --
```

The stack now prefers the dedicated Tauri desktop shell at
**`src/apps/pipeon-desktop/bin/pipeon-desktop`** instead of opening a normal
browser window.

For the full rebuild / refresh sequence when local changes are not showing up, see
**`../pipeon/assets/docs/pipeon-refresh.md`**.

## Boundary

Pipeon is the client surface. DorkPipe is orchestration and routing. DockPipe remains the mutation
boundary. See **`../pipeon/assets/docs/pipeon-dorkpipe-contract.md`**.
