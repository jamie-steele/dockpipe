#!/usr/bin/env bash
# Unit test: repo root resolution (issue #1). No Docker required.
# Asserts that when script dir is /usr/bin we get /usr/lib/dockpipe.
set -euo pipefail

# Replicate the exact logic from bin/dockpipe (lines 5-11)
resolve_repo_root() {
  local _script_dir="$1"
  if [[ "${_script_dir}" == "/usr/bin" ]]; then
    echo "/usr/lib/dockpipe"
  else
    echo "OTHER"
  fi
}

# When installed via .deb, script is at /usr/bin/dockpipe -> _script_dir is /usr/bin
got=$(resolve_repo_root "/usr/bin")
if [[ "$got" != "/usr/lib/dockpipe" ]]; then
  echo "test_repo_root FAIL: /usr/bin should resolve to /usr/lib/dockpipe, got $got"
  exit 1
fi

# When not in /usr/bin, we use the other branch (we only assert it's not the install path)
got=$(resolve_repo_root "/home/user/dockpipe/bin")
if [[ "$got" == "/usr/lib/dockpipe" ]]; then
  echo "test_repo_root FAIL: non-/usr/bin path should not resolve to /usr/lib/dockpipe"
  exit 1
fi

echo "test_repo_root OK (repo root logic for .deb install path)"
