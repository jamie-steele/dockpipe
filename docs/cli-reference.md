# CLI reference

**Run → isolate → act.** Overrides use the same names as workflow YAML. Precedence: **CLI** > config > environment.

For most users, the main knobs are:

- **`--workflow`** — which workflow to run
- **`--runtime`** — where it runs
- **`--resolver`** — which tool/profile it uses

Treat **`--isolate`** as the low-level image/template override, not the main way to think about workflow selection.

**Prerequisites:** **Docker** and **`bash` on the host** (dockpipe always invokes bash). **`git` on the host** for **`--repo`**, worktrees, and commit-on-host. See **[install.md](install.md)**.

**Workflow YAML (`config.yml`):** single-command layout (`run` / `isolate` / `act`) or multi-step **`steps:`** (ordering, **`outputs:`**, explicit **`group.mode: async`** for parallel work). In step mode, top-level `run` / `act` are not used. Full reference: **[workflow-yaml.md](workflow-yaml.md)**.

**Subcommands:** `dockpipe init`, `dockpipe install` (fetch **`templates/core`** from HTTPS — e.g. **Cloudflare R2** behind a public URL), `dockpipe clone` (copy a compiled workflow package to **`workflows/`** when **`allow_clone: true`** in **`package.yml`**), **`dockpipe build`** / **`clean`** / **`rebuild`** (compiled store under **`bin/.dockpipe/internal/packages/`**), `dockpipe package` (list, manifest, **`package build core`** / **`package build store`**, **`package compile …`**), `dockpipe release upload` (S3-compatible upload via **`aws` CLI** — self-hosted registries), `dockpipe workflow validate [path]`, `dockpipe pipelang compile|invoke|materialize` (optional typed authoring layer: compile inspectable artifacts, invoke methods via CLI, or recursively materialize `.pipe` files under configured compile roots), `dockpipe action init`, `dockpipe pre init`, `dockpipe template init`, `dockpipe doctor` (verify **bash**, **Docker**, bundled assets), `dockpipe core script-path <dotted>` (print absolute path to **`scripts/core.<dots>`** — same resolution as workflow YAML), `dockpipe terraform pipeline-path` (shortcut to **`terraform-pipeline.sh`** — **`packages/terraform/resolvers/terraform-core/`**, **`DOCKPIPE_TF_*`** in **`src/core/assets/scripts/README.md`**), `dockpipe runs list [--workdir]` (list **`bin/.dockpipe/runs/*.json`** for host **`kind: host`** steps), `dockpipe windows setup|doctor` — not covered in the flag table below; see **[README.md](../README.md)** and **[install.md](install.md)**.

## `dockpipe init`

Local project setup only: **no `git clone`**, no treating **`init`** as a remote bootstrap. Everything runs in the **current working directory**.

| Command | Purpose |
|---------|---------|
| `dockpipe init` | Create the **minimal project scaffold**: **`workflows/`**, **`README.md`**, **`dockpipe.config.json`**, and **`.env.vault.template.example`** when missing. If no DockPipe workflows exist yet, also create **`workflows/example/config.yml`** as a starter. It does **not** copy **`templates/core/`**, **`scripts/`**, or **`images/`** into the project. |
| `dockpipe init <name>` | Create **`workflows/<name>/config.yml`** as a **minimal empty workflow** (name + description only). Does **not** copy the bundled **`init`** template unless you pass **`--from init`**. If **`workflows/`** already exists but has no **`*/config.yml`** trees, the CLI prints a **warning** (often a conflict with GitHub Actions). |
| `dockpipe init <name> --workflows-dir <path>` | Put the new workflow under **`<path>/<name>/`** instead of **`workflows/`** (repo-relative or absolute). Same as env **`DOCKPIPE_WORKFLOWS_DIR`** for **`dockpipe run`**. |
| `dockpipe init <name> --from <source>` | Copy into **`workflows/<name>/`** (or **`--workflows-dir`**): **`--from`** may be **`blank`**, **`init`**, another **bundled** name (resolved under **`templates/&lt;name&gt;`** or the bundled workflows root), or a **filesystem path** (e.g. **`workflows/&lt;name&gt;`** in a dockpipe checkout). Not a Git URL. |
| `dockpipe init <name> --resolver <n> --runtime <n> --strategy <n>` | Optional; written into the new **`config.yml`**. New workflows use plain **`resolver:`** and **`runtime:`**. Example: **`dockpipe init my-pipeline --from run-apply --resolver codex --runtime dockerimage`**. |
| `dockpipe init --gitignore` | **Opt-in** only: append a marked block to **`.gitignore`** at the **git repository root** (`bin/.dockpipe/`, Go caches, `tmp/`). Idempotent if the block is already present. Requires a **git working tree** (run from inside a repo). |

