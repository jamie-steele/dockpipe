# Vault (secrets → env)

**`dockpipe.config.json`** points at an env **template** (**`secrets.vault_template`**, legacy **`op_inject_template`**) — often **`.env.template`** (agnostic name). With **`secrets.vault: op`**, lines use **`op://…`** for 1Password. Before workflow steps, DockPipe runs **`op inject`** (requires **[1Password CLI](https://developer.1password.com/docs/cli/)**’s **`op`** on **`PATH`**) unless **`DOCKPIPE_OP_INJECT=0`** or **`--no-op-inject`**.

## In memory only (no resolved secrets file from DockPipe)

The CLI runs **`op inject -i <template>`** with **no `--out-file`** so **`op`** writes to **stdout** (see **`op inject --help`**: `-o` means “to a file **instead of** stdout”). It reads **stdout into process memory** — it does **not** write a second “resolved” template file. Do **not** use **`op inject -i … -o -`**: **`-o`** takes a **path**; **`-`** is a file named **`-`** in the current directory, not stdout — same pitfall as shell **`> -`**.

If you see a file literally named **`-`** in the repo root, that almost always comes from a **shell mistake**: **`op inject … > -`** redirects output to a **file** named **`-`**, not to stdout. **Do not use `> -`.** When **`dockpipe`** builds step env (including when **`DOCKPIPE_OP_INJECT=0`** or **`--no-op-inject`**), it may **remove** that file if it looks like accidental inject output (env-like text); set **`DOCKPIPE_KEEP_DASH_FILE=1`** to keep it. For manual checks, run **`op inject -i .env.op.template`** (writes to stdout) or **`op inject -i .env.op.template | …`**, then **`rm -- -`** to remove any stray file. **Delete** that file if it contains plaintext secrets and rotate credentials if it was ever committed.

- **`secrets.vault`** — optional default **`op`** when workflow YAML **omits** **`vault:`**. With **`op`**, the template path must resolve and the template file must exist (strict). **Omit** **`secrets.vault`** for best-effort inject: merge only when the template file exists, without failing on missing config.
- **`vault: op`** on a workflow — same strict behavior as project **`secrets.vault: op`** when set.

Workflow **`vault:`** overrides **`secrets.vault`** when non-empty. Resolution order for the template file: **`vault_template`** first, then **`op_inject_template`**.

Bundled **secretstore** (dotenv) loads a plain file — see **`src/core/workflows/secretstore/README.md`**. **`dockpipe doctor`** reports template path and **`op`** availability when **`dockpipe.config.json`** is present.
