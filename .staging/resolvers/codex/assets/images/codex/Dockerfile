# Codex template: base-dev + Node.js + OpenAI Codex CLI. For AI-assisted workflows.
# Requires OPENAI_API_KEY or Codex login at run time.
FROM node:20-slim

# Do not install distro bubblewrap: Debian's /usr/bin/bwrap is often too old and lacks --argv0,
# which Codex's sandbox passes — that breaks exec with "bwrap: Unknown option --argv0".
# Codex falls back to its vendored bubblewrap when system bwrap is absent (see CLI warning).
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    jq \
    ripgrep \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @openai/codex

RUN git config --system user.email "dockpipe@local" \
    && git config --system user.name "Dockpipe" \
    && git config --system init.defaultBranch main \
    && git config --system --add safe.directory '*'

RUN mkdir -p /work && chmod 777 /work

COPY assets/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /work
# Quieter Rust logs for `codex exec` in demos/CI; override per run if you need debug.
ENV RUST_LOG=error
USER node
ENTRYPOINT ["/entrypoint.sh"]
