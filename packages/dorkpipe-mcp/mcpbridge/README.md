# `mcpbridge` — MCP stdio bridge (`dorkpipe.mcp`)

Authoring tree: **`packages/dorkpipe-mcp/mcpbridge/`** — dumb interface

This package implements a **minimal** JSON-RPC over stdio (Content-Length framing) server that exposes **named tools** mapping to:

- `dockpipe` and `dorkpipe` subprocesses (same binaries as the CLI), and  
- read-only discovery via `dockpipe/src/lib/infrastructure` (workflow names only).

**Do not** add orchestration, policy, or workflow parsing here. That stays in **DockPipe** / **DorkPipe**.

## Environment

| Variable | Meaning |
|----------|---------|
| `DOCKPIPE_MCP_SERVER_VERSION` | Server version string in `initialize` (set by `mcpd` binary `-ldflags` or manually). |
| `DOCKPIPE_MCP_DEBUG` | If non-empty, **stdio** mode prints a one-line startup message to **stderr**. Default: off — some MCP hosts label all stderr as errors in the UI. |
| `DOCKPIPE_MCP_TIER` | **`readonly`** \| **`validate`** \| **`exec`** — coarse IAM for which tools exist (see **`../README.md`**). Default when unset: **`validate`** (unless `DOCKPIPE_MCP_ALLOW_EXEC=1`, then **`exec`**). |
| `DOCKPIPE_MCP_ALLOWED_TOOLS` | Optional comma-separated allowlist; intersects with the tier (subset only). |
| `DOCKPIPE_MCP_ALLOW_EXEC` | Legacy: when **`DOCKPIPE_MCP_TIER` is unset**, `1` means tier **`exec`**. Prefer **`DOCKPIPE_MCP_TIER=exec`**. |
| `DOCKPIPE_MCP_RESTRICT_WORKDIR` | **Default on** (unset counts as on). **`dockpipe.run`** / **`dorkpipe.run_spec`** workdirs must stay under **repo root**. Disable only with **`0`**, **`false`**, **`no`**, or **`off`**. |
| `DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN` | **Default on** — **`DOCKPIPE_BIN`** / **`DORKPIPE_BIN`** must be **absolute** (mitigates **`PATH`** hijack). Set **`0`** / **`false`** / **`off`** to allow non-absolute names from **`PATH`**. |
| `DOCKPIPE_BIN` | Path to `dockpipe` (default: `PATH`). |
| `DORKPIPE_BIN` | Path to `dorkpipe` (default: `PATH`). |

### HTTP mode (`mcpd`)

When **`MCP_HTTP_LISTEN`** or **`mcpd -http <addr>`** is set, the server listens for **JSON-RPC POST** on **`/`** and **`/mcp`**. **TLS is required** unless **`MCP_HTTP_INSECURE_LOOPBACK=1`** (or **`-insecure-loopback`**) and the bind address is **loopback only** (`127.0.0.1`, `::1`, `localhost` — not bare **`:port`**).

Every request must authenticate with the same secret via **`Authorization: Bearer <key>`**, **`Authorization: ApiKey <key>`**, or **`X-API-Key: <key>`**.

| Variable / flag | Meaning |
|-----------------|--------|
| `MCP_HTTP_LISTEN` / `-http` | Listen address (e.g. `:8443`). Empty = stdio (default). |
| `MCP_HTTP_API_KEY` / `-api-key` | **Required** in HTTP mode **unless** `MCP_HTTP_KEY_TIERS_FILE` is set. |
| `MCP_HTTP_KEY_TIERS_FILE` / `-key-tiers-file` | JSON file: `[{"key":"secret","tier":"readonly"},…]` — **replaces** single `MCP_HTTP_API_KEY`; each key has its own tier. See **`docs/examples/mcp-http-key-tiers.json`**. |
| `MCP_TLS_CERT_FILE` / `-tls-cert` | PEM certificate (HTTPS). |
| `MCP_TLS_KEY_FILE` / `-tls-key` | PEM private key (HTTPS). |
| `MCP_HTTP_INSECURE_LOOPBACK` / `-insecure-loopback` | Allow plain HTTP on loopback only (dev). |

## Entrypoint

- `dorkpipe-mcp/cmd/mcpd` — stdio MCP (default), or **HTTPS/HTTP** when `-http` / `MCP_HTTP_LISTEN` is set. **`make maintainer-tools`** (repo root) writes **`packages/dorkpipe-mcp/bin/mcpd`** (not under **`src/bin/`**).

### Trying it in Cursor

Run **`make build`** then **`make maintainer-tools`** from repo root. Use **`.cursor/mcp.json`** — set **`"type": "stdio"`**, **`command`** to the absolute **`mcpd`** binary, and set **`DOCKPIPE_BIN`** / **`DORKPIPE_BIN`** to the absolute executables you want the bridge to launch. In this checkout that is typically **`packages/dorkpipe-mcp/bin/mcpd`**, **`src/bin/dockpipe`**, and **`packages/dorkpipe/bin/dorkpipe`**. If **`${workspaceFolder}`** is not expanded in **`env`**, set those paths to absolute. Cursor speaks Content-Length JSON-RPC over stdin/stdout; **`initialize`** echoes the client’s **`protocolVersion`** (e.g. **`2025-11-25`**) so the handshake completes with current Cursor hosts.

## See also

- [Package README](../README.md) — architecture, tiers, trust, Cursor
