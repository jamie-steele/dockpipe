from __future__ import annotations

import os
import shutil
import sys
import json
import getpass
from pathlib import Path


def repo_root(root: str | None = None) -> Path:
    candidate = Path(root or os.environ.get("DOCKPIPE_WORKDIR") or os.getcwd())
    return candidate.expanduser().resolve()


def resolve_dockpipe_bin(root: str | None = None) -> str | None:
    resolved_root = repo_root(root)
    candidate = resolved_root / "src" / "bin" / "dockpipe"
    if candidate.is_file() and os.access(candidate, os.X_OK):
        return str(candidate)
    return shutil.which("dockpipe")


class DockpipeSDK:
    def __init__(self, root: str | None = None) -> None:
        self.workdir = repo_root(root)
        self.dockpipe_bin = resolve_dockpipe_bin(str(self.workdir))
        self.workflow_name = os.environ.get("DOCKPIPE_WORKFLOW_NAME") or None
        self.script_dir = os.environ.get("DOCKPIPE_SCRIPT_DIR") or None
        self.package_root = os.environ.get("DOCKPIPE_PACKAGE_ROOT") or None
        self.assets_dir = os.environ.get("DOCKPIPE_ASSETS_DIR") or None

    def refresh(self, root: str | None = None) -> "DockpipeSDK":
        updated = DockpipeSDK(root)
        self.workdir = updated.workdir
        self.dockpipe_bin = updated.dockpipe_bin
        self.workflow_name = updated.workflow_name
        self.script_dir = updated.script_dir
        self.package_root = updated.package_root
        self.assets_dir = updated.assets_dir
        return self

    def prompt(
        self,
        kind: str,
        *,
        prompt_id: str | None = None,
        title: str = "",
        message: str = "",
        default: str = "",
        options: list[str] | None = None,
        sensitive: bool = False,
        intent: str = "",
        automation_group: str = "",
        allow_auto_approve: bool = False,
        auto_approve_value: str = "",
    ) -> str:
        return prompt(
            kind,
            prompt_id=prompt_id,
            title=title,
            message=message,
            default=default,
            options=options,
            sensitive=sensitive,
            intent=intent,
            automation_group=automation_group,
            allow_auto_approve=allow_auto_approve,
            auto_approve_value=auto_approve_value,
        )


def _truthy(value: str | None) -> bool:
    if not value:
        return False
    return value.strip().lower() in {"1", "true", "yes", "y", "on"}


def _prompt_mode() -> str:
    if os.environ.get("DOCKPIPE_SDK_PROMPT_MODE") == "json":
        return "json"
    if sys.stdin.isatty() and sys.stderr.isatty():
        return "terminal"
    return "noninteractive"


def prompt(
    kind: str,
    *,
    prompt_id: str | None = None,
    title: str = "",
    message: str = "",
    default: str = "",
    options: list[str] | None = None,
    sensitive: bool = False,
    intent: str = "",
    automation_group: str = "",
    allow_auto_approve: bool = False,
    auto_approve_value: str = "",
) -> str:
    options = list(options or [])
    prompt_id = prompt_id or f"prompt.{os.getpid()}.{id(options)}"
    message = message or title
    mode = _prompt_mode()

    if allow_auto_approve and _truthy(os.environ.get("DOCKPIPE_APPROVE_PROMPTS")):
        if auto_approve_value:
            return auto_approve_value
        if default:
            return default
        if kind == "confirm":
            return "yes"
        if kind == "choice" and options:
            return options[0]

    if mode == "json":
        payload = json.dumps(
            {
                "type": kind,
                "id": prompt_id,
                "title": title,
                "message": message,
                "default": default,
                "sensitive": sensitive,
                "intent": intent,
                "automation_group": automation_group,
                "allow_auto_approve": allow_auto_approve,
                "auto_approve_value": auto_approve_value,
                "options": options,
            },
            separators=(",", ":"),
        )
        print(f"::dockpipe-prompt::{payload}", file=sys.stderr)
        response = sys.stdin.readline()
        if response == "":
            raise RuntimeError("DockPipe prompt response stream closed")
        return response.rstrip("\r\n")

    if mode == "terminal":
        if title:
            print(title, file=sys.stderr)
        if kind == "confirm":
            default_yes = default.strip().lower() in {"1", "true", "yes", "y", "on"}
            suffix = " [Y/n] " if default_yes else " [y/N] "
            while True:
                print(f"{message}{suffix}", end="", file=sys.stderr)
                response = input()
                normalized = (response or ("yes" if default_yes else "no")).strip().lower()
                if normalized in {"y", "yes", "true", "1"}:
                    return "yes"
                if normalized in {"n", "no", "false", "0"}:
                    return "no"
                print("Please answer yes or no.", file=sys.stderr)
        if kind == "input":
            prompt_text = message if not default else f"{message} [{default}]"
            response = getpass.getpass(f"{prompt_text}: ") if sensitive else input(f"{prompt_text}: ")
            return response or default
        if kind == "choice":
            if not options:
                raise RuntimeError("DockPipe choice prompt requires at least one option")
            print(message, file=sys.stderr)
            default_index = 1
            for idx, option in enumerate(options, start=1):
                if default and option == default:
                    default_index = idx
                print(f"  {idx}. {option}", file=sys.stderr)
            while True:
                raw = input(f"Choose an option [{default_index}]: ").strip()
                if not raw:
                    return options[default_index - 1]
                if raw.isdigit():
                    selected = int(raw)
                    if 1 <= selected <= len(options):
                        return options[selected - 1]
                print(f"Enter a number between 1 and {len(options)}.", file=sys.stderr)
        raise RuntimeError(f"Unsupported DockPipe prompt kind: {kind}")

    if default:
        return default
    raise RuntimeError("DockPipe prompt requires a terminal or DOCKPIPE_SDK_PROMPT_MODE=json")
dockpipe = DockpipeSDK()
