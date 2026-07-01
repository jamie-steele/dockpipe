// @ts-check
const vscode = require("vscode");
const fs = require("fs/promises");
const path = require("path");

const DOCKPIPE_TOP_LEVEL_KEYS = [
  "name",
  "description",
  "category",
  "namespace",
  "types",
  "view",
  "docker_preflight",
  "inputs",
  "model_policy",
  "vars",
  "compose",
  "container",
  "security",
  "image",
  "steps",
  "finally",
  "resolver",
  "runtime",
  "run",
  "isolate",
  "act",
  "action",
  "strategy",
  "strategies",
  "vault",
  "compile_hooks",
  "imports",
  "inject"
];

const DOCKPIPE_STEP_KEYS = [
  "id",
  "kind",
  "cwd",
  "scopes",
  "cmd",
  "command",
  "run",
  "pre_script",
  "inputs",
  "vars",
  "vm",
  "container",
  "agent",
  "security",
  "image",
  "outputs",
  "workflow",
  "package",
  "runtime",
  "resolver",
  "act",
  "action",
  "capture_stdout",
  "manifest",
  "host_builtin",
  "is_blocking"
];

const TOP_LEVEL_KEY_DETAILS = {
  name: "Workflow name.",
  description: "Human-readable workflow description.",
  category: "Optional UI hint for control surfaces such as Pipeon or Launcher.",
  namespace: "Package namespace used for compiled workflow material.",
  types: "PipeLang entrypoint list used to drive model-backed vars help.",
  view: "Optional authored launcher form layout. Define pages, sections, and field paths while older workflows can keep using the default flat/tree rendering.",
  docker_preflight: "Enable or disable the Docker preflight check before running.",
  inputs: "Typed workflow inputs backed by the workflow's PipeLang types. Use field paths such as General.ExecMode instead of raw DOCKPIPE_* env names.",
  model_policy: "Optional AI model attempt, validation, and escalation policy for DorkPipe harnesses.",
  vars: "Workflow variables exported before execution unless already set in the environment.",
  compose: "Optional Docker Compose settings for host built-ins such as compose_up, compose_down, and compose_ps.",
  container: "Optional container mount defaults. Use this to override the primary /work bind for the workflow and declare additional host-to-container mounts.",
  security: "Optional authored container security policy. Choose a security profile, then apply bounded network/filesystem/process overrides.",
  image: "Optional image customization. Use image.packages.apt to materialize a derived Docker image with workflow-authored Debian packages.",
  steps: "Ordered workflow steps. Each step can run in a container or on the host.",
  finally: "Cleanup steps that always run after steps, including failure or interrupt paths. Use this for teardown such as stopping helper stacks.",
  resolver: "Tooling profile for this workflow. Acts as the default resolver for steps unless they override it.",
  runtime: "Execution substrate for this workflow. Acts as the default runtime for steps unless they override it.",
  run: "Host pre-scripts for a compact single-flow workflow. Do not combine with steps:.",
  isolate: "Advanced low-level image/template override. Prefer runtime + resolver first.",
  act: "Action script after the main command for compact single-flow workflows. Do not combine with steps:.",
  action: "Alias for act.",
  strategy: "Lifecycle wrapper that runs around the workflow.",
  strategies: "Available strategies for selection or validation.",
  vault: "Vault provider used for secret injection.",
  compile_hooks: "Shell hooks run during package compile against the staged copy before the tarball is written. Use DOCKPIPE_COMPILE_* env vars when the hook needs source or staging paths.",
  imports: "Additional workflow YAML merged into this workflow during authoring.",
  inject: "Compile-closure dependencies that should be included without merging their YAML into this workflow."
};

const STEP_KEY_DETAILS = {
  id: "Stable step label used in logs and outputs.",
  kind: "Step kind. Use host for host actions and container for normal isolated execution.",
  cwd: "Working directory for this step. Use repo/source to run from the checkout, or artifacts to run from generated workflow state while keeping source mounted and available.",
  scopes: "Optional path-scope bindings. Use source: repo with artifacts: artifacts when a step runs from the checkout but generated outputs should stay in workflow artifact state.",
  cmd: "Shell command line for this step. Runs inside the container by default, or on the host for kind: host steps.",
  command: "Alias for cmd.",
  run: "Command or script list to execute for this step.",
  pre_script: "Host-side scripts that run before the step body.",
  inputs: "Typed step-local inputs backed by the workflow's PipeLang types. Use field paths such as Advanced.KeepAlive and optional from/value bindings instead of raw env names.",
  vars: "Step-local variables merged for this step.",
  vm: "Declarative VM step settings. Use this to set guest path sync, interactive desktop or SSH session behavior, keepalive, and host port forwarding without hand-writing DOCKPIPE_VM_* variables or host prep steps.",
  container: "Optional step-level container mount override. Use this on container steps to change the primary /work source or add extra host mounts for one step.",
  agent: "Declarative agentic step settings. Put startup prompt, access.read/write/deny policy, model knobs, seeded worker profiles and worker policy, DorkPipe orchestration fanout/concurrency/apply data, and task-level model_policy here so shared scripts can drive agent behavior directly from workflow YAML.",
  security: "Optional step-level container security override. Use this to tighten or specialize policy for one container step.",
  image: "Optional step-level image customization. Use image.packages.apt for extra Debian packages needed only by this step.",
  outputs: "Dotenv-style outputs file merged into the environment for later steps. This is the normal way one step passes values forward.",
  workflow: "Marks this as a packaged workflow step and names the child workflow to run.",
  package: "Namespace of the child packaged workflow for a packaged workflow step.",
  runtime: "Override the workflow default runtime for this step. Not meaningful on kind: host steps.",
  resolver: "Override the workflow default resolver for this step. Not meaningful on kind: host steps.",
  act: "Action script for this step. Do not combine with packaged workflow steps.",
  action: "Alias for act.",
  capture_stdout: "Host path that also receives this step's stdout.",
  manifest: "Host path for a JSON manifest describing this step result.",
  host_builtin: "Built-in host action for host steps such as package_build_store, compose_up, compose_down, or compose_ps.",
  is_blocking: "Blocking join control for explicit async groups. For new workflow authoring, use group: { mode: async, tasks: [...] } instead of plain-step is_blocking: false."
};

const STEP_CWD_VALUE_DETAILS = {
  artifacts: "Run the step from this workflow's generated artifact root. Relative output paths stay out of source control.",
  repo: "Run the step from the source checkout/repo root. Alias for source.",
  source: "Run the step from the source checkout/repo root. This is the default.",
  workdir: "Run the step from the source checkout/workdir. Compatibility alias for source."
};

const STEP_SCOPE_KEY_DETAILS = {
  source: "Source-facing scope. Use repo/source for the checkout, or artifacts for generated workflow state.",
  artifacts: "Artifact/output-facing scope. Use artifacts to bind generated output to workflow state."
};

const DORKPIPE_AGENT_SCOPE_REF_DETAILS = {
  "scope:artifacts": {
    insertText: "scope:artifacts:${1:path/to/artifact}",
    detail: "Current workflow artifact path",
    documentation: "DorkPipe agent path reference resolved by orchestration before prompts/task JSON are written."
  },
  "scope:workflow": {
    insertText: "scope:workflow:${1:workflow.name}:${2:path/to/artifact}",
    detail: "Another workflow's artifact path",
    documentation: "DorkPipe agent path reference resolved through `dockpipe scope workflow <name> <path>`."
  },
  "scope:package": {
    insertText: "scope:package:${1:package-name}:${2:path/to/state}",
    detail: "Package state path",
    documentation: "DorkPipe agent path reference resolved through `dockpipe scope --package <name> <path>`."
  }
};

const DOCKPIPE_RUNTIME_ENV_DETAILS = {
  DOCKPIPE_SOURCE_ROOT: "Absolute source checkout/repo root. Use this when a script running elsewhere needs to read project files.",
  DOCKPIPE_ARTIFACT_ROOT: "Absolute generated artifact root for the current workflow.",
  DOCKPIPE_OUTPUT_ROOT: "Absolute output root selected by scopes.artifacts. Relative outputs files are resolved here.",
  DOCKPIPE_STEP_CWD: "Absolute process working directory selected by the step's cwd setting."
};

const PACKAGE_MANIFEST_KEYS = [
  "schema",
  "kind",
  "name",
  "version",
  "title",
  "description",
  "author",
  "website",
  "license",
  "icon",
  "artwork",
  "provider",
  "capability",
  "primitive",
  "namespace",
  "tags",
  "keywords",
  "min_dockpipe_version",
  "repository",
  "provides",
  "requires_capabilities",
  "requires_primitives",
  "requires_resolvers",
  "includes_resolvers",
  "depends",
  "allow_clone",
  "distribution",
  "image",
  "script_contract"
];

const PACKAGE_MANIFEST_KEY_DETAILS = {
  schema: "Manifest schema version.",
  kind: "Package kind hint such as package, workflow, resolver, core, assets, or bundle.",
  name: "Stable package name used in manifests, compile output, and dependencies.",
  version: "Package version string.",
  title: "Human-friendly package title.",
  description: "Long-form package summary shown in listings and docs.",
  author: "Package author or maintainer label.",
  website: "Optional project or docs URL.",
  license: "Package license identifier.",
  icon: "Optional package-owned icon path relative to package.yml.",
  artwork: "Optional named artwork variants such as icon, vscode, or cursor-dev, relative to package.yml.",
  provider: "Optional short vendor or platform id such as cloudflare or github.",
  capability: "Optional dotted capability id provided by this package, such as cli.codex.",
  primitive: "Deprecated alias for capability.",
  namespace: "Optional author or org namespace used for compiled artifacts and lookup preference.",
  tags: "Search and filtering tags.",
  keywords: "Additional search keywords.",
  min_dockpipe_version: "Optional minimum DockPipe version constraint.",
  repository: "Source repository URL.",
  provides: "Additional named capabilities or features exposed by this package.",
  requires_capabilities: "Capabilities a workflow package expects from its chosen resolver.",
  requires_primitives: "Deprecated alias for requires_capabilities.",
  requires_resolvers: "Resolver profile names suggested or required by a workflow package.",
  includes_resolvers: "Resolver names included under a kind: package umbrella tree.",
  depends: "Other package names this package expects in the compiled store.",
  allow_clone: "When true, dockpipe clone may copy this compiled package back into an authoring tree.",
  distribution: "Human/tooling hint such as source or binary.",
  image: "Optional package-owned runtime image reference. Use a normal OCI/registry ref; DockPipe compiles it into the effective image artifact manifest.",
  script_contract: "Generic package-level script context contract for assets/scripts consumers."
};

const PACKAGE_SCRIPT_CONTRACT_KEYS = ["inject"];

const PACKAGE_SCRIPT_CONTRACT_KEY_DETAILS = {
  inject: "Generic injected package script context fields such as workdir, workflow_name, script_dir, package_root, assets_dir, and dockpipe_bin."
};

const PACKAGE_SCRIPT_CONTRACT_INJECTABLES = {
  workdir: "Effective DockPipe workdir/repo root.",
  workflow_name: "Workflow name when the script is running inside a DockPipe workflow.",
  script_dir: "Directory containing the current package script.",
  package_root: "Nearest package root containing package.yml for the current script.",
  assets_dir: "Nearest assets/ directory for the current script.",
  dockpipe_bin: "Resolved dockpipe CLI path."
};

const DOCKPIPE_PROJECT_TOP_LEVEL_KEYS = ["schema", "compile", "packages", "secrets"];

const DOCKPIPE_PROJECT_TOP_LEVEL_KEY_DETAILS = {
  schema: "Project config schema version.",
  compile: "Compile roots and related project-level source discovery settings.",
  packages: "Defaults for package namespace and tarball lookup behavior.",
  secrets: "Project-level secret template and vault defaults."
};

