#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"

dorkpipe_orchestrate_init

action="${DORKPIPE_OPTIMIZER_ACTION:-prepare}"
target_workflow="${DORKPIPE_OPTIMIZER_TARGET_WORKFLOW:-docs.orchestrate}"
target_root="${DORKPIPE_OPTIMIZER_TARGET_ROOT:-bin/.dockpipe/packages/dorkpipe/orchestrate/${target_workflow}}"
optimizer_root="${DORKPIPE_OPTIMIZER_ROOT:-bin/.dockpipe/packages/dorkpipe/optimize/${target_workflow}}"

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
  prepare|assess|propose|apply|validate) ;;
  *)
    echo "orchestrate-optimize: unknown DORKPIPE_OPTIMIZER_ACTION=${action}" >&2
    exit 1
    ;;
esac

python3 - "${action}" "${ROOT}" "${target_dir}" "${optimizer_dir}" "${DORKPIPE_ORCH_ROOT}" "${DORKPIPE_ORCH_APPROVAL_MD}" "${result_json}" <<'PY'
import difflib
import json
import pathlib
import re
import shutil
import subprocess
import sys

action = sys.argv[1]
root = pathlib.Path(sys.argv[2]).resolve()
target_dir = pathlib.Path(sys.argv[3]).resolve()
optimizer_dir = pathlib.Path(sys.argv[4]).resolve()
orch_root = pathlib.Path(sys.argv[5]).resolve()
approval_path = pathlib.Path(sys.argv[6]).resolve()
result_path = pathlib.Path(sys.argv[7]).resolve()

target_workflow_config = root / "packages/agent/workflows/docs.orchestrate/config.yml"
verifier_script = root / "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-verify-results.sh"
patch_path = optimizer_dir / "proposed.patch"
assessment_md = optimizer_dir / "assessment.md"
recommendation_md = optimizer_dir / "recommendation.md"

