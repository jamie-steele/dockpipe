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

function findVarsBlockInfo(document, position) {
  let varsLine = -1;
  let varsIndent = -1;
  for (let i = position.line; i >= 0; i--) {
    const line = document.lineAt(i).text;
    if (!line.trim()) continue;
    const m = line.match(/^(\s*)vars:\s*$/);
    if (m) {
      varsLine = i;
      varsIndent = m[1].length;
      break;
    }
    const top = line.match(/^(\s*)([A-Za-z_][A-Za-z0-9_]*):\s*$/);
    if (top && top[1].length === 0 && i < position.line) break;
  }
  if (varsLine < 0) return null;
  const curr = document.lineAt(position.line).text;
  const currIndent = (curr.match(/^\s*/) || [""])[0].length;
  if (position.line <= varsLine || currIndent <= varsIndent) return null;
  return { varsLine, varsIndent };
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
    vscode.languages.registerCompletionItemProvider(
      { language: "yaml", scheme: "file" },
      {
        async provideCompletionItems(document, position) {
          if (!isDockpipeWorkflowFile(document)) {
            return [];
          }
          const modelCtx = await buildModelContext(document);
          const line = document.lineAt(position.line).text;
          const leading = line.match(/^\s*/)?.[0] || "";
          const lineToCursor = line.slice(0, position.character);
          const insideSteps = document.getText(new vscode.Range(new vscode.Position(0, 0), position)).includes("\nsteps:");
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
            if (insideSteps && (leading.length === 2 || leading.length === 4 || lineToCursor.trimStart().startsWith("- "))) {
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
                  if (implDefault) it.documentation = new vscode.MarkdownString(`${fi.doc || ""}\n\nDefault: \`${implDefault}\``);
                  items.push(it);
                }
                return items;
              }
            } else {
              const key = before.slice(0, colonIdx).trim().replace(/^- /, "");
              const implDefault = modelCtx.entryClass ? modelCtx.types[modelCtx.entryClass]?.fields[key]?.defaultValue : undefined;
              if (implDefault) {
                const it = new vscode.CompletionItem(implDefault, vscode.CompletionItemKind.Value);
                it.insertText = `'${implDefault}'`;
                it.detail = `${key} default from ${modelCtx.entryClass}`;
                items.push(it);
              }
              const keyN = key.toLowerCase().replace(/[^a-z0-9]/g, "");
              for (const kv of modelCtx.knownValues) {
                const valueN = kv.name.toLowerCase().replace(/[^a-z0-9]/g, "");
                const score = keyN && valueN.includes(keyN) ? "0" : "1";
                const it = new vscode.CompletionItem(kv.value, vscode.CompletionItemKind.EnumMember);
                it.insertText = `'${kv.value}'`;
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
          const varsInfo = findVarsBlockInfo(document, position);
          if (!varsInfo) return null;
          const range = wordRange(document, position);
          if (!range) return null;
          const word = document.getText(range);
          if (!word) return null;
          const modelCtx = await buildModelContext(document);
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
