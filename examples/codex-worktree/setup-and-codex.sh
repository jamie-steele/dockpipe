#!/usr/bin/env bash
# Example script: clone repo, create/reuse worktree, run Codex, commit all changes.
# Run inside dockpipe with: --template codex (named volume or bind mount at /dockpipe-data). Env:
#   REPO_URL, REPO_NAME, BRANCH, [BASE_BRANCH], [PROMPT], [GIT_PAT]
# No separate action needed; this script performs the commit.
set -euo pipefail

DATA="${DOCKPIPE_DATA:-/dockpipe-data}"
REPO_PATH="${DATA}/repos/${REPO_NAME:?REPO_NAME required}"
SAFE_BRANCH="${BRANCH//\//-}"
WORKTREE_PATH="${DATA}/repos/${REPO_NAME}/worktrees/${SAFE_BRANCH}"

if [[ -n "${GIT_PAT:-}" ]]; then
  git config --global credential.helper \
    '!f() { echo "username=pat"; echo "password='"${GIT_PAT}"'"; }; f'
fi

if [[ ! -d "${REPO_PATH}/.git" ]]; then
  echo "[codex-worktree] Cloning ${REPO_URL} ..."
  git clone "${REPO_URL}" "${REPO_PATH}"
else
  echo "[codex-worktree] Fetching latest ..."
  git -C "${REPO_PATH}" fetch --all --prune
fi

cd "${REPO_PATH}"

if [[ -z "${BASE_BRANCH:-}" ]]; then
  BASE_BRANCH=$(git ls-remote --symref origin HEAD | awk '/^ref:/ {sub("refs/heads/", "", $2); print $2}')
  echo "[codex-worktree] Detected default branch: ${BASE_BRANCH}"
fi

if [[ ! -d "${WORKTREE_PATH}" ]]; then
  echo "[codex-worktree] Creating worktree for branch: ${BRANCH}"
  mkdir -p "$(dirname "${WORKTREE_PATH}")"
  if git ls-remote --exit-code --heads origin "${BRANCH}" >/dev/null 2>&1; then
    git worktree add "${WORKTREE_PATH}" --track -b "${BRANCH}" "origin/${BRANCH}"
  else
    git worktree add "${WORKTREE_PATH}" -b "${BRANCH}" "origin/${BASE_BRANCH}"
  fi
else
  echo "[codex-worktree] Reusing worktree for branch: ${BRANCH}"
  git -C "${WORKTREE_PATH}" pull --rebase 2>/dev/null || true
fi

cd "${WORKTREE_PATH}"

if [[ -z "${PROMPT:-}" ]]; then
  echo "[codex-worktree] Starting interactive Codex (type /exit to finish) ..."
  codex
else
  echo "[codex-worktree] Running Codex ..."
  codex exec "${PROMPT}"
fi

if ! git diff --quiet HEAD 2>/dev/null || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
  git add -A
  git commit -m "codex: ${PROMPT:-automated}

Branch: ${BRANCH} (base: ${BASE_BRANCH})"
  echo "[codex-worktree] Committed: $(git log -1 --format='%H')"
else
  echo "[codex-worktree] No changes to commit."
fi
