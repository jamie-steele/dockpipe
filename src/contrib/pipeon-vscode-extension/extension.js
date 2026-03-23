/**
 * Pipeon — minimal VS Code extension (install into a Code OSS fork or stock VS Code).
 * @see src/pipeon/docs/pipeon-vscode-fork.md
 */
// @ts-check
const vscode = require("vscode");
const fs = require("fs/promises");
const path = require("path");

/** @param {vscode.ExtensionContext} context */
function activate(context) {
  const channel = vscode.window.createOutputChannel("Pipeon", { log: true });

  context.subscriptions.push(
    vscode.commands.registerCommand("pipeon.openContextBundle", async () => {
      const root = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
      if (!root) {
        vscode.window.showWarningMessage("Pipeon: open a workspace folder first.");
        return;
      }
      const ctxPath = path.join(root, ".dockpipe", "pipeon-context.md");
      try {
        const text = await fs.readFile(ctxPath, "utf8");
        channel.clear();
        channel.appendLine(text);
        channel.show(true);
      } catch {
        vscode.window.showInformationMessage(
          "Pipeon: no .dockpipe/pipeon-context.md — run `src/bin/pipeon bundle` (see src/pipeon/scripts/README.md) or generate artifacts first."
        );
      }
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("pipeon.showReadme", async () => {
      const root = vscode.workspace.workspaceFolders?.[0]?.uri;
      if (!root) {
        vscode.window.showWarningMessage("Pipeon: open the dockpipe repository as a workspace folder first.");
        return;
      }
      const doc = vscode.Uri.joinPath(root, "src", "pipeon", "docs", "pipeon-vscode-fork.md");
      try {
        await vscode.workspace.fs.stat(doc);
        const docShow = await vscode.workspace.openTextDocument(doc);
        await vscode.window.showTextDocument(docShow, { preview: true });
      } catch {
        vscode.window.showInformationMessage(
          "Pipeon: src/pipeon/docs/pipeon-vscode-fork.md not found in this workspace — open the dockpipe repo root."
        );
      }
    })
  );
}

function deactivate() {}

module.exports = { activate, deactivate };
