# Light dev base: minimal tooling for scripts and generic commands.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    iproute2 \
    jq \
    lsof \
    procps \
    ripgrep \
    && rm -rf /var/lib/apt/lists/*

# docker run -u <uid>:<gid> does not set HOME; Cursor/VS Code remote server installs to
# $HOME/.cursor-server — empty HOME becomes //.cursor-server and mkdir fails.
ENV HOME=/work

# Cursor's server install uses `base64 -D` (BSD); GNU coreutils only accepts `-d`.
# Previous sh wrapper began with `set --`, which cleared argv before the loop — base64 ran with no -d and
# corrupted install output (bash then tried to execute base64-looking tokens: "command not found").
RUN cat > /usr/local/bin/base64 <<'EOF'
#!/usr/bin/env bash
rb=/usr/bin/base64
[[ -x "$rb" ]] || rb=/bin/base64
args=()
for arg; do
  case "$arg" in
    -D) args+=(-d) ;;
    *) args+=("$arg") ;;
  esac
done
exec "$rb" "${args[@]}"
EOF
RUN chmod +x /usr/local/bin/base64
ENV PATH=/usr/local/bin:${PATH}

# HOME is /work; git --global needs that directory to exist before writing ~/.gitconfig.
RUN mkdir -p /work && chmod 777 /work \
    && git config --global --add safe.directory '*' \
    && git config --global init.defaultBranch main

COPY assets/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /work
ENTRYPOINT ["/entrypoint.sh"]
