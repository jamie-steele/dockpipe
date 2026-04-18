# DockPipe Language Support (VS Code)

Language support for DockPipe authoring:

- `.pipe` PipeLang syntax highlighting
- PipeLang snippets and keyword completion
- DockPipe `config.yml` IntelliSense for common workflow keys
- DockPipe `package.yml` hover/docs and top-level key completion
- DockPipe `dockpipe.config.json` hover/docs and section-key completion
- First-party package shell-script IntelliSense for `assets/scripts/lib/repo-tools.sh` helper sourcing and resolver function names
- Structure-aware YAML semantic coloring for workflow keys, step keys, `vars:` fields, and `types:` entries
- YAML parse diagnostics for DockPipe workflow files (`config.yml` / `config.yaml`)
- Hover/docs for top-level workflow keys, step keys, `types:` entries, and `vars:` fields from PipeLang XML summaries (`types:` entrypoint)
- `vars:` value suggestions from implementing class defaults and nearby `Struct` known-values
- Shell-script completion/hover for package-local helper functions such as `pipeon_resolve_dockpipe_bin`, `dorkpipe_resolve_dorkpipe_bin`, and related `repo-tools.sh` patterns

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
