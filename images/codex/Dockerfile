# Codex template: base-dev + Node.js + OpenAI Codex CLI. For AI-assisted workflows.
# Requires OPENAI_API_KEY or Codex login at run time.
FROM node:20-slim

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

COPY lib/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /work
USER node
ENTRYPOINT ["/entrypoint.sh"]
