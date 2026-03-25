# secretstore-r2-publish-test

## Start here: where is the mapping?

There is **no** separate “mapping table” in DockPipe. **You define names and 1Password references in one place:**

| What you want | Where to look / what to edit |
|---------------|------------------------------|
| **Vault item → environment variable name** (e.g. `CLOUDFLARE_API_TOKEN`, `R2_BUCKET`) | **`--workdir` / `.env.op.template`** — each line is `VAR_NAME=op://Vault/Item/field`. Example for vault **DockPipe** / item **CLOUDFLARE**: **`src/templates/secretstore/.env.op.template.example`**. In this repo, copy that file to **`.env.op.template`** at the repo root (gitignored). |
| **Hint list** of common var names (documentation only) | **`templates/core/resolvers/onepassword/profile`** → `DOCKPIPE_RESOLVER_ENV=...` |
| **Which script reads the template and where it writes** | This workflow’s **`vars:`** → `OP_ENV_FILE` (input) and `SECRET_ENV_OUT` (must match step 1 **`outputs:`**). Script: **`scripts/dockpipe/secretstore-op-inject-outputs.sh`**. |
| **What step 2 consumes** | **`scripts/dockpipe/r2-publish.sh`** and **`shipyard/workflows/r2-publish/README.md`** — same variable names as in your `.env.op.template` after `op inject`. |

**Flow:** `op inject` turns `op://…` into values but **keeps your left-hand names** (`VAR_NAME=`). DockPipe then loads that file as **`KEY=VAL`** into the process environment for step 2 — **no rename step**.

---

Internal workflow: **`op inject`** (1Password) → **outputs merge** → **`r2-publish`**.

## Why two steps

DockPipe merges the first step’s **`outputs:`** file into the environment **after** that step’s host script runs. The inject script writes **`SECRET_ENV_OUT`** (must match **`outputs:`**); the second step sees **`CLOUDFLARE_*`**, **`R2_*`**, etc. without nesting a second `dockpipe` process.

## Quick test

From a dockpipe git checkout (repo root):

```bash
cp src/templates/secretstore/.env.op.template.example .env.op.template
# Edit .env.op.template with real op:// fields for your vault items.
mkdir -p dist && echo test >dist/README.txt
R2_PUBLISH_DRY_RUN=1 R2_TF_BACKEND=local \
  ./src/bin/dockpipe --workflow-file shipyard/workflows/secretstore-r2-publish-test/config.yml --workdir . --
```

Use **`R2_TF_BACKEND=local`** until R2 state keys are in your template. Drop **`R2_PUBLISH_DRY_RUN`** when you want a real upload.

## Resolver

**`resolver: onepassword`** loads **`templates/core/resolvers/onepassword/profile`** (env hints for `op`-injected keys). It does not replace the 1Password app or CLI.

## Troubleshooting

**`op inject`: `invalid secret reference 'op://…/'`** — **`op inject` scans the whole file**, including comments. Do not put **partial** `op://` strings in `#` lines (e.g. `op://Vault/…` with an ellipsis). Use plain English in comments, or only full `VAR=op://Vault/Item/field` lines.
