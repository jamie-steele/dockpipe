# Second agnostic example: smallest useful image with dockpipe entrypoint (for demos and CI smoke).
# Build: docker build -t dockpipe-minimal -f templates/core/assets/images/minimal/Dockerfile .
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends bash ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY assets/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
WORKDIR /work
ENTRYPOINT ["/entrypoint.sh"]