const DOCKPIPE_PROJECT_SECTION_KEY_DETAILS = {
  compile: {
    core_from: "Optional override for the core slice source passed to compile core.",
    workflows: "Repo-relative or absolute roots scanned for workflows and resolver trees.",
    resolvers: "Deprecated extra resolver roots; merged into effective workflow roots when present.",
    bundles: "Deprecated extra bundle roots; merged into compile.workflows."
  },
  secrets: {
    vault_template: "Preferred env template file containing secret references such as op:// entries.",
    op_inject_template: "Legacy alias for vault_template.",
    vault: "Default vault mode used when workflow YAML omits vault.",
    notes: "Optional maintainer-facing notes shown by tooling such as dockpipe doctor."
  },
  packages: {
    tarball_dir: "Directory containing built dockpipe-workflow-*.tar.gz archives for local resolution.",
    namespace: "Default package namespace when manifests or workflows omit one.",
    registry_urls: "Optional future package registry base URLs."
  }
};

const VAR_KEY_FALLBACK_DETAIL = "Workflow variable. This key exports an environment variable for the workflow or step.";
const INPUT_BINDING_KEY_DETAILS = {
  from: "Read this typed input from another env var after DockPipe has merged workflow vars, env files, and vault injection.",
  value: "Literal fallback value for this typed input when from: is unset or resolves empty."
};

const CONTAINER_KEYS = new Set([
  "types",
  "view",
  "inputs",
  "vars",
  "vm",
  "steps",
  "finally",
  "run",
  "compose",
  "security",
  "image",
  "outputs",
  "imports",
  "inject",
  "strategies"
]);

const SEMANTIC_LEGEND = new vscode.SemanticTokensLegend([
  "keyword",
  "property",
  "variable",
  "type",
  "enumMember",
  "string",
  "number"
]);

const WORKFLOW_VIEW_KEY_DETAILS = {
  entry: "Optional authored entry routing choice. Bind a high-level selection to a model field before showing the rest of the workflow settings.",
  pages: "Ordered launcher pages for this workflow's authored settings experience."
};

const WORKFLOW_VIEW_ENTRY_KEY_DETAILS = {
  type: "Entry interaction type. Start with choice for a simple preflight selection screen.",
  field: "Typed model field path that receives the chosen value, such as General.BootSource.",
  title: "User-facing title for the entry choice screen.",
  description: "Optional help text shown above the entry selector.",
  options: "Ordered entry routing options."
};

const WORKFLOW_VIEW_ENTRY_OPTION_KEY_DETAILS = {
  value: "Value written back into the bound model field when this option is chosen.",
  label: "User-facing option label in the launcher.",
  next: "Optional primary page id to route to after this option is chosen.",
  pages: "Optional ordered page ids to show after this option is chosen."
};

const WORKFLOW_VIEW_PAGE_KEY_DETAILS = {
  id: "Stable internal page id.",
  title: "User-facing page title in the launcher.",
  description: "Optional page help text shown above the page content.",
  sections: "Ordered sections rendered inside this page."
};

const WORKFLOW_VIEW_SECTION_KEY_DETAILS = {
  id: "Stable internal section id.",
  title: "User-facing section title in the launcher.",
  description: "Optional section help text shown above the section controls.",
  fields: "Ordered field paths from the typed model, such as General.BootSource or Storage.Disk."
};

const STEP_VM_KEY_DETAILS = {
  mounts: "Optional list of host-to-guest mappings applied before the guest command runs. Use this when you want more than one synced path, or when you want the mapping to be explicit instead of relying on guest_path sugar.",
  host_context: "Optional host path to sync into the guest. When guest_path is set and host_context is omitted, DockPipe uses the effective DOCKPIPE_WORKDIR/workdir by default.",
  guest_path: "Guest destination path for host-to-guest sync before the VM guest command runs.",
  interactive_debug: "Launch the VM with a visible window and skip guest-command automation so you can inspect or configure the guest manually.",
  interactive_ssh: "Boot the VM, wait for SSH readiness, then open an authenticated interactive guest shell instead of running a one-shot guest command.",
  keepalive: "Keep the VM alive after the guest command exits so you can continue setup work manually.",
  keepalive_seconds: "Maximum keepalive window in seconds before DockPipe tears the VM down.",
  hostfwd: "Additional QEMU host forward string such as tcp::3389-:3389."
};

const STEP_VM_MOUNT_KEY_DETAILS = {
  host: "Host path to stage or mount into the guest before the guest command runs.",
  guest: "Guest destination path for this host mapping."
};

const CONTAINER_KEY_DETAILS = {
  workdir_host: "Optional host path to bind at /work for container execution. Relative paths resolve from the active workflow source/workdir, not the packaged workflow asset directory.",
  work_path: "Optional working subdirectory under /work. Keep this relative; DockPipe rejects absolute container paths here.",
  mounts: "Optional additional host-to-container bind mounts applied after the primary /work mount."
};

const CONTAINER_MOUNT_KEY_DETAILS = {
  host: "Host path for this additional bind mount. Relative paths resolve from the active workflow source/workdir.",
  guest: "Container destination path for this additional bind mount.",
  mode: "Optional bind mode. Use ro for read-only or rw for read-write."
};

const CORE_HELPER_PROFILES = {
  shellscript: {
    helperPath: "dockpipe sdk",
    sourceSnippet:
      'dockpipe scope source',
    sourceLabel: "dockpipe scope source",
    sourceDetail: "Resolve DockPipe source/artifact paths through the CLI",
    functions: [
      {
        name: "dockpipe scope source",
        detail: "Resolve the active source checkout path.",
        insertText: 'dockpipe scope source "$1"',
        filterText: "dockpipe scope source repo checkout root path",
        documentation: "First-hand CLI command that resolves a path under the active source root. Prefer `pwd` when workflow `cwd` already points at the source checkout."
      },
      {
        name: "dockpipe get workflow_name",
        detail: "Print the SDK workflow name.",
        insertText: "dockpipe get workflow_name",
        filterText: "dockpipe get workflow_name dockpipe workflow name",
        documentation: "First-hand shell CLI getter that prints `DOCKPIPE_WORKFLOW_NAME` when the script is running inside a DockPipe workflow."
      },
      {
        name: "dockpipe get script_dir",
        detail: "Print the current script directory.",
        insertText: "dockpipe get script_dir",
        filterText: "dockpipe get script_dir dockpipe script dir",
        documentation: "First-hand shell CLI getter that prints the current package script directory."
      },
      {
        name: "dockpipe get package_root",
        detail: "Print the nearest package root.",
        insertText: "dockpipe get package_root",
        filterText: "dockpipe get package_root dockpipe package root",
        documentation: "First-hand shell CLI getter that prints the nearest package root containing `package.yml` for the current script."
      },
      {
        name: "dockpipe get assets_dir",
        detail: "Print the nearest assets directory.",
        insertText: "dockpipe get assets_dir",
        filterText: "dockpipe get assets_dir dockpipe assets dir",
        documentation: "First-hand shell CLI getter that prints the nearest `assets/` directory for the current script."
      },
      {
        name: "dockpipe get dockpipe_bin",
        detail: "Print the resolved dockpipe binary path.",
        insertText: "dockpipe get dockpipe_bin",
        filterText: "dockpipe get dockpipe_bin dockpipe bin",
        documentation: "First-hand shell CLI getter that prints the resolved `dockpipe` binary path."
      },
      {
        name: "dockpipe scope",
        detail: "Print the current workflow scope object.",
        insertText: "dockpipe scope",
        filterText: "dockpipe scope workflow artifact output source json",
        documentation: "First-hand CLI command that prints the current workflow scope object as JSON."
      },
      {
        name: "dockpipe scope --package",
        detail: "Print a package scope object.",
        insertText: 'dockpipe scope --package "$1"',
        filterText: "dockpipe scope package json package state",
        documentation: "First-hand CLI command that prints a package scope object as JSON, or resolves a package path when suffix segments are provided."
      },
      {
        name: "dockpipe scope artifacts",
        detail: "Resolve a path under the active artifact root.",
        insertText: 'dockpipe scope artifacts "$1"',
        filterText: "dockpipe scope artifacts output generated path",
        documentation: "First-hand CLI command that resolves a path under the current workflow artifact root."
      },
      {
        name: "dockpipe scope workflow",
        detail: "Resolve another workflow's artifact path.",
        insertText: 'dockpipe scope workflow "$1" "$2"',
        filterText: "dockpipe scope workflow artifact path",
        documentation: "First-hand CLI command that resolves a path under a named workflow's artifact root."
      },
      {
        name: "dockpipe scope resolver",
        detail: "Resolve resolver-owned scope fields.",
        insertText: 'dockpipe scope resolver "$1" auth-dir',
        filterText: "dockpipe scope resolver auth dir container config",
        documentation: "First-hand CLI command that resolves resolver profile fields such as `auth-dir`, `container-auth-dir`, `auth-mount-mode`, `config-file`, and `container-config-file`."
      },
      {
        name: "dockpipe_sdk init-script",
        detail: "Initialize common script vars and enter the workdir.",
        insertText: "dockpipe_sdk init-script",
        filterText: "dockpipe_sdk init-script dockpipe init script root wf_ns workdir",
        documentation: "First-hand shell SDK action that initializes `ROOT` and `WF_NS` from the SDK context and changes into the DockPipe workdir."
      },
      {
        name: "dockpipe_sdk require workflow-name",
        detail: "Return workflow name or fail.",
        insertText: "dockpipe_sdk require workflow-name",
        filterText: "dockpipe_sdk require workflow-name dockpipe require workflow name",
        documentation: "First-hand shell SDK action that prints the workflow name and fails with a clear error if `DOCKPIPE_WORKFLOW_NAME` is unavailable."
      },
      {
        name: "dockpipe_sdk require dockpipe-bin",
        detail: "Return dockpipe binary or fail.",
        insertText: "dockpipe_sdk require dockpipe-bin",
        filterText: "dockpipe_sdk require dockpipe-bin dockpipe require bin",
        documentation: "First-hand shell SDK action that prints the resolved `dockpipe` binary path and fails with a clear error if it cannot be resolved."
      },
      {
        name: "dockpipe_sdk die",
        detail: "Exit with an SDK-prefixed error.",
        insertText: 'dockpipe_sdk die "$1"',
        filterText: "dockpipe_sdk die fail error exit",
        documentation: "First-hand shell SDK action that prints a workflow-prefixed error and exits nonzero."
      },
      {
        name: "dockpipe_sdk source terraform-pipeline",
        detail: "Source the canonical Terraform pipeline library.",
        insertText: "dockpipe_sdk source terraform-pipeline",
        filterText: "dockpipe_sdk source terraform-pipeline dockpipe terraform pipeline source",
        documentation: "First-hand shell SDK action that resolves and sources the canonical Terraform pipeline library into the current shell."
      },
      {
        name: "dockpipe_sdk refresh",
        detail: "Refresh the shell SDK object.",
        insertText: 'dockpipe_sdk refresh "$1"',
        documentation: "Recompute the shell SDK object when `DOCKPIPE_WORKDIR` or binary overrides change."
      }
    ]
  },
  powershell: {
    helperPath: "src/core/assets/scripts/lib/repo-tools.ps1",
    sourceSnippet:
      '. (Join-Path (if ($env:DOCKPIPE_WORKDIR) { $env:DOCKPIPE_WORKDIR } else { (Get-Location).Path }) "src/core/assets/scripts/lib/repo-tools.ps1")',
    sourceLabel: "dockpipe sdk import",
    sourceDetail: "Dot-source the canonical DockPipe PowerShell SDK",
    functions: [
      {
        name: "$dockpipe.Workdir",
        detail: "SDK workdir/root.",
        insertText: "$dockpipe.Workdir",
        documentation: "Object-style PowerShell SDK field for the effective DockPipe workdir/repo root."
      },
      {
        name: "$dockpipe.DockpipeBin",
        detail: "SDK dockpipe binary path.",
        insertText: "$dockpipe.DockpipeBin",
        documentation: "Object-style PowerShell SDK field for the resolved `dockpipe` binary path."
      },
      {
        name: "$dockpipe.WorkflowName",
        detail: "SDK workflow name.",
        insertText: "$dockpipe.WorkflowName",
        documentation: "Object-style PowerShell SDK field for `DOCKPIPE_WORKFLOW_NAME` when running inside a DockPipe workflow."
      },
      {
        name: "$dockpipe.ScriptDir",
        detail: "SDK script directory.",
        insertText: "$dockpipe.ScriptDir",
        documentation: "Object-style PowerShell SDK field for the current package script directory."
      },
      {
        name: "$dockpipe.PackageRoot",
        detail: "SDK package root.",
        insertText: "$dockpipe.PackageRoot",
        documentation: "Object-style PowerShell SDK field for the nearest package root containing `package.yml`."
      },
      {
        name: "$dockpipe.AssetsDir",
        detail: "SDK assets directory.",
        insertText: "$dockpipe.AssetsDir",
        documentation: "Object-style PowerShell SDK field for the nearest `assets/` directory for the current script."
      },
      {
        name: "Invoke-DockpipeScope",
        detail: "Resolve workflow/package scope objects or paths.",
        insertText: "Invoke-DockpipeScope",
        documentation: "PowerShell SDK function for `dockpipe scope`. No scope returns the current workflow object; `-Package <name>` returns a package object; path args resolve paths."
      }
    ]
  },
  python: {
    helperPath: "src.core.assets.scripts.lib.repo_tools",
    sourceSnippet:
      "from src.core.assets.scripts.lib.repo_tools import dockpipe",
    sourceLabel: "dockpipe sdk import",
    sourceDetail: "Import the canonical DockPipe Python SDK object",
    functions: [
      {
        name: "dockpipe.workdir",
        detail: "SDK workdir/root.",
        insertText: "dockpipe.workdir",
        documentation: "Object-style Python SDK field for the effective DockPipe workdir/repo root."
      },
      {
        name: "dockpipe.dockpipe_bin",
        detail: "SDK dockpipe binary path.",
        insertText: "dockpipe.dockpipe_bin",
        documentation: "Object-style Python SDK field for the resolved `dockpipe` binary path."
      },
      {
        name: "dockpipe.workflow_name",
        detail: "SDK workflow name.",
        insertText: "dockpipe.workflow_name",
        documentation: "Object-style Python SDK field for `DOCKPIPE_WORKFLOW_NAME` when running inside a DockPipe workflow."
      },
      {
        name: "dockpipe.script_dir",
        detail: "SDK script directory.",
        insertText: "dockpipe.script_dir",
        documentation: "Object-style Python SDK field for the current package script directory."
      },
      {
        name: "dockpipe.package_root",
        detail: "SDK package root.",
        insertText: "dockpipe.package_root",
        documentation: "Object-style Python SDK field for the nearest package root containing `package.yml`."
      },
      {
        name: "dockpipe.assets_dir",
        detail: "SDK assets directory.",
        insertText: "dockpipe.assets_dir",
        documentation: "Object-style Python SDK field for the nearest `assets/` directory for the current script."
      },
      {
        name: "dockpipe.scope",
        detail: "Resolve workflow/package scope objects or paths.",
        insertText: "dockpipe.scope($1)",
        documentation: "Python SDK method for `dockpipe scope`. No args returns the current workflow object; `package=\"name\"` returns a package object; args resolve paths."
      }
    ]
  },
  go: {
    helperPath: "dockpipe/src/core/assets/scripts/lib/repotools",
    sourceSnippet: 'import repotools "dockpipe/src/core/assets/scripts/lib/repotools"\n\ndockpipe, err := repotools.Load("")',
    sourceLabel: "dockpipe sdk import",
    sourceDetail: "Import the canonical DockPipe Go SDK and load the SDK object",
    functions: [
      {
        name: "dockpipe.Workdir",
        detail: "SDK workdir/root.",
        insertText: "dockpipe.Workdir",
        documentation: "Object-style Go SDK field for the effective DockPipe workdir/repo root."
      },
      {
        name: "dockpipe.DockpipeBin",
        detail: "SDK dockpipe binary path.",
        insertText: "dockpipe.DockpipeBin",
        documentation: "Object-style Go SDK field for the resolved `dockpipe` binary path."
      },
      {
        name: "dockpipe.WorkflowName",
        detail: "SDK workflow name.",
        insertText: "dockpipe.WorkflowName",
        documentation: "Object-style Go SDK field for `DOCKPIPE_WORKFLOW_NAME` when running inside a DockPipe workflow."
      },
      {
        name: "dockpipe.ScriptDir",
        detail: "SDK script directory.",
        insertText: "dockpipe.ScriptDir",
        documentation: "Object-style Go SDK field for the current package script directory."
      },
      {
        name: "dockpipe.PackageRoot",
        detail: "SDK package root.",
        insertText: "dockpipe.PackageRoot",
        documentation: "Object-style Go SDK field for the nearest package root containing `package.yml`."
      },
      {
        name: "dockpipe.AssetsDir",
        detail: "SDK assets directory.",
        insertText: "dockpipe.AssetsDir",
        documentation: "Object-style Go SDK field for the nearest `assets/` directory for the current script."
      },
      {
        name: "dockpipe.WorkflowScope",
        detail: "Load current workflow scope object.",
        insertText: "dockpipe.WorkflowScope()",
        documentation: "Go SDK method for `dockpipe scope` with no positional scope."
      },
      {
        name: "dockpipe.PackageScope",
        detail: "Load package scope object.",
        insertText: 'dockpipe.PackageScope("$1")',
        documentation: "Go SDK method for `dockpipe scope --package <name>`."
      }
    ]
  }
};

