# user-insight-process

**Location:** maintainer **`dockpipe`** package — **`resolvers/user-insight-process/`**. **`--workflow`** name: **`user-insight-process`**.

**Purpose:** Normalize **`queue.json`** → **`insights.json`** on the host (requires **`jq`** and **`scripts/dorkpipe/user-insight-rules.json`** at repo root).

```bash
bash scripts/dorkpipe/user-insight-enqueue.sh -m 'Your guidance here.'
./src/bin/dockpipe --workflow user-insight-process --workdir . --
```

## Layout (under `.dockpipe/analysis/`)

| File | Role |
|------|------|
| `queue.json` | Incoming items |
| `insights.json` | Normalized rows + status |
| `history.jsonl` | Audit trail |
| `by-category/*.json` | Optional views (generated) |

**Schemas:** `src/schemas/dockpipe-user-insight-queue.schema.json`, `dockpipe-user-insights.schema.json`.

**Repo overview:** **`docs/artifacts.md`** § User insight queue.
