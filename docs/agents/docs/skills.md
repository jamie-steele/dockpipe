# Skills

Read when routing tasks to DorkPipe skills or rendering assistant-specific skill formats.

## Principle

Skill ids are target-independent. Codex and Claude are render targets/adapters, not DockPipe concepts.

Use:

```yaml
skills:
  - dorkpipe-core-review
  - dorkpipe-token-optimization
```

Do not use target-specific skill routing keys. Keep routing neutral and let the renderer adapt the
skill for Codex, Claude, or another target.

## Installed Codex Skills

- `dorkpipe-agentic-yaml`
- `dorkpipe-core-review`
- `dorkpipe-package-authoring`
- `dorkpipe-token-optimization`
- `dorkpipe-yaml-workflows`

## Render Commands

```bash
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --list
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --target codex
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --target claude --output /path/to/claude-skills
```

Codex default output:

```text
~/.codex/skills/<skill-name>/SKILL.md
```

Claude requires `--output` until a safe documented global install path is confirmed.

## Source Of Truth

Curated DorkPipe skill sources live in:

```text
packages/dorkpipe/resolvers/dorkpipe/assets/skills/
```
