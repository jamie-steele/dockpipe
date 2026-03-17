#!/usr/bin/env bash
# Example script: clone repo, create/reuse worktree, run Claude, commit all changes.
# Run inside dockpipe with: --template claude --mount "$REPOS_DIR:/repos" and env:
#   REPO_URL, BRANCH, [BASE_BRANCH], [PROMPT], [GIT_PAT]
# No separate action needed; this script performs the commit.
set -euo pipefail

REPO_PATH="/repos/${REPO_NAME:?REPO_NAME required}"
SAFE_BRANCH="${BRANCH//\//-}"
WORKTREE_PATH="/repos/${REPO_NAME}/worktrees/${SAFE_BRANCH}"

# Git auth
if [[ -n "${GIT_PAT:-}" ]]; then
  git config --global credential.helper \
    '!f() { echo "username=pat"; echo "password='"${GIT_PAT}"'"; }; f'
fi

# Clone or refresh
if [[ ! -d "${REPO_PATH}/.git" ]]; then
  echo "[claude-worktree] Cloning ${REPO_URL} ..."
  git clone "${REPO_URL}" "${REPO_PATH}"
else
  echo "[claude-worktree] Fetching latest ..."
  git -C "${REPO_PATH}" fetch --all --prune
fi

cd "${REPO_PATH}"

if [[ -z "${BASE_BRANCH:-}" ]]; then
  BASE_BRANCH=$(git ls-remote --symref origin HEAD | awk '/^ref:/ {sub("refs/heads/", "", $2); print $2}')
  echo "[claude-worktree] Detected default branch: ${BASE_BRANCH}"
fi

if [[ ! -d "${WORKTREE_PATH}" ]]; then
  echo "[claude-worktree] Creating worktree for branch: ${BRANCH}"
  mkdir -p "$(dirname "${WORKTREE_PATH}")"
  if git ls-remote --exit-code --heads origin "${BRANCH}" >/dev/null 2>&1; then
    git worktree add "${WORKTREE_PATH}" --track -b "${BRANCH}" "origin/${BRANCH}"
  else
    git worktree add "${WORKTREE_PATH}" -b "${BRANCH}" "origin/${BASE_BRANCH}"
  fi
else
  echo "[claude-worktree] Reusing worktree for branch: ${BRANCH}"
  git -C "${WORKTREE_PATH}" pull --rebase 2>/dev/null || true
fi

cd "${WORKTREE_PATH}"

# Pre-trust for Claude
CLAUDE_JSON="/claude-home/.claude.json"
if [[ -f "${CLAUDE_JSON}" ]] || true; then
  node -e "
const fs = require('fs');
let c = {};
try { c = JSON.parse(fs.readFileSync('${CLAUDE_JSON}', 'utf8')); } catch(e) {}
if (!c.projects) c.projects = {};
if (!c.projects['${WORKTREE_PATH}']) c.projects['${WORKTREE_PATH}'] = {};
c.projects['${WORKTREE_PATH}'].hasTrustDialogAccepted = true;
fs.writeFileSync('${CLAUDE_JSON}', JSON.stringify(c, null, 2));
" 2>/dev/null || true
fi

# Run Claude
if [[ -z "${PROMPT:-}" ]]; then
  echo "[claude-worktree] Starting interactive Claude (type /exit to finish) ..."
  claude --dangerously-skip-permissions
else
  echo "[claude-worktree] Running Claude ..."
  claude --dangerously-skip-permissions -p "${PROMPT}"
fi

# Commit all changes
if ! git diff --quiet HEAD 2>/dev/null || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
  git add -A
  git commit -m "claude: ${PROMPT:-automated}

Branch: ${BRANCH} (base: ${BASE_BRANCH})"
  echo "[claude-worktree] Committed: $(git log -1 --format='%H')"
else
  echo "[claude-worktree] No changes to commit."
fi