/**
 * @typedef {{ doc?: string, type?: string, defaultValue?: string, annotations?: Record<string,string> }} FieldInfo
 * @typedef {{ name: string, kind: "Interface"|"Class"|"Struct", implements?: string, doc?: string, annotations?: Record<string,string>, fields: Record<string, FieldInfo> }} TypeInfo
 * @typedef {{ types: Record<string, TypeInfo>, entryInterface?: string, entryClass?: string, knownValues: Array<{name:string,value:string,doc?:string}> }} ModelContext
 */

/** @param {vscode.TextDocument} doc */
function isDockpipeWorkflowFile(doc) {
  const name = doc.fileName.toLowerCase();
  if (!(name.endsWith("/config.yml") || name.endsWith("\\config.yml") || name.endsWith("/config.yaml") || name.endsWith("\\config.yaml"))) {
    return false;
  }
  const text = doc.getText();
  return text.includes("steps:") || text.includes("vars:") || text.includes("name:");
}

/** @param {vscode.TextDocument} doc */
function isDockpipePackageManifestFile(doc) {
  const name = doc.fileName.toLowerCase();
  if (!(name.endsWith("/package.yml") || name.endsWith("\\package.yml") || name.endsWith("/package.yaml") || name.endsWith("\\package.yaml"))) {
    return false;
  }
  const text = doc.getText();
  return text.includes("schema:") && text.includes("name:");
}

/** @param {vscode.TextDocument} doc */
function isDockpipeProjectConfigFile(doc) {
  const name = doc.fileName.toLowerCase();
  return name.endsWith("/dockpipe.config.json") || name.endsWith("\\dockpipe.config.json");
}

/** @param {vscode.TextDocument} doc */
function isDockpipePackageScriptDocument(doc) {
  return /[\\\/]packages[\\\/].+[\\\/]assets[\\\/]scripts[\\\/]/i.test(doc.fileName);
}

/** @param {vscode.TextDocument} doc */
function coreHelperProfileForDocument(doc) {
  if (!isDockpipePackageScriptDocument(doc)) {
    return null;
  }
  return CORE_HELPER_PROFILES[doc.languageId] || null;
}

/**
 * @param {vscode.TextDocument} doc
 * @returns {Set<string>}
 */
function existingTopLevelKeys(doc) {
  const out = new Set();
  const lines = doc.getText().split(/\r?\n/);
  for (const line of lines) {
    const m = line.match(/^([A-Za-z_][A-Za-z0-9_-]*)\s*:/);
    if (m) out.add(m[1]);
  }
  return out;
}

function existingJsonObjectKeys(document, parents = []) {
  const out = new Set();
  for (const info of analyzeJsonStructure(document)) {
    if (info.key && JSON.stringify(info.parents) === JSON.stringify(parents)) {
      out.add(info.key);
    }
  }
  return out;
}

/**
 * @param {vscode.TextDocument} doc
 * @returns {vscode.Diagnostic[]}
 */
function validateYaml(doc) {
  const diagnostics = [];
  const lines = doc.getText().split(/\r?\n/);
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    // Basic guard: tabs in YAML are usually invalid and confusing for indentation.
    if (/\t/.test(line)) {
      const start = new vscode.Position(i, line.indexOf("\t"));
      const end = new vscode.Position(i, line.indexOf("\t") + 1);
      diagnostics.push(new vscode.Diagnostic(new vscode.Range(start, end), "Use spaces, not tabs, for YAML indentation.", vscode.DiagnosticSeverity.Warning));
    }
  }
  return diagnostics;
}

/**
 * @param {vscode.TextDocument} doc
 * @returns {string[]}
 */
function extractTypesEntries(doc) {
  const out = [];
  const lines = doc.getText().split(/\r?\n/);
  let inTypes = false;
  let typesIndent = -1;
  for (const line of lines) {
    if (!inTypes) {
      const m = line.match(/^(\s*)types:\s*$/);
      if (m) {
        inTypes = true;
        typesIndent = m[1].length;
      }
      continue;
    }
    if (!line.trim()) continue;
    const indent = (line.match(/^\s*/) || [""])[0].length;
    if (indent <= typesIndent) break;
    const m = line.match(/^\s*-\s*(.+?)\s*$/);
    if (m) out.push(m[1]);
  }
  return out;
}

async function resolveWorkflowTypeEntries(doc) {
  const explicit = extractTypesEntries(doc).map((spec) => ({ spec, baseDir: path.dirname(doc.fileName) }));
  if (explicit.length > 0) {
    return explicit;
  }
  const workflowDir = path.dirname(doc.fileName);
  const runtimeName = extractTopLevelScalarValue(doc, "runtime");
  const resolverName = extractTopLevelScalarValue(doc, "resolver");
  const inferred = [];
  if (runtimeName) {
    inferred.push(...await inferTypeEntriesForProfile(workflowDir, runtimeName));
  }
  if (resolverName) {
    inferred.push(...await inferTypeEntriesForProfile(workflowDir, resolverName));
  }
  return dedupeTypeEntries(inferred);
}

function extractTopLevelScalarValue(doc, key) {
  const lines = doc.getText().split(/\r?\n/);
  for (const line of lines) {
    const m = line.match(/^([ \t]*)([A-Za-z_][A-Za-z0-9_.-]*)\s*:\s*(.+?)\s*$/);
    if (!m || (m[1] || "").length !== 0 || m[2] !== key) {
      continue;
    }
    return yamlScalarValue(m[3]) || "";
  }
  return "";
}

