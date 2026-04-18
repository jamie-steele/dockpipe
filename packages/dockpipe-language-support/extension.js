// @ts-check
const vscode = require("vscode");
const fs = require("fs/promises");
const path = require("path");

const DOCKPIPE_TOP_LEVEL_KEYS = [
  "name",
  "description",
  "namespace",
  "types",
  "docker_preflight",
  "vars",
  "steps",
  "resolver",
  "default_resolver",
  "runtime",
  "default_runtime",
  "runtimes",
  "strategy",
  "strategies",
  "vault",
  "compile_hooks",
  "imports",
  "inject"
];

const DOCKPIPE_STEP_KEYS = [
  "id",
  "run",
  "pre_script",
  "skip_container",
  "vars",
  "outputs",
  "runtime",
  "resolver",
  "package",
  "host_builtin",
  "is_blocking"
];

const TOP_LEVEL_KEY_DETAILS = {
  name: "Workflow name.",
  description: "Human-readable workflow description.",
  namespace: "Package namespace used for compiled workflow material.",
  types: "PipeLang entrypoint list used to drive model-backed vars help.",
  docker_preflight: "Enable or disable the Docker preflight check before running.",
  vars: "Workflow variables exported before execution unless already set in the environment.",
  steps: "Ordered workflow steps. Each step can run in a container or on the host.",
  resolver: "Tooling profile for this workflow.",
  default_resolver: "Fallback resolver when a step does not set one.",
  runtime: "Execution substrate for this workflow.",
  default_runtime: "Fallback runtime when a step does not set one.",
  runtimes: "Allowlist of runtimes permitted for the workflow.",
  strategy: "Lifecycle wrapper that runs around the workflow.",
  strategies: "Available strategies for selection or validation.",
  vault: "Vault provider used for secret injection.",
  compile_hooks: "Shell hooks run during package compile before the tarball is written.",
  imports: "Additional workflow material pulled in during authoring or compile.",
  inject: "Injection rules for env/template expansion."
};

