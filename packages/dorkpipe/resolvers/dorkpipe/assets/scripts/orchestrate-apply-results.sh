#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init
[[ -f "${DORKPIPE_ORCH_PLAN_JSON}" ]] || { echo "missing plan artifact: ${DORKPIPE_ORCH_PLAN_JSON}" >&2; exit 1; }

if [[ "${DORKPIPE_ORCH_SKIP_APPLY:-0}" =~ ^(1|true|yes|on)$ ]]; then
  mkdir -p "${DORKPIPE_ORCH_APPLY_DIR}"
  cat > "${DORKPIPE_ORCH_APPLY_DIR}/result.json" <<EOF
{
  "status": "skipped",
  "reason": "DORKPIPE_ORCH_SKIP_APPLY is enabled",
  "applied": []
}
EOF
  printf '[dorkpipe] apply skipped at %s\n' "${DORKPIPE_ORCH_APPLY_DIR}/result.json" >&2
  exit 0
fi

python3 - "${ROOT}" "${DORKPIPE_ORCH_ROOT}" "${DORKPIPE_ORCH_PLAN_JSON}" "${DORKPIPE_ORCH_APPROVAL_MD}" "${DORKPIPE_ORCH_APPLY_DIR}/result.json" <<'PY'
import json
import pathlib
import shutil
import sys

root = pathlib.Path(sys.argv[1]).resolve()
artifact_root = pathlib.Path(sys.argv[2]).resolve()
plan_path = pathlib.Path(sys.argv[3])
approval_path = pathlib.Path(sys.argv[4])
result_path = pathlib.Path(sys.argv[5])

plan = json.loads(plan_path.read_text(encoding="utf-8"))
apply_cfg = plan.get("apply", {}) or {}
outputs = apply_cfg.get("outputs", []) or []
require_approval = bool(apply_cfg.get("require_approval", True))

def fail(message):
    result_path.parent.mkdir(parents=True, exist_ok=True)
    result_path.write_text(json.dumps({
        "status": "skipped",
        "reason": message,
        "applied": [],
    }, indent=2) + "\n", encoding="utf-8")
    raise SystemExit(message)

if not outputs:
    fail("no apply outputs declared")

if require_approval:
    if not approval_path.exists():
        fail("approval artifact is required before apply")
    approval_text = approval_path.read_text(encoding="utf-8")
    if "- Approved: yes" not in approval_text:
        fail("approval artifact does not approve apply")

applied = []
for item in outputs:
    if not isinstance(item, dict):
        fail("apply outputs must be mapping entries")
    source = str(item.get("source", "")).strip()
    target = str(item.get("path", "")).strip()
    if not source or not target:
        fail("each apply output needs source and path")

    source_path = (artifact_root / source).resolve()
    target_path = (root / target).resolve()
    try:
        source_path.relative_to(artifact_root)
    except ValueError:
        fail(f"apply source escapes artifact root: {source}")
    try:
        target_path.relative_to(root)
    except ValueError:
        fail(f"apply target escapes worktree: {target}")
    if not source_path.is_file():
        fail(f"apply source is missing: {source}")

    target_path.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source_path, target_path)
    applied.append({
        "source": str(source_path.relative_to(root)) if source_path.is_relative_to(root) else str(source_path),
        "path": str(target_path.relative_to(root)),
        "bytes": target_path.stat().st_size,
    })

result_path.parent.mkdir(parents=True, exist_ok=True)
result_path.write_text(json.dumps({
    "status": "applied",
    "applied": applied,
}, indent=2) + "\n", encoding="utf-8")
PY

printf '[dorkpipe] apply result ready at %s\n' "${DORKPIPE_ORCH_APPLY_DIR}/result.json" >&2
