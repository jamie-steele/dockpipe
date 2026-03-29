# DockPipe Language Support (VS Code)

Language support for DockPipe authoring:

- `.pipe` PipeLang syntax highlighting
- PipeLang snippets and keyword completion
- DockPipe `config.yml` IntelliSense for common workflow keys
- YAML parse diagnostics for DockPipe workflow files (`config.yml` / `config.yaml`)

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

- YAML IntelliSense is context-aware and parser-backed (`yaml` npm package).
- `types:` suggestions support the interface entrypoint pattern, for example:
  `models/IR2InfraConfig`