async function inferTypeEntriesForProfile(startDir, leafName) {
  const roots = await workflowCompileRootsForDoc(startDir);
  const candidates = [];
  for (const root of roots) {
    candidates.push(...await findNamedProfileDirs(root, leafName));
  }
  const out = [];
  for (const dirPath of candidates) {
    for (const fileName of ["types.yml", "config.yml"]) {
      const src = await readIfExists(path.join(dirPath, fileName));
      if (!src) continue;
      for (const spec of extractTypesEntriesFromSource(src)) {
        out.push({ spec, baseDir: dirPath });
      }
      break;
    }
  }
  return out;
}

async function workflowCompileRootsForDoc(startDir) {
  const projectRoot = await findProjectRoot(startDir);
  const configSrc = await readIfExists(path.join(projectRoot, "dockpipe.config.json"));
  if (!configSrc) {
    return [path.join(projectRoot, "workflows")];
  }
  try {
    const parsed = JSON.parse(configSrc);
    const roots = Array.isArray(parsed?.compile?.workflows) ? parsed.compile.workflows : ["workflows"];
    return roots
      .map((root) => String(root || "").trim())
      .filter(Boolean)
      .map((root) => (path.isAbsolute(root) ? root : path.join(projectRoot, root)));
  } catch {
    return [path.join(projectRoot, "workflows")];
  }
}

async function findProjectRoot(startDir) {
  let current = path.resolve(startDir);
  while (true) {
    try {
      await fs.access(path.join(current, "dockpipe.config.json"));
      return current;
    } catch {}
    const parent = path.dirname(current);
    if (parent === current) {
      return path.resolve(startDir);
    }
    current = parent;
  }
}

async function findNamedProfileDirs(root, leafName) {
  /** @type {string[]} */
  const out = [];
  const visit = async (dirPath) => {
    let entries = [];
    try {
      entries = await fs.readdir(dirPath, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      const child = path.join(dirPath, entry.name);
      if (entry.name === leafName) {
        try {
          await fs.access(path.join(child, "profile"));
          out.push(child);
        } catch {}
      }
      await visit(child);
    }
  };
  await visit(root);
  return out;
}

function extractTypesEntriesFromSource(src) {
  const out = [];
  const lines = String(src || "").split(/\r?\n/);
  let inTypes = false;
  let typesIndent = -1;
  for (const line of lines) {
    if (!inTypes) {
      const m = line.match(/^(\s*)types:\s*$/);
      if (m) {
        inTypes = true;
        typesIndent = m[1].length;
      }
      continue;
    }
    if (!line.trim()) continue;
    const indent = (line.match(/^\s*/) || [""])[0].length;
    if (indent <= typesIndent) break;
    const m = line.match(/^\s*-\s*(.+?)\s*$/);
    if (m) out.push(m[1]);
  }
  return out;
}

function dedupeTypeEntries(entries) {
  const seen = new Set();
  const out = [];
  for (const entry of entries) {
    const spec = String(entry?.spec || "").trim();
    const baseDir = String(entry?.baseDir || "").trim();
    if (!spec || !baseDir) continue;
    const key = `${baseDir}\u0000${spec}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({ spec, baseDir });
  }
  return out;
}

function summaryFromComment(raw) {
  const s = raw.replace(/^\s*\/\/\/\s*/gm, "").trim();
  const m = s.match(/<summary>([\s\S]*?)<\/summary>/i);
  if (m && m[1]) {
    return m[1].replace(/\s+/g, " ").trim();
  }
  return s.replace(/\s+/g, " ").trim();
}

/**
 * Parse a PipeLang file with lightweight regex extraction for docs/fields/defaults.
 * @param {string} source
 * @returns {TypeInfo[]}
 */
function parsePipeModel(source) {
  /** @type {TypeInfo[]} */
  const out = [];
  /** @type {TypeInfo|null} */
  let current = null;
  let depth = 0;
  let currentBodyStarted = false;
  /** @type {string[]} */
  let summaryLines = [];
  let collectingSummary = false;
  /** @type {Record<string, string>} */
  let pendingAnnotations = {};

  const lines = source.split(/\r?\n/);
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith("///")) {
      summaryLines.push(trimmed);
      if (trimmed.includes("<summary>")) collectingSummary = true;
      if (trimmed.includes("</summary>")) collectingSummary = false;
      continue;
    }
    const annotationMatch = trimmed.match(/^\[\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*"((?:[^"\\]|\\.)*)"\s*\]$/);
    if (annotationMatch) {
      pendingAnnotations[annotationMatch[1]] = annotationMatch[2].replace(/\\"/g, "\"");
      continue;
    }
    const annotationBoolMatch = trimmed.match(/^\[\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(true|false)\s*\]$/i);
    if (annotationBoolMatch) {
      pendingAnnotations[annotationBoolMatch[1]] = annotationBoolMatch[2].toLowerCase();
      continue;
    }
    const annotationNumMatch = trimmed.match(/^\[\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([0-9]+(?:\.[0-9]+)?)\s*\]$/);
    if (annotationNumMatch) {
      pendingAnnotations[annotationNumMatch[1]] = annotationNumMatch[2];
      continue;
    }
    const pendingSummary = summaryLines.length > 0 && !collectingSummary ? summaryFromComment(summaryLines.join("\n")) : undefined;

    const typeMatch = trimmed.match(/^(?:public|private)?\s*(Interface|Class|Struct)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?::\s*([A-Za-z_][A-Za-z0-9_]*))?/);
    if (typeMatch) {
      current = {
        name: typeMatch[2],
        kind: /** @type {"Interface"|"Class"|"Struct"} */ (typeMatch[1]),
        implements: typeMatch[3],
        doc: pendingSummary,
        annotations: Object.keys(pendingAnnotations).length ? pendingAnnotations : undefined,
        fields: {}
      };
      currentBodyStarted = false;
      out.push(current);
      summaryLines = [];
      pendingAnnotations = {};
    } else if (current) {
      const fieldInfo = parsePipeFieldLine(trimmed);
      if (fieldInfo) {
        let def = fieldInfo.defaultValue?.trim();
        if (typeof def === "string" && def.startsWith("\"") && def.endsWith("\"")) {
          def = def.slice(1, -1);
        }
        current.fields[fieldInfo.name] = {
          doc: pendingSummary,
          type: fieldInfo.type,
          defaultValue: def,
          annotations: Object.keys(pendingAnnotations).length ? pendingAnnotations : undefined
        };
        summaryLines = [];
        pendingAnnotations = {};
      } else if (!trimmed) {
        summaryLines = [];
      }
    }
    for (const ch of line) {
      if (ch === "{") depth++;
      if (ch === "}") depth--;
    }
    if (current && depth > 0) {
      currentBodyStarted = true;
    }
    if (current && currentBodyStarted && depth <= 0) {
      current = null;
      currentBodyStarted = false;
    }
    if (trimmed && !trimmed.startsWith("///")) {
      pendingAnnotations = {};
    }
  }
  return out;
}

/**
 * @param {string} line
 * @returns {{ type: string, name: string, defaultValue?: string } | null}
 */
function parsePipeFieldLine(line) {
  if (!/^(?:public|private)\b/.test(line)) {
    return null;
  }
  const withoutVisibility = line.replace(/^(?:public|private)\s+/, "");
  let depth = 0;
  let splitAt = -1;
  for (let i = 0; i < withoutVisibility.length; i++) {
    const ch = withoutVisibility[i];
    if (ch === "<") {
      depth++;
      continue;
    }
    if (ch === ">") {
      if (depth > 0) depth--;
      continue;
    }
    if (/\s/.test(ch) && depth === 0) {
      splitAt = i;
      break;
    }
  }
  if (splitAt <= 0) {
    return null;
  }
  const type = withoutVisibility.slice(0, splitAt).trim();
  const rest = withoutVisibility.slice(splitAt).trim();
  const fieldMatch = rest.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*(?:=\s*("([^"\\]|\\.)*"|[^;]+))?\s*;$/);
  if (!fieldMatch) {
    return null;
  }
  return {
    type,
    name: fieldMatch[1],
    defaultValue: fieldMatch[2]
  };
}

async function readIfExists(filePath) {
  try {
    return await fs.readFile(filePath, "utf8");
  } catch {
    return "";
  }
}

/**
 * @param {vscode.TextDocument} doc
 * @returns {Promise<ModelContext>}
 */
async function buildModelContext(doc) {
  const typeEntries = await resolveWorkflowTypeEntries(doc);
  if (typeEntries.length === 0) {
    return { types: {}, knownValues: [] };
  }
  const wfDir = path.dirname(doc.fileName);
  const modelsDir = path.join(wfDir, "models");

  /** @type {ModelContext} */
  const ctx = { types: {}, knownValues: [] };
  const loadedModelFiles = new Set();

  async function loadPipeDir(dirPath) {
    const entries = await fs.readdir(dirPath).catch(() => []);
    for (const name of entries.filter((n) => n.endsWith(".pipe"))) {
      const abs = path.join(dirPath, name);
      if (loadedModelFiles.has(abs)) continue;
      loadedModelFiles.add(abs);
      const src = await readIfExists(abs);
      if (!src) continue;
      for (const t of parsePipeModel(src)) {
        ctx.types[t.name] = t;
      }
    }
  }

  for (const entry of typeEntries) {
    const spec = String(entry.spec || "").trim();
    if (!spec) continue;
    const baseDir = entry.baseDir || wfDir;
    let left = spec;
    let explicitRef = "";
    const i = spec.indexOf("<");
    const j = spec.lastIndexOf(">");
    if (i >= 0 && j > i) {
      left = spec.slice(0, i).trim();
      explicitRef = spec.slice(i + 1, j).trim();
    }
    let rel = left;
    if (!rel.endsWith(".pipe")) rel += ".pipe";
    const primaryFile = path.isAbsolute(rel) ? rel : path.join(baseDir, rel);
    const primarySrc = await readIfExists(primaryFile);
    if (!primarySrc) continue;
    await loadPipeDir(modelsDir);
    await loadPipeDir(path.dirname(primaryFile));

    const primaryTypes = parsePipeModel(primarySrc);
    const primaryName = primaryTypes[0]?.name;
    if (!primaryName) continue;
    ctx.entryInterface = primaryName;

    if (explicitRef) {
      ctx.entryClass = explicitRef;
    } else {
      const iface = ctx.types[primaryName];
      if (iface?.kind === "Interface") {
        const impl = Object.values(ctx.types).filter((t) => t.implements === primaryName);
        if (impl.length === 1) ctx.entryClass = impl[0].name;
      } else {
        ctx.entryClass = primaryName;
      }
    }
  }

  for (const t of Object.values(ctx.types)) {
    if (t.kind !== "Struct") continue;
    for (const [fieldName, f] of Object.entries(t.fields)) {
      if (typeof f.defaultValue === "string" && f.defaultValue.length > 0) {
        ctx.knownValues.push({ name: `${t.name}.${fieldName}`, value: f.defaultValue, doc: f.doc });
      }
    }
  }
  return ctx;
}

function wordRange(document, position) {
  return document.getWordRangeAtPosition(position, /[A-Za-z0-9_.-]+/u);
}

function quoted(value) {
  return `'${String(value).replace(/'/g, "''")}'`;
}

function leadingSpaces(text) {
  return (text.match(/^\s*/) || [""])[0].length;
}

