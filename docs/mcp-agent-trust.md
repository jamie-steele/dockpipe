# MCP, agents, and progressive trust

This document separates **three things** people often mix up:

1. **On-disk context** (`.dockpipe/`, `.dorkpipe/`) — generated **for the user** (and readable by assistants) as **facts and handoffs**, not as a second orchestrator.
2. **MCP (`mcpd`)** — a **small, named** tool surface (JSON-RPC) that forwards to the same CLIs the human would run, with **tiered IAM** in the MCP process.
3. **DockPipe / DorkPipe themselves** — **execution truth**: containers, resolvers, DAG runs. Those stay **outside** MCP unless you deliberately expose them through **tier `exec`** (or run the CLIs / CI directly).

---

## 1. Artifact directories (`.dockpipe/`, `.dorkpipe/`)

| Area | Typical role |
|------|----------------|
| **`.dockpipe/`** | Handoff text, CI-normalized scan bundles, optional insights, Pipeon context — **read as context** when present. |
| **`.dorkpipe/`** | Orchestrator metadata, `self-analysis/` facts, metrics — **read as context** when present. |

**Agents should treat these as read-only grounding**, together with normal code reading. Regenerating them is a **user- or CI-driven** action (workflows, `make self-analysis*`, CI jobs), not something MCP “turns on” by default.

---

## 2. Tiered IAM (in-process)

The bridge resolves **`DOCKPIPE_MCP_TIER`** (and optional **`DOCKPIPE_MCP_ALLOWED_TOOLS`**) on every tool list / call. This is **not** multi-tenant identity inside one OS user — it is **policy bands** so deployments and launchers can match **compliance stages** to **capability**.

| Tier | Tools |
|------|--------|
| **`readonly`** | `dockpipe.version`, `capabilities.workflows` |
| **`validate`** | **readonly** + `dockpipe.validate_workflow`, `dorkpipe.validate_spec` |
| **`exec`** | **validate** + `dockpipe.run`, `dorkpipe.run_spec` |

**Precedence:**

1. **`DOCKPIPE_MCP_TIER`** — when set (`readonly` \| `validate` \| `exec`), it wins over the legacy flag.
2. Else **`DOCKPIPE_MCP_ALLOW_EXEC=1`** → **`exec`** (backward compatible).
3. Else → **`validate`** (default: validate tools on; run tools off).

**Optional narrowing:** **`DOCKPIPE_MCP_ALLOWED_TOOLS`** — comma-separated tool names. When set, only tools **both** allowed by the tier **and** listed here are visible and callable. Unknown names are ignored with a one-time stderr warning.

**Identity note:** “IAM” here means **authorization bands** — env for stdio, **or** per-HTTP-key when **`MCP_HTTP_KEY_TIERS_FILE`** is set (see **`src/lib/mcpbridge/README.md`**). It does **not** replace TLS or network policy for remote HTTP.

### HTTP: one API key per tier (implemented)

When **`mcpd`** listens with **`MCP_HTTP_KEY_TIERS_FILE`** (JSON array of `{"key":"…","tier":"readonly|validate|exec"}`), **each Bearer / `X-API-Key`** is looked up and the **request’s effective tier** is that key’s tier ( **`DOCKPIPE_MCP_TIER`** in the environment is ignored for that request). **`MCP_HTTP_API_KEY`** is not used when a key-tiers file is set.

Use this so e.g. automation gets **`readonly`** while an operator key gets **`exec`**, without separate processes.

### SSO / IdP — do we need it for DorkPipe, MCP, Pipeon?

**For typical dev + agent coding (Cursor stdio MCP, local `mcpd`, Pipeon on a repo): no.** Trust is **OS user**, **IDE**, and **env** (`DOCKPIPE_MCP_TIER`, or HTTP key file + TLS). Neither DorkPipe nor Pipeon needs OIDC **inside** `mcpd` for that model.

**Add SSO at the edge** (reverse proxy, API gateway, VPN) only when **untrusted networks** or **central audit** require it; terminate there and pass **one** secret or **mTLS** to `mcpd`, or keep **stdio** MCP on the workstation.

---

## 3. Relationship to `AGENTS.md`

**`AGENTS.md`** should stay the **maintainer contract** (architecture, primitives, where things live). It should **not** duplicate long “paste this prompt” workflows now that:

- **Artifacts** under `.dockpipe/` / `.dorkpipe/` supply **context**, and  
- **MCP** supplies **structured, tiered** tools.

Keep **`AGENTS.md`** short on “how to paste prompts”; link here and to **`docs/mcp-architecture.md`** instead.

---

## 4. Dogfooding in this repository

After **`make build`**, the repo includes **`src/bin/mcpd`**.

**Cursor (project MCP):** **`.cursor/mcp.json`** registers **`mcpd`** and sets **`DOCKPIPE_BIN`** / **`DORKPIPE_BIN`** to the repo launchers (required for **default absolute-binary** checks). By default (no **`DOCKPIPE_MCP_TIER`**) you get tier **`validate`**. For the **strictest** surface, add **`DOCKPIPE_MCP_TIER=readonly`** in **`env`**.

If **`${workspaceFolder}`** is not expanded on your platform, replace **`command`** and **`env`** paths with absolute paths to **`mcpd`**, **`dockpipe`**, and **`dorkpipe`**.

**Launcher (future):** a single entrypoint can set **`DOCKPIPE_MCP_TIER`** per profile (e.g. “reviewer” vs “builder”) without changing core code.

---

## 5. See also

- **`docs/mcp-architecture.md`** — component boundaries and tool table  
- **`docs/mcp-host-hardening.md`** — optional **workdir** / **`PATH`** controls on the host  
- **`src/lib/mcpbridge/README.md`** — env vars and HTTP mode  
- **`docs/compliance-ai-handoff.md`** — how to answer governance questions from **artifact** paths  
