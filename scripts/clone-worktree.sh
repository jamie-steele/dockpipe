# Run script: create/reuse a worktree for DOCKPIPE_REPO_BRANCH and export DOCKPIPE_WORKDIR.
# Exports DOCKPIPE_COMMIT_ON_HOST=1 so commit-worktree runs on the host after exit.
#
# When DOCKPIPE_USER_REPO_ROOT is set (same origin as DOCKPIPE_REPO_URL), the worktree is
# created with `git worktree add` from that checkout — same HEAD as your current branch,
# not origin/HEAD.
#
# Uncommitted work (tracked + untracked):
#   • Default: copy into the new worktree — `git diff` + apply + copy untracked files. Your
#     main checkout is NOT stashed or modified.
#   • Set DOCKPIPE_STASH_UNCOMMITTED=1 to use the legacy git stash flow instead (stash on main,
#     stash pop in worktree; trap restores stash on main if setup aborts).
#
# Gitignored local files (e.g. .env, appsettings.Development.json): optional
#   • `.dockpipe-worktreeinclude` (preferred) or `.worktreeinclude` (compatibility) — one glob
#     pattern per line (gitignore-style), # comments, same semantics as format v1 below.
#   • First line may be `# dockpipe-worktreeinclude-format: 1` (optional). If the format number
#     is not 1, dockpipe skips the file and warns (forward-compat if another tool changes format).
#
# Otherwise: clone/fetch DOCKPIPE_REPO_URL under DOCKPIPE_DATA_DIR (legacy mirror layout).

set -euo pipefail

# Never inherit a stale DOCKPIPE_WORKDIR from the parent shell (would mount the wrong tree).
unset DOCKPIPE_WORKDIR

# Copy working-tree changes from user_root into worktree_path (same HEAD). Does not touch
# the main checkout. On failure, resets worktree_path to a clean tree.
carry_uncommitted_copy_to_worktree() {
  local user_root="$1" worktree_path="$2"
  local patch_file
  patch_file=$(mktemp "${TMPDIR:-/tmp}/dockpipe-carry-XXXXXX.patch")

  git -C "$user_root" diff --binary HEAD >"${patch_file}"

  if [[ -s "${patch_file}" ]]; then
    if ! git -C "$worktree_path" apply "${patch_file}"; then
      rm -f "${patch_file}"
      echo "[dockpipe] clone-worktree: git apply failed (tracked changes); reverting worktree" >&2
      git -C "$worktree_path" reset --hard HEAD
      git -C "$worktree_path" clean -fd
      return 1
    fi
  fi
  rm -f "${patch_file}"

  # Untracked paths (may be empty; read must not trip set -e).
  local f err=0
  set +e
  while IFS= read -r -d '' f; do
    [[ -z "$f" ]] && continue
    mkdir -p "$(dirname "${worktree_path}/${f}")"
    cp -a "${user_root}/${f}" "${worktree_path}/${f}" || err=1
  done < <(git -C "$user_root" ls-files -o --exclude-standard -z 2>/dev/null || true)
  set -e
  if [[ "$err" -ne 0 ]]; then
    echo "[dockpipe] clone-worktree: copying untracked files failed; reverting worktree" >&2
    git -C "$worktree_path" reset --hard HEAD
    git -C "$worktree_path" clean -fd
    return 1
  fi

  return 0
}

# Copy one glob pattern line from repo root (gitignored files, etc.). pat must not contain spaces.
copy_worktree_include_pattern() {
  local user_root="$1" worktree_path="$2" pat="$3"
  local f rel
  set +e
  while IFS= read -r -d '' rel || [[ -n "${rel:-}" ]]; do
    [[ -z "$rel" ]] && continue
    mkdir -p "$(dirname "${worktree_path}/${rel}")"
    cp -a "${user_root}/${rel}" "${worktree_path}/${rel}"
  done < <(
    cd "$user_root" || exit 1
    shopt -s dotglob nullglob 2>/dev/null || true
    shopt -s globstar 2>/dev/null || true
    # Intentional pathname expansion on pat (one pattern per line; no spaces in pat).
    # shellcheck disable=SC2086
    for f in ${pat}; do
      [[ -e "$f" ]] || continue
      printf '%s\0' "$f"
    done
  )
  set -e
}

