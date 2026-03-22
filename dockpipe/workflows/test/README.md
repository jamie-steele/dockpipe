# test

**Purpose:** **This repo’s CI** — **scans in Docker, no `go test`**: **`go vet ./...`** in the container, plus isolation preamble + bundled security brief.

**Govulncheck** and **gosec** run on the **host** in the same GitHub Actions job (before/after this step) — not duplicated inside the workflow so we avoid Go toolchain / binary mismatches across images.

**For recordings** (real tests → scan → **Codex** review), use **`test-demo`** (requires **`OPENAI_API_KEY`** for the last step).

```bash
dockpipe --workflow test --workdir /path/to/repo --mount "$(go env GOPATH)/pkg:/go/pkg:rw" --
```

See **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** · **[../test-demo/README.md](../test-demo/README.md)**.