allowed_files = [
    target_workflow_config,
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
        "- Apply only after approval.",
        "- Write into the working tree only; never commit.",
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

def improve_workflow_config(text):
    text = text.replace(
        '              - Return exactly three bullets and no headings.\n'
        '              - Bullet 1 must start with "repo_shape:"',
        '              - Return exactly three bullets and no headings.\n'
        '              - The first character of the response must be "-".\n'
        '              - Do not add preamble, explanation, Note, Please note, or formatting commentary.\n'
        '              - Bullet 1 must start with "repo_shape:"',
    )
    text = text.replace(
        '              - Return exactly two bullets and no headings.\n'
        '              - Do not mention packages/agent ownership',
        '              - Return exactly two bullets and no headings.\n'
        '              - The first character of the response must be "-".\n'
        '              - Do not add preamble, explanation, Note, Please note, or formatting commentary.\n'
        '              - Do not mention packages/agent ownership',
    )
    text = text.replace(
        '              - Return exactly three bullets and no headings.\n'
        '              - Bullet 1 must start with "packages/agent owns"',
        '              - Return exactly three bullets and no headings.\n'
        '              - The first character of the response must be "-".\n'
        '              - Do not add preamble, explanation, Note, Please note, or formatting commentary.\n'
        '              - Bullet 1 must start with "packages/agent owns"',
    )
    text = text.replace(
        '              - Return exactly three bullets and no headings.\n'
        '              - Bullet 1 must start with "Verification".',
        '              - Return exactly three bullets and no headings.\n'
        '              - The first character of the response must be "-".\n'
        '              - Do not add preamble, explanation, Note, Please note, or formatting commentary.\n'
        '              - Bullet 1 must start with "Verification".',
    )
    text = text.replace(
        '              - Call out uncertainty explicitly.',
        '              - Only call out uncertainty if a required referenced file is missing; do not add an uncertainty footer.',
    )
    text = text.replace(
        '              - Do not add preamble, headings, examples, or an uncertainty footer unless a referenced file is missing.',
        '              - Do not add preamble, headings, examples, Note, Please note, formatting commentary, or an uncertainty footer unless a referenced file is missing.',
    )
    return text

def improve_verifier(text):
    needle = '    (re.compile(r"(?im)^\\s*Here (?:are|is)\\b"), "worker included preamble instead of direct artifact content"),\n'
    insert = (
        needle
        + '    (re.compile(r"(?im)^\\s*(?:Note|Please note)\\s*:"), "worker added a note/footer instead of direct artifact content"),\n'
        + '    (re.compile(r"\\bcould not be completed due to lack of information\\b", re.I), "worker added a false missing-information footer"),\n'
        + '    (re.compile(r"\\badheres to (?:the )?(?:specified )?formatting\\b", re.I), "worker added formatting commentary instead of direct artifact content"),\n'
    )
    if "worker added a note/footer instead of direct artifact content" not in text:
        text = text.replace(needle, insert)
    return text

def proposed_contents():
    changes = {}
    changes[target_workflow_config] = improve_workflow_config(read_text(target_workflow_config))
    changes[verifier_script] = improve_verifier(read_text(verifier_script))
    return changes

def write_patch():
    write_assessment()
    changes = proposed_contents()
    diff_lines = []
    changed_files = []
    for path, new_text in changes.items():
        old_text = read_text(path)
        if old_text == new_text:
            continue
        changed_files.append(rel(path))
        diff_lines.extend(difflib.unified_diff(
            old_text.splitlines(keepends=True),
            new_text.splitlines(keepends=True),
            fromfile=f"a/{rel(path)}",
            tofile=f"b/{rel(path)}",
        ))
    patch_path.write_text("".join(diff_lines), encoding="utf-8")
    recommendation_md.write_text(
        "\n".join([
            "# DorkPipe Ollama Optimizer Recommendation",
            "",
            "Apply the proposed patch after reviewing the latest optimizer synthesis and target run artifacts.",
            "",
            "## Proposed Scope",
            "",
            *(f"- `{item}`" for item in changed_files),
            "",
            "## Why",
            "",
            "- Tightens worker prompts so local Ollama workers return direct bounded artifacts.",
            "- Tightens verifier heuristics so preambles, note footers, false missing-information footers, and formatting commentary route to review.",
            "- Leaves commits to the human operator.",
            "",
        ]),
        encoding="utf-8",
    )
    write_json({
        "status": "ready" if diff_lines else "noop",
        "patch": display_path(patch_path),
        "recommendation": display_path(recommendation_md),
        "changed_files": changed_files,
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
    check_patch_paths()
    if not approval_path.exists() or "- Approved: yes" not in approval_path.read_text(encoding="utf-8", errors="replace"):
        write_json({
            "status": "skipped",
            "reason": "approval artifact is required before applying optimizer patch",
            "patch": display_path(patch_path),
        })
        return
    if not patch_path.read_text(encoding="utf-8").strip():
        write_json({"status": "noop", "reason": "proposed patch is empty"})
        return
    subprocess.run(["git", "apply", "--check", str(patch_path)], cwd=root, check=True)
    subprocess.run(["git", "apply", str(patch_path)], cwd=root, check=True)
    write_json({
        "status": "applied",
        "patch": display_path(patch_path),
        "applied_files": [rel(p) for p in allowed_files],
        "commit": False,
    })

def validate():
    commands = [
        ["./src/bin/dockpipe", "workflow", "validate", "packages/agent/workflows/docs.optimize-orchestrate/config.yml"],
        ["./src/bin/dockpipe", "workflow", "validate", "packages/agent/workflows/docs.orchestrate/config.yml"],
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
    write_assessment()
elif action == "propose":
    write_patch()
elif action == "apply":
    apply_patch()
elif action == "validate":
    validate()
else:
    raise SystemExit(f"unknown action {action}")
PY

printf '[dorkpipe] optimizer %s result ready at %s\n' "${action}" "${result_json}" >&2
