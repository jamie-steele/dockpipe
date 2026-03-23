# OSS VS Code (code-server) — same upstream family as templates/core/resolvers/vscode (codercom/code-server).
# Used with strategy worktree + --resolver vscode. For a browser IDE with host port publish, prefer --workflow vscode.
FROM codercom/code-server:latest

USER root
RUN mkdir -p /work && chmod 777 /work

COPY assets/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /work

# Parent image runs as `coder` (non-root).
USER coder

ENTRYPOINT ["/entrypoint.sh"]
