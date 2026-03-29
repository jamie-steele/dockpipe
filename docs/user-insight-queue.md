# User insight queue (structured guidance)

**Purpose:** Capture **high-value user guidance** (risks, constraints, architecture notes, compliance concerns) as **structured, attributable signals** — not unstructured memory and not treated as verified truth.

**Separation of concerns**

| Kind | Typical location | Role |
|------|------------------|------|
| **Repo-derived facts** | e.g. `.dorkpipe/self-analysis/` | Deterministic facts from the tree |
| **System / scan findings** | e.g. `.dockpipe/ci-analysis/findings.json` | Tool output (gosec, govulncheck, …) |
| **User insights** | `.dockpipe/analysis/insights.json` | Human guidance: **signal**, not truth |

Nothing in this design is hidden state: files are JSON (or JSONL history), **schemas** live under `src/schemas/`, and classification is **rules** in `scripts/dorkpipe/user-insight-rules.json` (editable, deterministic).

The **`dockpipe --workflow user-insight-process`** entry point is the bundled workflow under **`src/core/workflows/dorkpipe/user-insight-process/`** (YAML + README); the shell steps it invokes remain under **`scripts/dorkpipe/`**.

---

## Layout (under `.dockpipe/analysis/`)

| Path | Purpose |
|------|---------|
| `queue.json` | Capture queue (`kind: dockpipe_user_insight_queue`). Append-only items with `raw_text`, `scope`, `timestamp_utc`. |
| `insights.json` | Normalized insights (`kind: dockpipe_user_insights`). `separation` + `insights[]` with provenance and status. |
| `history.jsonl` | Append-only audit events (`enqueue`, `process`, `review_*`, `mark_stale`, `supersede`). |
| `by-category/*.json` | Optional **views** (generated); canonical source is **`insights.json`**. |

**Schemas:** `src/schemas/dockpipe-user-insight-queue.schema.json`, `src/schemas/dockpipe-user-insights.schema.json`.

---

## Flow: queue → normalized → review

1. **Capture** — `scripts/dorkpipe/user-insight-enqueue.sh` appends one item to `queue.json` and logs `enqueue` in `history.jsonl`.
2. **Process** — `scripts/dorkpipe/user-insight-process.sh` reads `queue.json` + `user-insight-rules.json`, runs `jq/process-user-insight-queue.jq`, and **skips** queue items whose `queue_item_id` already appears in `insights.json`. New rows get `status` **accepted** or **pending** per rules:
   - **auto_promote_categories** (e.g. `convention`, `architecture_note`, `future_work`) → often **accepted**
   - **review_required_categories** (e.g. `compliance`, `risk`, `constraint`) → **pending**
   - Any classifier with **`force_review: true`** → **pending**
3. **Review** — `scripts/dorkpipe/user-insight-review.sh accept|reject <id> [--reason …]` updates `status` and appends history.
4. **Export** — `scripts/dorkpipe/user-insight-export-by-category.sh` writes `by-category/<category>.json` (arrays only).
5. **Lifecycle** — `user-insight-mark-stale.sh`, `user-insight-supersede.sh` link or supersede insights without deleting provenance.

---

## Scripts (repo root)

| Script | Role |
|--------|------|
| `user-insight-enqueue.sh` | Append one queue item (`-m` or stdin); optional `--category-hint`, `--repo-path`, `--component`, `--workflow`. |
| `user-insight-process.sh` | Normalize **new** queue rows into `insights.json`. |
| `user-insight-review.sh` | Accept or reject by `insight-*` or `ui-*` id. |
| `user-insight-export-by-category.sh` | Regenerate `by-category/*.json`. |
| `user-insight-mark-stale.sh` | Set `stale: true` + history. |
| `user-insight-supersede.sh` | Link new insight to old (`supersedes` / `superseded`). |

**Requires:** `jq` on `PATH`. Optional: `openssl` for enqueue nonce (falls back to `$RANDOM`).

---

## How workflows consume insights (non-authoritative)

Workflows and planners should:

1. **Load** `.dockpipe/analysis/insights.json` if present (ignore if missing).
2. **Filter** by `category`, `scope.repo_path`, `status == "accepted"` (or include `pending` only when explicitly desired).
3. **Treat** `confidence.role == "user_signal"` as **guidance**; **do not** override verified facts from scans or repo analysis.

**Example (conceptual):** a planning step merges `insights[]` where `status == "accepted"` and `stale != true` into a **prompt context** section labeled **“User guidance (unverified)”** — separate from **“CI findings”** and **“Repo facts”**.

---

## Example: user text → structured insight

**Input (enqueue):**

```bash
export DOCKPIPE_WORKDIR=/path/to/repo
bash scripts/dorkpipe/user-insight-enqueue.sh -m 'GDPR: do not log raw PII in audit trails.' --repo-path lib/dockpipe
bash scripts/dorkpipe/user-insight-process.sh
```

**Effect:** A `queue_item` is stored with `raw_text` verbatim. Processing sets `category` to **compliance** (rule match), `status` to **pending** (high-impact category), and `provenance.classifier` records the **matched pattern** for traceability. After `user-insight-review.sh accept insight-ui-…`, the insight is **accepted** for downstream use.

---

## Bundled copy

Templates ship the same docs and scripts under `templates/core/assets/` so `dockpipe init` can merge them into downstream projects.

---

## AGENTS.md

This repo may mention **AGENTS.md** for humans; **discovery does not depend on it** — use paths and schemas above.
