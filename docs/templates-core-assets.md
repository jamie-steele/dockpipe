# `templates/core/assets/` — reusable support files

**Assets** are reusable **implementation/support artifacts** (scripts, image definitions, optional Compose examples). They are **not** new top-level primitives: **workflow**, **runtime**, **resolver**, and **strategy** remain defined in **[architecture-model.md](architecture-model.md)**.

---

## Layout

```
templates/core/
  assets/
    README.md
    scripts/          # agnostic shell/PowerShell only
    images/           # Dockerfiles for TemplateBuild / --isolate
    compose/          # optional docker-compose examples (not executed by dockpipe)
  bundles/            # domain asset packs (dorkpipe, pipeon, review-pipeline, …)
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
| Host helpers (`clone-worktree.sh`, `commit-worktree.sh`, …) | **SAFE TO BUNDLE** | Original shell — **agnostic**; stay at **`assets/scripts/`** root. |
| `example-run.sh`, `example-act.sh` | **SAFE TO BUNDLE** | Samples copied to project **`scripts/`**. |
| **`scripts/cursor-dev/*.sh`**, **`scripts/vscode/vscode-code-server.sh`** | **SAFE TO BUNDLE** | Implemented only under **`templates/core/resolvers/<name>/`**; the runner maps **`scripts/…`** paths there. Resolver-specific; tools may be **USER-SUPPLIED**. |
| **`scripts/dorkpipe/`**, **`scripts/pipeon/`**, **`scripts/review-pipeline/`**, … | **SAFE TO BUNDLE** | **Domain asset packs** — **not** DockPipe resolvers; canonical tree **`templates/core/bundles/<domain>/`** (runner maps **`scripts/…`** there). |
| `helloworld.ps1` | **SAFE TO BUNDLE** | Minimal PowerShell example asset. |

---

## Images

**`TemplateBuild`** / **`DockerfileDir`** search: **`resolvers/<name>/assets/images/<name>`** → **`bundles/<domain>/assets/images/<domain>`** → **`assets/images/<name>`** (agnostic fallback).

| Location | Classification | Notes |
|----------|------------------|-------|
| **`assets/images/`** | **Agnostic** | **`base-dev/`**, **`dev/`**, **`example/`**, **`minimal/`** — shared bases and demos. |
| **`resolvers/…/assets/images/`** | **Per resolver** | **claude**, **codex**, **vscode**, **code-server**, **ollama**, … |
| **`bundles/…/assets/images/`** | **Per bundle** | E.g. **steam-flatpak**. |

---

## Compose

**Compose is an asset**, not a runtime, resolver, or strategy. Optional **`docker-compose.yml`** files live with the domain that owns them: **`templates/core/resolvers/<name>/assets/compose/docker-compose.yml`** or **`templates/core/bundles/<domain>/assets/compose/`** (e.g. DorkPipe dev stack). **`templates/core/assets/compose/README.md`** only explains the pattern; per-example YAMLs are not parked under generic **`assets/compose/<name>/`** anymore.

See **`templates/core/assets/compose/README.md`** in the repo.

---

## PowerShell example

**`assets/scripts/helloworld.ps1`** — minimal reusable example; not a primitive.

---

## What stays outside `templates/core/`

- Top-level **`scripts/README.md`** and **`images/README.md`** — repo pointers only.
