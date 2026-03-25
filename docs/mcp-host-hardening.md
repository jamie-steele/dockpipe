# MCP bridge — host and top-level trust

`mcpd` runs on the **same machine and user** as its parent (stdio) or accepts **network clients** (HTTP). This document lists **extra** controls beyond tiers, TLS, and API keys.

---

## 1. What the bridge can still reach

- **Subprocesses:** `dockpipe` and `dorkpipe` as resolved from **`DOCKPIPE_BIN`**, **`DORKPIPE_BIN`**, or **`PATH`**.
- **Filesystem:** Repo root resolution (`DOCKPIPE_REPO_ROOT` / discovery), workflow and spec paths, and (unless restricted) **any workdir** you pass into exec tools.
- **Network / Docker:** Whatever those CLIs can do — same as if the operator ran them in a shell.

So “trust the host” means: **trust the OS user**, **trust `PATH`**, and **trust repo root** for path checks.

---

## 2. Env hardening (defaults **on**)

| Variable | Effect |
|----------|--------|
| **`DOCKPIPE_MCP_RESTRICT_WORKDIR`** | **Default on** (unset = on). For **`dockpipe.run`** and **`dorkpipe.run_spec`**, workdirs must stay under the resolved **repo root**. Empty workdir → **repo root**; **`dorkpipe.run_spec`** always passes **`--workdir`** when restriction is on. **Opt out:** `0`, `false`, `no`, or `off`. |
| **`DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN`** | **Default on** (unset = on). Refuses to spawn `dockpipe` / `dorkpipe` unless resolution is an **absolute** path. **Opt out:** `0`, `false`, `no`, or `off`. |

For local dev, set **`DOCKPIPE_BIN`** / **`DORKPIPE_BIN`** to your repo’s **`src/bin/dockpipe`** and **`src/bin/dorkpipe`** (see **`.cursor/mcp.json`**). Relax only when you intentionally rely on **`PATH`** lookup.

---

## 3. Practices outside the binary

- **Dedicated OS user** for `mcpd` in production; no root.
- **systemd** (or similar): `ProtectSystem=strict`, `PrivateTmp=yes`, `NoNewPrivileges=yes` where compatible with Docker socket use.
- **HTTP:** Terminate TLS at **`mcpd`** or at a **reverse proxy**; rate-limit and log at the proxy; do not expose plain HTTP off loopback without a reason.
- **Key tier JSON:** `chmod 600`, exclude from backups to untrusted stores.
- **stdio MCP:** The **IDE** is the trust boundary — same as running `dockpipe` in the integrated terminal.

---

## 4. What we are not doing in-process

- **SSO / OIDC** inside `mcpd` — use an edge proxy or VPN if you need org login (see **`docs/mcp-agent-trust.md`**).
- **seccomp / Landlock** around child processes — would fight Docker and toolchains; use containerized deployment if you need kernel-level sandboxing of the whole service.
