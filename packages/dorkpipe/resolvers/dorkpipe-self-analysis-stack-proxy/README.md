# dorkpipe-self-analysis-stack-proxy

This is the first DorkPipe workflow that consumes DockPipe's proxy-backed
network policy path end to end.

Lifecycle:

1. `stack_up` starts Postgres, Ollama, and an allowlist egress proxy with the DorkPipe host stack helper so Ollama can use Docker GPU access when available.
2. `self_analysis` runs in a disposable container with `security.profile: sidecar-client` and `security.network.mode: allowlist`, which compiles to proxy-backed enforcement.
3. `stack_down` tears the stack down with the DorkPipe host stack helper unless `DORKPIPE_DEV_STACK_AUTODOWN=0`.

Run it with:

```bash
make build
dockpipe --workflow dorkpipe-self-analysis-stack-proxy --workdir . --
```

Keep services up after the run:

```bash
DORKPIPE_DEV_STACK_AUTODOWN=0 dockpipe --workflow dorkpipe-self-analysis-stack-proxy --workdir . --
```

What DockPipe owns here:

- compiled runtime policy
- proxy env injection into the isolated container
- per-step proxy URL tokenization so the proxy sees the compiled `allow` / `block` rules

What stays package-specific:

- stack lifecycle and Ollama GPU setup/remediation behavior
- workflow vars that point the isolated analysis container at the host stack endpoints
- which services the stack runs
- which domains this workflow allowlists
- the proxy implementation under `packages/dorkpipe/...`

GPU note:

- **`DORKPIPE_DEV_STACK_GPU=auto`** is the default and will use NVIDIA when Docker can expose it to the Ollama container
- if the host has NVIDIA but Docker GPU access is missing, the workflow now offers the same remediation / CPU fallback prompt path used by Pipeon

Host endpoint note:

- like the existing stack workflow, this one currently exports `host.docker.internal` endpoints
- on Linux engines that do not provide that alias automatically, override the relevant env as needed
