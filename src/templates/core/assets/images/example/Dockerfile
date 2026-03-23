# Minimal example image for templates/core/assets/scripts/example-run.sh (tag dockpipe-example). Safe to bundle: Debian + bash only.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends bash ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /work
CMD ["bash", "-lc", "echo 'dockpipe-example: edit templates/core/assets/images/example/Dockerfile or use dockpipe --workflow <name>'"]
