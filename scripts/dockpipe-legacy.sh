#!/usr/bin/env bash
# dockpipe — Run, isolate, and act. Pipe commands into disposable containers and act on the result.
#
# Layout: resolve repo root → usage/helpers → action subcommand (early exit) → option loop →
#         resolve template/image, action path, data volume → source runner → build image → dockpipe_run.
set -euo pipefail

# ------------------------------------------------------------------------------
# Repo root (install path). When installed via .deb, /usr/bin/dockpipe is a symlink
# so dirname would be /usr; we use a fixed path. From source, we use script dir/..
# ------------------------------------------------------------------------------
_script_dir="$(dirname "${BASH_SOURCE[0]}")"
if [[ "${_script_dir}" == "/usr/bin" ]]; then
  DOCKPIPE_REPO_ROOT="${DOCKPIPE_REPO_ROOT:-/usr/lib/dockpipe}"
else
  DOCKPIPE_REPO_ROOT="${DOCKPIPE_REPO_ROOT:-$(cd "${_script_dir}/.." && pwd)}"
fi
export DOCKPIPE_REPO_ROOT
# shellcheck source=lib/config-vars.sh
source "${DOCKPIPE_REPO_ROOT}/lib/config-vars.sh"

usage() {
  cat <<EOF
dockpipe — Run, isolate, and act. Pipe commands into disposable containers and act on the result.

Usage:
  dockpipe [options] -- <command> [args...]
  dockpipe --workflow <name> [options] -- <command>   Use a workflow (config: run, isolate, act). Override with --run, --isolate, --act.
  dockpipe --workflow <name> [options]               Multi-step: if config has steps:, each step runs in order; optional -- for last step cmd only.
  dockpipe action init [--from <bundled>] <filename>   New action script; --from copies a bundled one
  dockpipe pre init [--from <bundled>] <filename>   New run script (host); --from copies a bundled one
  dockpipe template init [--from <bundled>] <dirname>   New workflow folder; --from copies a bundled template (llm-worktree)
  dockpipe init [<template-name>] [<dest>]   New workspace: scripts/, images/, templates/. Optional name = new template + samples.
  <stdin> | dockpipe [options] -- <command> [args...]

Options (run → isolate → act):
  --workflow <name>    Use templates/<name>/config.yml (run, isolate, act).
  --run <path>        Run: script(s) on host before container (e.g. clone-worktree). Can be repeated.
  --isolate <name>    Isolate: image or template (base-dev | claude | codex | …). Builds if template.
  --act <path>        Act: script after command (e.g. commit-worktree). Runs in container or on host when commit-on-host.
  --resolver <name>   Resolver (claude | codex); sets isolate + command. Use with --repo/--branch.
  --repo <url>        Clone and use a worktree for this repo (with --branch). Worktree on host; commit on host.
  --branch <name>     Work branch (optional). Omit for new branch each run (e.g. claude/agent-<timestamp>).
  --work-path <path>  Subfolder inside repo to open in container (cwd = /work/<path>).
  --work-branch <name> Default branch name when using --repo without --branch.
  --bundle-out <path> After commit-on-host, write a git bundle here.
  --workdir <path>    Host path to mount at /work (default: current directory)
  --data-vol <name>   Named volume for persistent data (default: dockpipe-data). Same volume each run = reusable agent environment.
  --data-dir <path>   Bind mount host path for persistent data (e.g. \$HOME/.dockpipe). Mounted at /dockpipe-data; HOME set there.
  --no-data           Do not mount the data volume (minimal run; no persistence)
  --reinit            Remove the named data volume before running (fresh volume). Prompts to confirm unless -f. Use if a tool exits immediately due to bad/corrupt state.
  -f, --force         With --reinit, skip confirmation (still show warning).
  --mount <v>         Extra volume or bind mount (e.g. /host/path:/container/path). Can be repeated.
  --env <KEY=VAL>     Pass env var into container. Can be repeated.
  --env-file <path>   With --workflow: load KEY=VAL into workflow env (merge if unset). Can be repeated.
  --var <KEY=VAL>     With --workflow: set workflow variable (overrides yml and .env). Can be repeated.
  --build <path>      Build image from path (relative to repo or absolute) and use as --image
  -d, --detach        Run container in background; do not attach. Container stays up until command exits.
  --help, -h           Show this help

Examples:
  dockpipe --workflow llm-worktree --repo https://github.com/you/repo.git -- claude -p "Fix the bug"
  dockpipe -- ls -la
  dockpipe --isolate claude --repo https://github.com/you/repo.git --run scripts/clone-worktree.sh --act scripts/commit-worktree.sh -- claude -p "Fix the bug"
  dockpipe -d --isolate agent-dev -- claude -p "review this code"
  dockpipe --isolate alpine -- sh -c "echo hello"
  dockpipe --workdir /path/to/repo --act scripts/commit-worktree.sh -- claude -p "Your prompt"
  dockpipe --workdir /path/to/repo -- bash -c "npm test"

Run → isolate → act. Workflow config.yml may define vars: (defaults), merged with templates/<name>/.env, repo .env, \$DOCKPIPE_ENV_FILE, then --env-file and --var. Existing shell env is not overwritten by yml/.env; use --var to force.

Multi-step workflows (steps: in config.yml): each step can set run, isolate, act, cmd, vars, outputs. After each step, if \$WORKDIR/<outputs> exists (default .dockpipe/outputs.env, KEY=VAL lines), those variables are exported for the next step and passed into the next container. Keys set with --var are never overwritten. Requires python3. See templates/chain-test/config.yml.
EOF
  exit 0
}

