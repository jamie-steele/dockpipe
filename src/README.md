# `src/` — source tree

- **`cmd/`** — DockPipe CLI entrypoint (`main.go`)  
- **`cmd/dorkpipe`** — DorkPipe DAG orchestrator CLI  
- **`lib/`** (`application`, `domain`, `infrastructure`) — DockPipe library  
- **`lib/dorkpipe`** — DorkPipe library  
- **`bin/`** — Launcher scripts (`dockpipe`, `pipeon`, …) and **`make` outputs** (`dockpipe.bin`, `dorkpipe`, `dockpipe.exe`)  
- **`Makefile`** — `build`, `build-windows`, `test` (included from the **repository root** `Makefile`)

**`go.mod`** stays at the repository root (same module: **`dockpipe`**). **`embed.go`** also stays at the root so `//go:embed` can include `src/core/`, `assets/entrypoint.sh`, and `VERSION` without `..` paths.

Run **`make`** from the **repository root** — it includes `src/Makefile` for Go targets.
