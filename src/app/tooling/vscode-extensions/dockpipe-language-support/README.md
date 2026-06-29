# DockPipe Language Support (VS Code)

Language support for DockPipe authoring:

- `.pipe` PipeLang syntax highlighting
- PipeLang snippets and keyword completion
- PipeLang model awareness for primitive, object/interface, and `List<T>` field types
- DockPipe `config.yml` IntelliSense for common workflow keys, including `cwd` and `scopes` value suggestions (`repo`, `source`, `artifacts`)
- DorkPipe agent path snippets for `scope:artifacts:...`, `scope:workflow:<name>:...`, and `scope:package:<name>:...` references
- DockPipe `config.yml` support for optional authored `view:` metadata (entry routing, pages, sections, and field-path driven launcher layouts)
- Up-to-date workflow help for packaged workflow steps (`workflow:` + `package:`), Compose host built-ins, and authored security/runtime policy blocks
- DockPipe `package.yml` hover/docs and top-level key completion
- DockPipe `package.yml` `icon` / `artwork` metadata hints for package-owned launcher/tooling assets
- DockPipe `package.yml` image metadata hints for package-owned OCI image refs
- DockPipe `package.yml` support for `script_contract.inject` with valid generic injectable suggestions
- DockPipe `dockpipe.config.json` hover/docs and section-key completion
- First-party package script IntelliSense for workflow cwd, `dockpipe scope`, and focused DockPipe SDK helpers in shell, PowerShell, Python, and Go
- Runtime path env suggestions for scripts: `DOCKPIPE_SOURCE_ROOT`, `DOCKPIPE_ARTIFACT_ROOT`, `DOCKPIPE_OUTPUT_ROOT`, and `DOCKPIPE_STEP_CWD`
- Structure-aware YAML semantic coloring for workflow keys, step keys, `vars:` fields, and `types:` entries
- YAML parse diagnostics for DockPipe workflow files (`config.yml` / `config.yaml`)
- Hover/docs for top-level workflow keys, step keys, `types:` entries, and `vars:` fields from PipeLang XML summaries (`types:` entrypoint)
- `vars:` value suggestions from implementing class defaults and nearby `Struct` known-values
- Completion/hover for SDK-object patterns:
  - shell:
    - cwd/source: prefer `pwd` under explicit workflow `cwd`; use `dockpipe scope source` when a script must resolve the source checkout from another cwd
    - getters: `dockpipe get workflow_name`, `dockpipe get script_dir`, `dockpipe get package_root`, `dockpipe get assets_dir`, `dockpipe get dockpipe_bin`
    - scopes: `dockpipe scope`, `dockpipe scope artifacts <path>`, `dockpipe scope source <path>`, `dockpipe scope workflow <name> <path>`, `dockpipe scope --package <name>`, `dockpipe scope resolver <name> auth-dir`
    - shell-only actions: `eval "$(dockpipe sdk)"` then `dockpipe_sdk init-script`, `dockpipe_sdk require dockpipe-bin`, `dockpipe_sdk require workflow-name`, `dockpipe_sdk source terraform-pipeline`, `dockpipe_sdk die`
  - PowerShell: `$dockpipe.Workdir`, `$dockpipe.DockpipeBin`, `$dockpipe.WorkflowName`, `$dockpipe.ScriptDir`, `$dockpipe.PackageRoot`, `$dockpipe.AssetsDir`, `Invoke-DockpipeScope`
  - Python: `dockpipe.workdir`, `dockpipe.dockpipe_bin`, `dockpipe.workflow_name`, `dockpipe.script_dir`, `dockpipe.package_root`, `dockpipe.assets_dir`, `dockpipe.scope(...)`
  - Go: `dockpipe.Workdir`, `dockpipe.DockpipeBin`, `dockpipe.WorkflowName`, `dockpipe.ScriptDir`, `dockpipe.PackageRoot`, `dockpipe.AssetsDir`, `dockpipe.WorkflowScope()`, `dockpipe.PackageScope(...)`

## Install (dev)

```bash
make package-dockpipe-language-support
```

This writes a VSIX to:
`bin/.dockpipe/extensions/dockpipe-language-support-<version>.vsix`

Install the generated `.vsix` from Cursor/VS Code:
`Extensions` -> `...` -> `Install from VSIX...`

Or install via CLI:

```bash
make install-dockpipe-language-support
```

## Notes

- YAML IntelliSense is context-aware and uses lightweight nesting analysis from the workflow document.
- Workflow authoring help tracks the current public model: steps + runtime + resolver first, with top-level runtime/resolver as defaults, step-level runtime/resolver as overrides, step-level `security` supported for container-only policy tightening, `isolate` treated as the advanced low-level override, top-level `run` / `act` treated as single-flow shorthand only, and async authoring expressed through explicit `group: { mode: async, tasks: [...] }`.
- When present, workflow `view:` stays a declarative launcher presentation layer over the typed model rather than replacing `vars:` / env mappings.
- `types:` suggestions support the interface entrypoint pattern, for example:
  `models/IR2InfraConfig`
- PipeLang editor support understands interface/object field types and generic list shapes such as `List<string>` and `List<IImageResource>`.
- Shared script support points authors at the canonical DockPipe SDK under `src/core/assets/scripts/lib/` and `dockpipe sdk`.
- Workflow scripts can use `dockpipe scope` / SDK scope helpers for checkout, workflow artifact, and package state paths. Runtime env such as `DOCKPIPE_SOURCE_ROOT`, `DOCKPIPE_STEP_CWD`, `DOCKPIPE_OUTPUT_ROOT`, and `DOCKPIPE_ARTIFACT_ROOT` remains available for low-level integrations.
- DorkPipe agent workflow path lists can use `scope:...` references; the orchestration planner resolves them through `dockpipe scope` before writing prompts and task JSON.
- `package.yml` may declare package-owned artwork via `icon:` and `artwork:` paths relative to the manifest.
- `package.yml` may also declare a package-owned OCI image reference via `image:`; DockPipe compiles that into the effective runtime/image artifact manifests.
- `package.yml` `script_contract.inject` declares the generic injected fields. In shell, the public
  way to read those values is `dockpipe get ...`; the backing runtime env vars are
  `DOCKPIPE_WORKDIR`, `DOCKPIPE_WORKFLOW_NAME`, `DOCKPIPE_SCRIPT_DIR`,
  `DOCKPIPE_PACKAGE_ROOT`, and `DOCKPIPE_ASSETS_DIR`. Workflow step cwd/scope support also injects
  `DOCKPIPE_SOURCE_ROOT`, `DOCKPIPE_ARTIFACT_ROOT`, `DOCKPIPE_OUTPUT_ROOT`, and `DOCKPIPE_STEP_CWD`.
