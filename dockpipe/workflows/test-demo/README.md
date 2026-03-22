# test-demo

**Purpose:** **Video / showcase** — **tests → `go vet` → security brief** in four steps.

**CI** uses **`test`** (no `go test`; **`go vet`** only in the container). **Govulncheck** / **gosec** stay on the **host** in CI.

```bash
dockpipe --workflow test-demo --workdir /path/to/repo --mount "$(go env GOPATH)/pkg:/go/pkg:rw" --
```

See **`make demo-record`**. See **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)**.
