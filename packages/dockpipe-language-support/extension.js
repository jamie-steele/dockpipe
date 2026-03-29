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
  "enumMember"
]);

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
    if (depth <= 0) {
      current = null;
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
  return document.getWordRangeAtPosition(position, /[A-Za-z0-9_.-]+/);
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

function hoverForWorkflowKey(word, docs, range) {
  const doc = docs[word];
  if (!doc) return null;
  const md = new vscode.MarkdownString();
  md.appendMarkdown(`**${word}**`);
  md.appendMarkdown(`\n\n${doc}`);
  return new vscode.Hover(md, range);
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
          if (!isDockpipeWorkflowFile(document)) {
            return null;
          }
          const modelCtx = await buildModelContext(document);
          const infos = analyzeYamlStructure(document);
          const builder = new vscode.SemanticTokensBuilder(SEMANTIC_LEGEND);

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
              const implDefault = modelCtx.entryClass ? modelCtx.types[modelCtx.entryClass]?.fields[key]?.defaultValue : undefined;
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
                  const it = new vscode.CompletionItem(fieldName, vscode.CompletionItemKind.Variable);
                  it.insertText = `${fieldName}: `;
                  it.detail = `${fi.type || "string"} (from ${iface.name})`;
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
              const implDefault = modelCtx.entryClass ? modelCtx.types[modelCtx.entryClass]?.fields[key]?.defaultValue : undefined;
              if (implDefault) {
                const it = new vscode.CompletionItem(implDefault, vscode.CompletionItemKind.Value);
                it.insertText = quoted(implDefault);
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
          if (!isDockpipeWorkflowFile(document)) return null;
          const info = structureInfoAt(document, position);
          const range = wordRange(document, position);
          if (!range) return null;
          const word = document.getText(range);
          if (!word) return null;

          if (info?.kind === "topLevelKey") {
            return hoverForWorkflowKey(word, TOP_LEVEL_KEY_DETAILS, range);
          }

          if (info?.kind === "stepKey") {
            return hoverForWorkflowKey(word, STEP_KEY_DETAILS, range);
          }

          const modelCtx = await buildModelContext(document);

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
              return new vscode.Hover(md, range);
            }
          }

          const varsInfo = findVarsBlockInfo(document, position);
          if (!varsInfo) return null;

          const iface = modelCtx.entryInterface ? modelCtx.types[modelCtx.entryInterface] : undefined;
          const impl = modelCtx.entryClass ? modelCtx.types[modelCtx.entryClass] : undefined;
          const fi = iface?.fields[word];
          if (fi) {
            const md = new vscode.MarkdownString();
            md.appendMarkdown(`**${word}**`);
            md.appendMarkdown(`\n\nType: \`${fi.type || "string"}\``);
            if (fi.doc) md.appendMarkdown(`\n\n${fi.doc}`);
            const def = impl?.fields[word]?.defaultValue;
            if (def) md.appendMarkdown(`\n\nDefault: \`${def}\``);
            return new vscode.Hover(md, range);
          }

          const kv = modelCtx.knownValues.find((k) => k.value === word || k.name.endsWith("." + word));
          if (kv) {
            const md = new vscode.MarkdownString();
            md.appendMarkdown(`**${kv.name}**`);
            md.appendMarkdown(`\n\nValue: \`${kv.value}\``);
            if (kv.doc) md.appendMarkdown(`\n\n${kv.doc}`);
            return new vscode.Hover(md, range);
          }

          const lineKey = info?.kind === "varKey" ? info.key : null;
          if (lineKey) {
            const field = iface?.fields[lineKey];
            if (field) {
              const md = new vscode.MarkdownString();
              md.appendMarkdown(`**${lineKey}**`);
              md.appendMarkdown(`\n\nType: \`${field.type || "string"}\``);
              if (field.doc) md.appendMarkdown(`\n\n${field.doc}`);
              const def = impl?.fields[lineKey]?.defaultValue;
              if (def) md.appendMarkdown(`\n\nModel default: \`${def}\``);
              md.appendMarkdown(`\n\nCurrent value: \`${word}\``);
              return new vscode.Hover(md, range);
            }
          }

          return null;
        }
      }
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
}

function deactivate() {}

module.exports = { activate, deactivate };
