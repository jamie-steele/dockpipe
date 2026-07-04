# `dorkpipe` package tests

**Go:** `go test` from **`lib/`** (module `dorkpipe.orchestrator`):

```bash
go test -C packages/dorkpipe/lib ./...
```

**Shell:** resolver scripts under **`resolvers/dorkpipe/assets/scripts/`**:

```bash
dockpipe package test --only dorkpipe
```

Individual scripts: `test_normalize_ci_scans.sh`, `test_user_insight_queue.sh`.