const STEP_KEY_DETAILS = {
  id: "Stable step label used in logs and outputs.",
  run: "Command or script list to execute for this step.",
  pre_script: "Host-side scripts that run before the step body.",
  skip_container: "Run this step on the host instead of inside the runtime container.",
  vars: "Step-local variables merged for this step.",
  outputs: "Named outputs passed to later steps.",
  runtime: "Override the runtime for this step.",
  resolver: "Override the resolver for this step.",
  package: "Nested workflow/package selector for package runtime steps.",
  host_builtin: "Built-in host action instead of a script.",
  is_blocking: "When false, allows async grouping with surrounding steps."
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
  "distribution"
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
  distribution: "Human/tooling hint such as source or binary."
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

const VAR_KEY_FALLBACK_DETAIL = "Workflow variable override. This key exports an environment variable for the workflow or step.";

const CONTAINER_KEYS = new Set([
  "types",
  "vars",
  "steps",
  "run",
  "outputs",
  "imports",
  "inject",
  "runtimes",
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

const CORE_HELPER_PROFILES = {
  shellscript: {
    helperPath: "dockpipe sdk",
    sourceSnippet:
      'eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"',
    sourceLabel: 'eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"',
    sourceDetail: "Bootstrap the canonical DockPipe shell SDK via the CLI",
    functions: [
      {
        name: "dockpipe_sdk workdir",
        detail: "Print the SDK workdir/root.",
        insertText: "dockpipe_sdk workdir",
        filterText: "dockpipe_sdk workdir dockpipe root workdir",
        documentation: "First-hand shell SDK action that prints the effective DockPipe workdir/repo root."
      },
      {
        name: "dockpipe_sdk init-script",
        detail: "Initialize common script vars and enter the workdir.",
        insertText: "dockpipe_sdk init-script",
        filterText: "dockpipe_sdk init-script dockpipe init script root wf_ns workdir",
        documentation: "First-hand shell SDK action that initializes `ROOT` and `WF_NS` from the SDK context and changes into the DockPipe workdir."
      },
      {
        name: "dockpipe_sdk cd-workdir",
        detail: "Change directory to the SDK workdir.",
        insertText: "dockpipe_sdk cd-workdir",
        filterText: "dockpipe_sdk cd-workdir dockpipe cd workdir root chdir",
        documentation: "First-hand shell SDK action that changes the current directory to the effective DockPipe workdir."
      },
      {
        name: "dockpipe_sdk workflow-name",
        detail: "Return workflow name when available.",
        insertText: "dockpipe_sdk workflow-name",
        filterText: "dockpipe_sdk workflow-name dockpipe workflow name",
        documentation: "First-hand shell SDK action that prints the workflow name when the script is running inside a DockPipe workflow."
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
      }
    ]
  }
};

/**
 * @typedef {{ doc?: string, type?: string, defaultValue?: string }} FieldInfo
 * @typedef {{ name: string, kind: "Interface"|"Class"|"Struct", implements?: string, doc?: string, fields: Record<string, FieldInfo> }} TypeInfo
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

  const lines = source.split(/\r?\n/);
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith("///")) {
      summaryLines.push(trimmed);
      if (trimmed.includes("<summary>")) collectingSummary = true;
      if (trimmed.includes("</summary>")) collectingSummary = false;
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
        fields: {}
      };
      currentBodyStarted = false;
      out.push(current);
      summaryLines = [];
    } else if (current) {
      const fieldMatch = trimmed.match(/^(?:public|private)\s+(string|int|bool|float)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:=\s*("([^"\\]|\\.)*"|[^;]+))?\s*;/);
      if (fieldMatch) {
        let def = fieldMatch[3]?.trim();
        if (typeof def === "string" && def.startsWith("\"") && def.endsWith("\"")) {
          def = def.slice(1, -1);
        }
        current.fields[fieldMatch[2]] = {
          doc: pendingSummary,
          type: fieldMatch[1],
          defaultValue: def
        };
        summaryLines = [];
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
  }
  return out;
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
  const typeEntries = extractTypesEntries(doc);
  if (typeEntries.length === 0) {
    return { types: {}, knownValues: [] };
  }
  const wfDir = path.dirname(doc.fileName);
  const modelsDir = path.join(wfDir, "models");

  /** @type {ModelContext} */
  const ctx = { types: {}, knownValues: [] };
  for (const entry of typeEntries) {
    const spec = String(entry).trim();
    if (!spec) continue;
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
    const primaryFile = path.join(wfDir, rel);
    const primarySrc = await readIfExists(primaryFile);
    if (!primarySrc) continue;

    const allModelFiles = (await fs.readdir(modelsDir).catch(() => []))
      .filter((f) => f.endsWith(".pipe"))
      .map((f) => path.join(modelsDir, f));

    for (const mf of allModelFiles) {
      const src = await readIfExists(mf);
      for (const t of parsePipeModel(src)) {
        ctx.types[t.name] = t;
      }
    }

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
  const mapMatch = text.match(/^(\s*)([A-Za-z_][A-Za-z0-9_-]*)\s*:\s*(.*)$/);
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

  const listKeyMatch = text.match(/^(\s*)-\s+([A-Za-z_][A-Za-z0-9_-]*)\s*:\s*(.*)$/);
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
      inSteps: parents.includes("steps"),
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
      } else if (parents.includes("steps")) {
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
    const exportName = exportedVarName(name);
    const detail = field?.doc ? ` - ${field.doc}` : "";
    md.appendMarkdown(`\n- \`${exportName}\`${detail}`);
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

function modelFieldInfo(modelCtx, fieldName) {
  if (!fieldName) return null;
  const iface = modelCtx.entryInterface ? modelCtx.types[modelCtx.entryInterface] : undefined;
  const impl = modelCtx.entryClass ? modelCtx.types[modelCtx.entryClass] : undefined;
  let sourceName = fieldName;
  let ifaceField = iface?.fields[fieldName];
  if (!ifaceField && iface) {
    for (const name of Object.keys(iface.fields)) {
      if (exportedVarName(name) === fieldName) {
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
    exportedName: exportedVarName(sourceName),
    type: ifaceField?.type || implField?.type || "string",
    doc: ifaceField?.doc || implField?.doc,
    defaultValue: implField?.defaultValue
  };
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
                  const exportName = exportedVarName(fieldName);
                  const it = new vscode.CompletionItem(exportName, vscode.CompletionItemKind.Variable);
                  it.insertText = `${exportName}: `;
                  it.detail = `${fi.type || "string"} (from ${iface.name}.${fieldName})`;
                  if (fi.doc) it.documentation = fi.doc;
                  const implDefault = modelCtx.entryClass ? modelCtx.types[modelCtx.entryClass]?.fields[fieldName]?.defaultValue : undefined;
                  if (implDefault) {
                    it.documentation = new vscode.MarkdownString(`${fi.doc || ""}\n\nDefault: \`${implDefault}\``);
                  }
                  items.push(it);
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
            return null;
          }

          if (!isDockpipeWorkflowFile(document)) return null;
          const target = hoverTargetAt(document, position);
          if (!target) return null;
          const { info, range, word } = target;
          if (!word || !range) return null;

          const modelCtx = await buildModelContext(document);

          if (info?.kind === "topLevelKey") {
            if (word === "vars") {
              return hoverForVarsContainer(range, modelCtx, TOP_LEVEL_KEY_DETAILS.vars);
            }
            return hoverForWorkflowKey(word, TOP_LEVEL_KEY_DETAILS, range);
          }

          if (info?.kind === "stepKey") {
            if (word === "vars") {
              return hoverForVarsContainer(range, modelCtx, STEP_KEY_DETAILS.vars);
            }
            return hoverForWorkflowKey(word, STEP_KEY_DETAILS, range);
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
          if (!varsInfo) return null;

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
