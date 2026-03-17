# Heavier dev base: base-dev + common dev tooling.
FROM dockpipe-base-dev:latest

# Ensure we have the base image name for rebuild order; in CI you build base-dev first.
# Here we extend with more tools.
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    file \
    less \
    man-db \
    openssh-client \
    patch \
    unzip \
    zip \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /work