# ------------------------------------------------------------------------------
# resolve_template <name> — output "IMAGE BUILD_PATH". Images live in images/ (shared).
# ------------------------------------------------------------------------------
resolve_template() {
  local name="${1:-}"
  case "$name" in
    base-dev)   echo "dockpipe-base-dev ${DOCKPIPE_REPO_ROOT}/images/base-dev" ;;
    dev)        echo "dockpipe-dev ${DOCKPIPE_REPO_ROOT}/images/dev" ;;
    agent-dev|claude) echo "dockpipe-claude ${DOCKPIPE_REPO_ROOT}/images/claude" ;;
    codex)      echo "dockpipe-codex ${DOCKPIPE_REPO_ROOT}/images/codex" ;;
    *)          echo "" ;;
  esac
}

# shellcheck source=lib/workflow-steps.sh
source "${DOCKPIPE_REPO_ROOT}/lib/workflow-steps.sh"

# ------------------------------------------------------------------------------
# action_subcommand — "dockpipe action init [--from <bundled>] <filename>"
# Creates a new action script: either boilerplate or a copy of a bundled action.
# ------------------------------------------------------------------------------
action_subcommand() {
  local sub="${1:-}"
  if [[ "$sub" != "init" && "$sub" != "create" ]]; then
    echo "Usage: dockpipe action init [--from <bundled>] <filename>" >&2
    echo "       dockpipe action create [--from <bundled>] <filename>" >&2
    echo "Creates a new action script. Use --from to copy a bundled action (commit-worktree, export-patch, print-summary)." >&2
    exit 1
  fi
  shift
  local name=""
  local from=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --from)
        [[ $# -lt 2 ]] && { echo "Error: --from requires an argument" >&2; exit 1; }
        from="$2"
        shift 2
        ;;
      *)
        [[ -z "$name" ]] && name="$1"
        shift
        ;;
    esac
  done
  [[ -z "$name" ]] && name="my-action.sh"
  [[ "$name" != *.sh ]] && name="${name}.sh"
  local dest="$name"
  [[ "$dest" != /* ]] && dest="$(pwd)/$dest"
  mkdir -p "$(dirname "$dest")"
  if [[ -f "$dest" ]]; then
    echo "Error: $dest already exists" >&2
    exit 1
  fi
  if [[ -n "$from" ]]; then
    local base="${from%.sh}"
    local src="${DOCKPIPE_REPO_ROOT}/scripts/${base}.sh"
    [[ ! -f "$src" ]] && src="${DOCKPIPE_REPO_ROOT}/scripts/${base}.sh"
    if [[ ! -f "$src" ]]; then
      echo "Error: unknown bundled action '$from'. Available: commit-worktree, export-patch, print-summary" >&2
      exit 1
    fi
    cp "$src" "$dest"
    chmod +x "$dest"
    echo "Created: $dest (from bundled ${base}.sh)"
  else
    cat > "$dest" << 'ACTIONEOF'
#!/usr/bin/env bash
# dockpipe action — runs inside the container after your command.
# Available env:
#   DOCKPIPE_EXIT_CODE       Exit code of the command that just ran
#   DOCKPIPE_CONTAINER_WORKDIR  Work dir in container (default /work)
set -euo pipefail

cd "${DOCKPIPE_CONTAINER_WORKDIR:-/work}"

if [[ "${DOCKPIPE_EXIT_CODE:-1}" -eq 0 ]]; then
  echo "Command succeeded, acting on results..."
  # Your logic here (e.g. git commit, curl a webhook, write a summary)
else
  echo "Command failed with code ${DOCKPIPE_EXIT_CODE}" >&2
fi
exit "${DOCKPIPE_EXIT_CODE:-1}"
ACTIONEOF
    chmod +x "$dest"
    echo "Created: $dest"
  fi
  echo "Use: dockpipe --action $name -- <your-command>"
}

# ------------------------------------------------------------------------------
# template_subcommand — "dockpipe template init [--from <bundled>] <dirname>"
# Copies a workflow template into the current directory so you can customize it
# without contributing back. Add your own templates and init from them.
# ------------------------------------------------------------------------------
template_subcommand() {
  local sub="${1:-}"
  if [[ "$sub" != "init" && "$sub" != "create" ]]; then
    echo "Usage: dockpipe template init [--from <bundled>] <dirname>" >&2
    echo "       dockpipe template create [--from <bundled>] <dirname>" >&2
    echo "Copies a workflow template (config.yml, resolvers/, isolate/) so you can use it with --workflow. No run script in template; config points to scripts/." >&2
    echo "Bundled: llm-worktree (default). Use: dockpipe --workflow <name> --repo <url> -- ..." >&2
    exit 1
  fi
  shift
  local name=""
  local from=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --from)
        [[ $# -lt 2 ]] && { echo "Error: --from requires an argument" >&2; exit 1; }
        from="$2"
        shift 2
        ;;
      *)
        [[ -z "$name" ]] && name="$1"
        shift
        ;;
    esac
  done
  [[ -z "$name" ]] && name="my-workflow"
  from="${from:-llm-worktree}"
  local src_dir="${DOCKPIPE_REPO_ROOT}/templates/${from}"
  if [[ ! -d "$src_dir" ]]; then
    echo "Error: unknown bundled template '$from'. Available: llm-worktree" >&2
    exit 1
  fi
  local dest="$name"
  [[ "$dest" != /* ]] && dest="$(pwd)/$dest"
  if [[ -e "$dest" ]]; then
    echo "Error: $dest already exists" >&2
    exit 1
  fi
  mkdir -p "$dest"
  cp -r "${src_dir}"/* "$dest"
  chmod +x "$dest"/*.sh 2>/dev/null || true
  echo "Created: $dest (from template ${from})"
  echo "Run: dockpipe --workflow $(basename "$dest") --repo <url> [--resolver claude|codex] -- claude -p '...'"
}

# ------------------------------------------------------------------------------
# init_subcommand — "dockpipe init [<template-name>] [<dest>]" or "dockpipe init --from <url> [<dest>]"
# Creates workspace with top-level scripts/, images/, templates/. No name: templates/ has only README.
# With name: new template in templates/<name>/ + sample scripts/images copied in.
# ------------------------------------------------------------------------------
init_subcommand() {
  local from_url=""
  local template_name=""
  local dest=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --from)
        [[ $# -lt 2 ]] && { echo "Error: --from requires an argument" >&2; exit 1; }
        from_url="$2"
        shift 2
        ;;
      -*)
        echo "Error: unknown option $1" >&2
        exit 1
        ;;
      *)
        if [[ -z "$template_name" ]] && [[ -z "$dest" ]]; then
          if [[ "$1" == */* ]] || [[ "$1" == . ]]; then
            dest="$1"
          else
            template_name="$1"
          fi
        elif [[ -z "$dest" ]]; then
          dest="$1"
        fi
        shift
        ;;
    esac
  done
  [[ -z "$dest" ]] && dest="."
  mkdir -p "$(dirname "$dest")"
  if [[ -d "$dest" ]]; then
    dest="$(cd "$dest" && pwd)"
  else
    dest="$(cd "$(dirname "$dest")" && pwd)/$(basename "$dest")"
  fi
  if [[ -e "$dest" ]] && [[ "$(ls -A "$dest" 2>/dev/null)" != "" ]]; then
    echo "Error: destination exists and is not empty: ${dest}" >&2
    exit 1
  fi

  if [[ -n "$from_url" ]]; then
    echo "[dockpipe] Cloning ${from_url} into ${dest} ..." >&2
    git clone "$from_url" "$dest"
    echo "Created: ${dest} (cloned from ${from_url})"
    echo "cd ${dest} && DOCKPIPE_REPO_ROOT=\$(pwd) dockpipe --workflow <name> -- ..."
  else
    local init_tpl="${DOCKPIPE_REPO_ROOT}/templates/init"
    local rwt_tpl="${DOCKPIPE_REPO_ROOT}/templates/llm-worktree"
    if [[ ! -f "${init_tpl}/config.yml" ]]; then
      echo "Error: init template not found (expected ${init_tpl}/config.yml)" >&2
      exit 1
    fi
    echo "[dockpipe] Creating workspace at ${dest} ..." >&2
    mkdir -p "$dest/scripts" "$dest/images" "$dest/templates"
    # Generated top-level README at the spot the user inits
    cat > "$dest/README.md" << 'INITREADME'
# Dockpipe workspace

Top-level folders:

- **scripts/** — Run and act scripts. Workflow config (in templates) points here.
- **images/** — Dockerfiles. Build with `docker build -t <name> images/<name>`.
- **templates/** — One folder per workflow (config.yml, resolvers/). Use `dockpipe --workflow <name> -- ...`.

## Add a template

Copy a bundled template: `dockpipe template init my-workflow --from llm-worktree`

Or scaffold one with example scripts and image: `dockpipe init my-template .` (from an empty dir).

Then run: `DOCKPIPE_REPO_ROOT="$(pwd)" dockpipe --workflow <name> --repo <url> -- ...`
INITREADME
    if [[ -n "$template_name" ]]; then
      mkdir -p "$dest/templates/$template_name"
      cp "${init_tpl}/config.yml" "$dest/templates/$template_name/"
      cp -r "${init_tpl}/resolvers" "$dest/templates/$template_name/"
      cp "${DOCKPIPE_REPO_ROOT}/scripts/example-run.sh" "${DOCKPIPE_REPO_ROOT}/scripts/example-act.sh" "$dest/scripts/" 2>/dev/null || true
      cp -r "${DOCKPIPE_REPO_ROOT}/images/example" "$dest/images/" 2>/dev/null || true
      if [[ -d "$rwt_tpl/resolvers" ]]; then
        for f in "${rwt_tpl}/resolvers/"*; do
          [[ -f "$f" ]] && cp "$f" "$dest/templates/$template_name/resolvers/"
        done
      fi
      chmod +x "$dest"/scripts/*.sh 2>/dev/null || true
      echo "Created: ${dest} with templates/${template_name}/ and example scripts/images"
      echo "cd ${dest} && DOCKPIPE_REPO_ROOT=\$(pwd) dockpipe --workflow ${template_name} --repo <url> -- ..."
    else
      echo "Created: ${dest} (scripts/, images/, templates/)"
      echo "cd ${dest} && dockpipe template init <name> --from llm-worktree, or dockpipe init <template-name> . to scaffold with examples"
    fi
  fi
}

# ------------------------------------------------------------------------------
# pre_subcommand — "dockpipe pre init [--from <bundled>] <filename>"
# Creates a new run script: boilerplate or copy of a bundled one (runs on host before container).
# ------------------------------------------------------------------------------
pre_subcommand() {
  local sub="${1:-}"
  if [[ "$sub" != "init" && "$sub" != "create" ]]; then
    echo "Usage: dockpipe pre init [--from <bundled>] <filename>" >&2
    echo "       dockpipe pre create [--from <bundled>] <filename>" >&2
    echo "Creates a new run script (runs on host before the container). Use --from to copy a bundled one (clone-worktree)." >&2
    exit 1
  fi
  shift
  local name=""
  local from=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --from)
        [[ $# -lt 2 ]] && { echo "Error: --from requires an argument" >&2; exit 1; }
        from="$2"
        shift 2
        ;;
      *)
        [[ -z "$name" ]] && name="$1"
        shift
        ;;
    esac
  done
  [[ -z "$name" ]] && name="my-pre.sh"
  [[ "$name" != *.sh ]] && name="${name}.sh"
  local dest="$name"
  [[ "$dest" != /* ]] && dest="$(pwd)/$dest"
  mkdir -p "$(dirname "$dest")"
  if [[ -f "$dest" ]]; then
    echo "Error: $dest already exists" >&2
    exit 1
  fi
  if [[ -n "$from" ]]; then
    local base="${from%.sh}"
    local src="${DOCKPIPE_REPO_ROOT}/scripts/${base}.sh"
    [[ ! -f "$src" ]] && src="${DOCKPIPE_REPO_ROOT}/scripts/${base}.sh"
    if [[ ! -f "$src" ]]; then
      echo "Error: unknown bundled pre-script '$from'. Available: clone-worktree" >&2
      exit 1
    fi
    cp "$src" "$dest"
    chmod +x "$dest"
    echo "Created: $dest (from bundled ${base}.sh)"
  else
    cat > "$dest" << 'PREEOF'
#!/usr/bin/env bash
# dockpipe pre-script — runs on the host before the container. Export vars to control flow.
# Export DOCKPIPE_WORKDIR to set the work dir (and skip built-in --repo worktree). Export DOCKPIPE_COMMIT_ON_HOST=1 to run commit-worktree on host after exit.
# Available env: DOCKPIPE_REPO_URL, DOCKPIPE_REPO_BRANCH, DOCKPIPE_DATA_DIR, DOCKPIPE_REPO_ROOT, RESOLVER, TEMPLATE.
set -euo pipefail

# Example: use scripts/clone-worktree.sh (it exports DOCKPIPE_WORKDIR and DOCKPIPE_COMMIT_ON_HOST=1).
# export DOCKPIPE_WORKDIR
# export DOCKPIPE_COMMIT_ON_HOST=1
PREEOF
    chmod +x "$dest"
    echo "Created: $dest"
  fi
  echo "Use: dockpipe --run $name [--repo <url>] -- <your-command>"
}

# ------------------------------------------------------------------------------
# Early exit: "dockpipe init ...", "dockpipe action init ...", "dockpipe pre init ...", "dockpipe template init ..."
# ------------------------------------------------------------------------------
if [[ "${1:-}" == "init" ]]; then
  shift
  init_subcommand "$@"
  exit 0
fi
if [[ "${1:-}" == "action" ]]; then
  shift
  action_subcommand "$@"
  exit 0
fi
if [[ "${1:-}" == "pre" ]]; then
  shift
  pre_subcommand "$@"
  exit 0
fi
if [[ "${1:-}" == "template" ]]; then
  shift
  template_subcommand "$@"
  exit 0
fi

# ------------------------------------------------------------------------------
# Main option parsing. All options must appear before "--"; command follows "--".
# ------------------------------------------------------------------------------
DOCKPIPE_IMAGE=""
DOCKPIPE_ACTION=""
DOCKPIPE_WORKDIR=""
DOCKPIPE_REPO_URL=""
DOCKPIPE_REPO_BRANCH=""
DOCKPIPE_WORK_PATH=""
DOCKPIPE_WORK_BRANCH=""
DOCKPIPE_BUNDLE_OUT=""
DOCKPIPE_BUILD=""
DOCKPIPE_BUILD_CONTEXT=""
DOCKPIPE_EXTRA_MOUNTS=""
DOCKPIPE_EXTRA_ENV=""
DOCKPIPE_DETACH=""
DOCKPIPE_DATA_VOLUME=""
DOCKPIPE_DATA_DIR=""
DOCKPIPE_NO_DATA=""
DOCKPIPE_REINIT=""
DOCKPIPE_FORCE=""
DOCKPIPE_PRE_SCRIPTS=()
DOCKPIPE_ENV_FILE_EXTRAS=()
DOCKPIPE_WORKFLOW_VAR_OVERRIDES=()
DOCKPIPE_ISOLATE=""
DOCKPIPE_WORKFLOW=""
TEMPLATE=""
RESOLVER=""
SEEN_DASH=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --help|-h) usage ;;
    -d|--detach) DOCKPIPE_DETACH=1; shift ;;
    -f|--force) DOCKPIPE_FORCE=1; shift ;;
    --reinit) DOCKPIPE_REINIT=1; shift ;;
    --data-vol|--data-volume)
      [[ $# -lt 2 ]] && { echo "Error: $1 requires an argument" >&2; exit 1; }
      DOCKPIPE_DATA_VOLUME="$2"
      shift 2
      ;;
    --data-dir)
      [[ $# -lt 2 ]] && { echo "Error: --data-dir requires an argument" >&2; exit 1; }
      DOCKPIPE_DATA_DIR="$2"
      shift 2
      ;;
    --no-data) DOCKPIPE_NO_DATA=1; shift ;;
    --run)
      [[ $# -lt 2 ]] && { echo "Error: --run requires an argument" >&2; exit 1; }
      DOCKPIPE_PRE_SCRIPTS+=("$2")
      shift 2
      ;;
    --isolate)
      [[ $# -lt 2 ]] && { echo "Error: --isolate requires an argument" >&2; exit 1; }
      DOCKPIPE_ISOLATE="$2"
      shift 2
      ;;
    --act)
      [[ $# -lt 2 ]] && { echo "Error: --act requires an argument" >&2; exit 1; }
      DOCKPIPE_ACTION="$2"
      shift 2
      ;;
    --action)  # alias for --act
      [[ $# -lt 2 ]] && { echo "Error: --action requires an argument" >&2; exit 1; }
      DOCKPIPE_ACTION="$2"
      shift 2
      ;;
    --pre-script)  # alias for --run
      [[ $# -lt 2 ]] && { echo "Error: --pre-script requires an argument" >&2; exit 1; }
      DOCKPIPE_PRE_SCRIPTS+=("$2")
      shift 2
      ;;
    --template|--image)  # alias for --isolate
      [[ $# -lt 2 ]] && { echo "Error: $1 requires an argument" >&2; exit 1; }
      DOCKPIPE_ISOLATE="$2"
      shift 2
      ;;
    --workflow)
      [[ $# -lt 2 ]] && { echo "Error: --workflow requires an argument" >&2; exit 1; }
      DOCKPIPE_WORKFLOW="$2"
      shift 2
      ;;
    --workdir)
      [[ $# -lt 2 ]] && { echo "Error: --workdir requires an argument" >&2; exit 1; }
      DOCKPIPE_WORKDIR="$2"
      shift 2
      ;;
    --repo)
      [[ $# -lt 2 ]] && { echo "Error: --repo requires an argument" >&2; exit 1; }
      DOCKPIPE_REPO_URL="$2"
      shift 2
      ;;
    --branch)
      [[ $# -lt 2 ]] && { echo "Error: --branch requires an argument" >&2; exit 1; }
      DOCKPIPE_REPO_BRANCH="$2"
      shift 2
      ;;
    --work-path)
      [[ $# -lt 2 ]] && { echo "Error: --work-path requires an argument" >&2; exit 1; }
      DOCKPIPE_WORK_PATH="$2"
      shift 2
      ;;
    --work-branch)
      [[ $# -lt 2 ]] && { echo "Error: --work-branch requires an argument" >&2; exit 1; }
      DOCKPIPE_WORK_BRANCH="$2"
      shift 2
      ;;
    --bundle-out)
      [[ $# -lt 2 ]] && { echo "Error: --bundle-out requires an argument" >&2; exit 1; }
      DOCKPIPE_BUNDLE_OUT="$2"
      shift 2
      ;;
    --resolver)
      [[ $# -lt 2 ]] && { echo "Error: --resolver requires an argument" >&2; exit 1; }
      RESOLVER="$2"
      shift 2
      ;;
    --mount)
      [[ $# -lt 2 ]] && { echo "Error: --mount requires an argument" >&2; exit 1; }
      DOCKPIPE_EXTRA_MOUNTS="${DOCKPIPE_EXTRA_MOUNTS:+$DOCKPIPE_EXTRA_MOUNTS }$2"
      shift 2
      ;;
    --env)
      [[ $# -lt 2 ]] && { echo "Error: --env requires an argument" >&2; exit 1; }
      DOCKPIPE_EXTRA_ENV="${DOCKPIPE_EXTRA_ENV:+$DOCKPIPE_EXTRA_ENV$'\n'}$2"
      shift 2
      ;;
    --env-file)
      [[ $# -lt 2 ]] && { echo "Error: --env-file requires an argument" >&2; exit 1; }
      DOCKPIPE_ENV_FILE_EXTRAS+=("$2")
      shift 2
      ;;
    --var)
      [[ $# -lt 2 ]] && { echo "Error: --var requires KEY=VAL" >&2; exit 1; }
      [[ "$2" != *=* ]] && { echo "Error: --var requires KEY=VAL" >&2; exit 1; }
      DOCKPIPE_WORKFLOW_VAR_OVERRIDES+=("$2")
      shift 2
      ;;
    --build)
      [[ $# -lt 2 ]] && { echo "Error: --build requires an argument" >&2; exit 1; }
      if [[ "$2" == /* ]]; then
        DOCKPIPE_BUILD="$2"
        DOCKPIPE_BUILD_CONTEXT="$2"
      else
        DOCKPIPE_BUILD="${DOCKPIPE_REPO_ROOT}/${2}"
        DOCKPIPE_BUILD_CONTEXT="${DOCKPIPE_REPO_ROOT}"
      fi
      shift 2
      ;;
    --)
      SEEN_DASH=1
      shift
      break
      ;;
    *)
      echo "Error: unknown option $1" >&2
      exit 1
      ;;
  esac
done

# ------------------------------------------------------------------------------
# Workflow template: load config.yml and apply defaults (CLI already parsed; overrides win).
# ------------------------------------------------------------------------------
config_get() {
  local f="$1" k="$2"
  [[ ! -f "$f" ]] && return
  # grep exits 1 when no match; do not trigger set -e in callers (e.g. optional resolver: key).
  { grep -E "^${k}:" "$f" 2>/dev/null || true; } | sed -E 's/^[^:]+:[[:space:]]*//' | head -1
}

