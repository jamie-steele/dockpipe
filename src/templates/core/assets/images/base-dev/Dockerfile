# Light dev base: minimal tooling for scripts and generic commands.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    jq \
    ripgrep \
    && rm -rf /var/lib/apt/lists/*

RUN git config --global --add safe.directory '*' \
    && git config --global init.defaultBranch main

RUN mkdir -p /work && chmod 777 /work

COPY lib/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /work
ENTRYPOINT ["/entrypoint.sh"]
