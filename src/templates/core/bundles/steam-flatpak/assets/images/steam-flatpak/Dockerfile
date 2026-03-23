# Steam (Flathub) + dockpipe entrypoint. Build context: repository root (COPY assets/entrypoint.sh).
# Runtime needs a host script: scripts/steam-flatpak/docker-steam-flatpak.sh (X11/Wayland, /dev/dri, dbus, --privileged).
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    dbus \
    flatpak \
    fuse3 \
    gnupg \
    libglib2.0-bin \
    && rm -rf /var/lib/apt/lists/*

RUN flatpak remote-add --system --if-not-exists flathub https://dl.flathub.org/repo/flathub.flatpakrepo \
    && flatpak install --system -y --noninteractive flathub com.valvesoftware.Steam

COPY assets/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN mkdir -p /work && chmod 777 /work
WORKDIR /work

ENTRYPOINT ["/entrypoint.sh"]
CMD ["flatpak", "run", "com.valvesoftware.Steam"]
