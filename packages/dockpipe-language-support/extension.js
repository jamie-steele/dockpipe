// @ts-check
const vscode = require("vscode");
const YAML = require("yaml");

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
  const parsed = YAML.parseDocument(doc.getText(), { prettyErrors: false });
  const root = parsed.contents;
  if (!root || root.type !== "MAP" || !Array.isArray(root.items)) {
    return out;
  }
  for (const item of root.items) {
    const k = item?.key?.value;
    if (typeof k === "string") {
      out.add(k);
    }
  }
  return out;
}

/**
 * @param {vscode.TextDocument} doc
 * @returns {vscode.Diagnostic[]}
 */
function validateYaml(doc) {
  const parsed = YAML.parseDocument(doc.getText(), { prettyErrors: true });
  const diagnostics = [];
  for (const err of parsed.errors || []) {
    const pos = err.pos || [0, 1];
    const start = doc.positionAt(Math.max(0, pos[0] || 0));
    const end = doc.positionAt(Math.max(start.character + 1, pos[1] || (pos[0] || 0) + 1));
    diagnostics.push(new vscode.Diagnostic(new vscode.Range(start, end), String(err.message || "YAML parse error"), vscode.DiagnosticSeverity.Error));
  }
  return diagnostics;
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
        provideCompletionItems(document, position) {
          if (!isDockpipeWorkflowFile(document)) {
            return [];
          }
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
            const it = new vscode.CompletionItem("models/IR2InfraConfig", vscode.CompletionItemKind.Reference);
            it.detail = "Interface entrypoint (compiler infers implementing class)";
            items.push(it);
          }
          return items;
        }
      },
      ":",
      "-"
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
