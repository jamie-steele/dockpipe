# DockPipe Language Support (VS Code)

Language support for DockPipe authoring:

- `.pipe` PipeLang syntax highlighting
- PipeLang snippets and keyword completion
- DockPipe `config.yml` IntelliSense for common workflow keys
- DockPipe `package.yml` hover/docs and top-level key completion
- DockPipe `dockpipe.config.json` hover/docs and section-key completion
- First-party package script IntelliSense for the shared DockPipe SDK surface in shell, PowerShell, Python, and Go
- Structure-aware YAML semantic coloring for workflow keys, step keys, `vars:` fields, and `types:` entries
- YAML parse diagnostics for DockPipe workflow files (`config.yml` / `config.yaml`)
- Hover/docs for top-level workflow keys, step keys, `types:` entries, and `vars:` fields from PipeLang XML summaries (`types:` entrypoint)
- `vars:` value suggestions from implementing class defaults and nearby `Struct` known-values
- Completion/hover for SDK-object patterns:
  - shell: `eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"` then shell authors use `dockpipe_sdk ...`
    - actions: `dockpipe_sdk init-script`, `dockpipe_sdk workdir`, `dockpipe_sdk cd-workdir`, `dockpipe_sdk workflow-name`, `dockpipe_sdk require dockpipe-bin`, `dockpipe_sdk require workflow-name`, `dockpipe_sdk source terraform-pipeline`, `dockpipe_sdk die`
  - PowerShell: `$dockpipe.Workdir`, `$dockpipe.DockpipeBin`, `$dockpipe.WorkflowName`
  - Python: `dockpipe.workdir`, `dockpipe.dockpipe_bin`, `dockpipe.workflow_name`
  - Go: `dockpipe.Workdir`, `dockpipe.DockpipeBin`, `dockpipe.WorkflowName`

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
