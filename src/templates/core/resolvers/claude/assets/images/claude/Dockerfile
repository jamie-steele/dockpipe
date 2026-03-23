# Claude Code template: base-dev + Node.js + Claude Code. For AI-assisted workflows.
# Requires ANTHROPIC_API_KEY or ~/.claude credentials at run time.
FROM node:20-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    jq \
    ripgrep \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

# --system so non-root USER below still sees identity (global would be under /root only).
RUN git config --system user.email "dockpipe@local" \
    && git config --system user.name "Dockpipe" \
    && git config --system init.defaultBranch main \
    && git config --system --add safe.directory '*'

RUN mkdir -p /work /claude-home && chmod 777 /work /claude-home

# Use generic dockpipe entrypoint (must copy from context when building from repo root)
COPY lib/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /work
# Non-root: Windows omits -u (Getuid -1); Linux still passes -u host:host which overrides USER.
# Avoids CLIs that refuse --dangerously-skip-permissions as root.
USER node
ENTRYPOINT ["/entrypoint.sh"]