On a **dockpipe source checkout**, **`--workflow`** resolves **`workflows/<name>/config.yml`** first (when that tree has workflows), then **nested** **`config.yml`** under directories listed in **`dockpipe.config.json` `compile.workflows`** (e.g. **`.staging/packages/...`** in this repo — see **[package-model.md](package-model.md)**), then **`src/core/workflows/<name>/config.yml`**. In a **typical downstream project**, named workflows live under **`workflows/<name>/config.yml`** by default; **`templates/<name>/config.yml`** remains a **legacy** fallback. Override the primary folder with **`--workflows-dir`** or **`DOCKPIPE_WORKFLOWS_DIR`**. This repo keeps **lean CI** under **`workflows/`** and **grouped maintainer packages** under **`.staging/packages/`** when listed in config (see **[AGENTS.md](../AGENTS.md)**).

**`dockpipe init`** creates the root **`README.md`** when missing and ensures **`workflows/`** exists.

## `dockpipe install`

Fetches a published **`templates/core`** tree over **HTTPS** and replaces **`<workdir>/templates/core`**. Intended for packages hosted on **Cloudflare R2** (or any static HTTPS origin); credentials are **not** required for public URLs.

| Command | Purpose |
|---------|---------|
| `dockpipe install core --dry-run --base-url <https://…>` | Show resolved URLs only. |
| `dockpipe install core --base-url <https://…>` | **`latest`**: GET **`<base>/install-manifest.json`**, follow **`packages.core`**, verify **`sha256`**, extract. |
| `dockpipe install core --base-url <https://…> --version 0.6.0` | GET **`<base>/templates-core-0.6.0.tar.gz`** (+ optional **`.sha256`**). |
| `dockpipe install core --url <https://…/file.tar.gz>` | Direct tarball URL; optional **`--sha256`** or sibling **`.sha256`**. |

**Environment:** **`DOCKPIPE_INSTALL_BASE_URL`**, optional **`DOCKPIPE_INSTALL_VERSION`**, **`DOCKPIPE_INSTALL_MANIFEST`** (default **`install-manifest.json`**), **`DOCKPIPE_INSTALL_ALLOW_INSECURE_HTTP=1`** for **`http://`** (local tests only).

**Publish (self-hosted mirror):** **`make package-templates-core`** or **`dockpipe package build core`** → **`release/artifacts/templates-core-<VERSION>.tar.gz`**, **`.sha256`**, **`release/artifacts/install-manifest.json`** (same layout as **`release/packaging/package-templates-core.sh`**; override dir with **`DOCKPIPE_ARTIFACTS_DIR`**). After **`dockpipe build`**, **`dockpipe package build store`** writes one **`.tar.gz`** (+ **`.sha256`**) per compiled package under **`bin/.dockpipe/internal/packages/`** (core, workflows, resolvers; bundles only with **`--only bundles`**) and **`packages-store-manifest.json`** — suitable for mirroring. When a workflow **`config.yml` inside that tarball** declares **`namespace:`**, **`dockpipe run --workflow <name>`** can resolve and **stream** from **`release/artifacts/dockpipe-workflow-<name>-*.tar.gz`** (see **`packages.tarball_dir`** / **`packages.namespace`** in **`dockpipe.config.json`**) if no on-disk workflow wins. Upload those files to the same **`--base-url`** path, then run your publish workflow (e.g. **`dockpipe.cloudflare.r2publish`** from **`.staging/packages/…`** when that tree is on **`compile.workflows`**) or **`dockpipe release upload <file>`** / **`aws s3 cp`** to R2. Official releases may use a different pipeline; **`package build`** / **`release upload`** are for self-hosted mirrors and in-repo mirrors.

