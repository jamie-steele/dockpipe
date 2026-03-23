# demo-gui-cursor

Minimal **cursor-dev** demo: a long-lived **base-dev** container with **`/work`** mounted from **`--workdir`**, plus Cursor launch on the host. Writes **`.dockpipe/cursor-dev/AGENT-MCP.md`** and **`mcp.json.example`** so the Cursor agent can align on repo root and MCP (**`mcpd`**) for a **same-night demo**.

```bash
dockpipe --workflow demo-gui-cursor --resolver cursor-dev --runtime docker --workdir /path/to/checkout --
```

From the **dockpipe** checkout, run **`make build`**, then in Cursor enable MCP (see **`.cursor/mcp.json`**). See **`templates/core/resolvers/cursor-dev/README.md`** for options.
