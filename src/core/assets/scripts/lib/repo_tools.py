from __future__ import annotations

import os
import shutil
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


def resolve_dorkpipe_bin(root: str | None = None) -> str | None:
    resolved_root = repo_root(root)
    candidate = resolved_root / "packages" / "dorkpipe" / "bin" / "dorkpipe"
    if candidate.is_file() and os.access(candidate, os.X_OK):
        return str(candidate)
    return shutil.which("dorkpipe")


class DockpipeSDK:
    def __init__(self, root: str | None = None) -> None:
        self.workdir = repo_root(root)
        self.dockpipe_bin = resolve_dockpipe_bin(str(self.workdir))
        self.dorkpipe_bin = resolve_dorkpipe_bin(str(self.workdir))
        self.workflow_name = os.environ.get("DOCKPIPE_WORKFLOW_NAME") or None

    def refresh(self, root: str | None = None) -> "DockpipeSDK":
        updated = DockpipeSDK(root)
        self.workdir = updated.workdir
        self.dockpipe_bin = updated.dockpipe_bin
        self.dorkpipe_bin = updated.dorkpipe_bin
        self.workflow_name = updated.workflow_name
        return self


dockpipe = DockpipeSDK()
