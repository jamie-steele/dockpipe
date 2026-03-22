# Reusable review prep (`scripts/review/`)

Deterministic helpers that write **`.dockpipe/`** artifacts for a **final** resolver step (e.g. Codex), so the expensive LLM pass reads a **compact bundle** instead of rediscovering the repo.

| Script | Role |
|--------|------|
| **`collect-go-review-signals.sh`** | Go file list (bounded) + bounded `grep` pattern hits. |
| **`aggregate-review-context.sh`** | Builds **`review-context.md`**, **`review-context.json`**, **`review-summary.env`** from env + collect outputs. |
| **`optional-local-model-summary.sh`** | Calls **Ollama** over HTTP at **`OLLAMA_HOST`** (default **`http://127.0.0.1:11434`**) with **`DOCKPIPE_OLLAMA_MODEL`** (default **`llama3.2`**), writes **`.dockpipe/local-model-notes.txt`** and status env. Requires **`curl`** and **`jq`** or **`python3`** for JSON. On the **host** only: **`DOCKPIPE_OLLAMA_DOCKER=1`** can start **`ollama/ollama`** (container **`dockpipe-ollama-local`**) if nothing is listening. **`DOCKPIPE_LOCAL_MODEL_CMD`** overrides the whole call (advanced). |
| **`run-local-model-with-ollama-daemon.sh`** | For **`isolate: ollama`** (dockpipe-built **`dockpipe-ollama`**): runs **`ollama serve`**, waits for the API, then **`optional-local-model-summary.sh`**. |

**Workflow integration:** **`test-demo`** uses **`isolate: ollama`** so dockpipe **builds** the image via **`TemplateBuild`** (same as `codex`, `claude`, …). Merge keys into **`.dockpipe/outputs.env`** as usual. For a **host-only** Ollama (no dockpipe image), use **`skip_container: true`** + **`run: …/optional-local-model-summary.sh`** instead.

**Promotion:** copy or reference these from **`templates/core/assets/scripts/`** in resolver-oriented workflows; keep resolver-specific flags in **resolver profiles** / workflow YAML.
