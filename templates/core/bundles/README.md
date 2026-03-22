# Domain bundles (`templates/core/bundles/`)

**DorkPipe**, **Pipeon**, **review-pipeline**, **steam-flatpak** (niche isolate), and similar trees live **here** — **not** under DockPipe **`resolvers/`** for the **`--resolver`** profile taxonomy, except that **both** trees use the same **`assets/`** shape:

- **`assets/scripts/`** — host scripts referenced as **`scripts/<domain>/…`**
- **`assets/compose/`** — optional **`docker-compose.yml`** (e.g. **dorkpipe** dev stack)
- **`assets/docs/`** — domain markdown shipped with **`dockpipe init`** (Pipeon / DorkPipe docs)
- **`assets/images/`** — when a bundle owns an isolate image (e.g. **steam-flatpak**), **`TemplateBuild`** resolves **`bundles/<domain>/assets/images/<domain>/`**

**Resolvers** (**`templates/core/resolvers/`**) hold **`profile`**, **`config.yml`**, and the same **`assets/`** mirror for tool-specific Dockerfiles and scripts (**`scripts/cursor-dev/…`**, **`scripts/vscode/…`**, etc.). **`paths.go`** and **`template.go`** tie the layout together.

Workflows still reference bundles as **`scripts/<domain>/…`**; **`ResolveWorkflowScript`** resolves there after user **`scripts/`** and resolver paths. See **`lib/dockpipe/infrastructure/paths.go`**.
