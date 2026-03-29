# compliance-handoff

**Purpose:** One command so **humans and AI** can **discover** where compliance/security **signals** live — without adding logic to **`src/lib/dockpipe/`** (see **AGENTS.md**).

**Run (repo root, after `make build`):**

```bash
make compliance-handoff
# or
./src/bin/dockpipe --workflow compliance-handoff --workdir . --
```

**Reads:** **`docs/compliance-ai-handoff.md`** (contract for answers like “do we have compliance issues?”).

**Workflow:** `config.yml` sets **`docker_preflight: false`** so host-only **`run:`** steps skip the Docker socket check (see **`docs/workflow-yaml.md`**). Do not use that if your host scripts call **`docker`**.

**Touches:** **`scripts/dorkpipe/compliance-handoff.sh`** — prints artifact presence + short summaries.

**Copy elsewhere:** `dockpipe init myproj --from /path/to/workflows/compliance-handoff` or copy this directory.