DOCKPIPE_WORKFLOW_ROOT=""
DOCKPIPE_WORKFLOW_CONFIG=""
DOCKPIPE_STEPS_MODE=0
if [[ -n "${DOCKPIPE_WORKFLOW}" ]]; then
  workflow_root="${DOCKPIPE_REPO_ROOT}/templates/${DOCKPIPE_WORKFLOW}"
  config_file="${workflow_root}/config.yml"
  if [[ ! -f "${config_file}" ]]; then
    echo "Error: workflow '${DOCKPIPE_WORKFLOW}' not found (expected ${config_file})" >&2
    exit 1
  fi
  DOCKPIPE_WORKFLOW_ROOT="${workflow_root}"
  DOCKPIPE_WORKFLOW_CONFIG="${config_file}"
  if dockpipe_config_has_steps "${config_file}"; then
    DOCKPIPE_STEPS_MODE=1
    echo "[dockpipe] Multi-step workflow (${DOCKPIPE_WORKFLOW})" >&2
  fi
  dockpipe_workflow_apply_vars "${workflow_root}/config.yml" "${workflow_root}" "${DOCKPIPE_REPO_ROOT}"
  # Apply defaults from config (run, isolate, act) only when user did not set via CLI
  if [[ -z "${RESOLVER:-}" ]]; then
    # Multi-step: top-level `isolate:` is often a Docker image, not resolvers/<name>. Use resolver/default_resolver only.
    if [[ "${DOCKPIPE_STEPS_MODE}" -eq 1 ]]; then
      RESOLVER="$(config_get "${config_file}" "resolver")"
      [[ -z "$RESOLVER" ]] && RESOLVER="$(config_get "${config_file}" "default_resolver")"
    else
      RESOLVER="$(config_get "${config_file}" "isolate")"
      [[ -z "$RESOLVER" ]] && RESOLVER="$(config_get "${config_file}" "default_resolver")"
    fi
    [[ -n "$RESOLVER" ]] && echo "[dockpipe] Using resolver from workflow: ${RESOLVER}" >&2
  fi
  if [[ "${DOCKPIPE_STEPS_MODE}" -ne 1 ]]; then
    if [[ ${#DOCKPIPE_PRE_SCRIPTS[@]} -eq 0 ]]; then
      _run="$(config_get "${config_file}" "run")"
      [[ -z "$_run" ]] && _run="$(config_get "${config_file}" "pre_script")"
      if [[ -n "$_run" ]]; then
        _run_root="${DOCKPIPE_REPO_ROOT}"
        [[ "$_run" != scripts/* ]] && _run_root="${workflow_root}"
        DOCKPIPE_PRE_SCRIPTS=("${_run_root}/${_run}")
        echo "[dockpipe] Using run from workflow: ${_run}" >&2
      fi
      unset _run _run_root
    fi
    if [[ -z "${DOCKPIPE_ACTION:-}" ]]; then
      _act="$(config_get "${config_file}" "act")"
      [[ -z "$_act" ]] && _act="$(config_get "${config_file}" "action")"
      if [[ -n "$_act" ]]; then
        _act_root="${DOCKPIPE_REPO_ROOT}"
        [[ "$_act" != scripts/* ]] && _act_root="${workflow_root}"
        DOCKPIPE_ACTION="${_act_root}/${_act}"
        echo "[dockpipe] Using act from workflow: ${_act}" >&2
      fi
    fi
  fi
  unset _act config_file workflow_root
fi

# ------------------------------------------------------------------------------
# Resolver: load from workflow template or workspace root (dockpipe init).
# When --workflow was used, pre/action already set from config; only set from resolver when no workflow.
# ------------------------------------------------------------------------------
if [[ -n "${RESOLVER}" ]]; then
  if [[ -n "${DOCKPIPE_WORKFLOW_ROOT:-}" ]]; then
    _resolver_base="${DOCKPIPE_WORKFLOW_ROOT}/resolvers"
  else
    _resolver_base="${DOCKPIPE_REPO_ROOT}/templates/${DOCKPIPE_WORKFLOW:-llm-worktree}/resolvers"
  fi
  resolver_file="${_resolver_base}/${RESOLVER}"
  if [[ ! -f "${resolver_file}" ]]; then
    echo "Error: resolver '${RESOLVER}' not found (expected ${resolver_file})" >&2
    exit 1
  fi
  unset _resolver_base
  # shellcheck source=templates/llm-worktree/resolvers/claude
  source "${resolver_file}"
  TEMPLATE="${DOCKPIPE_RESOLVER_TEMPLATE:-$TEMPLATE}"
  # Resolver can set default pre-script and action only when we did not get them from workflow config
  if [[ -z "${DOCKPIPE_WORKFLOW:-}" ]]; then
    if [[ ${#DOCKPIPE_PRE_SCRIPTS[@]} -eq 0 ]] && [[ -n "${DOCKPIPE_RESOLVER_PRE_SCRIPT:-}" ]]; then
      DOCKPIPE_PRE_SCRIPTS=("${DOCKPIPE_REPO_ROOT}/${DOCKPIPE_RESOLVER_PRE_SCRIPT}")
    fi
    if [[ -z "${DOCKPIPE_ACTION:-}" ]] && [[ -n "${DOCKPIPE_RESOLVER_ACTION:-}" ]]; then
      DOCKPIPE_ACTION="${DOCKPIPE_REPO_ROOT}/${DOCKPIPE_RESOLVER_ACTION}"
    fi
  fi
fi

# ------------------------------------------------------------------------------
# --isolate: resolve to template (build) or use as image name. (Skipped for multi-step workflows.)
# ------------------------------------------------------------------------------
if [[ "${DOCKPIPE_STEPS_MODE:-0}" -ne 1 ]]; then
  if [[ -n "${DOCKPIPE_ISOLATE:-}" ]]; then
    _iso_resolved=$(resolve_template "${DOCKPIPE_ISOLATE}")
    if [[ -n "$_iso_resolved" ]]; then
      TEMPLATE="${DOCKPIPE_ISOLATE}"
    else
      DOCKPIPE_IMAGE="${DOCKPIPE_ISOLATE}"
    fi
    unset _iso_resolved
  fi

  # ------------------------------------------------------------------------------
  # Resolve image and build path from template (if TEMPLATE was set by resolver or --isolate).
  # ------------------------------------------------------------------------------
  if [[ -n "${TEMPLATE}" ]]; then
    resolved=$(resolve_template "$TEMPLATE")
    if [[ -z "$resolved" ]]; then
      echo "Error: unknown template '${TEMPLATE}'" >&2
      exit 1
    fi
    DOCKPIPE_IMAGE="${resolved%% *}"
    build_path="${resolved#* }"
    if [[ -d "$build_path" ]]; then
      DOCKPIPE_BUILD="$build_path"
      DOCKPIPE_BUILD_CONTEXT="${DOCKPIPE_REPO_ROOT}"
    fi
  fi

  # Default image when neither --image nor --template was given.
  if [[ -z "${DOCKPIPE_IMAGE}" ]]; then
    DOCKPIPE_IMAGE="dockpipe-base-dev"
    DOCKPIPE_BUILD="${DOCKPIPE_REPO_ROOT}/images/base-dev"
    DOCKPIPE_BUILD_CONTEXT="${DOCKPIPE_REPO_ROOT}"
  fi

# Versioned image tag when installed via .deb (version file present). Ensures upgrades get a new image.
# Only applies to dockpipe-* image names, not arbitrary registry images (e.g. alpine).
if [[ "${DOCKPIPE_IMAGE}" != *:* ]] && [[ -f "${DOCKPIPE_REPO_ROOT}/version" ]]; then
  case "${DOCKPIPE_IMAGE}" in
    dockpipe-*) DOCKPIPE_IMAGE="${DOCKPIPE_IMAGE}:$(cat "${DOCKPIPE_REPO_ROOT}/version")" ;;
  esac
fi

  # Resolve relative action path: repo root, scripts/, then cwd.
  if [[ -n "${DOCKPIPE_ACTION}" ]] && [[ "${DOCKPIPE_ACTION}" != /* ]]; then
    if [[ -f "${DOCKPIPE_REPO_ROOT}/${DOCKPIPE_ACTION}" ]]; then
      DOCKPIPE_ACTION="${DOCKPIPE_REPO_ROOT}/${DOCKPIPE_ACTION}"
    elif [[ -f "${DOCKPIPE_REPO_ROOT}/scripts/${DOCKPIPE_ACTION}" ]]; then
      DOCKPIPE_ACTION="${DOCKPIPE_REPO_ROOT}/scripts/${DOCKPIPE_ACTION}"
    elif [[ -f "${DOCKPIPE_ACTION}" ]]; then
      DOCKPIPE_ACTION="$(cd "$(dirname "${DOCKPIPE_ACTION}")" && pwd)/$(basename "${DOCKPIPE_ACTION}")"
    else
      DOCKPIPE_ACTION="$(pwd)/${DOCKPIPE_ACTION}"
    fi
  fi
  if [[ -n "${DOCKPIPE_ACTION}" ]] && [[ ! -f "${DOCKPIPE_ACTION}" ]]; then
    echo "Error: action script not found: ${DOCKPIPE_ACTION}" >&2
    exit 1
  fi

  # When using the bundled commit-worktree action, run the commit on the host after the container exits
  # so the AI never has git access (host does the commit with host's identity).
  bundled_commits=(
    "${DOCKPIPE_REPO_ROOT}/scripts/commit-worktree.sh"
  )
  if [[ -n "${DOCKPIPE_ACTION}" ]]; then
    resolved_action="$(cd "$(dirname "${DOCKPIPE_ACTION}")" && pwd)/$(basename "${DOCKPIPE_ACTION}")"
    is_bundled_commit=""
    for bundled_commit in "${bundled_commits[@]}"; do
      if [[ -f "${bundled_commit}" ]]; then
        resolved_bundled="$(cd "$(dirname "${bundled_commit}")" && pwd)/$(basename "${bundled_commit}")"
        if [[ "$resolved_action" == "$resolved_bundled" ]]; then
          is_bundled_commit=1
          break
        fi
      fi
    done
    if [[ -n "${is_bundled_commit}" ]]; then
      DOCKPIPE_COMMIT_ON_HOST=1
      export DOCKPIPE_COMMIT_ON_HOST
      # Provider/verb for commit-on-host: keep vars/.env DOCKPIPE_BRANCH_PREFIX if already set
      if [[ -z "${DOCKPIPE_BRANCH_PREFIX:-}" ]]; then
        if [[ -n "${RESOLVER:-}" ]]; then
          export DOCKPIPE_BRANCH_PREFIX="${RESOLVER}"
        elif [[ -n "${TEMPLATE:-}" ]]; then
          case "$TEMPLATE" in
            claude|agent-dev) export DOCKPIPE_BRANCH_PREFIX=claude ;;
            codex)            export DOCKPIPE_BRANCH_PREFIX=codex ;;
            *)                export DOCKPIPE_BRANCH_PREFIX=dockpipe ;;
          esac
        fi
      fi
      while IFS= read -r e; do
        case "$e" in
          DOCKPIPE_COMMIT_MESSAGE=*) export "$e" ;;
          DOCKPIPE_WORK_BRANCH=*) export "$e" ;;
          DOCKPIPE_BUNDLE_OUT=*) export "$e" ;;
        esac
      done <<< "${DOCKPIPE_EXTRA_ENV}"
      DOCKPIPE_ACTION="" # do not mount or run action in container
    fi
  fi
else
  DOCKPIPE_USER_ISOLATE_OVERRIDE="${DOCKPIPE_ISOLATE:-}"
  DOCKPIPE_USER_ACT_OVERRIDE="${DOCKPIPE_ACTION:-}"
  DOCKPIPE_FIRST_STEP_EXTRA_PRE=("${DOCKPIPE_PRE_SCRIPTS[@]+"${DOCKPIPE_PRE_SCRIPTS[@]}"}")
  DOCKPIPE_PRE_SCRIPTS=()
  DOCKPIPE_LOCKED_VAR_NAMES=()
  for pv in "${DOCKPIPE_WORKFLOW_VAR_OVERRIDES[@]+"${DOCKPIPE_WORKFLOW_VAR_OVERRIDES[@]}"}"; do
    [[ -z "$pv" ]] && continue
    DOCKPIPE_LOCKED_VAR_NAMES+=("${pv%%=*}")
  done
  unset DOCKPIPE_IMAGE DOCKPIPE_ACTION DOCKPIPE_BUILD DOCKPIPE_BUILD_CONTEXT DOCKPIPE_COMMIT_ON_HOST pv
fi

# Data volume: default named volume "dockpipe-data". --data-dir = bind mount; --no-data = no volume.
if [[ -n "${DOCKPIPE_NO_DATA}" ]]; then
  DOCKPIPE_DATA_VOLUME=""
  DOCKPIPE_DATA_DIR=""
else
  if [[ -n "${DOCKPIPE_DATA_DIR}" ]]; then
    DOCKPIPE_DATA_VOLUME=""
  else
    DOCKPIPE_DATA_VOLUME="${DOCKPIPE_DATA_VOLUME:-dockpipe-data}"
  fi
fi

# ------------------------------------------------------------------------------
# Default branch name when --repo without --branch (so pre-scripts see it).
# ------------------------------------------------------------------------------
if [[ -n "${DOCKPIPE_REPO_URL}" ]] && [[ -z "${DOCKPIPE_REPO_BRANCH}" ]]; then
  branch_prefix=""
  if [[ -n "${DOCKPIPE_BRANCH_PREFIX:-}" ]]; then
    branch_prefix="${DOCKPIPE_BRANCH_PREFIX}"
  else
    branch_prefix="${RESOLVER:-}"
    if [[ -z "$branch_prefix" ]] && [[ -n "${TEMPLATE:-}" ]]; then
      case "$TEMPLATE" in
        claude|agent-dev) branch_prefix=claude ;;
        codex)            branch_prefix=codex ;;
        *)                branch_prefix=dockpipe ;;
      esac
    fi
  fi
  branch_prefix="${branch_prefix:-dockpipe}"
  DOCKPIPE_REPO_BRANCH="${DOCKPIPE_WORK_BRANCH:-${branch_prefix}/agent-$(date +%Y%m%d-%H%M%S)}"
  echo "[dockpipe] No --branch; using new branch: ${DOCKPIPE_REPO_BRANCH}" >&2
fi

# ------------------------------------------------------------------------------
# When --repo without any run script: use clone-worktree so worktree logic lives in scripts/ only.
# ------------------------------------------------------------------------------
if [[ -n "${DOCKPIPE_REPO_URL}" ]] && [[ ${#DOCKPIPE_PRE_SCRIPTS[@]} -eq 0 ]]; then
  if [[ -z "${DOCKPIPE_DATA_DIR:-}" ]]; then
    DOCKPIPE_DATA_DIR="${HOME:-/tmp}/.dockpipe"
    echo "[dockpipe] Using ${DOCKPIPE_DATA_DIR} for worktree (set --data-dir to override)" >&2
    DOCKPIPE_DATA_VOLUME=""
  fi
  DOCKPIPE_PRE_SCRIPTS=("${DOCKPIPE_REPO_ROOT}/scripts/clone-worktree.sh")
fi

# ------------------------------------------------------------------------------
# Pre-scripts: run on host before container. Can set DOCKPIPE_WORKDIR, DOCKPIPE_COMMIT_ON_HOST, etc.
# (Multi-step workflows run pre-scripts inside the step loop.)
# ------------------------------------------------------------------------------
if [[ "${DOCKPIPE_STEPS_MODE:-0}" -ne 1 ]]; then
  for _pre in "${DOCKPIPE_PRE_SCRIPTS[@]:-}"; do
    [[ -z "$_pre" ]] && continue
    _pre_path="$_pre"
    if [[ "$_pre_path" != /* ]]; then
      if [[ -f "${DOCKPIPE_REPO_ROOT}/${_pre_path}" ]]; then
        _pre_path="${DOCKPIPE_REPO_ROOT}/${_pre_path}"
      fi
    fi
    if [[ ! -f "$_pre_path" ]]; then
      echo "Error: pre-script not found: ${_pre_path}" >&2
      exit 1
    fi
    echo "[dockpipe] Running pre-script: ${_pre_path}" >&2
    # shellcheck source=/dev/null
    source "${_pre_path}"
  done
  unset _pre _pre_path
fi

# When commit-on-host will run (worktree or pre-script), export commit-related vars from --env
if [[ -n "${DOCKPIPE_COMMIT_ON_HOST:-}" ]] && [[ -n "${DOCKPIPE_EXTRA_ENV:-}" ]]; then
  while IFS= read -r e; do
    case "$e" in
      DOCKPIPE_COMMIT_MESSAGE=*) export "$e" ;;
      DOCKPIPE_WORK_BRANCH=*) export "$e" ;;
      DOCKPIPE_BUNDLE_OUT=*) export "$e" ;;
      GIT_PAT=*) export "$e" ;;
    esac
  done <<< "${DOCKPIPE_EXTRA_ENV}"
fi

export DOCKPIPE_IMAGE DOCKPIPE_ACTION DOCKPIPE_WORKDIR DOCKPIPE_WORK_PATH DOCKPIPE_WORK_BRANCH DOCKPIPE_BUNDLE_OUT DOCKPIPE_EXTRA_MOUNTS DOCKPIPE_EXTRA_ENV DOCKPIPE_DETACH DOCKPIPE_DATA_VOLUME DOCKPIPE_DATA_DIR DOCKPIPE_REINIT DOCKPIPE_FORCE DOCKPIPE_COMMIT_ON_HOST

# Pass build path to runner (runner does not build; we build below before calling dockpipe_run).
if [[ -n "${DOCKPIPE_BUILD:-}" ]] && [[ -d "${DOCKPIPE_BUILD}" ]]; then
  export DOCKPIPE_BUILD DOCKPIPE_BUILD_CONTEXT
fi

source "${DOCKPIPE_REPO_ROOT}/lib/runner.sh"

if [[ "$SEEN_DASH" != "1" ]]; then
  if [[ "${DOCKPIPE_STEPS_MODE:-0}" -eq 1 ]]; then
    :
  else
    echo "Error: expected -- before command (e.g. dockpipe -- ls -la)" >&2
    exit 1
  fi
fi
if [[ $# -eq 0 ]] && [[ "${DOCKPIPE_STEPS_MODE:-0}" -ne 1 ]]; then
  echo "Error: no command given after --" >&2
  exit 1
fi

# Build image if a build path was set (from --template or --build or default).
if [[ "${DOCKPIPE_STEPS_MODE:-0}" -ne 1 ]]; then
  if [[ -n "${DOCKPIPE_BUILD:-}" ]] && [[ -n "${DOCKPIPE_BUILD_CONTEXT:-}" ]]; then
    # dev image Dockerfile FROMs dockpipe-base-dev; ensure base exists first.
    if [[ "${DOCKPIPE_IMAGE}" == "dockpipe-dev" ]]; then
    if ! docker image inspect dockpipe-base-dev:latest &>/dev/null; then
      docker build -q -t dockpipe-base-dev -f "${DOCKPIPE_REPO_ROOT}/images/base-dev/Dockerfile" "${DOCKPIPE_BUILD_CONTEXT}"
      fi
    fi
    docker build -q -t "${DOCKPIPE_IMAGE}" -f "${DOCKPIPE_BUILD}/Dockerfile" "${DOCKPIPE_BUILD_CONTEXT}"
    unset DOCKPIPE_BUILD DOCKPIPE_BUILD_CONTEXT
  fi
fi

if [[ "${DOCKPIPE_STEPS_MODE:-0}" -eq 1 ]]; then
  dockpipe_workflow_run_steps "${DOCKPIPE_WORKFLOW_CONFIG}" "${DOCKPIPE_WORKFLOW_ROOT}" "${DOCKPIPE_REPO_ROOT}" "$@"
  exit $?
fi

dockpipe_run "$@"
