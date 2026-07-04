# DorkPipe YAML Workflows

Use this skill when authoring or reviewing workflow YAML.

## Mental Model

- Workflow/template: what happens.
- Runtime: where execution runs.
- Resolver: which tool/profile performs work.
- Strategy: lifecycle wrapper.
- Packaged workflow call: `workflow:` plus `package:`.

## Hard Rules

- Keep target-specific behavior in package assets/scripts.
- Use `kind: host` for host execution.
- Use step-level `runtime` and `resolver` for overrides.
- Do not reintroduce `runtime: package`.
- Keep authored surface in sync with schema and language support.

## Checks

- Run `dockpipe workflow validate <config.yml>`.
- For package workflows, run `dockpipe package compile workflows --from <package> --force`.
- Confirm scripts resolve through existing `scripts/<domain>/...` rules.
