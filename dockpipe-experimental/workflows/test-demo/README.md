# test-demo

**Purpose:** **Video / full showcase** — real **`go test ./...`**, real **`go vet ./...`**, **deterministic review prep** (file list + bounded greps → **`.dockpipe/review-context.md`**), **local Ollama** summary (**`isolate: ollama`** → dockpipe builds **`dockpipe-ollama`**; **`run-local-model-with-ollama-daemon.sh`** starts **`ollama serve`** then summarizes), then a **final Codex** pass (`codex exec`, **resolver codex**, **`isolate: codex`**). Reusable scripts: **`templates/core/bundles/review-pipeline/`**. State: **`.dockpipe/outputs.env`** merge + on-disk artifacts under **`.dockpipe/`**.

**Claude variant:** workflow **`test-demo-claude`** is the same chain with a **Claude Code** review step (`claude -p`) instead of Codex — use **`--workflow test-demo-claude --resolver claude`** and **`ANTHROPIC_API_KEY`** or **`CLAUDE_API_KEY`**.

**Requirements**

- **Docker**, **bash**, Go module cache mount as below.
- **`OPENAI_API_KEY`** (or **`CODEX_API_KEY`**) **exported** (or in repo-root **`.env`**) before **`dockpipe`**. The review step sets **`CODEX_API_KEY` from `OPENAI_API_KEY`** when unset — Codex’s HTTPS fallback (after WebSocket errors) often requires **`CODEX_API_KEY`** even if **`OPENAI_API_KEY`** is set.

**Final prompt:** **`prompts/codex-final-review.md`** (short; Codex reads **`/.dockpipe/review-context.md`** on disk — not the long legacy **`codex-chain-review.md`**). **`codex exec --dangerously-bypass-approvals-and-sandbox`**: **Docker** is the isolation boundary (see **`templates/core/resolvers/codex/README.md`**). **`vars`** **`RUST_LOG=error`** on the review step. **`local-summary`** runs **Ollama** on the host (see **`scripts/review-pipeline/README.md`**); set **`DOCKPIPE_LOCAL_MODEL_CMD`** only to override the default HTTP path.

**CI** uses **`test`** (no `go test` in the lighter path; **`go vet`** only in Docker). **Govulncheck** / **gosec** stay on the **host** in CI.

```bash
export OPENAI_API_KEY="…"   # or rely on repo-root .env
dockpipe --workflow test-demo --resolver codex --runtime docker --workdir /path/to/repo \
  --mount "$(go env GOPATH)/pkg:/go/pkg:rw" --
```

See **`make demo-record`**, **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)**, **[docs/cli-reference.md](../../docs/cli-reference.md)** (env / `--var`). For DAG-style orchestration (parallel levels, pgvector, escalation) see **[docs/dorkpipe.md](../../docs/dorkpipe.md)** and **`dockpipe-experimental/workflows/dorkpipe-orchestrator/`**.
