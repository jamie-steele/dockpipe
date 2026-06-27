# agent

Tracked first-party package family for governed AI stages in DockPipe.

What lives here:

- `resolvers/claude/` — promoted from `.staging/packages/agent/resolvers/claude/`
- `resolvers/codex/` — promoted from `.staging/packages/agent/resolvers/codex/`
- `resolvers/ollama/` — promoted from `.staging/packages/agent/resolvers/ollama/`
- `workflows/docs.orchestrate/` — YAML-first governed documentation orchestration example

This package keeps DockPipe's separation of concerns intact:

- DockPipe is the governed runtime and orchestration layer.
- Resolver profiles such as `claude` and `codex` are tool adapters, not the product.
- AI workers are workflow/package stages with explicit YAML contracts: prompts, context, access
  boundaries, model policy, output artifacts, verification, and approval requirements.
- DorkPipe is the preferred harness when those stages need model execution, verifier artifacts,
  escalation, and approval handling.

The original staging copies remain under `.staging/packages/agent/` for reviewed cleanup in a
separate change. This tracked package is the canonical first-party location going forward.

## Promoted resolvers

The promoted `claude`, `codex`, and `ollama` resolver trees preserve their existing:

- `config.yml` delegate workflow
- `profile` environment contract
- `README.md` usage notes
- `assets/compose/` examples
- `assets/images/<resolver>/Dockerfile`

No binaries, caches, vendor trees, local environment files, or build outputs were copied during
promotion.

## First workflow

`workflows/docs.orchestrate/` demonstrates the next layer:

1. declare agentic intent in `config.yml`
2. materialize DorkPipe request, plan, task, merge, verify, and approval artifacts
3. run workers through normal resolver profiles (`ollama`, `codex`, `claude`)
4. track cloud model budget and halt when policy requires it
5. require explicit human approval before treating generated output as promotable

See [workflows/docs.orchestrate/README.md](./workflows/docs.orchestrate/README.md).
