# dorkpipe-self-analysis-stack-proxy

This is the first DorkPipe workflow that consumes DockPipe's proxy-backed
network policy path end to end.

Lifecycle:

1. `stack_up` starts Postgres, Ollama, and an allowlist egress proxy with DockPipe's `compose_up` builtin.
2. `self_analysis` runs in a disposable container with `security.profile: sidecar-client` and `security.network.mode: allowlist`, which compiles to proxy-backed enforcement.
3. `stack_down` tears the stack down with DockPipe's `compose_down` builtin unless `DORKPIPE_DEV_STACK_AUTODOWN=0`.

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

- Compose lifecycle
- compose-to-workflow exports for `DOCKPIPE_POLICY_PROXY_URL`
- compiled runtime policy
- proxy env injection into the isolated container
- per-step proxy URL tokenization so the proxy sees the compiled `allow` / `block` rules

What stays package-specific:

- which services the stack runs
- which domains this workflow allowlists
- the proxy implementation under `packages/dorkpipe/...`

Host endpoint note:

- like the existing stack workflow, this one currently exports `host.docker.internal` endpoints
- on Linux engines that do not provide that alias automatically, override the relevant env as needed
