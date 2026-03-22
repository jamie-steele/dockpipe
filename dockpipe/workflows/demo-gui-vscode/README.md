# demo-gui-vscode

Minimal **code-server** demo: host script runs Docker with **`-v "$DOCKPIPE_WORKDIR:/work"`** so your project (e.g. this repo with **`--workdir .`**) is what you edit in the browser.

```bash
dockpipe --workflow demo-gui-vscode --resolver vscode --runtime docker --workdir /path/to/checkout --
```

See **`templates/core/resolvers/vscode/README.md`** for **`CODE_SERVER_PORT`**, auth, and tuning.
