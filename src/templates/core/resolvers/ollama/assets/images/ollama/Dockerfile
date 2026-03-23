# Local model pass: official Ollama binary + dockpipe entrypoint.
# Build from repo root: docker build -t dockpipe-ollama -f templates/core/resolvers/ollama/assets/images/ollama/Dockerfile .
# Runtime: workflow starts `ollama serve` then calls scripts/review-pipeline/optional-local-model-summary.sh.
FROM ollama/ollama:latest
USER root
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends bash curl jq python3 ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY assets/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
WORKDIR /work
ENTRYPOINT ["/entrypoint.sh"]
