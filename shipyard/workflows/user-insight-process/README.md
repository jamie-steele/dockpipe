# user-insight-process

**Purpose:** Run **`user-insight-process`** + **`user-insight-export-by-category`** on the host after you have captured items with **`user-insight-enqueue.sh`**.

**Prerequisites:** `jq`, **`scripts/dorkpipe/user-insight-rules.json`**.

**From repo root (after `make build`).**

```bash
export DOCKPIPE_WORKDIR="$PWD"
bash scripts/dorkpipe/user-insight-enqueue.sh -m 'Your guidance here.'
./src/bin/dockpipe --workflow user-insight-process --workdir . --
```

**Docs:** **`docs/user-insight-queue.md`**
