# test

**Purpose:** **CI dogfood** — run **`go test`**, **`go vet`**, **`govulncheck`**, and **`gosec`** **inside Docker**, matching **`scripts/ci-local.sh`** / the host job. The host job still runs the same tools first for fast failure; this workflow re-runs them in the isolate to prove DockPipe’s multi-step + mounts.

**Mounts:** use **`--mount "$(go env GOPATH)/pkg:/go/pkg:rw`** so the module cache is shared. **Do not** mount the host **`GOBIN`**: **`govulncheck`** / **`gosec`** are **`go install`**’d in-container so they match the image’s Go. **`govulncheck`** needs outbound network (HTTPS to **`vuln.go.dev`**) on first run in CI; local runs need the same unless you only rely on the host job.

**For recordings** (real tests → scan → **Codex** review), use **`test-demo`** (requires **`OPENAI_API_KEY`** for the last step).

```bash
dockpipe --workflow test --workdir /path/to/repo \
  --mount "$(go env GOPATH)/pkg:/go/pkg:rw" \
  --
```

See **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** · **[../test-demo/README.md](../test-demo/README.md)**.
