# Framework images (`templates/core/assets/images/`)

Dockerfiles used by **`TemplateBuild`** and **`--isolate`** for template names (`base-dev`, `dev`, `claude`, `codex`, `vscode`, `ollama`, …). Build context is always the **repository root** (for **`COPY lib/entrypoint.sh`**).

**Ollama image (`ollama/Dockerfile`):** extends **`ollama/ollama`** with bash/curl/jq/python3 and the dockpipe entrypoint; workflows use **`isolate: ollama`** so the CLI builds **`dockpipe-ollama`** like other templates. The step starts **`ollama serve`** then runs the review summary script (see **`templates/core/assets/scripts/review/run-local-model-with-ollama-daemon.sh`**).

**Codex image (`codex/Dockerfile`):** do **not** install Debian **`bubblewrap`** — distro **`bwrap`** is often too old (`--argv0`); Codex uses vendored bwrap when **`/usr/bin/bwrap`** is absent. **`ENV RUST_LOG=error`** keeps **`codex exec`** logs quieter in CI/demos (override when debugging).

**Bundling and licensing:** **[docs/templates-core-assets.md](../../../../docs/templates-core-assets.md)**.
