# Vault (secrets → env)

DockPipe treats **vault** as an abstract layer: workflow YAML can set **`vault: <name>`** to choose how **`dockpipe.config.json`** maps secret references into process environment before steps run.

## 1Password CLI (`op inject`)

**[1Password CLI](https://developer.1password.com/docs/cli/)** is installable on macOS, Linux, and Windows; it provides **`op`** and **`op inject`**. In workflow YAML use **`vault: op`** or **`vault: 1password`** (same behavior). Opt out for a workflow with **`vault: none`**, **`off`**, **`false`**, **`no`**, or **`0`**.

## Vendor-neutral path in core (no `op`)

Bundled **`secretstore`** with resolver **`dotenv`** loads a plain dotenv file (default **`.env.secretstore`**) on the host — **no third-party vault CLI**, fully open workflow + scripts. See **`src/core/workflows/secretstore/README.md`** and **`src/core/resolvers/dotenv/README.md`**. Use **`vault: none`** on workflows that rely on that path so DockPipe does not also run **`op inject`** at startup. You can still add other resolver profiles under **`templates/core/resolvers/`** later.

## Project config

- **`secrets.vault_template`** — repo-relative or absolute path to the env template file (preferred).
- **`secrets.op_inject_template`** — legacy alias; used only when **`vault_template`** is absent or empty.

Resolution order: **`ResolveVaultTemplatePath`** in code — **`vault_template` first**, then **`op_inject_template`**.

## Workflow YAML

- **`vault: op`** or **`vault: 1password`** — requires project config with a template path and a readable template file; requires **`op`** on **`PATH`** unless **`DOCKPIPE_OP_INJECT=0`** or **`--no-op-inject`**.
- **`vault: none`** (or the opt-out values above) — skips **`op inject`** merge for that workflow even if a template is configured (use with **`secretstore`** / **dotenv** or other host-side secret loading).

## Files

- **`dockpipe init`** drops **`.env.vault.template.example`** at the project root (and keeps the same file under **`templates/core/assets/`**). Copy the example to **`.env.vault.template`** (gitignored) and edit **`op://`** lines.
- **`op`** lines use 1Password **`op://Vault/Item/field`** addresses; future vault backends can add their own template conventions and resolver names without overloading **resolver:** (tool profiles under **`templates/core/resolvers/`**).

## See also

- **`dockpipe doctor`** — reports template path and **`op`** availability when **`dockpipe.config.json`** is present.
