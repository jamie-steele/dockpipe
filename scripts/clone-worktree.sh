# Run script: clone (or fetch) repo and create/reuse a worktree for DOCKPIPE_REPO_BRANCH.
# Exports DOCKPIPE_WORKDIR and DOCKPIPE_COMMIT_ON_HOST=1 so the container runs in that worktree
# and the commit-worktree action runs on host after exit.
#
# Use with: dockpipe --repo <url> [--branch <name>] --run scripts/clone-worktree.sh -- ...
# When --branch is omitted, dockpipe sets DOCKPIPE_REPO_BRANCH before run scripts.

set -euo pipefail

if [[ -z "${DOCKPIPE_REPO_URL:-}" ]]; then
  echo "[dockpipe] clone-worktree: DOCKPIPE_REPO_URL not set; skipping." >&2
  return 0
fi

data_dir="${DOCKPIPE_DATA_DIR:-${HOME:-/tmp}/.dockpipe}"
mkdir -p "${data_dir}"

repo_name="$(basename "${DOCKPIPE_REPO_URL}" .git)"
safe_branch="${DOCKPIPE_REPO_BRANCH//\//-}"
repo_path="${data_dir}/repos/${repo_name}"
worktree_path="${data_dir}/repos/${repo_name}/worktrees/${safe_branch}"

tmp_config=""
if [[ -n "${GIT_PAT:-}" ]]; then
  tmp_config=$(mktemp)
  echo '[credential]' >> "${tmp_config}"
  echo "	helper = !f() { echo username=pat; echo password=${GIT_PAT}; }; f" >> "${tmp_config}"
  export GIT_CONFIG_GLOBAL="${tmp_config}"
fi

if [[ ! -d "${repo_path}/.git" ]]; then
  echo "[dockpipe] Cloning ${DOCKPIPE_REPO_URL} ..." >&2
  git clone "${DOCKPIPE_REPO_URL}" "${repo_path}"
else
  echo "[dockpipe] Fetching latest ..." >&2
  git -C "${repo_path}" fetch --all --prune 2>/dev/null || true
fi

if [[ -n "${tmp_config}" ]]; then
  rm -f "${tmp_config}"
  unset GIT_CONFIG_GLOBAL
fi

base_branch="${BASE_BRANCH:-}"
if [[ -z "$base_branch" ]]; then
  base_branch=$(git -C "${repo_path}" ls-remote --symref origin HEAD 2>/dev/null | awk '/^ref:/ {sub("refs/heads/", "", $2); print $2}')
  [[ -z "$base_branch" ]] && base_branch="main"
fi

if [[ ! -d "${worktree_path}" ]]; then
  echo "[dockpipe] Creating worktree for branch: ${DOCKPIPE_REPO_BRANCH}" >&2
  mkdir -p "$(dirname "${worktree_path}")"
  if git -C "${repo_path}" ls-remote --exit-code --heads origin "${DOCKPIPE_REPO_BRANCH}" >/dev/null 2>&1; then
    git -C "${repo_path}" worktree add "${worktree_path}" --track -b "${DOCKPIPE_REPO_BRANCH}" "origin/${DOCKPIPE_REPO_BRANCH}"
  else
    git -C "${repo_path}" worktree add "${worktree_path}" -b "${DOCKPIPE_REPO_BRANCH}" "origin/${base_branch}"
  fi
else
  echo "[dockpipe] Reusing worktree for branch: ${DOCKPIPE_REPO_BRANCH}" >&2
  git -C "${worktree_path}" pull --rebase 2>/dev/null || true
fi

export DOCKPIPE_WORKDIR="${worktree_path}"
export DOCKPIPE_COMMIT_ON_HOST=1
