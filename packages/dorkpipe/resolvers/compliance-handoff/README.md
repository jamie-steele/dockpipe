# compliance-handoff

**Purpose:** One command so **humans and AI** can **discover** where compliance/security **signals** live — without adding logic to **`src/lib/`** (see **AGENTS.md**).

**Run (repo root, after `make build`):**

```bash
make compliance-handoff
# or
dockpipe --workflow compliance-handoff --workdir . --
```

**Reads:** **`docs/artifacts.md`** (governance / artifact contract).

**Workflow:** `config.yml` sets **`docker_preflight: false`** so host-only **`run:`** steps skip the Docker socket check (see **`docs/workflow-yaml.md`**). Do not use that if your host scripts call **`docker`**.

**Touches:** **`scripts/dorkpipe/compliance-handoff.sh`** — prints artifact presence + short summaries.

**Copy elsewhere:** keep this as a packaged workflow/resolver tree or clone the directory intentionally; this is not a bundled `dockpipe init --from ...` starter template.
