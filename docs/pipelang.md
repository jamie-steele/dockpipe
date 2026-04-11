# PipeLang (v0.0.0.1)

PipeLang is an optional typed authoring layer for DockPipe.

It does not replace YAML workflows. YAML remains first-class.

## Scope in v0.0.0.1

Supported:
- Primitive types: `string`, `int`, `bool`, `float`
- `Interface` declarations (structural contracts)
- `Class` declarations with optional defaults
- Split declarations across multiple `.pipe` files in the same module tree
- `Class : Interface` conformance checks
- Expression-bodied methods (`=>`) with static type-checking
- Deterministic artifact generation
- CLI-only method invocation

Not supported in this version:
- Side effects in methods
- Runtime/resolver execution through methods
- Hidden execution during compile

## Commands

Compile artifacts:

```bash
dockpipe pipelang compile --in workflows/mywf/model.pipe --entry DefaultDeployConfig --out .dockpipe/pipelang
```

Invoke method via CLI:

```bash
dockpipe pipelang invoke --in workflows/mywf/model.pipe --class DefaultDeployConfig --method FullImage --format text
```

Materialize all `.pipe` files under configured compile roots:

```bash
dockpipe pipelang materialize --workdir .
```

## Artifacts

Compile emits:
- `<Class>.workflow.yml`
- `<Class>.bindings.json`
- `<Class>.bindings.env`

`bindings.env` is intended for direct script consumption:

```bash
source .dockpipe/pipelang/DefaultDeployConfig.bindings.env
echo "$PIPELANG_IMAGE"
```

## Architecture boundary

PipeLang is an authoring/compiler feature.

DockPipe execution still uses compiled YAML and existing workflow execution paths.

`dockpipe run` does not parse PipeLang directly.