# Copy paths from .dockpipe-worktreeinclude or .worktreeinclude (gitignored files, etc.).
# Patterns are relative to user_root; use bash 4+ globstar for ** (enable with shopt).
copy_worktree_include_files() {
  local user_root="$1" worktree_path="$2"
  local incfile=""
  if [[ -f "${user_root}/.dockpipe-worktreeinclude" ]]; then
    incfile="${user_root}/.dockpipe-worktreeinclude"
  elif [[ -f "${user_root}/.worktreeinclude" ]]; then
    incfile="${user_root}/.worktreeinclude"
  else
    return 0
  fi

  echo "[dockpipe] Applying worktree include file: ${incfile}" >&2

  local line first=1
  set +e
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line//$'\r'/}"
    if [[ "$first" -eq 1 ]]; then
      first=0
      if [[ "$line" =~ ^[[:space:]]*#[[:space:]]*dockpipe-worktreeinclude-format:[[:space:]]*([0-9]+)[[:space:]]*$ ]]; then
        if [[ "${BASH_REMATCH[1]}" != "1" ]]; then
          echo "[dockpipe] clone-worktree: skipping ${incfile}: unsupported dockpipe-worktreeinclude-format ${BASH_REMATCH[1]} (only 1 is supported)" >&2
          set -e
          return 0
        fi
        continue
      fi
    fi
    [[ -z "${line//[[:space:]]/}" ]] && continue
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"

    local pat="$line"
    if [[ "$pat" =~ ^/ ]]; then
      echo "[dockpipe] clone-worktree: skipping absolute pattern in worktree include: ${pat}" >&2
      continue
    fi

    copy_worktree_include_pattern "$user_root" "$worktree_path" "$pat"
  done <"${incfile}"
  set -e
  return 0
}

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
    copy_carry=0
    dirty=0
    git -C "$user_root" diff --quiet || dirty=1
    git -C "$user_root" diff --cached --quiet || dirty=1
    if [[ -n "$(git -C "$user_root" ls-files -o --exclude-standard 2>/dev/null || true)" ]]; then
      dirty=1
    fi

    if [[ "$dirty" -eq 1 ]] && [[ "${DOCKPIPE_STASH_UNCOMMITTED:-0}" == "1" ]]; then
      echo "[dockpipe] Stashing uncommitted changes (incl. untracked) to replay in the worktree..." >&2
      git -C "$user_root" stash push -u -m "dockpipe worktree carry" || {
        echo "[dockpipe] clone-worktree: git stash push failed" >&2
        return 1
      }
      stash_made=1
      restore_stash_on_abort() {
        if [[ "${stash_made:-0}" -eq 1 ]] && [[ "${stash_restored_or_applied:-0}" -eq 0 ]]; then
          echo "[dockpipe] Restoring uncommitted changes on your main checkout (worktree setup did not finish)." >&2
          git -C "$user_root" stash pop || true
        fi
      }
      trap restore_stash_on_abort EXIT
    elif [[ "$dirty" -eq 1 ]]; then
      copy_carry=1
      echo "[dockpipe] Carrying uncommitted changes into the worktree (copy/apply; main checkout unchanged)..." >&2
      carry_failed_cleanup() {
        if [[ "${carry_started:-0}" -eq 1 ]] && [[ "${carry_complete:-0}" -eq 0 ]]; then
          echo "[dockpipe] Reverting worktree after incomplete carry..." >&2
          git -C "${worktree_path}" reset --hard HEAD || true
          git -C "${worktree_path}" clean -fd || true
        fi
      }
      trap carry_failed_cleanup EXIT
    fi

    if ! git -C "$user_root" worktree add -b "${DOCKPIPE_REPO_BRANCH}" "${worktree_path}"; then
      trap - EXIT
      if [[ "$stash_made" -eq 1 ]]; then
        git -C "$user_root" stash pop || true
        stash_made=0
      fi
      echo "[dockpipe] clone-worktree: git worktree add failed" >&2
      return 1
    fi

    carry_started=0
    carry_complete=0
    if [[ "$copy_carry" -eq 1 ]]; then
      carry_started=1
      if ! carry_uncommitted_copy_to_worktree "$user_root" "${worktree_path}"; then
        trap - EXIT
        echo "[dockpipe] clone-worktree: could not copy uncommitted changes into worktree" >&2
        return 1
      fi
      carry_complete=1
      trap - EXIT
    fi

    stash_restored_or_applied=0
    if [[ "$stash_made" -eq 1 ]]; then
      if ! git -C "${worktree_path}" stash pop; then
        echo "[dockpipe] warning: stash pop in worktree had conflicts — resolve in: ${worktree_path}" >&2
      fi
      stash_restored_or_applied=1
    fi
    trap - EXIT
  else
    echo "[dockpipe] Reusing worktree: ${DOCKPIPE_REPO_BRANCH}" >&2
    git -C "${worktree_path}" pull --rebase 2>/dev/null || true
  fi

  copy_worktree_include_files "${user_root}" "${worktree_path}" || true

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
