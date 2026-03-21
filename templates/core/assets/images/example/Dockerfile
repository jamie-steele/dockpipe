# Minimal example image for scripts/example-run.sh (IMAGE=dockpipe-example).
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends bash ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /work
CMD ["bash", "-lc", "echo 'dockpipe-example: edit images/example/Dockerfile or use dockpipe --workflow <name>'"]
