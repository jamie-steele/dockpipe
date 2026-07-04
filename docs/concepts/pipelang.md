# PipeLang (v0.0.0.1)

PipeLang is an optional typed authoring layer for DockPipe.

It does not replace YAML workflows. YAML remains first-class.

Use PipeLang for:
- typed workflow/package configuration models
- defaults and documentation summaries close to the model
- launcher/editor metadata derived from those types

Do not use PipeLang as a replacement for workflow execution logic. DockPipe still runs normal workflow YAML.

## Scope in v0.0.0.1

Supported:
- Primitive types: `string`, `int`, `bool`, `float`
- `Interface` declarations (structural contracts)
- `Class` declarations with optional defaults
- Object/interface-typed fields inside other classes
- Generic list shapes such as `List<string>` and `List<IImageResource>`
- Split declarations across multiple `.pipe` files in the same module tree
- `Class : Interface` conformance checks
- Expression-bodied methods (`=>`) with static type-checking
- Deterministic artifact generation
- CLI-only method invocation

Not supported in this version:
- Side effects in methods
- Runtime/resolver execution through methods
- Hidden execution during compile
- General-purpose scripting/runtime behavior

Reserved/plumbed but not fully implemented yet:
- `IComparable` for custom object comparison. The parser/type checker understands the contract boundary, but custom object comparisons should still fail clearly until the runtime semantics land.

## Authoring model

PipeLang is the **data model** layer.

Typical responsibilities:
- define the root workflow config type
- define nested objects such as `General`, `Storage`, or `Network`
- define list-valued fields such as `List<string>`
- define defaults on the implementing class
- keep XML summaries and field docs close to the type

Typical non-responsibilities:
- page layout
- section ordering
- launcher tab/group structure
- select-vs-create UX

Those presentation concerns live in workflow YAML `view:` metadata, not in the PipeLang type system.

## Workflow binding

Workflow YAML binds to PipeLang through top-level `types:`.

Example:

```yaml
types:
  - ../../resolvers/qemu/models/QemuVmResolverConfig.pipe
```

This means:
- the referenced `.pipe` file is the **entrypoint**
- DockPipe reads the **module tree** rooted beside that file
- sibling `.pipe` files in that module may contribute additional interfaces/classes
- the selected entry type becomes the **root model** for tooling/catalog/launcher use

The workflow still executes through normal YAML/env resolution. PipeLang only shapes the authored model and the generated metadata.

## Environment mapping

Leaf fields bind to environment variables through:
- explicit annotations such as `[EnvName = "DOCKPIPE_VM_DISK"]`, or
- inferred names when the model/catalog layer can derive them

Example:

```pipelang
public Interface IWindowsVmStorage
{
    [EnvName = "DOCKPIPE_VM_DISK"]
    public string Disk;
}
```

This keeps the strong typed model separate from the runtime’s existing env contract.

## Workflow `view:` relationship

PipeLang works together with workflow YAML `view:`.

- PipeLang defines **what the data is**
- workflow YAML `view:` defines **how a launcher may present it**

Example shape:

```yaml
types:
  - ../../resolvers/qemu/models/QemuVmResolverConfig.pipe

view:
  entry:
    type: choice
    field: General.BootSource
    options:
      - value: image
        pages: [image]
      - value: installer-iso
        pages: [install]
  pages:
    - id: image
      title: Existing Image
      sections:
        - id: media
          title: Existing Image
          fields:
            - Storage.Disk
    - id: install
      title: Install Media
      sections:
        - id: media
          title: Images And Media
          fields:
            - Storage.Cdrom
```

The field paths in `view:` reference the PipeLang root model recursively. Entry routing still binds back to the same model field; it is not a separate runtime state machine.

## Commands

Compile artifacts:

```bash
dockpipe pipelang compile --in workflows/mywf/model.pipe --entry DefaultDeployConfig --out bin/.dockpipe/pipelang
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

Those artifacts are inspectable authoring outputs. They are not a second execution engine.

`bindings.env` is intended for direct script consumption:

```bash
source bin/.dockpipe/pipelang/DefaultDeployConfig.bindings.env
echo "$PIPELANG_IMAGE"
```

## Architecture boundary

PipeLang is an authoring/compiler feature.

DockPipe execution still uses compiled YAML and existing workflow execution paths.

`dockpipe run` does not parse PipeLang directly.

Think of the layering as:

- PipeLang: types, defaults, docs
- workflow YAML: execution plus optional authored `view:` metadata
- launcher/tools: render the model/view contract
- runner: consume the existing workflow/env contract
