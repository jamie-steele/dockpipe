# DockPipe Language Support (VS Code)

Language support for DockPipe authoring:

- `.pipe` PipeLang syntax highlighting
- PipeLang snippets and keyword completion
- DockPipe `config.yml` IntelliSense for common workflow keys
- DockPipe `package.yml` hover/docs and top-level key completion
- DockPipe `package.yml` support for `script_contract.inject` with valid generic injectable suggestions
- DockPipe `dockpipe.config.json` hover/docs and section-key completion
- First-party package script IntelliSense for the shared DockPipe SDK surface in shell, PowerShell, Python, and Go
- Structure-aware YAML semantic coloring for workflow keys, step keys, `vars:` fields, and `types:` entries
- YAML parse diagnostics for DockPipe workflow files (`config.yml` / `config.yaml`)
- Hover/docs for top-level workflow keys, step keys, `types:` entries, and `vars:` fields from PipeLang XML summaries (`types:` entrypoint)
- `vars:` value suggestions from implementing class defaults and nearby `Struct` known-values
- Completion/hover for SDK-object patterns:
  - shell:
    - getters: `dockpipe get workdir`, `dockpipe get workflow_name`, `dockpipe get script_dir`, `dockpipe get package_root`, `dockpipe get assets_dir`, `dockpipe get dockpipe_bin`
    - shell-only actions: `eval "$(dockpipe sdk)"` then `dockpipe_sdk init-script`, `dockpipe_sdk cd-workdir`, `dockpipe_sdk require dockpipe-bin`, `dockpipe_sdk require workflow-name`, `dockpipe_sdk source terraform-pipeline`, `dockpipe_sdk die`
  - PowerShell: `$dockpipe.Workdir`, `$dockpipe.DockpipeBin`, `$dockpipe.WorkflowName`, `$dockpipe.ScriptDir`, `$dockpipe.PackageRoot`, `$dockpipe.AssetsDir`
  - Python: `dockpipe.workdir`, `dockpipe.dockpipe_bin`, `dockpipe.workflow_name`, `dockpipe.script_dir`, `dockpipe.package_root`, `dockpipe.assets_dir`
  - Go: `dockpipe.Workdir`, `dockpipe.DockpipeBin`, `dockpipe.WorkflowName`, `dockpipe.ScriptDir`, `dockpipe.PackageRoot`, `dockpipe.AssetsDir`

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
- `types:` suggestions support the interface entrypoint pattern, for example:
  `models/IR2InfraConfig`
- Shared script support points authors at the canonical DockPipe SDK under `src/core/assets/scripts/lib/` and `dockpipe sdk`.
- `package.yml` `script_contract.inject` declares the generic injected fields. In shell, the public
  way to read those values is `dockpipe get ...`; the backing runtime env vars are
  `DOCKPIPE_WORKDIR`, `DOCKPIPE_WORKFLOW_NAME`, `DOCKPIPE_SCRIPT_DIR`,
  `DOCKPIPE_PACKAGE_ROOT`, and `DOCKPIPE_ASSETS_DIR`.
