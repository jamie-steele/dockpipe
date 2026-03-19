# Run script: create/reuse a worktree for DOCKPIPE_REPO_BRANCH and export DOCKPIPE_WORKDIR.
# Exports DOCKPIPE_COMMIT_ON_HOST=1 so commit-worktree runs on the host after exit.
#
# When DOCKPIPE_USER_REPO_ROOT is set (same origin as DOCKPIPE_REPO_URL), the worktree is
# created with `git worktree add` from that checkout — same HEAD as your current branch,
# not origin/HEAD. Uncommitted changes are stashed on the main checkout and popped in the
# new worktree (disable with DOCKPIPE_STASH_UNCOMMITTED=0).
#
# Otherwise: clone/fetch DOCKPIPE_REPO_URL under DOCKPIPE_DATA_DIR (legacy mirror layout).

set -euo pipefail

# Never inherit a stale DOCKPIPE_WORKDIR from the parent shell (would mount the wrong tree).
unset DOCKPIPE_WORKDIR

if [[ -z "${DOCKPIPE_REPO_URL:-}" ]]; then
  echo "[dockpipe] clone-worktree: DOCKPIPE_REPO_URL not set (pass --repo or run dockpipe from a git clone with origin)." >&2
  return 1
fi

data_dir="${DOCKPIPE_DATA_DIR:-${HOME:-/tmp}/.dockpipe}"
mkdir -p "${data_dir}"

repo_name="$(basename "${DOCKPIPE_REPO_URL}" .git)"
safe_branch="${DOCKPIPE_REPO_BRANCH//\//-}"
repo_path="${data_dir}/repos/${repo_name}"
worktree_path="${data_dir}/repos/${repo_name}/worktrees/${safe_branch}"

user_root="${DOCKPIPE_USER_REPO_ROOT:-}"

if [[ -n "$user_root" ]] && git -C "$user_root" rev-parse --is-inside-work-tree &>/dev/null; then
  echo "[dockpipe] Worktree from local repo (new branch from your current HEAD): ${DOCKPIPE_REPO_BRANCH}" >&2
  mkdir -p "$(dirname "${worktree_path}")"

  if [[ ! -d "${worktree_path}" ]]; then
    stash_made=0
    if [[ "${DOCKPIPE_STASH_UNCOMMITTED:-1}" != "0" ]]; then
      dirty=0
      git -C "$user_root" diff --quiet || dirty=1
      git -C "$user_root" diff --cached --quiet || dirty=1
      if [[ -n "$(git -C "$user_root" ls-files -o --exclude-standard 2>/dev/null || true)" ]]; then
        dirty=1
      fi
      if [[ "$dirty" -eq 1 ]]; then
        echo "[dockpipe] Stashing uncommitted changes (incl. untracked) to replay in the worktree..." >&2
        git -C "$user_root" stash push -u -m "dockpipe worktree carry" || {
          echo "[dockpipe] clone-worktree: git stash push failed" >&2
          return 1
        }
        stash_made=1
      fi
    fi

    if ! git -C "$user_root" worktree add -b "${DOCKPIPE_REPO_BRANCH}" "${worktree_path}"; then
      if [[ "$stash_made" -eq 1 ]]; then
        git -C "$user_root" stash pop || true
      fi
      echo "[dockpipe] clone-worktree: git worktree add failed" >&2
      return 1
    fi

    if [[ "$stash_made" -eq 1 ]]; then
      if ! git -C "${worktree_path}" stash pop; then
        echo "[dockpipe] warning: stash pop in worktree had conflicts — resolve in: ${worktree_path}" >&2
      fi
    fi
  else
    echo "[dockpipe] Reusing worktree: ${DOCKPIPE_REPO_BRANCH}" >&2
    git -C "${worktree_path}" pull --rebase 2>/dev/null || true
  fi

  export DOCKPIPE_WORKDIR="${worktree_path}"
  export DOCKPIPE_COMMIT_ON_HOST=1
  return 0
fi

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