**Archive format:** `tar czf -C src core --exclude='core/workflows'` — entries must be **`core/…`** (category dirs only; see **`release/packaging/package-templates-core.sh`**). After extract, the CLI **re-reads the tarball** and checks **every file on disk** matches the archive, then prints the **tarball sha256** (and compares to manifest/`.sha256` when present).

## `dockpipe clone`

| Command | Purpose |
|---------|---------|
| `dockpipe clone <name> [--workdir <path>] [--to <dest>] [--force]` | Copy **`bin/.dockpipe/internal/packages/workflows/<name>/`** to **`--to`** (default **`<workdir>/workflows/<name>`**) only if **`package.yml`** has **`allow_clone: true`**. Refused when **`allow_clone`** is false or omitted (typical for commercial **binary-only** drops). |

See **[package-model.md](package-model.md)** (**Clone, education, and commercial packages**).

## `dockpipe build` / `clean` / `rebuild`

Familiar names for the **compiled package store** under **`bin/.dockpipe/internal/packages/`** (see **[package-model.md](package-model.md)**).

| Command | Purpose |
|---------|---------|
| `dockpipe build [options]` | Runs **`dockpipe pipelang materialize`** first (roots from effective `compile.workflows`), then same as **`dockpipe package compile all --force`**: replaces existing compiled outputs, then materializes **core → resolvers → workflows** from **`compile.workflows`** in **`dockpipe.config.json`** (legacy **`compile.bundles`** merged into workflow roots; and defaults). After compile, prebuilds Dockerfile-backed image artifacts by default; pass **`--no-images`** to emit manifests only. If **`--workdir`** is omitted, the project directory is the folder containing **`dockpipe.config.json`** (walking up from the current directory), or the current directory if that file is absent. Options: **`--workdir`**, **`--for-workflow <name>`**, **`--images`**, **`--no-images`**. |
| `dockpipe clean [--workdir <path>]` | **`rm -rf`** the packages root (default **`<workdir>/bin/.dockpipe/internal/packages`**; respects **`DOCKPIPE_PACKAGES_ROOT`**). Does not wipe other **`bin/.dockpipe/`** files. |
| `dockpipe rebuild [options]` | **`clean`** (only **`--workdir`** is forwarded) then **`build`** with the full option set. |

## `dockpipe package`

Inspect **installed** package metadata. Store-backed installs are intended to land under **`bin/.dockpipe/internal/packages/`** (workflows, core slices, assets); see **[package-model.md](package-model.md)** for **authoring vs compiled run modes**, **install vs run network boundary**, and the **compile → package → release** pipeline (compile will grow to pull in domain assets; **`package compile workflow`** today copies the source tree after validate).

| Command | Purpose |
|---------|---------|
| `dockpipe package list [--workdir <path>]` | Walk **`bin/.dockpipe/internal/packages/`** for **`package.yml`** files; print **path**, **name**, **version**, **description** (tab-separated). |
| `dockpipe package images [--workdir <path>]` | Merge planned image artifacts from compiled workflow tarballs with materialized/cached receipts under **`bin/.dockpipe/internal/images/by-fingerprint/`**; print **fingerprint**, **status**, **state**, **source**, **image_ref**, **workflow**, **package**, **step_id**, **image_key** (tab-separated). Status can show **`ready`**, **`missing`**, **`stale`**, **`planned`**, **`referenced`**, or **`docker-error`**. |
| `dockpipe package manifest` | Print an example **`package.yml`** (schema: **name**, **version**, **title**, **description**, **author**, **website**, **license**, optional **kind**). |
| `dockpipe package build core [--repo-root <path>] [--out <dir>] [--version <ver>]` | Write **`templates/core`** tarball (**`core/…`** prefix), **`.sha256`**, **`install-manifest.json`** for **`dockpipe install core`**. Default **`--out`**: **`release/artifacts/`**. |
| `dockpipe package build store [--workdir <path>] [--out <dir>] [--only <slice>] [--version <ver>]` | Gzip tar per compiled package under **`bin/.dockpipe/internal/packages/`** (after **`dockpipe build`**), archive paths **`core/`**, **`workflows/<name>/`**, **`resolvers/<name>/`**. **`--only bundles`** adds bundle packages. **`packages-store-manifest.json`** lists artifacts. Default **`--out`**: **`release/artifacts/`**. |
| `dockpipe package compile workflow <source-dir> [--workdir <path>] [--name <n>] [--force]` | **`workflow validate`** on **`source-dir/config.yml`**, then copy the tree to **`bin/.dockpipe/internal/packages/workflows/<name>/`** (writes **`package.yml`** if missing, with **`allow_clone: true`** and **`distribution: source`** for local round-trips). |

