# `pipeon-dev-stack`

First-party Pipeon product workflow for the local assistant stack.

Normal behavior: **close the Pipeon window and the stack should come down automatically**. The
companion stop workflow exists only as a manual recovery path if a session is left behind.

What it does:

- ensures **`src/bin/dockpipe`**, **`packages/dorkpipe/bin/dorkpipe`**, and **`packages/dorkpipe-mcp/bin/mcpd`**
- brings up the DorkPipe sidecars (**Ollama** + **Postgres/pgvector**)
- starts **`mcpd`** on loopback HTTP with a generated API key
- refreshes the Pipeon context bundle
- opens the branded Pipeon editor session against the current workdir

Companion workflows:

- **`pipeon-dev-stack-status`** — inspection / debugging
- **`pipeon-dev-stack-stop`** — manual cleanup only when the normal window-close teardown does not fire

Typical use from the repo root:

```bash
./src/bin/dockpipe --workflow pipeon-dev-stack --workdir . --
```

## Boundary

Pipeon is the client surface. DorkPipe is orchestration and routing. DockPipe remains the mutation
boundary. See **`../pipeon/assets/docs/pipeon-dorkpipe-contract.md`**.
