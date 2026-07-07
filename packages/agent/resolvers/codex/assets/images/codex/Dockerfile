# Codex template: Node.js + Codex CLI + the worker baseline DorkPipe orchestration lanes expect.
# Requires OPENAI_API_KEY or Codex login at run time.
FROM node:20-slim

# Do not install distro bubblewrap: Debian's /usr/bin/bwrap is often too old and lacks --argv0,
# which Codex's sandbox passes — that breaks exec with "bwrap: Unknown option --argv0".
# Codex falls back to its vendored bubblewrap when system bwrap is absent (see CLI warning).
# Keep this base image lean. Consumer workflows can add project-specific tooling at run time with
# DORKPIPE_ORCH_CONTAINER_APT_PACKAGES or through normal image.packages-derived images.
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    file \
    git \
    gnupg \
    iproute2 \
    jq \
    less \
    lsof \
    man-db \
    openssh-client \
    patch \
    procps \
    ripgrep \
    unzip \
    util-linux \
    wget \
    zip \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @openai/codex

RUN git config --system user.email "dockpipe@local" \
    && git config --system user.name "Dockpipe" \
    && git config --system init.defaultBranch main \
    && git config --system --add safe.directory '*'

RUN mkdir -p /work && chmod 777 /work

COPY assets/entrypoint.sh /entrypoint.sh
RUN cat > /codex-entrypoint.sh <<'EOF'
#!/bin/bash
set -euo pipefail
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin${PATH:+:$PATH}"

# Clear sudo-like markers some CLIs interpret as elevated mode.
unset SUDO_COMMAND SUDO_USER SUDO_UID SUDO_GID 2>/dev/null || true

# Start as root only so the entrypoint can drop to node consistently.
if [[ -z "${DOCKPIPE_SKIP_DROP_TO_NODE:-}" ]] && [[ "$(id -u)" -eq 0 ]] && [[ -z "${DOCKPIPE_ENTRYPOINT_DROPPED:-}" ]] && id -u node &>/dev/null; then
  export DOCKPIPE_ENTRYPOINT_DROPPED=1
  if command -v runuser >/dev/null 2>&1; then
    exec runuser -u node -- /bin/bash /entrypoint.sh "$@"
  fi
  if command -v setpriv >/dev/null 2>&1; then
    exec setpriv --reuid="$(id -u node)" --regid="$(id -g node)" --init-groups -- /bin/bash /entrypoint.sh "$@"
  fi
  echo "[dockpipe] codex entrypoint: FATAL: need runuser or setpriv (util-linux) to drop root -> node" >&2
  exit 1
fi

# Claude/Codex-style CLIs treat IS_SANDBOX=1 as disposable-sandbox mode.
if [[ -z "${DOCKPIPE_NO_SANDBOX_ENV:-}" ]] && [[ -z "${IS_SANDBOX:-}" ]]; then
  export IS_SANDBOX=1
fi

exec /bin/bash /entrypoint.sh "$@"
EOF
RUN sed -i 's/\r$//' /entrypoint.sh /codex-entrypoint.sh \
    && chmod +x /entrypoint.sh /codex-entrypoint.sh

WORKDIR /work
# Quieter Rust logs for `codex exec` in demos/CI; override per run if you need debug.
ENV RUST_LOG=error
ENTRYPOINT ["/bin/bash", "-lc", "set -e; tr -d '\\015' < /entrypoint.sh > /tmp/entrypoint.sh; tr -d '\\015' < /codex-entrypoint.sh > /tmp/codex-entrypoint.sh; cat /tmp/entrypoint.sh > /entrypoint.sh; cat /tmp/codex-entrypoint.sh > /codex-entrypoint.sh; chmod +x /entrypoint.sh /codex-entrypoint.sh; exec /bin/bash /codex-entrypoint.sh \"$@\"", "dockpipe-codex"]