**Environment:** **`DOCKPIPE_PACKAGES_ROOT`** — override packages root (default **`<workdir>/bin/.dockpipe/internal/packages`**).

## `dockpipe pipelang`

Optional typed authoring helper. PipeLang compiles to inspectable artifacts; workflow execution still uses normal DockPipe YAML.

| Command | Purpose |
|---------|---------|
| `dockpipe pipelang compile --in <file.pipe> [--entry <Class>] [--out <dir>]` | Parse + type-check PipeLang and emit artifacts: `<Class>.workflow.yml`, `<Class>.bindings.json`, `<Class>.bindings.env` (default out: `bin/.dockpipe/pipelang/`). |
| `dockpipe pipelang invoke --in <file.pipe> [--class <Class>] --method <name> [--arg <value>]... [--format text|json|env]` | Invoke a PipeLang method through CLI only. No resolver/runtime execution path is used. |
| `dockpipe pipelang materialize [--workdir <path>] [--from <root>]... [--force]` | Recursively compile `.pipe` files under roots into sidecar `.pipelang/` artifacts. Default roots are resolved from `dockpipe.config.json` `compile.workflows` (plus merged `compile.bundles`). |

Method support (v0.0.0.1): expression-bodied methods are parsed, type-checked, included in bindings metadata, and invocable only via `dockpipe pipelang invoke`.

## `dockpipe release`

Upload a single file to an **S3-compatible** bucket (e.g. **Cloudflare R2**) using the **`aws`** CLI. Requires **`AWS_ACCESS_KEY_ID`** and **`AWS_SECRET_ACCESS_KEY`** (or compatible credentials). Not required for **official** DockPipe distribution; use for **self-hosted** package mirrors.

| Command | Purpose |
|---------|---------|
| `dockpipe release upload <local-file> [--bucket <name>] [--key <object-key>] [--endpoint-url <url>] [--region <name>] [--content-type <ct>] [--dry-run]` | **`aws s3 cp`** to **`s3://bucket/key`**. **Bucket:** **`--bucket`** or **`DOCKPIPE_RELEASE_BUCKET`** or **`R2_BUCKET`**. **Endpoint:** **`--endpoint-url`** or **`R2_ENDPOINT_URL`** / **`AWS_ENDPOINT_URL_S3`**, or derive **`https://<id>.r2.cloudflarestorage.com`** from **`CLOUDFLARE_ACCOUNT_ID`** / **`R2_ACCOUNT_ID`**. **Key** defaults to the file’s basename. |

## Workflow variables (`--workflow`)

When you use `--workflow <name>`, the resolved **`config.yml`** (see **[workflow-yaml.md](workflow-yaml.md)** for lookup order) may include a top-level **`vars:`** block: indented `KEY: value` lines. Those names are **exported** before run scripts, **only if** they are not already set in your shell (so your environment and CI secrets win).

Additional sources (each only sets **unset** variables, except `--var`):

