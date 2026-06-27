#!/usr/bin/env bash
set -euo pipefail

ASSETS_DIR="${DOCKPIPE_ASSETS_DIR:-}"
if [[ -z "$ASSETS_DIR" ]]; then
  SCRIPT_DIR="${DOCKPIPE_SCRIPT_DIR:?DOCKPIPE_SCRIPT_DIR is required}"
  ASSETS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
fi

python3 - "$ASSETS_DIR" "$@" <<'PY'
import argparse
import hashlib
import json
import os
import pathlib
import re
import sys

import yaml

assets_dir = pathlib.Path(sys.argv[1]).resolve()
source_dir = assets_dir / "skills"
argv = sys.argv[2:]
if not argv and os.environ.get("DOCKPIPE_ARGS_JSON"):
    try:
        loaded = json.loads(os.environ["DOCKPIPE_ARGS_JSON"])
        if isinstance(loaded, list) and all(isinstance(item, str) for item in loaded):
            argv = loaded
    except json.JSONDecodeError as exc:
        raise SystemExit(f"invalid DOCKPIPE_ARGS_JSON: {exc}")

parser = argparse.ArgumentParser(
    prog="skills-render",
    description="Render curated DorkPipe skills to target-specific formats.",
)
parser.add_argument("--target", choices=["codex", "claude", "generic"], default="generic")
parser.add_argument("--output")
parser.add_argument("--dry-run", action="store_true")
parser.add_argument("--force", action="store_true")
parser.add_argument("--list", action="store_true")
parser.add_argument("--skills", help="Comma-separated skill ids to render")
args = parser.parse_args(argv)

name_re = re.compile(r"^[a-z][a-z0-9-]*$")

def expand_output(raw: str) -> pathlib.Path:
    expanded = os.path.expandvars(os.path.expanduser(raw))
    return pathlib.Path(expanded).resolve()

def default_output_for_target(target: str) -> pathlib.Path:
    if target == "codex":
        return pathlib.Path.home() / ".codex" / "skills"
    if target == "claude":
        return pathlib.Path.home() / ".claude" / "skills"
    raise SystemExit(f"--target {target} requires --output <path>")

def read_skill(skill_dir: pathlib.Path) -> dict:
    meta_path = skill_dir / "skill.yml"
    instructions_path = skill_dir / "instructions.md"
    if not meta_path.is_file():
        raise ValueError(f"{skill_dir.name}: missing skill.yml")
    if not instructions_path.is_file():
        raise ValueError(f"{skill_dir.name}: missing instructions.md")
    meta = yaml.safe_load(meta_path.read_text()) or {}
    instructions = instructions_path.read_text().strip()
    name = str(meta.get("name", "")).strip()
    description = str(meta.get("description", "")).strip()
    if not name_re.match(name):
        raise ValueError(f"{skill_dir.name}: invalid name {name!r}")
    if name != skill_dir.name:
        raise ValueError(f"{skill_dir.name}: skill.yml name must match directory")
    if not description:
        raise ValueError(f"{name}: missing description")
    if not instructions:
        raise ValueError(f"{name}: empty instructions")
    meta["name"] = name
    meta["description"] = description
    meta["short_description"] = str(meta.get("short_description", "")).strip()
    meta["instructions"] = instructions
    return meta

def discover() -> list[dict]:
    if not source_dir.is_dir():
        raise SystemExit(f"missing skills source directory: {source_dir}")
    skills = []
    for child in sorted(source_dir.iterdir()):
        if child.is_dir():
            skills.append(read_skill(child))
    return skills

def codex_skill(skill: dict) -> dict[str, str]:
    short = skill.get("short_description") or skill["description"]
    body = "\n".join([
        "---",
        f"name: {skill['name']}",
        "description: " + skill["description"].replace("\n", " "),
        "metadata:",
        f"  short-description: {short.replace(chr(10), ' ')}",
        "---",
        "",
        skill["instructions"].rstrip(),
        "",
    ])
    return {"SKILL.md": body}

