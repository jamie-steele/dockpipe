# Vault (secrets → env)

**`dockpipe.config.json`** points at an env **template** (**`secrets.vault_template`**, legacy **`op_inject_template`**) with **`op://…`** lines. Before workflow steps, DockPipe runs **`op inject`** (requires **[1Password CLI](https://developer.1password.com/docs/cli/)**’s **`op`** on **`PATH`**) unless **`DOCKPIPE_OP_INJECT=0`** or **`--no-op-inject`**.

- **`secrets.vault`** — optional default **`op`** when workflow YAML **omits** **`vault:`**. With **`op`**, the template path must resolve and the template file must exist (strict). **Omit** **`secrets.vault`** for best-effort inject: merge only when the template file exists, without failing on missing config.
- **`vault: op`** on a workflow — same strict behavior as project **`secrets.vault: op`** when set.

Workflow **`vault:`** overrides **`secrets.vault`** when non-empty. Resolution order for the template file: **`vault_template`** first, then **`op_inject_template`**.

Bundled **secretstore** (dotenv) loads a plain file — see **`src/core/workflows/secretstore/README.md`**. **`dockpipe doctor`** reports template path and **`op`** availability when **`dockpipe.config.json`** is present.
