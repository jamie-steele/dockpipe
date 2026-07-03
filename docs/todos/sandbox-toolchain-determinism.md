# TODO-009 Sandbox Toolchain Determinism

## Problem

DockPipe package tests currently depend on tools discovered from the invoking shell `PATH`. In Codex
or other sandboxed hosts, a tool can appear installed but still be unusable. One concrete example is
Git Bash resolving `jq` through the WinGet link under `C:\Users\...\AppData\Local\Microsoft\WinGet`,
which PowerShell can see but the Codex sandbox cannot execute.

That makes test failures look like product regressions when the real problem is the execution
environment.

## Desired Direction

- Prefer DockPipe-managed repo-local tooling before user-profile shims or global `PATH`.
- Add a small test/tool resolver used by package tests and CI helpers.
- Add a sandbox/toolchain preflight that verifies required tools from the same shell that will run
  the tests.
- Report missing or non-executable dependencies as explicit preflight failures, not as late test
  assertion failures.
- Keep this separate from the operation-result rollout, but emit operation-result units for any
  long-running tool install/bootstrap work once implemented.

## Candidate Implementation

- Resolve tools in this order:
  - `bin/.dockpipe/tooling/bin`
  - explicit DockPipe-managed cache path
  - shell `PATH`
- For Windows Git Bash, prefer real `.exe` paths over WinGet/app-execution aliases when possible.
- Provide a package-test helper such as `dorkpipe_test_require_tool jq` so tests do not duplicate
  dependency discovery logic.
- Add a preflight command or script that checks `jq`, `python3`, `go`, Docker, and any package-local
  tools required by the selected suite.
- Document how Codex, CI, and host PowerShell/Git Bash should bootstrap the same toolchain.

## Validation

- Package tests fail fast with a clear message when a required tool is absent or non-executable.
- `packages/dorkpipe/tests/run.sh` passes from Git Bash inside Codex without host escalation when
  repo-local tools are available.
- Host execution and CI continue to use the same resolver path.

## Status

Open. Captured after `jq` succeeded from host execution but failed from Codex sandboxed Git Bash due
to the WinGet link being non-executable.
