# Claude Code template: Node.js + Claude CLI + the worker baseline DorkPipe orchestration lanes expect.
# Requires ANTHROPIC_API_KEY or ~/.claude credentials at run time.
FROM node:20-slim

# DorkPipe orchestration invokes the resolver image directly, so bake in the same toolchain shape
# the bundled claude workflow would otherwise add via image.packages.apt.
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    build-essential \
    ca-certificates \
    cargo \
    curl \
    file \
    git \
    golang-go \
    iproute2 \
    jq \
    less \
    lsof \
    man-db \
    openssh-client \
    patch \
    procps \
    python3 \
    ripgrep \
    rustc \
    unzip \
    util-linux \
    zip \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

# --system so non-root USER below still sees identity (global would be under /root only).
RUN git config --system user.email "dockpipe@local" \
    && git config --system user.name "Dockpipe" \
    && git config --system init.defaultBranch main \
    && git config --system --add safe.directory '*'

RUN mkdir -p /work /claude-home && chmod 777 /work /claude-home

COPY assets/entrypoint.sh /entrypoint.sh
RUN cat > /claude-entrypoint.sh <<'EOF'
#!/bin/bash
set -euo pipefail
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin${PATH:+:$PATH}"

# Clear sudo-like markers some CLIs interpret as elevated mode.
unset SUDO_COMMAND SUDO_USER SUDO_UID SUDO_GID 2>/dev/null || true

# Some hosts still start the container as root even when the image sets USER node.
# Claude behaves better when it runs as node in the disposable resolver image.
if [[ -z "${DOCKPIPE_SKIP_DROP_TO_NODE:-}" ]] && [[ "$(id -u)" -eq 0 ]] && [[ -z "${DOCKPIPE_ENTRYPOINT_DROPPED:-}" ]] && id -u node &>/dev/null; then
  export DOCKPIPE_ENTRYPOINT_DROPPED=1
  if command -v runuser >/dev/null 2>&1; then
    exec runuser -u node -- /bin/bash /entrypoint.sh "$@"
  fi
  if command -v setpriv >/dev/null 2>&1; then
    exec setpriv --reuid="$(id -u node)" --regid="$(id -g node)" --init-groups -- /bin/bash /entrypoint.sh "$@"
  fi
  echo "[dockpipe] claude entrypoint: FATAL: need runuser or setpriv (util-linux) to drop root -> node" >&2
  exit 1
fi

# Claude/Codex-style CLIs treat IS_SANDBOX=1 as disposable-sandbox mode.
if [[ -z "${DOCKPIPE_NO_SANDBOX_ENV:-}" ]] && [[ -z "${IS_SANDBOX:-}" ]]; then
  export IS_SANDBOX=1
fi

exec /bin/bash /entrypoint.sh "$@"
EOF
RUN chmod +x /entrypoint.sh /claude-entrypoint.sh

WORKDIR /work
# Non-root: Windows omits -u (Getuid -1); Linux still passes -u host:host which overrides USER.
# Avoids CLIs that refuse --dangerously-skip-permissions as root.
USER node
ENTRYPOINT ["/claude-entrypoint.sh"]
