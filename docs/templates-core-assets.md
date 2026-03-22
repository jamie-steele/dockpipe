# `templates/core/assets/` — reusable support files

**Assets** are reusable **implementation/support artifacts** (scripts, image definitions, optional Compose examples). They are **not** new top-level primitives: **workflow**, **runtime**, **resolver**, and **strategy** remain defined in **[architecture-model.md](architecture-model.md)**.

---

## Layout

```
templates/core/assets/
  README.md
  scripts/          # shell, PowerShell, …
  images/           # Dockerfiles for TemplateBuild / --isolate
  compose/          # optional docker-compose examples (not executed by dockpipe)
```

Merged into user projects by **`dockpipe init`** as part of **`templates/core/`**.

---

## Classification legend

| Label | Meaning |
|-------|---------|
| **SAFE TO BUNDLE** | DockPipe-authored or clearly redistributable; safe to ship in the binary. |
| **REFERENCE ONLY** | Upstream image or user build; DockPipe ships only our Dockerfile overlay or pointers. |
| **USER-SUPPLIED** (runtime) | Credentials, licenses, or installations the user provides. |

---

## Scripts (`assets/scripts/`)

| Asset | Classification | Notes |
|-------|----------------|-------|
| Host helpers (`clone-worktree.sh`, `commit-worktree.sh`, …) | **SAFE TO BUNDLE** | Original shell. |
| `example-run.sh`, `example-act.sh` | **SAFE TO BUNDLE** | Samples copied to project **`scripts/`**. |
| `cursor-*.sh`, `vscode-code-server.sh` | **SAFE TO BUNDLE** | Invoke user environment; tools may be **USER-SUPPLIED**. |
| `helloworld.ps1` | **SAFE TO BUNDLE** | Minimal PowerShell example asset. |

---

## Images (`assets/images/`)

| Image dir | Classification | Notes |
|-----------|----------------|-------|
| `base-dev/`, `dev/`, `example/` | **SAFE TO BUNDLE** | OSS base images + entrypoint. |
| `claude/`, `codex/` | **SAFE TO BUNDLE** (Dockerfile) / **USER-SUPPLIED** (runtime) | Public npm installs; API access is user-owned. |
| `vscode/` | **SAFE TO BUNDLE** (Dockerfile) / **REFERENCE ONLY** (base) | `FROM codercom/code-server:latest` + dockpipe entrypoint for **isolate** runs. |
| `code-server/` | **SAFE TO BUNDLE** (Dockerfile) / **REFERENCE ONLY** (base) | **Browser** code-server: Coder image + Pipeon extension + defaults; **no** dockpipe entrypoint. Build **`dockpipe-code-server:latest`**. |

---

## Compose (`assets/compose/`)

**Compose is an asset**, not a runtime, resolver, or strategy. Optional **`docker-compose.yml`** examples under **`compose/<resolver-name>/`** illustrate multi-service or sidecar setups; normal **`dockpipe --resolver …`** flows do **not** require Compose.

See **`templates/core/assets/compose/README.md`** in the repo.

---

## PowerShell example

**`assets/scripts/helloworld.ps1`** — minimal reusable example; not a primitive.

---

## What stays outside `templates/core/`

- Top-level **`scripts/README.md`** and **`images/README.md`** — repo pointers only.