1. `vars:` defaults in `config.yml`
2. `.env` beside the resolved workflow **`config.yml`** (e.g. **`workflows/<name>/.env`**, **`src/core/workflows/<name>/.env`**)
3. `.env` at the bundled materialized root (default: user cache; layout under **[install.md](install.md#bundled-templates-no-extra-install-tree)**) or at **`DOCKPIPE_REPO_ROOT`** if you set it
4. Each `--env-file <path>` (in order)
5. Path in **`DOCKPIPE_ENV_FILE`** if set
6. **`--var KEY=VAL`** (always exported; overrides yml and `.env`)

Use **`--var`** for one-off overrides; use **`.env`** files for local secrets and team defaults.

## Flags

All options must appear **before** a standalone **`--`**. The command and its arguments follow **`--`**.

**`--workdir`:** Optional. If omitted, the project directory is the **current working directory** (`PWD`). **Typical local use:** **`cd`** to the repo root and run **`dockpipe`** with **no** **`--workdir`**. Use **`--workdir <path>`** when the process is not started from that directory (CI, **`make`**, wrappers) or when you want an explicit path without **`cd`**.

| Flag | Aliases | Purpose |
|------|---------|---------|
| `--workflow <name>` | | Load workflow YAML: resolution order in **`workflow_dirs.go`** — embedded **`bundle/workflows/<name>/`** (materialized cache under **`~/.cache/dockpipe/bundled-<ver>/`**), nested roots from **`compile.workflows`**, then **`dockpipe-workflow-<name>-*.tar.gz`** as **`tar://`** (read from package store), then project **`workflows/<name>/`** (or **`DOCKPIPE_WORKFLOWS_DIR`**), then legacy **`templates/`** / **`src/core/workflows/`**, then resolver delegate **`…/core/resolvers/<name>/`**. **`compile for-workflow`** still prefers on-disk **`workflows/<name>/config.yml`** when present. With **`steps:`**, a final **`--`** is optional (see **[workflow-yaml.md](workflow-yaml.md)**). Mutually exclusive with **`--workflow-file`**. |
| `--workflow-file <path>` | | Load workflow YAML from an arbitrary path (same shape as bundled **`config.yml`**). Relative **`run:`** / **`act:`** paths resolve next to that file. **Resolver** profiles load only from **`templates/core/resolvers/`** (or **`bundle/core/resolvers/`** in the materialized bundle) — not from folders beside the YAML file. Mutually exclusive with **`--workflow`**. |
| `--workflows-dir <path>` | | Repo-relative or absolute directory for **`--workflow <name>`** resolution (default **`workflows/`**). Same as **`DOCKPIPE_WORKFLOWS_DIR`**. Also **`dockpipe init <name> --workflows-dir …`**. |
| `--run <path>` | `--pre-script` | **Run:** script(s) on the host before the container. Repeatable. |
| `--isolate <name>` | `--template`, `--image` | Low-level image/template override. Builds if the value is a template. Prefer **`--runtime`** + **`--resolver`** for the normal path, and use **`--isolate`** when you need to pin the exact image/template. |
| `--act <path>` | `--action` | **Act:** script after the container command (e.g. commit-worktree). |
| `--runtime <name>` | | Main **where does this run?** flag. Selects a runtime profile from **`templates/core/runtimes/<name>`** or **`bundle/core/runtimes/<name>`**. May be combined with **`--resolver`**. |
| `--resolver <name>` | | Main **which tool/profile does this use?** flag. Selects a resolver profile from **`templates/core/resolvers/<name>`** or **`bundle/core/resolvers/<name>`** (or **`…/profile`** under the same dir). Often used with **`--repo`** / worktree flows. Both flags may be set; the runner **merges** the two files. |
| `--strategy <name>` | | Named lifecycle wrapper from **`templates/<workflow>/strategies/<name>`** or **`workflows/<workflow>/strategies/<name>`** (optional) or **`templates/core/strategies/<name>`** / **`bundle/core/strategies/<name>`** (bundle cache): runs **before** / **after** host scripts around the workflow body. Overrides **`strategy:`** in workflow YAML when both are set. |
| `--repo <url>` | | Clone URL for worktree flows. If omitted and the workflow uses **`clone-worktree.sh`**, dockpipe sets the URL from **`git remote get-url origin`** in the current dir (or **`--workdir`**). When that URL matches your **`origin`**, the worktree can be created from **your local checkout** (not only a fresh remote default). |
| `--branch <name>` | | Explicit work branch for **`--repo`** (optional). |
| `--work-branch <name>` | | When **`--repo`** is set and **`--branch`** is **omitted**, use this as the **full** branch name. If both are omitted, dockpipe generates **`prefix/<adj>-<noun>-<adj>-<noun>`** (random hyphenated slug; **`prefix`** from resolver / template — see **`domain.RandomWorkBranchSlug`** in **[branchslug.go](../src/lib/domain/branchslug.go)**). If **`--branch`** is set, it takes precedence over **`--work-branch`**. |
| `--workdir <path>` | | Host path mounted at `/work`. Default: **current directory** (see note above — omit when you **`cd`** to the project first). |
| `--work-path <path>` | | Subdirectory inside the repo used as the container working directory (`/work/<path>`). |
| `--bundle-out <path>` | | After commit-on-host, write a git bundle here (default: **current branch only**). Set **`DOCKPIPE_BUNDLE_ALL=1`** for `git bundle create … --all`. |
| `--env KEY=VAL` | | Pass env into the container. Repeatable. When `/work` is a git worktree, dockpipe also sets **`DOCKPIPE_WORKTREE_BRANCH`**, **`DOCKPIPE_WORKTREE_HEAD`**, or **`DOCKPIPE_WORKTREE_DETACHED=1`** from the **host** `git` view. |
| `--mount <host:container>` | | Extra volume mount. Repeatable. |
| `--data-dir <path>` | | Bind-mount host path for persistent tool data (`HOME` in container). |
| `--data-vol <name>` | `--data-volume` | Named Docker volume for persistent data (default: `dockpipe-data` when neither **`--data-dir`** nor **`--no-data`**). |
| `--no-data` | | Do not mount a data volume / data dir. |
| `--reinit` | | With a data volume: remove that volume before run (fresh HOME). Confirms on TTY; use **`-f`** / **`--force`** to skip confirmation. |
| `-f` / `--force` | | With **`--reinit`**, skip confirmation (warning may still print). |
| `--build <path>` | | Parsed (relative to repo root or POSIX absolute). **Not currently consumed** by the Go runner — image build dirs come from **`--isolate`** / template resolution. Kept for CLI compatibility; see **`CliOpts.BuildPath`** in **[flags.go](../src/lib/application/flags.go)**. |
| `--env-file <path>` | | Merge `KEY=VAL` lines into workflow env (unset keys only unless overridden). Repeatable. |
| `--var KEY=VAL` | | Set workflow var; **overrides** yaml / `.env`. Repeatable. |
| `-d` / `--detach` | | Run the container in the background. |
| `-h` / `--help` | | Print usage. |
| `--version`, `-v`, `-V` | | Print version (handled in **`main`** before **`Run`**). |

## Windows subcommands (`dockpipe windows …`)

**Optional.** For **`DOCKPIPE_USE_WSL_BRIDGE=1`**: install **`dockpipe`** in WSL and run **`dockpipe windows setup`** / **`doctor`**. Native **`dockpipe.exe`** users can ignore this. See **Optional: WSL bridge** in **[install.md](install.md)**.

| Command | Purpose |
|---------|---------|
| `dockpipe windows setup` | Interactive: select WSL distro, bootstrap env, optional **`--install-command`** in the distro. |
| `dockpipe windows setup --distro <name> --non-interactive` | Scripted setup (**`--distro`** required). |
| `dockpipe windows setup --bootstrap-wsl` | If WSL is missing or has no distros, run **`wsl --install`** (default distro **Alpine** when unset; may UAC / reboot), then continue. |
| `dockpipe windows setup --install-dockpipe` | Download latest **`dockpipe_*_linux_*.tar.gz`** from GitHub into **`~/.local/bin`** in WSL (for the bridge). |
| `dockpipe windows setup --install-command "<cmd>"` | Run a command inside the selected distro during setup (overrides **`--install-dockpipe`**). |
| `dockpipe windows doctor` | Print **`wsl --version`**, list distros and configured default, or hints if WSL is missing. |

### Windows host: native vs WSL bridge

**Default:** **`dockpipe.exe`** runs on **Windows**. **Docker Desktop** supplies **`docker`**; **`bash`** and **`git`** must be on **`PATH`** separately (e.g. **Git for Windows**) — Docker Desktop does not ship them.

**Container user:** **Linux/macOS** pass **`-u`** with the host **uid:gid** (overrides **`USER`** in the image). If you run **`dockpipe` as root** (e.g. **`sudo`**), that maps to **`-u 0:0`**, which breaks CLIs that refuse flags like **`--dangerously-skip-permissions`** as root. For **claude / codex / agent-dev** images, dockpipe defaults to **`-u node`** when the host uid is **0** (unless **`DOCKPIPE_FORCE_ROOT_CONTAINER=1`**). Override with **`DOCKPIPE_CONTAINER_USER`**.

**Windows:** host uid is unavailable; dockpipe **does not** pass **`-u`** unless **`DOCKPIPE_WINDOWS_CONTAINER_USER`** is set (image **`USER`** applies — e.g. **`node`** in **claude/codex** images). Defaulting **`-u node`** from the CLI caused bind-mount stalls for some Docker Desktop setups, so use an **explicit** value when needed: **`DOCKPIPE_WINDOWS_CONTAINER_USER=node`** for Claude Code with **`--dangerously-skip-permissions`**, or **`0`** for root.

**Claude Code `--dangerously-skip-permissions`:** The CLI blocks that flag when it thinks you’re root/sudo. Two mechanisms in **`assets/entrypoint.sh`:** (1) **`IS_SANDBOX=1`** is set by default (Claude Code’s supported way to treat the run as sandboxed — see **[anthropics/claude-code#9184](https://github.com/anthropics/claude-code/issues/9184)**). Already set **`IS_SANDBOX`** via **`-e`**? Left as-is. Opt out: **`DOCKPIPE_NO_SANDBOX_ENV=1`**. (2) **root → `node`** via **`runuser`**/**`setpriv`** when the container still starts as uid 0. Opt out: **`DOCKPIPE_SKIP_DROP_TO_NODE=1`**. **`DOCKPIPE_DEBUG=1`** prints **`id`**. Rebuild the image after **`entrypoint.sh`** changes (`docker rmi dockpipe-claude:…`). **`DOCKPIPE_WINDOWS_CONTAINER_USER`** remains available if you want an explicit **`-u`** from the host.

**Design: loose defaults in isolation.** Dockpipe targets **disposable containers**; defaults skew **automation-friendly** (e.g. **`IS_SANDBOX=1`**, host **uid:gid** on Unix). People who want stricter behavior can opt out with the env vars above. **Dockpipe does not append** **`--dangerously-skip-permissions`** to your command — add it after **`--`** if you want that mode. **Security** is mostly **what you mount** (only **`/work`** etc.), **secrets** in env/volumes, and **network** — not “CLI tightness” alone.

**Windows + host `git` / Docker mounts:** Bash scripts may export **`DOCKPIPE_WORKDIR`** as **`/c/Users/...`** (Git Bash). Native **`git.exe`** and **`docker -v`** need normal Windows paths; dockpipe converts MSYS-style paths before **`git -C`**, **`docker run` bind mounts**, **`--data-dir`**, and **`--mount`**. If **`/work`** looks empty in the container, rebuild with a current dockpipe and confirm the printed **Mount /work ← …** path exists on the host.

**Code edits vs host user:** On **Linux/macOS**, the container usually runs as **your host uid:gid**, so files written under **`/work`** are owned by **you** on the host (unless you override **`-u`**). **Claude**’s permission model (prompts vs **`--dangerously-skip-permissions`**) is **application-level**; it does not replace **POSIX** ownership — both apply: the process uid writes files, and Claude may still ask or gate tools unless you pass flags it documents.

**WSL bridge (opt-in):** set **`DOCKPIPE_USE_WSL_BRIDGE=1`** so every command **except** `dockpipe windows …` runs inside WSL via **`wsl.exe -d <distro>`**. The current Windows working directory is mapped with **`wslpath`**.

Environment: **`DOCKPIPE_WINDOWS_BRIDGE=1`** is set for the **inner** process when the bridge is used. **Windows-style paths** in path-like flags (`--workdir`, `--work-path`, `--data-dir`, `--mount`, `--build`, `--env-file`, `--run` / `--act`, `--isolate` / `--template` / `--image` when the value is a host path, `--env` / `--var` when the value is a path, etc.) and in **`init` / `action init` / `pre init` / `template init`** destination args are translated to WSL paths: **`wslpath -u`** first, then a **drive-letter** fallback (`C:\…` → `/mnt/c/…`). **UNC** paths (`\\server\share\…`) use a fallback that avoids breaking **`filepath.Clean`**. Everything after a standalone **`--`** is passed through unchanged.

## Examples

**Minimal (workflow does the rest):**

```bash
dockpipe --workflow my-ai --resolver claude --repo https://github.com/you/repo.git -- claude -p "Fix the bug"
```

**Override branch, add env:**

```bash
dockpipe --workflow my-ai --resolver claude --repo https://github.com/you/repo.git --branch my-feature \
  --env "GIT_PAT=$GIT_PAT" -- claude -p "Implement the plan"
```

**No workflow — override run/isolate/act:**

```bash
dockpipe --resolver claude --repo https://github.com/you/repo.git \
  --run ./my-run.sh --act ./my-act.sh -- claude -p "Task"
```