function parseYamlLine(text) {
  const mapMatch = text.match(/^(\s*)([A-Za-z_][A-Za-z0-9_.-]*)\s*:\s*(.*)$/);
  if (mapMatch) {
    const indent = mapMatch[1].length;
    const key = mapMatch[2];
    const afterColon = mapMatch[3] || "";
    const keyStart = indent;
    const keyEnd = keyStart + key.length;
    const colonIndex = text.indexOf(":", keyEnd);
    return {
      indent,
      key,
      keyStart,
      keyEnd,
      isListKey: false,
      hasInlineValue: afterColon.trim().length > 0,
      valueStart: colonIndex >= 0 ? colonIndex + 1 : -1,
      valueText: afterColon
    };
  }

  const listKeyMatch = text.match(/^(\s*)-\s+([A-Za-z_][A-Za-z0-9_.-]*)\s*:\s*(.*)$/);
  if (listKeyMatch) {
    const indent = listKeyMatch[1].length;
    const key = listKeyMatch[2];
    const afterColon = listKeyMatch[3] || "";
    const keyStart = indent + 2;
    const keyEnd = keyStart + key.length;
    const colonIndex = text.indexOf(":", keyEnd);
    return {
      indent,
      key,
      keyStart,
      keyEnd,
      isListKey: true,
      hasInlineValue: afterColon.trim().length > 0,
      valueStart: colonIndex >= 0 ? colonIndex + 1 : -1,
      valueText: afterColon
    };
  }

  const scalarListMatch = text.match(/^(\s*)-\s*([^\s#][^#]*?)\s*$/);
  if (scalarListMatch) {
    const indent = scalarListMatch[1].length;
    const value = scalarListMatch[2];
    return {
      indent,
      scalarListValue: value,
      scalarStart: indent + 2,
      scalarEnd: indent + 2 + value.length
    };
  }

  return { indent: leadingSpaces(text) };
}

function analyzeYamlStructure(document) {
  const infos = [];
  const stack = [];

  for (let lineNo = 0; lineNo < document.lineCount; lineNo++) {
    const text = document.lineAt(lineNo).text;
    const parsed = parseYamlLine(text);
    const trimmed = text.trim();

    while (stack.length && parsed.indent <= stack[stack.length - 1].indent) {
      stack.pop();
    }

    const parents = stack.map((entry) => entry.key);
    /** @type {any} */
    const info = {
      line: lineNo,
      text,
      indent: parsed.indent,
      trimmed,
      parents,
      inVars: parents.includes("vars"),
      inInputs: parents.includes("inputs"),
      inSteps: parents.includes("steps") || parents.includes("finally"),
      inTypes: parents.includes("types")
    };

    if (parsed.key) {
      info.key = parsed.key;
      info.keyRange = new vscode.Range(lineNo, parsed.keyStart, lineNo, parsed.keyEnd);
      info.isListKey = parsed.isListKey;
      info.valueText = parsed.valueText;
      info.valueRange = parsed.valueStart >= 0
        ? new vscode.Range(lineNo, parsed.valueStart, lineNo, text.length)
        : null;

      if (parsed.indent === 0 && !parsed.isListKey) {
        info.kind = "topLevelKey";
      } else if (parents.includes("vars")) {
        info.kind = "varKey";
      } else if (parents.includes("steps") || parents.includes("finally")) {
        info.kind = "stepKey";
      } else {
        info.kind = "key";
      }

      const shouldPush = !parsed.hasInlineValue || CONTAINER_KEYS.has(parsed.key);
      if (shouldPush) {
        stack.push({ indent: parsed.indent, key: parsed.key });
      }
    } else if (parsed.scalarListValue) {
      info.scalarListValue = parsed.scalarListValue;
      info.scalarRange = new vscode.Range(lineNo, parsed.scalarStart, lineNo, parsed.scalarEnd);
      if (parents.includes("types")) {
        info.kind = "typeEntry";
      }
    }

    infos.push(info);
  }

  return infos;
}

function trimmedValueRange(info) {
  if (!info?.valueRange || typeof info.valueText !== "string") return null;
  const leading = info.valueText.match(/^\s*/)?.[0].length || 0;
  const trimmed = info.valueText.trim();
  if (!trimmed) return null;
  const start = info.valueRange.start.character + leading;
  return new vscode.Range(info.line, start, info.line, start + trimmed.length);
}

function packageManifestValueTokenType(key, value) {
  const raw = String(value || "").trim();
  if (!raw) return null;
  if (key === "schema" && /^\d+$/.test(raw)) {
    return "number";
  }
  return "string";
}

function packageManifestListTokenType(parentKey) {
  if (!parentKey) return null;
  if (parentKey === "tags" || parentKey === "keywords") {
    return "enumMember";
  }
  return "string";
}

function runtimeEnvCompletionItems(languageId) {
  return Object.entries(DOCKPIPE_RUNTIME_ENV_DETAILS).map(([name, doc]) => {
    const item = new vscode.CompletionItem(name, vscode.CompletionItemKind.Variable);
    item.detail = "DockPipe runtime path";
    item.documentation = doc;
    switch (languageId) {
      case "powershell":
        item.insertText = `$env:${name}`;
        break;
      case "python":
        item.insertText = `os.environ.get("${name}", "")`;
        item.filterText = `${name} os.environ`;
        break;
      case "go":
        item.insertText = `os.Getenv("${name}")`;
        item.filterText = `${name} os.Getenv`;
        break;
      default:
        item.insertText = `$${name}`;
        item.filterText = `${name} dockpipe runtime path source artifact cwd`;
        break;
    }
    return item;
  });
}

function parseJsonLine(text) {
  const keyMatch = text.match(/^(\s*)"([^"]+)"\s*:\s*(.*)$/);
  if (!keyMatch) {
    return { indent: leadingSpaces(text) };
  }
  const indent = keyMatch[1].length;
  const key = keyMatch[2];
  const keyStart = text.indexOf(`"${key}"`) + 1;
  const keyEnd = keyStart + key.length;
  const valueText = keyMatch[3] || "";
  const colonIndex = text.indexOf(":", keyEnd);
  return {
    indent,
    key,
    keyStart,
    keyEnd,
    valueStart: colonIndex >= 0 ? colonIndex + 1 : -1,
    valueText
  };
}

function analyzeJsonStructure(document) {
  const infos = [];
  const stack = [];

  for (let lineNo = 0; lineNo < document.lineCount; lineNo++) {
    const text = document.lineAt(lineNo).text;
    const parsed = parseJsonLine(text);
    const trimmed = text.trim();

    while (stack.length && parsed.indent <= stack[stack.length - 1].indent) {
      stack.pop();
    }

    const parents = stack.map((entry) => entry.key);
    /** @type {any} */
    const info = {
      line: lineNo,
      text,
      indent: parsed.indent,
      trimmed,
      parents
    };

    if (parsed.key) {
      info.key = parsed.key;
      info.keyRange = new vscode.Range(lineNo, parsed.keyStart, lineNo, parsed.keyEnd);
      info.valueText = parsed.valueText;
      info.valueRange = parsed.valueStart >= 0
        ? new vscode.Range(lineNo, parsed.valueStart, lineNo, text.length)
        : null;
      info.kind = parents.length === 0 ? "topLevelKey" : "key";

      if (/\{\s*,?\s*$/.test(parsed.valueText)) {
        stack.push({ indent: parsed.indent, key: parsed.key });
      }
    }

    infos.push(info);
  }

  return infos;
}

function jsonStructureInfoAt(document, position) {
  return analyzeJsonStructure(document)[position.line];
}

function structureInfoAt(document, position) {
  return analyzeYamlStructure(document)[position.line];
}

function findVarsBlockInfo(document, position) {
  const info = structureInfoAt(document, position);
  if (!info) return null;
  if (info.kind === "varKey" || info.inVars) {
    return info;
  }
  return null;
}

function findInputsBlockInfo(document, position) {
  const info = structureInfoAt(document, position);
  if (!info) return null;
  if (info.inInputs) {
    return info;
  }
  return null;
}

function isDirectChildOfInputs(info) {
  if (!info?.parents) return false;
  return info.parents[info.parents.length - 1] === "inputs";
}

function rangeContains(range, position) {
  if (!range) return false;
  return range.contains(position);
}

function hoverTargetAt(document, position) {
  const info = structureInfoAt(document, position);
  if (!info) return null;

  if (rangeContains(info.keyRange, position) && info.key) {
    return { info, range: info.keyRange, word: info.key };
  }

  if (rangeContains(info.scalarRange, position) && info.scalarListValue) {
    return { info, range: info.scalarRange, word: info.scalarListValue };
  }

  const range = wordRange(document, position);
  if (!range) return info ? { info, range: null, word: "" } : null;
  const word = document.getText(range);
  return { info, range, word };
}

function hoverForWorkflowKey(word, docs, range) {
  const doc = docs[word];
  if (!doc) return null;
  const md = new vscode.MarkdownString();
  md.appendMarkdown(`**${word}**`);
  md.appendMarkdown(`\n\n${doc}`);
  return new vscode.Hover(md);
}

function hoverForJsonKey(word, docs, range, sectionName) {
  const doc = docs?.[word];
  if (!doc) return null;
  const md = new vscode.MarkdownString();
  md.appendMarkdown(`**${word}**`);
  if (sectionName) {
    md.appendMarkdown(`\n\nSection: \`${sectionName}\``);
  }
  md.appendMarkdown(`\n\n${doc}`);
  return new vscode.Hover(md);
}

function hoverForVarsContainer(range, modelCtx, fallbackDoc) {
  const iface = modelCtx.entryInterface ? modelCtx.types[modelCtx.entryInterface] : undefined;
  const md = new vscode.MarkdownString();
  md.appendMarkdown("**vars**");
  md.appendMarkdown(`\n\n${fallbackDoc || TOP_LEVEL_KEY_DETAILS.vars}`);
  if (!iface) {
    return new vscode.Hover(md);
  }

  const names = Object.keys(iface.fields);
  if (names.length === 0) {
    return new vscode.Hover(md);
  }

  md.appendMarkdown("\n\nPossible variables:");
  for (const name of names) {
    const field = iface.fields[name];
    const exportNames = fieldExportNames(name, field);
    const detail = field?.doc ? ` - ${field.doc}` : "";
    for (const exportName of exportNames) {
      md.appendMarkdown(`\n- \`${exportName}\`${detail}`);
    }
  }
  return new vscode.Hover(md);
}

function hoverForInputsContainer(range, modelCtx, fallbackDoc) {
  const fields = modelInputFields(modelCtx);
  const md = new vscode.MarkdownString();
  md.appendMarkdown("**inputs**");
  md.appendMarkdown(`\n\n${fallbackDoc || TOP_LEVEL_KEY_DETAILS.inputs}`);
  if (fields.length === 0) {
    return new vscode.Hover(md);
  }
  md.appendMarkdown("\n\nPossible typed inputs:");
  for (const field of fields) {
    const detail = field.doc ? ` - ${field.doc}` : "";
    md.appendMarkdown(`\n- \`${field.path}\`${detail}`);
  }
  return new vscode.Hover(md);
}

function yamlScalarValue(text) {
  return String(text || "").trim().replace(/^['"]|['"]$/g, "");
}

function splitIdentifierWords(name) {
  const value = String(name || "").trim();
  if (!value) return [];
  const parts = [];
  let start = 0;
  for (let i = 1; i < value.length; i++) {
    const ch = value[i];
    const prev = value[i - 1];
    const next = value[i + 1] || "";
    const isUpper = ch >= "A" && ch <= "Z";
    const prevIsLower = prev >= "a" && prev <= "z";
    const prevIsDigit = prev >= "0" && prev <= "9";
    const prevIsUpper = prev >= "A" && prev <= "Z";
    const nextIsLower = next >= "a" && next <= "z";
    if (isUpper && (prevIsLower || prevIsDigit || (prevIsUpper && nextIsLower))) {
      parts.push(value.slice(start, i));
      start = i;
    }
  }
  parts.push(value.slice(start));
  return parts.filter(Boolean);
}

function exportedVarName(name) {
  const value = String(name || "").trim();
  if (!value) return "";
  if (value.includes("_")) return value;
  const parts = splitIdentifierWords(value);
  if (parts.length >= 2 && /^tf$/i.test(parts[0]) && /^var$/i.test(parts[1])) {
    if (parts.length === 2) return "TF_VAR";
    return `TF_VAR_${parts.slice(2).join("_").toLowerCase()}`;
  }
  return parts.join("_").toUpperCase();
}

function fieldExportNames(sourceName, fieldInfo) {
  const out = new Set();
  const derived = exportedVarName(sourceName);
  if (derived) {
    out.add(derived);
  }
  const annotated = String(fieldInfo?.annotations?.EnvName || "").trim();
  if (annotated) {
    out.add(annotated);
  }
  return Array.from(out);
}

function modelFieldInfo(modelCtx, fieldName) {
  if (!fieldName) return null;
  const iface = modelCtx.entryInterface ? modelCtx.types[modelCtx.entryInterface] : undefined;
  const impl = modelCtx.entryClass ? modelCtx.types[modelCtx.entryClass] : undefined;
  let sourceName = fieldName;
  let ifaceField = iface?.fields[fieldName];
  if (!ifaceField && iface) {
    for (const name of Object.keys(iface.fields)) {
      if (fieldExportNames(name, iface.fields[name]).includes(fieldName)) {
        sourceName = name;
        ifaceField = iface.fields[name];
        break;
      }
    }
  }
  const implField = impl?.fields[sourceName];
  if (!ifaceField && !implField) return null;
  return {
    sourceName,
    exportedName: String(ifaceField?.annotations?.EnvName || implField?.annotations?.EnvName || exportedVarName(sourceName)),
    type: ifaceField?.type || implField?.type || "string",
    doc: ifaceField?.doc || implField?.doc,
    defaultValue: implField?.defaultValue
  };
}

function resolveModelTypeSurface(modelCtx, typeName) {
  const exact = modelCtx.types[typeName];
  if (!exact) {
    return { iface: undefined, impl: undefined };
  }
  if (exact.kind === "Interface") {
    const impls = Object.values(modelCtx.types).filter((t) => t.implements === typeName);
    return {
      iface: exact,
      impl: impls.length === 1 ? impls[0] : undefined
    };
  }
  const ifaceName = `I${typeName}`;
  const iface = modelCtx.types[ifaceName]?.kind === "Interface" ? modelCtx.types[ifaceName] : undefined;
  return {
    iface,
    impl: exact
  };
}

function isPrimitiveLikeType(typeName) {
  const t = String(typeName || "").trim();
  return t === "string" || t === "int" || t === "bool" || t === "float";
}

function listElementType(typeName) {
  const t = String(typeName || "").trim();
  const match = t.match(/^List<\s*([^>]+)\s*>$/);
  return match ? match[1].trim() : "";
}

function modelInputFields(modelCtx) {
  const start = modelCtx.entryClass || modelCtx.entryInterface;
  if (!start) return [];
  const out = [];
  const seen = new Set();

  function visit(typeName, prefix) {
    const { iface, impl } = resolveModelTypeSurface(modelCtx, typeName);
    const typeInfo = iface || impl;
    if (!typeInfo) return;
    const visitKey = `${typeName}:${prefix}`;
    if (seen.has(visitKey)) return;
    seen.add(visitKey);
    const fieldNames = new Set([
      ...Object.keys(iface?.fields || {}),
      ...Object.keys(impl?.fields || {})
    ]);
    for (const fieldName of fieldNames) {
      const ifaceField = iface?.fields?.[fieldName];
      const implField = impl?.fields?.[fieldName];
      const field = ifaceField || implField;
      if (!field) continue;
      const pathName = prefix ? `${prefix}.${fieldName}` : fieldName;
      const elementType = listElementType(field.type);
      if (isPrimitiveLikeType(field.type) || (elementType && isPrimitiveLikeType(elementType))) {
        out.push({
          path: pathName,
          sourceName: fieldName,
          type: ifaceField?.type || implField?.type || "string",
          doc: ifaceField?.doc || implField?.doc,
          defaultValue: implField?.defaultValue,
          exportedName: String(ifaceField?.annotations?.EnvName || implField?.annotations?.EnvName || "").trim()
        });
        continue;
      }
      visit(implField?.type || ifaceField?.type, pathName);
    }
  }

  visit(start, "");
  return out;
}

function modelInputFieldInfo(modelCtx, fieldPath) {
  return modelInputFields(modelCtx).find((field) => field.path === fieldPath) || null;
}

function hoverForModelField(fieldName, fieldInfo, range, currentValue) {
  if (!fieldInfo) return null;
  const md = new vscode.MarkdownString();
  md.appendMarkdown(`**${fieldName}**`);
  if (fieldInfo.sourceName && fieldInfo.sourceName !== fieldName) {
    md.appendMarkdown(`\n\nModel field: \`${fieldInfo.sourceName}\``);
  }
  md.appendMarkdown(`\n\nType: \`${fieldInfo.type || "string"}\``);
  if (fieldInfo.doc) md.appendMarkdown(`\n\n${fieldInfo.doc}`);
  if (fieldInfo.defaultValue) md.appendMarkdown(`\n\nModel default: \`${fieldInfo.defaultValue}\``);
  if (currentValue !== undefined) md.appendMarkdown(`\n\nCurrent value: \`${currentValue}\``);
  return new vscode.Hover(md);
}

function hoverForKnownValue(fieldName, rawValue, fieldInfo, knownValue, range) {
  if (!rawValue) return null;
  const md = new vscode.MarkdownString();
  md.appendMarkdown(`**${rawValue}**`);
  if (fieldName) md.appendMarkdown(`\n\nValue for \`${fieldName}\``);
  if (fieldInfo?.doc) md.appendMarkdown(`\n\n${fieldInfo.doc}`);
  if (fieldInfo?.defaultValue) md.appendMarkdown(`\n\nModel default: \`${fieldInfo.defaultValue}\``);
  if (knownValue) {
    md.appendMarkdown(`\n\nKnown value: \`${knownValue.name}\``);
    if (knownValue.doc) md.appendMarkdown(`\n\n${knownValue.doc}`);
  }
  return new vscode.Hover(md);
}

function findTypeEntryModel(modelCtx, entry) {
  if (!entry) return null;
  let left = String(entry).trim();
  let explicitRef = "";
  const i = left.indexOf("<");
  const j = left.lastIndexOf(">");
  if (i >= 0 && j > i) {
    explicitRef = left.slice(i + 1, j).trim();
    left = left.slice(0, i).trim();
  }
  const name = left.split("/").pop() || left;
  return {
    entry: modelCtx.types[name],
    implementation: explicitRef ? modelCtx.types[explicitRef] : undefined
  };
}

/** @param {vscode.ExtensionContext} context */
function activate(context) {
  const yamlDiagnostics = vscode.languages.createDiagnosticCollection("dockpipe-yaml");
  context.subscriptions.push(yamlDiagnostics);

  const refreshYamlDiagnostics = (doc) => {
    if (!doc || doc.languageId !== "yaml" || !isDockpipeWorkflowFile(doc)) {
      return;
    }
    yamlDiagnostics.set(doc.uri, validateYaml(doc));
  };

  for (const doc of vscode.workspace.textDocuments) {
    refreshYamlDiagnostics(doc);
  }

  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument(refreshYamlDiagnostics),
    vscode.workspace.onDidChangeTextDocument((e) => refreshYamlDiagnostics(e.document)),
    vscode.workspace.onDidCloseTextDocument((doc) => yamlDiagnostics.delete(doc.uri))
  );

  context.subscriptions.push(
    vscode.languages.registerDocumentSemanticTokensProvider(
      { language: "yaml", scheme: "file" },
      {
        async provideDocumentSemanticTokens(document) {
          if (!isDockpipeWorkflowFile(document) && !isDockpipePackageManifestFile(document)) {
            return null;
          }
          const infos = analyzeYamlStructure(document);
          const builder = new vscode.SemanticTokensBuilder(SEMANTIC_LEGEND);

          if (isDockpipePackageManifestFile(document)) {
            for (const info of infos) {
              if (info.keyRange && info.kind === "topLevelKey") {
                builder.push(info.keyRange, "property");
              }

              if (info.key && info.kind === "topLevelKey") {
                const valueRange = trimmedValueRange(info);
                const tokenType = packageManifestValueTokenType(info.key, info.valueText);
                if (valueRange && tokenType) {
                  builder.push(valueRange, tokenType);
                }
              }

              if (info.scalarRange) {
                const tokenType = packageManifestListTokenType(info.parents?.[info.parents.length - 1]);
                if (tokenType) {
                  builder.push(info.scalarRange, tokenType);
                }
              }
            }
            return builder.build();
          }

          const modelCtx = await buildModelContext(document);

          for (const info of infos) {
            if (info.keyRange) {
              if (info.kind === "topLevelKey") {
                builder.push(info.keyRange, "keyword");
              } else if (info.kind === "stepKey") {
                builder.push(info.keyRange, "property");
              } else if (info.kind === "varKey") {
                builder.push(info.keyRange, "variable");
              }
            }

            if (info.kind === "typeEntry" && info.scalarRange) {
              builder.push(info.scalarRange, "type");
            }

            if (info.kind === "varKey" && info.valueText && info.valueRange) {
              const key = info.key;
              const rawValue = String(info.valueText).trim().replace(/^['"]|['"]$/g, "");
              const fieldInfo = modelFieldInfo(modelCtx, key);
              const implDefault = fieldInfo?.defaultValue;
              const knownValue = modelCtx.knownValues.find((kv) => kv.value === rawValue);
              if ((implDefault && implDefault === rawValue) || knownValue) {
                const valueOffset = info.text.indexOf(rawValue, info.valueRange.start.character);
                if (valueOffset >= 0) {
                  builder.push(
                    new vscode.Range(info.line, valueOffset, info.line, valueOffset + rawValue.length),
                    "enumMember"
                  );
                }
              }
            }
          }

          return builder.build();
        }
      },
      SEMANTIC_LEGEND
    ),
    vscode.languages.registerCompletionItemProvider(
      { language: "yaml", scheme: "file" },
      {
        async provideCompletionItems(document, position) {
          if (!isDockpipeWorkflowFile(document)) {
            return [];
          }
          const modelCtx = await buildModelContext(document);
          const info = structureInfoAt(document, position);
          const line = document.lineAt(position.line).text;
          const leading = line.match(/^\s*/)?.[0] || "";
          const lineToCursor = line.slice(0, position.character);
          const items = [];

          if (/^\s*-\s*["']?scope/.test(lineToCursor)) {
            for (const [label, meta] of Object.entries(DORKPIPE_AGENT_SCOPE_REF_DETAILS)) {
              const it = new vscode.CompletionItem(label, vscode.CompletionItemKind.Reference);
              it.insertText = new vscode.SnippetString(meta.insertText);
              it.detail = meta.detail;
              it.documentation = meta.documentation;
              items.push(it);
            }
            return items;
          }

          if (info?.kind === "stepKey" && info.key === "cwd" && lineToCursor.includes(":")) {
            return Object.entries(STEP_CWD_VALUE_DETAILS).map(([value, doc]) => {
              const it = new vscode.CompletionItem(value, vscode.CompletionItemKind.EnumMember);
              it.insertText = value;
              it.detail = "DockPipe step cwd";
              it.documentation = doc;
              return it;
            });
          }
          if (info?.kind === "nestedKey" && (info.key === "source" || info.key === "artifacts") && (info.parents || []).includes("scopes") && lineToCursor.includes(":")) {
            return Object.entries(STEP_CWD_VALUE_DETAILS).map(([value, doc]) => {
              const it = new vscode.CompletionItem(value, vscode.CompletionItemKind.EnumMember);
              it.insertText = value;
              it.detail = "DockPipe step scope";
              it.documentation = doc;
              return it;
            });
          }

          if (!lineToCursor.includes(":") && info?.kind === "nestedKey") {
            const parentChain = info.parents || [];
            let docs = null;
            if (parentChain.length === 1 && parentChain[0] === "view") {
              docs = WORKFLOW_VIEW_KEY_DETAILS;
            } else if (parentChain.length === 3 && parentChain[0] === "view" && parentChain[1] === "entry") {
              docs = WORKFLOW_VIEW_ENTRY_KEY_DETAILS;
            } else if (parentChain.length === 5 && parentChain[0] === "view" && parentChain[1] === "entry" && parentChain[3] === "options") {
              docs = WORKFLOW_VIEW_ENTRY_OPTION_KEY_DETAILS;
            } else if (parentChain.length === 3 && parentChain[0] === "view" && parentChain[1] === "pages") {
              docs = WORKFLOW_VIEW_PAGE_KEY_DETAILS;
            } else if (parentChain.length === 5 && parentChain[0] === "view" && parentChain[1] === "pages" && parentChain[3] === "sections") {
              docs = WORKFLOW_VIEW_SECTION_KEY_DETAILS;
            } else if (parentChain.length === 1 && parentChain[0] === "container") {
              docs = CONTAINER_KEY_DETAILS;
            } else if (info?.inSteps && parentChain.length === 1 && parentChain[0] === "vm") {
              docs = STEP_VM_KEY_DETAILS;
            } else if (info?.inSteps && parentChain.length === 1 && parentChain[0] === "container") {
              docs = CONTAINER_KEY_DETAILS;
            } else if (info?.inSteps && parentChain.length === 1 && parentChain[0] === "scopes") {
              docs = STEP_SCOPE_KEY_DETAILS;
            } else if (parentChain.length === 3 && parentChain[0] === "container" && parentChain[1] === "mounts") {
              docs = CONTAINER_MOUNT_KEY_DETAILS;
            } else if (info?.inSteps && parentChain.length === 3 && parentChain[0] === "vm" && parentChain[1] === "mounts") {
              docs = STEP_VM_MOUNT_KEY_DETAILS;
            }
            if (docs) {
              for (const [k, doc] of Object.entries(docs)) {
                const it = new vscode.CompletionItem(k, vscode.CompletionItemKind.Property);
                it.insertText = `${k}: `;
                it.documentation = doc;
                items.push(it);
              }
              return items;
            }
          }

          if (!lineToCursor.includes(":")) {
            if (leading.length === 0) {
              const seen = existingTopLevelKeys(document);
              for (const k of DOCKPIPE_TOP_LEVEL_KEYS) {
                if (seen.has(k)) {
                  continue;
                }
                const it = new vscode.CompletionItem(k, vscode.CompletionItemKind.Property);
                it.insertText = `${k}: `;
                items.push(it);
              }
              return items;
            }
            if (info?.inSteps && !info?.inVars && leading.length > 0) {
              for (const k of DOCKPIPE_STEP_KEYS) {
                const it = new vscode.CompletionItem(k, vscode.CompletionItemKind.Property);
                it.insertText = `${k}: `;
                items.push(it);
              }
              return items;
            }
          }

          if (line.match(/^\s*types:\s*$/) || line.match(/^\s*-\s*models\//)) {
            const modelsDir = path.join(path.dirname(document.fileName), "models");
            const entries = await fs.readdir(modelsDir).catch(() => []);
            for (const name of entries.filter((n) => n.endsWith(".pipe"))) {
              const it = new vscode.CompletionItem(`models/${name.replace(/\.pipe$/, "")}`, vscode.CompletionItemKind.Reference);
              it.detail = "Type entrypoint (compiler infers implementing class when interface is selected)";
              items.push(it);
            }
          }

          const varsInfo = findVarsBlockInfo(document, position);
          if (varsInfo) {
            const before = lineToCursor;
            const colonIdx = before.indexOf(":");
            if (colonIdx < 0) {
              const iface = modelCtx.entryInterface ? modelCtx.types[modelCtx.entryInterface] : undefined;
              if (iface) {
                for (const [fieldName, fi] of Object.entries(iface.fields)) {
                  const exportNames = fieldExportNames(fieldName, fi);
                  const implDefault = modelCtx.entryClass ? modelCtx.types[modelCtx.entryClass]?.fields[fieldName]?.defaultValue : undefined;
                  for (const exportName of exportNames) {
                    const it = new vscode.CompletionItem(exportName, vscode.CompletionItemKind.Variable);
                    it.insertText = `${exportName}: `;
                    it.detail = `${fi.type || "string"} (from ${iface.name}.${fieldName})`;
                    if (fi.doc) it.documentation = fi.doc;
                    if (implDefault) {
                      it.documentation = new vscode.MarkdownString(`${fi.doc || ""}\n\nDefault: \`${implDefault}\``);
                    }
                    items.push(it);
                  }
                }
                return items;
              }
            } else {
              const key = before.slice(0, colonIdx).trim().replace(/^- /, "");
              const fieldInfo = modelFieldInfo(modelCtx, key);
              if (fieldInfo?.defaultValue) {
                const it = new vscode.CompletionItem(fieldInfo.defaultValue, vscode.CompletionItemKind.Value);
                it.insertText = quoted(fieldInfo.defaultValue);
                it.detail = `${key} default from ${modelCtx.entryClass}`;
                items.push(it);
              }
              const keyN = key.toLowerCase().replace(/[^a-z0-9]/g, "");
              for (const kv of modelCtx.knownValues) {
                const valueN = kv.name.toLowerCase().replace(/[^a-z0-9]/g, "");
                const score = keyN && valueN.includes(keyN) ? "0" : "1";
                const it = new vscode.CompletionItem(kv.value, vscode.CompletionItemKind.EnumMember);
                it.insertText = quoted(kv.value);
                it.detail = kv.name;
                it.sortText = `${score}-${kv.name}`;
                if (kv.doc) it.documentation = kv.doc;
                items.push(it);
              }
              if (items.length > 0) return items;
            }
          }
          const inputsInfo = findInputsBlockInfo(document, position);
          if (inputsInfo) {
            const before = lineToCursor;
            const colonIdx = before.indexOf(":");
            if (colonIdx < 0) {
              if (inputsInfo.parents.length >= 2 && inputsInfo.parents.includes("inputs")) {
                for (const [k, doc] of Object.entries(INPUT_BINDING_KEY_DETAILS)) {
                  const it = new vscode.CompletionItem(k, vscode.CompletionItemKind.Property);
                  it.insertText = `${k}: `;
                  it.documentation = doc;
                  items.push(it);
                }
                return items;
              }
              for (const field of modelInputFields(modelCtx)) {
                const it = new vscode.CompletionItem(field.path, vscode.CompletionItemKind.Variable);
                it.insertText = `${field.path}: `;
                it.detail = `${field.type || "string"}${field.exportedName ? ` → ${field.exportedName}` : ""}`;
                if (field.doc || field.defaultValue || field.exportedName) {
                  const md = new vscode.MarkdownString();
                  if (field.doc) md.appendMarkdown(field.doc);
                  if (field.defaultValue) md.appendMarkdown(`\n\nDefault: \`${field.defaultValue}\``);
                  if (field.exportedName) md.appendMarkdown(`\n\nEnv: \`${field.exportedName}\``);
                  it.documentation = md;
                }
                items.push(it);
              }
              return items;
            } else {
              const key = before.slice(0, colonIdx).trim().replace(/^- /, "");
              const fieldInfo = modelInputFieldInfo(modelCtx, key);
              if (fieldInfo?.defaultValue) {
                const it = new vscode.CompletionItem(fieldInfo.defaultValue, vscode.CompletionItemKind.Value);
                it.insertText = quoted(fieldInfo.defaultValue);
                it.detail = `${key} default from typed model`;
                items.push(it);
              }
              if (items.length > 0) return items;
            }
          }
          return items;
        }
      },
      ":",
      "-"
    )
  );

  context.subscriptions.push(
    vscode.languages.registerHoverProvider(
      { language: "yaml", scheme: "file" },
      {
        async provideHover(document, position) {
          if (isDockpipePackageManifestFile(document)) {
            const target = hoverTargetAt(document, position);
            if (!target) return null;
            const { info, range, word } = target;
            if (info?.kind === "topLevelKey") {
              return hoverForWorkflowKey(word, PACKAGE_MANIFEST_KEY_DETAILS, range);
            }
            if (info?.kind === "nestedKey" && info.parents.length === 1 && info.parents[0] === "script_contract") {
              return hoverForWorkflowKey(word, PACKAGE_SCRIPT_CONTRACT_KEY_DETAILS, range);
            }
            if (info?.kind === "listValue" && info.parents.length === 2 && info.parents[0] === "script_contract" && info.parents[1] === "inject") {
              const doc = PACKAGE_SCRIPT_CONTRACT_INJECTABLES[word];
              if (!doc) return null;
              return new vscode.Hover(new vscode.MarkdownString(`**${word}**\n\n${doc}`), range);
            }
            return null;
          }

          if (!isDockpipeWorkflowFile(document)) return null;
          const target = hoverTargetAt(document, position);
          if (!target) return null;
          const { info, range, word } = target;
          if (!word || !range) return null;

          const modelCtx = await buildModelContext(document);

          if (info?.kind === "topLevelKey") {
            if (word === "inputs") {
              return hoverForInputsContainer(range, modelCtx, TOP_LEVEL_KEY_DETAILS.inputs);
            }
            if (word === "vars") {
              return hoverForVarsContainer(range, modelCtx, TOP_LEVEL_KEY_DETAILS.vars);
            }
            return hoverForWorkflowKey(word, TOP_LEVEL_KEY_DETAILS, range);
          }

          if (info?.kind === "stepKey") {
            if (word === "inputs") {
              return hoverForInputsContainer(range, modelCtx, STEP_KEY_DETAILS.inputs);
            }
            if (word === "vars") {
              return hoverForVarsContainer(range, modelCtx, STEP_KEY_DETAILS.vars);
            }
            return hoverForWorkflowKey(word, STEP_KEY_DETAILS, range);
          }

          if (info?.kind === "nestedKey") {
            const parentChain = info.parents || [];
            if (parentChain.length === 1 && parentChain[0] === "view") {
              return hoverForWorkflowKey(word, WORKFLOW_VIEW_KEY_DETAILS, range);
            }
            if (parentChain.length === 1 && parentChain[0] === "container") {
              return hoverForWorkflowKey(word, CONTAINER_KEY_DETAILS, range);
            }
            if (info?.inSteps && parentChain.length === 1 && parentChain[0] === "vm") {
              return hoverForWorkflowKey(word, STEP_VM_KEY_DETAILS, range);
            }
            if (info?.inSteps && parentChain.length === 1 && parentChain[0] === "container") {
              return hoverForWorkflowKey(word, CONTAINER_KEY_DETAILS, range);
            }
            if (info?.inSteps && parentChain.length === 1 && parentChain[0] === "scopes") {
              return hoverForWorkflowKey(word, STEP_SCOPE_KEY_DETAILS, range);
            }
            if (parentChain.length === 3 && parentChain[0] === "container" && parentChain[1] === "mounts") {
              return hoverForWorkflowKey(word, CONTAINER_MOUNT_KEY_DETAILS, range);
            }
            if (info?.inSteps && parentChain.length === 3 && parentChain[0] === "vm" && parentChain[1] === "mounts") {
              return hoverForWorkflowKey(word, STEP_VM_MOUNT_KEY_DETAILS, range);
            }
            if (parentChain.includes("inputs") && INPUT_BINDING_KEY_DETAILS[word]) {
              return hoverForWorkflowKey(word, INPUT_BINDING_KEY_DETAILS, range);
            }
            if (parentChain.length === 3 && parentChain[0] === "view" && parentChain[1] === "entry") {
              return hoverForWorkflowKey(word, WORKFLOW_VIEW_ENTRY_KEY_DETAILS, range);
            }
            if (parentChain.length === 5 && parentChain[0] === "view" && parentChain[1] === "entry" && parentChain[3] === "options") {
              return hoverForWorkflowKey(word, WORKFLOW_VIEW_ENTRY_OPTION_KEY_DETAILS, range);
            }
            if (parentChain.length === 3 && parentChain[0] === "view" && parentChain[1] === "pages") {
              return hoverForWorkflowKey(word, WORKFLOW_VIEW_PAGE_KEY_DETAILS, range);
            }
            if (parentChain.length === 5 && parentChain[0] === "view" && parentChain[1] === "pages" && parentChain[3] === "sections") {
              return hoverForWorkflowKey(word, WORKFLOW_VIEW_SECTION_KEY_DETAILS, range);
            }
          }

          if (info?.kind === "typeEntry") {
            const match = findTypeEntryModel(modelCtx, info.scalarListValue);
            if (match?.entry) {
              const md = new vscode.MarkdownString();
              md.appendMarkdown(`**${info.scalarListValue}**`);
              md.appendMarkdown(`\n\nEntry type: \`${match.entry.kind} ${match.entry.name}\``);
              if (match.entry.doc) md.appendMarkdown(`\n\n${match.entry.doc}`);
              if (match.implementation) {
                md.appendMarkdown(`\n\nImplementation: \`${match.implementation.name}\``);
              } else if (modelCtx.entryClass) {
                md.appendMarkdown(`\n\nImplementation: \`${modelCtx.entryClass}\``);
              }
              return new vscode.Hover(md);
            }
          }

          const varsInfo = findVarsBlockInfo(document, position);
          if (varsInfo) {
            const lineKey = varsInfo.kind === "varKey" ? varsInfo.key : null;
            const lineField = modelFieldInfo(modelCtx, lineKey);
            if (lineKey && rangeContains(info?.keyRange, position)) {
              return hoverForModelField(lineKey, lineField || { type: "string", doc: VAR_KEY_FALLBACK_DETAIL }, info.keyRange, yamlScalarValue(info.valueText));
            }

            const kv = modelCtx.knownValues.find((k) => k.value === word || k.name.endsWith("." + word));
            if (kv) {
              return hoverForKnownValue(lineKey, kv.value, lineField, kv, range);
            }

            if (lineKey && rangeContains(info?.valueRange, position)) {
              const rawValue = yamlScalarValue(info.valueText);
              const matchedKnownValue = modelCtx.knownValues.find((k) => k.value === rawValue || k.name.endsWith("." + rawValue));
              if (lineField || matchedKnownValue) {
                return hoverForKnownValue(lineKey, rawValue, lineField, matchedKnownValue, range);
              }
              return hoverForKnownValue(lineKey, rawValue, { type: "string", doc: VAR_KEY_FALLBACK_DETAIL }, matchedKnownValue, range);
            }

            if (lineKey && lineField) {
              return hoverForModelField(lineKey, lineField, range, word);
            }
          }

          const inputsInfo = findInputsBlockInfo(document, position);
          if (!inputsInfo) return null;
          const inputField = modelInputFieldInfo(modelCtx, inputsInfo.key);
          if (isDirectChildOfInputs(inputsInfo) && inputField && rangeContains(info?.keyRange, position)) {
            return hoverForModelField(inputsInfo.key, inputField, info.keyRange, yamlScalarValue(info.valueText));
          }
          if (isDirectChildOfInputs(inputsInfo) && inputField && rangeContains(info?.valueRange, position)) {
            return hoverForKnownValue(inputsInfo.key, yamlScalarValue(info.valueText), inputField, null, range);
          }
          return null;
        }
      }
    )
  );

  context.subscriptions.push(
    vscode.languages.registerCompletionItemProvider(
      { language: "yaml", scheme: "file" },
      {
        provideCompletionItems(document, position) {
          if (!isDockpipePackageManifestFile(document)) {
            return [];
          }
          const structure = yamlStructureInfoAt(document, position);
          if (structure?.kind === "nestedKey" && structure.parents.length === 1 && structure.parents[0] === "script_contract") {
            const line = document.lineAt(position.line).text;
            const lineToCursor = line.slice(0, position.character);
            if (lineToCursor.includes(":")) {
              return [];
            }
            return PACKAGE_SCRIPT_CONTRACT_KEYS.map((k) => {
              const it = new vscode.CompletionItem(k, vscode.CompletionItemKind.Property);
              it.insertText = `${k}: `;
              it.documentation = PACKAGE_SCRIPT_CONTRACT_KEY_DETAILS[k];
              return it;
            });
          }
          if (structure?.kind === "listValue" && structure.parents.length === 2 && structure.parents[0] === "script_contract" && structure.parents[1] === "inject") {
            return Object.entries(PACKAGE_SCRIPT_CONTRACT_INJECTABLES).map(([k, doc]) => {
              const it = new vscode.CompletionItem(k, vscode.CompletionItemKind.EnumMember);
              it.insertText = k;
              it.documentation = doc;
              return it;
            });
          }
          const line = document.lineAt(position.line).text;
          const lineToCursor = line.slice(0, position.character);
          if (lineToCursor.includes(":")) {
            return [];
          }
          const leading = line.match(/^\s*/)?.[0] || "";
          if (leading.length !== 0) {
            return [];
          }
          const seen = existingTopLevelKeys(document);
          return PACKAGE_MANIFEST_KEYS
            .filter((k) => !seen.has(k))
            .map((k) => {
              const it = new vscode.CompletionItem(k, vscode.CompletionItemKind.Property);
              it.insertText = `${k}: `;
              const doc = PACKAGE_MANIFEST_KEY_DETAILS[k];
              if (doc) it.documentation = doc;
              return it;
            });
        }
      },
      ":"
    )
  );

  context.subscriptions.push(
    vscode.languages.registerHoverProvider(
      [
        { language: "json", scheme: "file" },
        { language: "jsonc", scheme: "file" }
      ],
      {
        provideHover(document, position) {
          if (!isDockpipeProjectConfigFile(document)) return null;
          const info = jsonStructureInfoAt(document, position);
          if (!info?.keyRange || !rangeContains(info.keyRange, position) || !info.key) {
            return null;
          }
          if (info.parents.length === 0) {
            return hoverForJsonKey(info.key, DOCKPIPE_PROJECT_TOP_LEVEL_KEY_DETAILS, info.keyRange);
          }
          const sectionName = info.parents[0];
          const docs = DOCKPIPE_PROJECT_SECTION_KEY_DETAILS[sectionName];
          return hoverForJsonKey(info.key, docs, info.keyRange, sectionName);
        }
      }
    )
  );

  context.subscriptions.push(
    vscode.languages.registerCompletionItemProvider(
      [
        { language: "json", scheme: "file" },
        { language: "jsonc", scheme: "file" }
      ],
      {
        provideCompletionItems(document, position) {
          if (!isDockpipeProjectConfigFile(document)) {
            return [];
          }
          const info = jsonStructureInfoAt(document, position);
          const line = document.lineAt(position.line).text;
          const lineToCursor = line.slice(0, position.character);
          if (lineToCursor.includes(":")) {
            return [];
          }

          const items = [];
          const parentKeys = info?.parents || [];
          if (parentKeys.length === 0) {
            const seen = existingJsonObjectKeys(document, []);
            for (const key of DOCKPIPE_PROJECT_TOP_LEVEL_KEYS) {
              if (seen.has(key)) continue;
              const it = new vscode.CompletionItem(key, vscode.CompletionItemKind.Property);
              it.insertText = `"${key}": `;
              if (DOCKPIPE_PROJECT_TOP_LEVEL_KEY_DETAILS[key]) {
                it.documentation = DOCKPIPE_PROJECT_TOP_LEVEL_KEY_DETAILS[key];
              }
              items.push(it);
            }
            return items;
          }

          const sectionName = parentKeys[0];
          const docs = DOCKPIPE_PROJECT_SECTION_KEY_DETAILS[sectionName];
          if (!docs) {
            return items;
          }
          const seen = existingJsonObjectKeys(document, parentKeys);
          for (const key of Object.keys(docs)) {
            if (seen.has(key)) continue;
            const it = new vscode.CompletionItem(key, vscode.CompletionItemKind.Property);
            it.insertText = `"${key}": `;
            it.documentation = docs[key];
            items.push(it);
          }
          return items;
        }
      },
      "\""
    )
  );

  context.subscriptions.push(
    vscode.languages.registerCompletionItemProvider(
      { language: "pipelang", scheme: "file" },
      {
        provideCompletionItems() {
          const kws = [
            "public",
            "private",
            "Interface",
            "Class",
            "Struct",
            "List",
            "IComparable",
            "string",
            "int",
            "bool",
            "float",
            "true",
            "false"
          ];
          return kws.map((k) => new vscode.CompletionItem(k, vscode.CompletionItemKind.Keyword));
        }
      }
    )
  );

  context.subscriptions.push(
    vscode.languages.registerCompletionItemProvider(
      [
        { language: "shellscript", scheme: "file" },
        { language: "powershell", scheme: "file" },
        { language: "python", scheme: "file" },
        { language: "go", scheme: "file" }
      ],
      {
        provideCompletionItems(document) {
          const profile = coreHelperProfileForDocument(document);
          if (!profile) {
            return [];
          }

          const items = [];
          items.push(...runtimeEnvCompletionItems(document.languageId));

          const sourceItem = new vscode.CompletionItem(profile.sourceLabel, vscode.CompletionItemKind.Snippet);
          sourceItem.insertText = new vscode.SnippetString(profile.sourceSnippet);
          sourceItem.detail = profile.sourceDetail;
          sourceItem.documentation = `Use the canonical core helper \`${profile.helperPath}\` instead of open-coding repo/PATH binary lookup.`;
          if (document.languageId === "shellscript") {
            sourceItem.filterText = 'eval dockpipe sdk';
          }
          items.push(sourceItem);

          for (const helper of profile.functions) {
            const item = new vscode.CompletionItem(helper.name, vscode.CompletionItemKind.Function);
            item.insertText = helper.insertText;
            item.detail = helper.detail;
            item.documentation = helper.documentation;
            if (helper.filterText) {
              item.filterText = helper.filterText;
            }
            items.push(item);
          }

          return items;
        }
      },
      "$",
      "[",
      "."
    )
  );

  context.subscriptions.push(
    vscode.languages.registerHoverProvider(
      [
        { language: "shellscript", scheme: "file" },
        { language: "powershell", scheme: "file" },
        { language: "python", scheme: "file" },
        { language: "go", scheme: "file" }
      ],
      {
        provideHover(document, position) {
          const profile = coreHelperProfileForDocument(document);
          if (!profile) {
            return null;
          }

          const range = document.getWordRangeAtPosition(position, /[A-Za-z_$\[\].-][A-Za-z0-9_$\[\].-]*/);
          if (!range) {
            return null;
          }
          const word = document.getText(range);
          const helper = profile.functions.find((fn) => fn.name === word);
          if (!helper) {
            return null;
          }

          const md = new vscode.MarkdownString();
          md.appendMarkdown(`**${helper.name}**`);
          md.appendMarkdown(`\n\n${helper.documentation}`);
          md.appendMarkdown(`\n\nPackage helper: \`${profile.helperPath}\``);
          return new vscode.Hover(md, range);
        }
      }
    )
  );
}

function deactivate() {}

module.exports = { activate, deactivate };
