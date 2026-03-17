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

RUN git config --global user.email "dockpipe@local" \
    && git config --global user.name "Dockpipe" \
    && git config --global init.defaultBranch main \
    && git config --global --add safe.directory '*'

RUN mkdir -p /work /claude-home && chmod 777 /work /claude-home

# Use generic dockpipe entrypoint (must copy from context when building from repo root)
COPY lib/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /work
ENTRYPOINT ["/entrypoint.sh"]
