#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init

action="${DORKPIPE_OPTIMIZER_ACTION:-prepare}"
target_workflow="${DORKPIPE_OPTIMIZER_TARGET_WORKFLOW:-docs.orchestrate}"
target_root="$(dockpipe_sdk scope workflow "${target_workflow}" orchestrate)"
optimizer_root="$(dockpipe_sdk scope workflow "${target_workflow}" optimize)"

resolve_path() {
  local path="${1:?path}"
  if [[ "${path}" = /* ]]; then
    printf '%s\n' "${path}"
  else
    printf '%s/%s\n' "${ROOT}" "${path}"
  fi
}

optimizer_dir="$(resolve_path "${optimizer_root}")"
target_dir="$(resolve_path "${target_root}")"
result_json="${optimizer_dir}/${action}/result.json"

mkdir -p "${optimizer_dir}/${action}"

case "${action}" in
  iterate|prepare|assess|propose|apply|apply-if-enabled|validate) ;;
  *)
    echo "orchestrate-optimize: unknown DORKPIPE_OPTIMIZER_ACTION=${action}" >&2
    exit 1
    ;;
esac

if [[ "${action}" == "iterate" ]]; then
  iterations="${DORKPIPE_OPTIMIZER_ITERATIONS:-1}"
  child_package="${DORKPIPE_OPTIMIZER_CHILD_PACKAGE:-}"
  child_workflow="${DORKPIPE_OPTIMIZER_CHILD_WORKFLOW:-docs.optimize-orchestrate}"
  target_package="${DORKPIPE_OPTIMIZER_TARGET_PACKAGE:-}"
  iteration_root="$(dockpipe_sdk scope workflow "${target_workflow}" optimize iterations)"
  stop_on_invalid_patch="${DORKPIPE_OPTIMIZER_STOP_ON_INVALID_PATCH:-1}"
  refresh_target_after_apply="${DORKPIPE_OPTIMIZER_REFRESH_TARGET_AFTER_APPLY:-0}"
  case "${iterations}" in
    ''|*[!0-9]*)
      echo "orchestrate-optimize: DORKPIPE_OPTIMIZER_ITERATIONS must be a positive integer" >&2
      exit 1
      ;;
  esac
  if (( iterations < 1 || iterations > 50 )); then
    echo "orchestrate-optimize: DORKPIPE_OPTIMIZER_ITERATIONS must be between 1 and 50" >&2
    exit 1
  fi

  if (( iterations == 1 )); then
    cat > "${result_json}" <<EOF
{
  "status": "skipped",
  "reason": "single optimizer pass",
  "iterations": 1
}
EOF
    printf '[dorkpipe] optimizer %s result ready at %s\n' "${action}" "${result_json}" >&2
    exit 0
  fi

  run_id="$(date -u +%Y%m%dT%H%M%SZ)"
  run_dir="$(resolve_path "${iteration_root}")/${run_id}"
  mkdir -p "${run_dir}"
  cat > "${run_dir}/summary.md" <<EOF
# Optimizer Iteration Run

- Child workflow: \`${child_workflow}\`
- Iterations requested: ${iterations}
- Apply enabled: ${DORKPIPE_OPTIMIZER_APPLY:-0}

EOF

  child_args=()
  if [[ -n "${child_package}" ]]; then
    child_args+=(--package "${child_package}")
  fi
  child_args+=(--workflow "${child_workflow}")

  target_args=()
  if [[ -n "${target_package}" ]]; then
    target_args+=(--package "${target_package}")
  fi
  target_args+=(--workflow "${target_workflow}")

  for i in $(seq 1 "$((iterations - 1))"); do
    printf '\n[dorkpipe] optimizer iteration %02d/%02d\n' "${i}" "${iterations}" >&2
    DORKPIPE_OPTIMIZER_ITERATIONS=1 \
      DORKPIPE_OPTIMIZER_ITERATION="${i}" \
      dockpipe "${child_args[@]}" --

    iter_dir="${run_dir}/iter-${i}"
    mkdir -p "${iter_dir}"
    if [[ -d "${optimizer_dir}" ]]; then
      mkdir -p "${iter_dir}/optimize"
      while IFS= read -r -d '' optimizer_item; do
        if [[ "$(basename "${optimizer_item}")" == "iterations" ]]; then
          continue
        fi
        cp -a "${optimizer_item}" "${iter_dir}/optimize/"
      done < <(find "${optimizer_dir}" -mindepth 1 -maxdepth 1 -print0)
    fi
    orch_dir="$(resolve_path "${DORKPIPE_ORCH_ROOT}")"
    if [[ -d "${orch_dir}" ]]; then
      cp -a "${orch_dir}" "${iter_dir}/orchestrate"
    fi
    git -C "${ROOT}" status --short > "${iter_dir}/git-status.txt" || true
    printf -- '- iter-%02d: `%s`\n' "${i}" "${iter_dir}" >> "${run_dir}/summary.md"

    apply_status="$(
      python3 - "${optimizer_dir}/apply-if-enabled/result.json" <<'PY' 2>/dev/null || true
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
if path.exists():
    print((json.loads(path.read_text()).get("status") or "").strip())
PY
    )"
    propose_invalid="$(
      python3 - "${optimizer_dir}/propose/result.json" <<'PY' 2>/dev/null || true
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
if not path.exists():
    print("false")
    raise SystemExit
data = json.loads(path.read_text())
print("true" if data.get("status") == "review" and data.get("validation_error") and data.get("changed_files") else "false")
PY
    )"
    if [[ "${propose_invalid}" == "true" ]]; then
      printf '[dorkpipe] optimizer iteration %02d stopped: Codex proposed an invalid patch\n' "${i}" >&2
      cat > "${result_json}" <<EOF
{
  "status": "stopped",
  "reason": "invalid_patch",
  "iterations": ${iterations},
  "completed_child_iterations": ${i},
  "run_dir": "${run_dir}"
}
EOF
      if [[ "${stop_on_invalid_patch}" =~ ^(1|true|yes|on)$ ]]; then
        exit 1
      fi
    fi

    if [[ "${apply_status}" == "applied" && "${refresh_target_after_apply}" =~ ^(1|true|yes|on)$ ]]; then
      printf '[dorkpipe] optimizer iteration %02d/%02d refreshing target workflow %s\n' "${i}" "${iterations}" "${target_workflow}" >&2
      DORKPIPE_ORCH_APPROVAL_MODE=auto-no \
        DORKPIPE_ORCH_SKIP_APPLY=1 \
        DORKPIPE_DEV_STACK_RELOAD=1 \
        dockpipe "${target_args[@]}" --
    fi
  done

  cat > "${result_json}" <<EOF
{
  "status": "ready",
  "iterations": ${iterations},
  "completed_child_iterations": $((iterations - 1)),
  "final_iteration": "current workflow",
  "child_package": "${child_package}",
  "child_workflow": "${child_workflow}",
  "run_dir": "${run_dir}"
}
EOF
  printf '[dorkpipe] optimizer %s result ready at %s\n' "${action}" "${result_json}" >&2
  exit 0
fi

python3 - "${action}" "${ROOT}" "${target_dir}" "${optimizer_dir}" "${DORKPIPE_ORCH_ROOT}" "${DORKPIPE_ORCH_APPROVAL_MD}" "${result_json}" <<'PY'
import json
import pathlib
import re
import subprocess
import sys

action = sys.argv[1]
root = pathlib.Path(sys.argv[2]).resolve()
target_dir = pathlib.Path(sys.argv[3]).resolve()
optimizer_dir = pathlib.Path(sys.argv[4]).resolve()
orch_root = pathlib.Path(sys.argv[5]).resolve()
approval_path = pathlib.Path(sys.argv[6]).resolve()
result_path = pathlib.Path(sys.argv[7]).resolve()

target_workflow_config = root / "workflows/agent/docs.orchestrate/config.yml"
verifier_script = root / "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-verify-results.sh"
patch_path = optimizer_dir / "proposed.patch"
assessment_md = optimizer_dir / "assessment.md"
recommendation_md = optimizer_dir / "recommendation.md"
history_dir = optimizer_dir / "history"

allowed_files = [
    root / "workflows/agent/docs.optimize-orchestrate/README.md",
    root / "workflows/agent/docs.optimize-orchestrate/config.yml",
    target_workflow_config,
    root / "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-optimize.sh",
    verifier_script,
]

def write_json(data):
    result_path.parent.mkdir(parents=True, exist_ok=True)
    result_path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")

def rel(path):
    return str(path.resolve().relative_to(root))

def display_path(path):
    path = path.resolve()
    try:
        return str(path.relative_to(root))
    except ValueError:
        return str(path)

def read_text(path):
    if not path.exists():
        return ""
    return path.read_text(encoding="utf-8", errors="replace")

def snapshot_previous_optimizer_run():
    history_dir.mkdir(parents=True, exist_ok=True)
    copies = [
        (recommendation_md, history_dir / "previous-recommendation.md"),
        (patch_path, history_dir / "previous-proposed.patch"),
        (optimizer_dir / "propose" / "result.json", history_dir / "previous-propose-result.json"),
        (orch_root / "tasks" / "codex_patch_decision" / "response.md", history_dir / "previous-codex-response.md"),
        (orch_root / "merge" / "final.md", history_dir / "previous-merge-final.md"),
        (orch_root / "verify" / "result.json", history_dir / "previous-verify-result.json"),
    ]
    snapshot = []
    for src, dst in copies:
        if src.exists() and src.is_file():
            dst.write_text(src.read_text(encoding="utf-8", errors="replace"), encoding="utf-8")
            snapshot.append(display_path(dst))
    snapshot_lines = [f"- `{item}`" for item in snapshot] if snapshot else ["- No previous optimizer artifacts were available."]
    (history_dir / "previous-run-summary.md").write_text(
        "\n".join([
            "# Previous Optimizer Run",
            "",
            *snapshot_lines,
            "",
        ]),
        encoding="utf-8",
    )

def latest_target_snapshot():
    files = {
        "merge": target_dir / "merge/result.json",
        "verify": target_dir / "verify/result.json",
        "cloud_usage": target_dir / "cloud-usage.json",
    }
    responses = sorted((target_dir / "tasks").glob("*/response.md")) if (target_dir / "tasks").exists() else []
    return files, responses

def collect_issues():
    files, responses = latest_target_snapshot()
    issues = []
    verify = {}
    if files["verify"].exists():
        try:
            verify = json.loads(files["verify"].read_text(encoding="utf-8"))
            for issue in verify.get("issues", []) or []:
                issues.append(f"verifier: {issue}")
        except Exception as exc:
            issues.append(f"verify/result.json could not be parsed: {exc}")
    else:
        issues.append(f"missing target verify artifact: {files['verify'].relative_to(root)}")

    smell_patterns = [
        (re.compile(r"(?im)^\s*(?:Note|Please note)\s*:"), "note/footer instead of direct artifact content"),
        (re.compile(r"(?im)^\s*Here (?:are|is)\b"), "preamble before requested artifact"),
        (re.compile(r"\bcould not be completed due to lack of information\b", re.I), "false missing-information footer"),
        (re.compile(r"\badheres to (?:the )?(?:specified )?formatting\b", re.I), "formatting commentary"),
    ]
    for response in responses:
        text = response.read_text(encoding="utf-8", errors="replace")
        task_id = response.parent.name
        for pattern, label in smell_patterns:
            if pattern.search(text):
                issues.append(f"{task_id}: {label}")
                break
    return issues, verify, responses

def codex_response_path():
    return orch_root / "tasks" / "codex_patch_decision" / "response.md"

def extract_unified_diff(text):
    fences = re.findall(r"```(?:diff|patch)?\s*\n(.*?)```", text, flags=re.I | re.S)
    candidates = fences if fences else [text]
    for candidate in candidates:
        candidate = candidate.strip() + "\n"
        if "--- a/" in candidate and "+++ b/" in candidate and "@@" in candidate:
            return candidate
    return ""

def validate_patch_text(text):
    if not text.strip():
        return False, "codex response did not include a unified diff"
    allowed = {p.resolve() for p in allowed_files}
    touched = []
    for line in text.splitlines():
        if not line.startswith(("--- a/", "+++ b/")):
            continue
        path = line[6:]
        if path == "/dev/null":
            continue
        candidate = (root / path).resolve()
        if candidate not in allowed:
            return False, f"patch touches non-allowlisted path: {path}"
        touched.append(path)
    if not touched:
        return False, "patch did not declare any allowlisted file paths"
    check = subprocess.run(["git", "apply", "--recount", "--check", "-"], cwd=root, text=True, input=text, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if check.returncode != 0:
        return False, check.stdout.strip() or "git apply --check failed"
    return True, ""

def write_assessment():
    issues, verify, responses = collect_issues()
    lines = [
        "# DorkPipe Ollama Optimizer Assessment",
        "",
        f"- Target artifact root: `{target_dir.relative_to(root) if target_dir.is_relative_to(root) else target_dir}`",
        f"- Target verify status: `{verify.get('status', 'missing')}`",
        f"- Target confidence: `{verify.get('confidence', 'unknown')}`",
        f"- Response artifacts inspected: {len(responses)}",
        "",
        "## Findings",
        "",
    ]
    if issues:
        lines.extend(f"- {issue}" for issue in issues)
    else:
        lines.append("- No known optimizer smell patterns found in the latest target run.")
    lines.extend([
        "",
        "## Optimizer Policy",
        "",
        "- Keep this loop local-first with Ollama workers.",
        "- Let Codex make the code-change decision.",
        "- Write proposed patch artifacts only; never modify the working tree in proposal mode.",
        "- Restrict edits to the docs orchestration workflow and DorkPipe verifier heuristics.",
        "",
    ])
    assessment_md.write_text("\n".join(lines), encoding="utf-8")
    write_json({
        "status": "ready",
        "target_root": str(target_dir.relative_to(root)) if target_dir.is_relative_to(root) else str(target_dir),
        "issues": issues,
        "assessment": display_path(assessment_md),
    })

def write_patch():
    write_assessment()
    response_path = codex_response_path()
    response_text = read_text(response_path)
    patch_text = extract_unified_diff(response_text)
    valid, validation_error = validate_patch_text(patch_text)
    if valid:
        patch_path.write_text(patch_text, encoding="utf-8")
    else:
        patch_path.write_text("", encoding="utf-8")
    changed_files = []
    for line in patch_text.splitlines():
        if line.startswith("+++ b/"):
            path = line[6:]
            if path != "/dev/null" and path not in changed_files:
                changed_files.append(path)
    scope_lines = [f"- `{item}`" for item in changed_files] if changed_files else ["- No valid patch proposed."]
    recommendation_md.write_text(
        "\n".join([
            "# DorkPipe Codex Optimizer Recommendation",
            "",
            "Codex-authored patch proposal. Review this artifact before applying anything to the working tree.",
            "",
            "## Proposed Scope",
            "",
            *scope_lines,
            "",
            "## Why",
            "",
            "- Codex owns the code-change decision in this workflow.",
            "- DorkPipe validates the diff path allowlist and `git apply --check` only.",
            "- Proposal mode never modifies the working tree and never commits.",
            "",
        ]),
        encoding="utf-8",
    )
    write_json({
        "status": "ready" if valid else "review",
        "patch": display_path(patch_path),
        "recommendation": display_path(recommendation_md),
        "codex_response": display_path(response_path),
        "changed_files": changed_files,
        "validation_error": validation_error,
        "applied": False,
    })

def check_patch_paths():
    if not patch_path.exists():
        raise SystemExit(f"missing proposed patch: {patch_path}")
    text = patch_path.read_text(encoding="utf-8")
    for line in text.splitlines():
        if not line.startswith(("--- a/", "+++ b/")):
            continue
        path = line[6:]
        candidate = (root / path).resolve()
        if candidate not in [p.resolve() for p in allowed_files]:
            raise SystemExit(f"patch touches non-allowlisted path: {path}")

def apply_patch():
    if not apply_enabled():
        write_json({
            "status": "skipped",
            "reason": "set DORKPIPE_OPTIMIZER_APPLY=1 to apply the proposed patch to the working tree",
            "patch": display_path(patch_path),
            "commit": False,
        })
        return
    check_patch_paths()
    if not patch_path.read_text(encoding="utf-8").strip():
        write_json({"status": "noop", "reason": "proposed patch is empty"})
        return
    subprocess.run(["git", "apply", "--recount", "--check", str(patch_path)], cwd=root, check=True)
    subprocess.run(["git", "apply", "--recount", str(patch_path)], cwd=root, check=True)
    write_json({
        "status": "applied",
        "patch": display_path(patch_path),
        "applied_files": [rel(p) for p in allowed_files],
        "commit": False,
    })

def apply_enabled():
    import os
    return str(os.environ.get("DORKPIPE_OPTIMIZER_APPLY", "")).strip().lower() in {"1", "true", "yes", "on"}

def validate():
    commands = [
        ["./src/bin/dockpipe", "workflow", "validate", "workflows/agent/docs.optimize-orchestrate/config.yml"],
        ["./src/bin/dockpipe", "workflow", "validate", "workflows/agent/docs.orchestrate/config.yml"],
    ]
    results = []
    ok = True
    for command in commands:
        proc = subprocess.run(command, cwd=root, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        results.append({
            "command": " ".join(command),
            "exit_code": proc.returncode,
            "output": proc.stdout[-4000:],
        })
        if proc.returncode != 0:
            ok = False
    write_json({
        "status": "pass" if ok else "fail",
        "results": results,
    })
    if not ok:
        raise SystemExit("optimizer validation failed")

if action in {"prepare", "assess"}:
    snapshot_previous_optimizer_run()
    write_assessment()
elif action == "propose":
    write_patch()
elif action == "apply":
    apply_patch()
elif action == "apply-if-enabled":
    apply_patch()
elif action == "validate":
    validate()
else:
    raise SystemExit(f"unknown action {action}")
PY

printf '[dorkpipe] optimizer %s result ready at %s\n' "${action}" "${result_json}" >&2
