# example.brain

`example.brain` is a package-owned starter workflow for repos that want durable,
repo-native brain guidance instead of one-off AI summaries.

It seeds:

- source-of-truth rules
- a repo knowledge page
- an open-gaps page
- a small index page

Default output paths:

- `docs/agents/brain/index.md`
- `docs/agents/brain/source-of-truth-rules.md`
- `docs/agents/brain/repo-knowledge.md`
- `docs/agents/brain/open-gaps.md`

The workflow is intentionally documentation-first. It favors stable guidance and reviewable docs over
deep implementation mutation.

## Use

```bash
dockpipe --package dorkpipe --workflow example.brain --
```

The generated files are uncommitted working-tree changes so maintainers can inspect and refine them.

## What it seeds

- Repo-native wording only. Durable output should not talk about mounts, `/work`, artifact roots,
  worker lanes, or orchestration internals unless the consumer repo explicitly owns those concepts.
- Source precedence. Current implementation claims should come from code and repo docs first.
- Conflict handling. Current state and intended direction should remain separate when they disagree.
- Small maintained follow-up docs. Open gaps should survive the run as durable repo guidance.

## When to customize

Fork or wrap this workflow when a repo needs:

- an external design corpus or SharePoint-backed notes index
- a richer TODO/index system
- repo-specific subsystem inventories
- stricter validation or stronger cloud-lane routing

The package-owned baseline docs live under:

- `packages/dorkpipe/resolvers/dorkpipe/assets/docs/example-brain/`

`example.brain` is currently the only package-owned native guidance workflow that materializes
durable consumer-repository documentation. Its `example_brain_baseline` shared collector loads
`baseline-rules.md` once, and the package helper places that seed before every task's repo-specific
context.

Materialized Markdown and YAML are checked before they become durable files. Stable guest paths may
appear in source packets, but durable output must cite repo-relative paths. A guest or host path is
rewritten only when one explicit mapping proves the repo-relative target; external or ambiguous
references and orchestration-only terminology are rejected.
