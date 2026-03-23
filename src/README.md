# `src/` — Go source

- **`cmd/dockpipe`** — DockPipe CLI entrypoint  
- **`cmd/dorkpipe`** — DorkPipe DAG orchestrator CLI  
- **`lib/dockpipe`** — DockPipe library (domain / application / infrastructure)  
- **`lib/dorkpipe`** — DorkPipe library  
- **`Makefile`** — `build`, `build-windows`, `test` (included from the **repository root** `Makefile`)

**`go.mod`** stays at the repository root (same module: **`dockpipe`**). **`embed.go`** also stays at the root so `//go:embed` can include `templates/`, `lib/entrypoint.sh`, and `VERSION` without `..` paths.

Run **`make`** from the **repository root** — it includes `src/Makefile` for Go targets.