def generic_skill(skill: dict) -> dict[str, str]:
    meta = {
        "name": skill["name"],
        "description": skill["description"],
        "short_description": skill.get("short_description", ""),
    }
    return {
        "skill.json": json.dumps(meta, indent=2) + "\n",
        "instructions.md": skill["instructions"].rstrip() + "\n",
    }

def claude_skill(skill: dict) -> dict[str, str]:
    body = "\n".join([
        "---",
        f"name: {skill['name']}",
        "description: " + skill["description"].replace("\n", " "),
        "---",
        "",
        skill["instructions"].rstrip(),
        "",
    ])
    return {"SKILL.md": body}

def render_files(skill: dict) -> dict[str, str]:
    if args.target == "codex":
        return codex_skill(skill)
    if args.target == "claude":
        return claude_skill(skill)
    return generic_skill(skill)

def content_hash(files: dict[str, str]) -> str:
    h = hashlib.sha256()
    for rel in sorted(files):
        h.update(rel.encode())
        h.update(b"\0")
        h.update(files[rel].encode())
        h.update(b"\0")
    return h.hexdigest()

def has_generated_marker(text: str) -> bool:
    return "dorkpipe-skills-render:" in text.splitlines()[0:3].__str__()

skills = discover()
if args.list:
    for skill in skills:
        print(f"{skill['name']}\t{skill['description']}")
    raise SystemExit(0)

selected = set()
if args.skills:
    selected = {item.strip() for item in args.skills.split(",") if item.strip()}
    unknown = selected - {skill["name"] for skill in skills}
    if unknown:
        raise SystemExit("unknown skills: " + ", ".join(sorted(unknown)))
if selected:
    skills = [skill for skill in skills if skill["name"] in selected]

base = expand_output(args.output) if args.output else default_output_for_target(args.target)
base.mkdir(parents=True, exist_ok=True) if not args.dry_run else None
base_real = base.resolve()

report = []
failures = 0

for skill in skills:
    try:
        files = render_files(skill)
        digest = content_hash(files)
        manifest = {
            "renderer": "dorkpipe-skills-render",
            "source": skill["name"],
            "target": args.target,
            "sha256": digest,
        }
        files[".dorkpipe-skill-render.json"] = json.dumps(manifest, indent=2) + "\n"
        skill_dir = (base_real / skill["name"]).resolve()
        if base_real not in [skill_dir, *skill_dir.parents]:
            raise ValueError("refusing to write outside output directory")
        existing_files = [skill_dir / rel for rel in files if (skill_dir / rel).exists()]
        changed_existing = []
        for path in existing_files:
            current = path.read_text()
            desired = files[path.name]
            if current == desired:
                continue
            changed_existing.append(path.name)
        if changed_existing and not args.force:
            report.append((skill["name"], "skipped", "existing user-modified files: " + ", ".join(changed_existing)))
            continue
        if args.dry_run:
            action = "would-render"
        else:
            skill_dir.mkdir(parents=True, exist_ok=True)
            if args.target == "claude":
                for stale in ("CLAUDE.md", "agent.json"):
                    stale_path = skill_dir / stale
                    if not stale_path.exists():
                        continue
                    manifest_path = skill_dir / ".dorkpipe-skill-render.json"
                    if manifest_path.exists():
                        try:
                            manifest_data = json.loads(manifest_path.read_text())
                        except json.JSONDecodeError:
                            manifest_data = {}
                        if manifest_data.get("renderer") == "dorkpipe-skills-render":
                            stale_path.unlink()
            for rel, text in files.items():
                target = skill_dir / rel
                if base_real not in [target.resolve(), *target.resolve().parents]:
                    raise ValueError(f"refusing to write outside output directory: {rel}")
                target.write_text(text)
            action = "rendered"
        report.append((skill["name"], action, str(skill_dir)))
    except Exception as exc:
        failures += 1
        report.append((skill.get("name", "<unknown>"), "failed", str(exc)))

print("DorkPipe skills render report")
print(f"target: {args.target}")
print(f"output: {base_real}")
for name, status, detail in report:
    print(f"- {status}: {name} ({detail})")

if args.target == "claude":
    stale_agents = base_real / "claude-agents.json"
    if not args.dry_run and stale_agents.exists():
        stale_agents.unlink()

if failures:
    raise SystemExit(1)
PY
