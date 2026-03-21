# vscode (code-server)

## What it does

Starts **[code-server](https://github.com/coder/code-server)** (MIT-licensed VS Code in the browser) in a **separate Docker container** on the **host**, with your project at **`/work`**. The container listens on **8080** inside; Docker publishes it to **`127.0.0.1` + host port only** (not your LAN). By default the **host port is random** in the IANA dynamic range **49152–65535** each run unless you set **`CODE_SERVER_PORT`**.

Dockpipe’s built-in container run does **not** publish ports to the host, so this template uses a **host script** that runs `docker run … -p …` for you. That keeps the feature in a **template**, not core.

**Docker must be running** before you start this workflow (the script calls `docker` on the host). Dockpipe checks the daemon up front; **`dockpipe doctor`** verifies Docker + bash + bundled assets.

### “Like Slack / VS Code” — without bundling Electron

Desktop **Slack** and **VS Code** feel native because they ship **Electron** (a full Chromium runtime inside the app). That’s tens or hundreds of MB **per app**, not “zero dependency.”

**code-server** in Docker only speaks **HTTP**. Something on your machine has to render that UI. Dockpipe does **not** ship its own Chromium/Electron binary (that would be a huge second download and another thing to update).

- **Windows 10+:** the script opens **Microsoft Edge** in **`--app`** mode (a single window, app-like). Edge is **part of Windows** — you are **not** asked to install a separate browser. If Edge was removed, it falls back to Chrome **only if** it’s already installed.
- **macOS / Linux:** it looks for Edge or Chrome in the usual install locations, or a Chromium-based binary in `PATH` — same idea: **app window**, not “pick a random browser tab.”

If you don’t want any launcher, set **`CODE_SERVER_LAUNCH=none`** and open the printed URL however you like.

## Why use it

- Quick browser-based editing against the same tree you mount for other Dockpipe workflows.
- Disposable server container (`docker stop …` removes it when combined with `--rm`).

## How to run

From your project (or set `--workdir`):

```bash
dockpipe --workflow vscode
```

**Configuration** — use **`vars`** in the workflow YAML (e.g. `templates/vscode/config.yml` or your **`dockpipe.yml`**). **Environment variables work too:** export **`CODE_SERVER_*`** before `dockpipe`, or put them in **`.env`** in the workflow directory or repo root (see **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** and **[docs/cli-reference.md](../../docs/cli-reference.md)** for merge order). Keys **not** listed under **`vars`** in YAML keep their value from the environment — so e.g. **`CODE_SERVER_AUTH: "password"`** in YAML plus **`export CODE_SERVER_PASSWORD=…`** (or CI secrets) avoids putting the secret in the file.

One-off CLI overrides: **`--var KEY=value`** (locks the key for that run).

- **`CODE_SERVER_PORT`** — omit, **`auto`**, or **`random`** for a **random host port** each run; or set a fixed port in YAML (e.g. **`CODE_SERVER_PORT: "8443"`**). Published as **`127.0.0.1:port` only** (localhost).
- **Auth** — default in YAML is **`CODE_SERVER_AUTH: "none"`** (no password on **127.0.0.1** only). For a login page, set **`CODE_SERVER_AUTH: "password"`** and supply **`CODE_SERVER_PASSWORD`** via **`vars`**, **`.env`**, or **shell env** (omit everywhere to get a generated password printed in the log). Do not use `none` on shared networks or if the port is exposed beyond localhost.
- **`DOCKPIPE_SKIP_PULL=1`** — skip `docker pull` if the image is already local.
- **`CODE_SERVER_LAUNCH=app`** (default) — opens **Edge** (Windows: included with the OS) or **Chrome** if present, in **`--app`** mode. Set to **`none`** to skip launching.
- **`CODE_SERVER_WAIT=1`** (default) — dockpipe **stays running** until you **close the app window** or **Ctrl+C**; then the code-server container is stopped. **How “close” is detected (default):** **`CODE_SERVER_WAIT_SIGNAL=connections`** — poll **established TCP** to **`127.0.0.1:<port>`**. **Why shutdown can feel slow:** code-server often opens **several** sockets; the **last** one can stay **ESTABLISHED** for **seconds** after the window closes, so “wait for count = 0” lags. **Defaults now:** **`CODE_SERVER_DISCONNECT_TAIL_MAX=1`** + **`CODE_SERVER_DISCONNECT_MULTI_THRESHOLD=2`** — if we ever saw **≥2** connections, we also treat **≤1** (after **2** stable polls + confirm) as “session over” so you don’t wait on that last stray socket. Set **`CODE_SERVER_DISCONNECT_TAIL_MAX=0`** to require **strict zero** (slower but paranoid). **Windows** uses **`netstat.exe`** for counts (fast); PowerShell is only a fallback. **`CODE_SERVER_DISCONNECT_CONFIRM_SEC`**, **`CODE_SERVER_DISCONNECT_POLL_SEC`**, **`CODE_SERVER_CONNECT_POLL_SEC`** tune timing. **`CODE_SERVER_WAIT_SIGNAL=process`** uses browser PID / profile polling instead.
- **Browser profile / UI** — stable profile dir per port (see **`CODE_SERVER_BROWSER_PROFILE_DIR`**); **window title** **`CODE_SERVER_BROWSER_WINDOW_TITLE`** (default `VS Code`); **`CODE_SERVER_BROWSER_EXTRA_FLAGS`**.
- **`CODE_SERVER_WAIT=0`** — start code-server in the background and return immediately.

## Caveats / legal

- **Not** Microsoft Visual Studio Code. **code-server** is a third-party OSS project; **Coder** publishes **`codercom/code-server`** on Docker Hub. Review their license and terms.
- You run arbitrary Docker images; trust the image you set in **`CODE_SERVER_IMAGE`**.
- Bind-mounting your repo gives the container full read/write to those files.

## What persists

- **Ephemeral:** the code-server container (stopped → removed with `--rm`).
- **On disk:** your project files (normal bind mount); the **Edge/Chrome profile** for this workflow **persists** under the paths above (pin **`CODE_SERVER_PORT`** if you want one consistent profile instead of a new folder per random port).
