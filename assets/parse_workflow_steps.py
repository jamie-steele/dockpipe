#!/usr/bin/env python3
"""
Parse dockpipe workflow `steps:` from config.yml (subset of YAML; stdlib only).
Writes a staging directory layout for bash:

  n_steps          — integer
  step_<i>/run     — newline-separated host script paths (optional)
  step_<i>/isolate — template/image name or empty (inherit)
  step_<i>/act     — act script path or empty
  step_<i>/cmd     — shell command string for container (optional)
  step_<i>/skip_container — "0" or "1"
  step_<i>/outputs — path relative to workdir for outputs.env (default .dockpipe/outputs.env)
  step_<i>/vars.env — KEY=VAL lines for step-local exports
"""
from __future__ import annotations

import os
import re
import shlex
import sys


def strip_comment(val: str) -> str:
    if "#" in val:
        # crude: only strip if # is not inside quotes
        in_single = in_double = False
        for i, c in enumerate(val):
            if c == "'" and not in_double:
                in_single = not in_single
            elif c == '"' and not in_single:
                in_double = not in_double
            elif c == "#" and not in_single and not in_double:
                val = val[:i].rstrip()
                break
    return val.strip()


def unquote(val: str) -> str:
    val = val.strip()
    if len(val) >= 2 and val[0] == val[-1] and val[0] in "\"'":
        return val[1:-1]
    return val


def find_steps_start(lines: list[str]) -> int:
    for i, line in enumerate(lines):
        if re.match(r"^steps:\s*($|#)", line):
            return i
    return -1


def is_top_level_key(line: str) -> bool:
    s = line.rstrip("\n")
    if not s.strip():
        return False
    if s[0] not in "a-zA-Z_":
        return False
    if s.startswith("  "):
        return False
    return bool(re.match(r"^[a-zA-Z_][a-zA-Z0-9_]*:", s))


def collect_step_blocks(lines: list[str], start: int) -> list[list[str]]:
    """Lines inside steps: that belong to list items (raw, including indentation)."""
    blocks: list[list[str]] = []
    i = start + 1
    while i < len(lines):
        line = lines[i]
        if is_top_level_key(line):
            break
        if re.match(r"^  -\s+", line):
            block = [line]
            i += 1
            while i < len(lines):
                n = lines[i]
                if re.match(r"^  -\s+", n):
                    break
                if is_top_level_key(n):
                    break
                if n.startswith("  ") or not n.strip():
                    block.append(n)
                    i += 1
                else:
                    break
            blocks.append(block)
            continue
        i += 1
    return blocks


def parse_step_block(raw_lines: list[str]) -> dict:
    """Parse one step list item; keys at 4 spaces under list."""
    if not raw_lines:
        return {}
    first = raw_lines[0]
    m = re.match(r"^  -\s*(.*)$", first)
    rest = [m.group(1)] if m else [first.lstrip()]
    for ln in raw_lines[1:]:
        if ln.startswith("    "):
            rest.append(ln[4:].rstrip("\n"))
        elif not ln.strip():
            rest.append("")
    # Merge first line content if non-empty key
    lines = []
    if rest[0].strip():
        lines.append(rest[0])
    lines.extend(rest[1:])

    d: dict = {
        "run": [],
        "isolate": "",
        "act": "",
        "cmd": "",
        "skip_container": False,
        "outputs": ".dockpipe/outputs.env",
        "vars": {},
    }

    i = 0
    while i < len(lines):
        line = lines[i]
        if not line.strip() or line.strip().startswith("#"):
            i += 1
            continue
        if re.match(r"^vars:\s*($|#)", line):
            i += 1
            while i < len(lines):
                inner = lines[i]
                if not inner.strip():
                    i += 1
                    continue
                if not inner.startswith("  ") and ":" in inner.split("\n", 1)[0]:
                    break
                vm = re.match(r"^\s+([a-zA-Z_][a-zA-Z0-9_]*):\s*(.*)$", inner)
                if vm:
                    d["vars"][vm.group(1)] = unquote(strip_comment(vm.group(2)))
                i += 1
            continue
        if ":" not in line:
            i += 1
            continue
        key, _, val = line.partition(":")
        key = key.strip()
        val = unquote(strip_comment(val))
        i += 1
        if key in ("run", "pre_script"):
            if val:
                d["run"].append(val)
        elif key == "isolate":
            d["isolate"] = val
        elif key in ("act", "action"):
            d["act"] = val
        elif key in ("cmd", "command"):
            d["cmd"] = val
        elif key == "skip_container":
            d["skip_container"] = val.lower() in ("1", "true", "yes")
        elif key == "outputs":
            d["outputs"] = val or ".dockpipe/outputs.env"
        else:
            pass
    return d


def write_staging(out_dir: str, steps: list[dict]) -> None:
    os.makedirs(out_dir, exist_ok=True)
    with open(os.path.join(out_dir, "n_steps"), "w", encoding="utf-8") as f:
        f.write(str(len(steps)))
    for idx, st in enumerate(steps):
        sdir = os.path.join(out_dir, f"step_{idx}")
        os.makedirs(sdir, exist_ok=True)
        run_path = os.path.join(sdir, "run")
        with open(run_path, "w", encoding="utf-8") as f:
            for p in st.get("run") or []:
                f.write(p + "\n")
        for name, val in (
            ("isolate", st.get("isolate") or ""),
            ("act", st.get("act") or ""),
            ("cmd", st.get("cmd") or ""),
            ("outputs", st.get("outputs") or ".dockpipe/outputs.env"),
        ):
            with open(os.path.join(sdir, name), "w", encoding="utf-8") as f:
                f.write(val)
        parts = shlex.split(st.get("cmd") or "", posix=True)
        with open(os.path.join(sdir, "argv"), "wb") as af:
            af.write(b"\0".join(p.encode("utf-8") for p in parts))
        sc = "1" if st.get("skip_container") else "0"
        with open(os.path.join(sdir, "skip_container"), "w", encoding="utf-8") as f:
            f.write(sc)
        vpath = os.path.join(sdir, "vars.env")
        with open(vpath, "w", encoding="utf-8") as f:
            for k, v in (st.get("vars") or {}).items():
                f.write(f"{k}={v}\n")


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: parse_workflow_steps.py <config.yml> <staging_dir>", file=sys.stderr)
        return 2
    cfg = sys.argv[1]
    out_dir = sys.argv[2]
    with open(cfg, encoding="utf-8") as fp:
        lines = fp.readlines()
    si = find_steps_start(lines)
    if si < 0:
        print("no steps: in config", file=sys.stderr)
        return 1
    blocks = collect_step_blocks(lines, si)
    if not blocks:
        print("steps: has no entries", file=sys.stderr)
        return 1
    steps = [parse_step_block(b) for b in blocks]
    write_staging(out_dir, steps)
    return 0


if __name__ == "__main__":
    sys.exit(main())
