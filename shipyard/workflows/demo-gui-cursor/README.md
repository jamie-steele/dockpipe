# demo-gui-cursor

Minimal **cursor-dev** demo: a long-lived **base-dev** container with **`/work`** mounted from **`--workdir`**, plus Cursor launch on the host.

```bash
dockpipe --workflow demo-gui-cursor --resolver cursor-dev --runtime docker --workdir /path/to/checkout --
```

See **`templates/core/resolvers/cursor-dev/README.md`** for options.
