# vscode (code-server)

## What it does

Starts **[code-server](https://github.com/coder/code-server)** (MIT-licensed VS Code in the browser) in a **separate Docker container** on the **host**, with your project at **`/work`** and port **`8080`** inside mapped to **`CODE_SERVER_PORT`** on localhost.

Dockpipe’s built-in container run does **not** publish ports to the host, so this template uses a **host script** that runs `docker run … -p …` for you. That keeps the feature in a **template**, not core.

## Why use it

- Quick browser-based editing against the same tree you mount for other Dockpipe workflows.
- Disposable server container (`docker stop …` removes it when combined with `--rm`).

## How to run

From your project (or set `--workdir`):

```bash
dockpipe --workflow vscode
```

Optional:

- **`--var CODE_SERVER_PORT=8443`** — host port (container stays on 8080 internally).
- **`--var CODE_SERVER_PASSWORD=…`** — stable password; if unset, a password is generated and printed once.
- **`DOCKPIPE_SKIP_PULL=1`** — skip `docker pull` if the image is already local.

## Caveats / legal

- **Not** Microsoft Visual Studio Code. **code-server** is a third-party OSS project; **Coder** publishes **`codercom/code-server`** on Docker Hub. Review their license and terms.
- You run arbitrary Docker images; trust the image you set in **`CODE_SERVER_IMAGE`**.
- Bind-mounting your repo gives the container full read/write to those files.

## What persists

- **Ephemeral:** the code-server container (stopped → removed with `--rm`).
- **On disk:** your project files (normal bind mount); nothing else is added unless you change **`CODE_SERVER_IMAGE`** behavior.
