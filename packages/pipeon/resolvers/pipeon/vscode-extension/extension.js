/**
 * DorkPipe — minimal VS Code extension (install into a Code OSS fork or stock VS Code).
 * @see pipeon resolver assets/docs/pipeon-vscode-fork.md (maintainer IDE pack)
 */
// @ts-check
const vscode = require("vscode");
const fs = require("fs/promises");
const path = require("path");
const http = require("http");
const https = require("https");
const cp = require("child_process");

const DEFAULT_OLLAMA_HOST = "http://127.0.0.1:11434";
const DEFAULT_MODEL = "llama3.2";
const CHAT_VIEW_ID = "pipeon.chatView";

async function ensureDir(dir) {
  await fs.mkdir(dir, { recursive: true });
}

function shellQuote(value) {
  return `'${String(value).replace(/'/g, `'\\''`)}'`;
}

async function resolveContextBundlePath(root) {
  const candidates = [
    path.join(root, "bin", ".dockpipe", "pipeon-context.md"),
    path.join(root, ".dockpipe", "pipeon-context.md"),
  ];
  for (const p of candidates) {
    try {
      await fs.stat(p);
      return p;
    } catch {
      // keep trying
    }
  }
  return null;
}

async function readContextBundle(root) {
  const ctxPath = await resolveContextBundlePath(root);
  if (!ctxPath) {
    return { text: "", path: null };
  }
  return { text: await fs.readFile(ctxPath, "utf8"), path: ctxPath };
}

async function writeTaskFile(root, kind, prompt) {
  const dir = path.join(root, ".dockpipe", "pipeon", "tasks");
  await ensureDir(dir);
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  const file = path.join(dir, `${kind}-${stamp}.md`);
  const body = [
    `# DorkPipe ${kind} task`,
    "",
    `Created: ${new Date().toISOString()}`,
    "",
    "## Request",
    "",
    prompt,
    "",
    "## Suggested flow",
    "",
    "1. Inspect the relevant files and current behavior.",
    "2. Make the smallest useful code change.",
    "3. Run the relevant validation workflow or tests.",
    "4. Summarize the diff, logs, and any follow-up risk.",
    "",
  ].join("\n");
  await fs.writeFile(file, body, "utf8");
  return file;
}

function systemPrompt(contextText) {
  return [
    "You are DorkPipe, a local-first repo-aware IDE assistant.",
    "Ground your answer in the provided repository context bundle when relevant.",
    "Be concise, practical, and explicit about uncertainty.",
    contextText ? `\nRepository context bundle:\n\n${contextText}` : "",
  ].join("\n");
}

function buildOllamaUrl(host) {
  return new URL("/api/chat", host.endsWith("/") ? host : `${host}/`);
}

function ollamaChat({ host, model, system, user }) {
  return new Promise((resolve, reject) => {
    let url;
    try {
      url = buildOllamaUrl(host);
    } catch (err) {
      reject(new Error(`Invalid OLLAMA_HOST: ${host}`));
      return;
    }
    const payload = JSON.stringify({
      model,
      stream: false,
      messages: [
        { role: "system", content: system },
        { role: "user", content: user },
      ],
    });
    const transport = url.protocol === "https:" ? https : http;
    const req = transport.request(
      url,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": Buffer.byteLength(payload),
        },
      },
      (res) => {
        let body = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => {
          body += chunk;
        });
        res.on("end", () => {
          if (res.statusCode && res.statusCode >= 400) {
            reject(new Error(`Ollama returned HTTP ${res.statusCode}: ${body}`));
            return;
          }
          try {
            const parsed = JSON.parse(body);
            resolve(parsed.message?.content || parsed.response || "");
          } catch (err) {
            reject(new Error(`Could not parse Ollama response: ${body}`));
          }
        });
      }
    );
    req.on("error", (err) => reject(err));
    req.write(payload);
    req.end();
  });
}

function ollamaChatStream({ host, model, system, user, onToken }) {
  return new Promise((resolve, reject) => {
    let url;
    try {
      url = buildOllamaUrl(host);
    } catch (err) {
      reject(new Error(`Invalid OLLAMA_HOST: ${host}`));
      return;
    }
    const payload = JSON.stringify({
      model,
      stream: true,
      messages: [
        { role: "system", content: system },
        { role: "user", content: user },
      ],
    });
    const transport = url.protocol === "https:" ? https : http;
    let buffer = "";
    let fullText = "";
    const req = transport.request(
      url,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": Buffer.byteLength(payload),
        },
      },
      (res) => {
        if (res.statusCode && res.statusCode >= 400) {
          let body = "";
          res.setEncoding("utf8");
          res.on("data", (chunk) => {
            body += chunk;
          });
          res.on("end", () => reject(new Error(`Ollama returned HTTP ${res.statusCode}: ${body}`)));
          return;
        }
        res.setEncoding("utf8");
        res.on("data", (chunk) => {
          buffer += chunk;
          const lines = buffer.split("\n");
          buffer = lines.pop() || "";
          for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed) continue;
            try {
              const parsed = JSON.parse(trimmed);
              const piece = parsed.message?.content || parsed.response || "";
              if (piece) {
                fullText += piece;
                onToken(piece, fullText);
              }
              if (parsed.done) {
                resolve(fullText);
                return;
              }
            } catch {
              // ignore partial/invalid line noise
            }
          }
        });
        res.on("end", () => resolve(fullText));
      }
    );
    req.on("error", (err) => reject(err));
    req.write(payload);
    req.end();
  });
}

function runCommand(command, cwd, extraEnv = {}) {
  return new Promise((resolve) => {
    cp.exec(command, { cwd, env: { ...process.env, ...extraEnv }, maxBuffer: 1024 * 1024 }, (error, stdout, stderr) => {
      resolve({
        ok: !error,
        code: error && typeof error.code === "number" ? error.code : 0,
        stdout: stdout || "",
        stderr: stderr || "",
      });
    });
  });
}

function launchTerminalCommand(root, name, command, extraEnv = {}) {
  const terminal = vscode.window.createTerminal({
    name,
    cwd: root,
    env: {
      DOCKPIPE_PIPEON: process.env.DOCKPIPE_PIPEON || "1",
      DOCKPIPE_PIPEON_ALLOW_PRERELEASE: process.env.DOCKPIPE_PIPEON_ALLOW_PRERELEASE || "1",
      ...extraEnv,
    },
  });
  terminal.show(true);
  terminal.sendText(command, true);
}

async function handleLocalCommand(root, rawText) {
  const text = rawText.trim();
  if (!text.startsWith("/")) {
    return null;
  }
  const parts = text.split(/\s+/);
  const cmd = parts[0].toLowerCase();
  const args = parts.slice(1);
  switch (cmd) {
    case "/help":
      return {
        mode: "local",
        text: [
          "DorkPipe local commands:",
          "/help",
          "/status",
          "/bundle",
          "/context",
          "/test",
          "/ci",
          "/validate [path]",
          "/plan <task>",
          "/edit <task>",
          "/workflow <name>",
          "/workflow-validate <path-to-config.yml>",
        ].join("\n"),
      };
    case "/status": {
      const result = await runCommand("./packages/pipeon/resolvers/pipeon/bin/pipeon status", root, {
        DOCKPIPE_WORKDIR: root,
        DOCKPIPE_PIPEON: process.env.DOCKPIPE_PIPEON || "1",
        DOCKPIPE_PIPEON_ALLOW_PRERELEASE: process.env.DOCKPIPE_PIPEON_ALLOW_PRERELEASE || "1",
      });
      return {
        mode: "local",
        text: (result.stdout || result.stderr || "No output").trim(),
      };
    }
    case "/bundle": {
      const result = await runCommand("./packages/pipeon/resolvers/pipeon/bin/pipeon bundle", root, {
        DOCKPIPE_WORKDIR: root,
        DOCKPIPE_PIPEON: process.env.DOCKPIPE_PIPEON || "1",
        DOCKPIPE_PIPEON_ALLOW_PRERELEASE: process.env.DOCKPIPE_PIPEON_ALLOW_PRERELEASE || "1",
      });
      return {
        mode: "local",
        text: (result.stdout || result.stderr || "No output").trim(),
      };
    }
    case "/context": {
      const ctxPath = await resolveContextBundlePath(root);
      return {
        mode: "local",
        text: ctxPath ? `Context bundle: ${path.relative(root, ctxPath)}` : "No DorkPipe context bundle found yet. Run /bundle first.",
      };
    }
    case "/workflow": {
      if (args.length === 0) {
        return { mode: "local", text: "Usage: /workflow <name>" };
      }
      const workflow = args[0];
      launchTerminalCommand(root, `DorkPipe workflow: ${workflow}`, `./src/bin/dockpipe --workflow ${workflow} --workdir . --`);
      return {
        mode: "local",
        text: `Started workflow \`${workflow}\` in a terminal.`,
      };
    }
    case "/test":
      launchTerminalCommand(root, "DorkPipe test", "./src/bin/dockpipe --workflow test --workdir . --");
      return { mode: "local", text: "Started `test` workflow in a terminal." };
    case "/ci":
      launchTerminalCommand(root, "DorkPipe ci-emulate", "./src/bin/dockpipe --workflow ci-emulate --workdir . --");
      return { mode: "local", text: "Started `ci-emulate` workflow in a terminal." };
    case "/validate": {
      if (args.length > 0) {
        const target = args.join(" ");
        const result = await runCommand(`./src/bin/dockpipe workflow validate ${target}`, root);
        return { mode: "local", text: (result.stdout || result.stderr || "No output").trim() };
      }
      launchTerminalCommand(root, "DorkPipe validate", "./src/bin/dockpipe --workflow test --workdir . --");
      return { mode: "local", text: "Started validation via the `test` workflow in a terminal." };
    }
    case "/plan":
    case "/edit": {
      if (args.length === 0) {
        return { mode: "local", text: `Usage: ${cmd} <task description>` };
      }
      const task = args.join(" ");
      const file = await writeTaskFile(root, cmd.slice(1), task);
      const doc = await vscode.workspace.openTextDocument(file);
      await vscode.window.showTextDocument(doc, { preview: false });
      return {
        mode: "local",
        text: `Created ${cmd.slice(1)} task scaffold at \`${path.relative(root, file)}\`.\nNext: run /test, /ci, or /workflow <name> after you wire the execution step you want.`,
      };
    }
    case "/workflow-validate": {
      if (args.length === 0) {
        return { mode: "local", text: "Usage: /workflow-validate <path-to-config.yml>" };
      }
      const result = await runCommand(`./src/bin/dockpipe workflow validate ${args.join(" ")}`, root);
      return {
        mode: "local",
        text: (result.stdout || result.stderr || "No output").trim(),
      };
    }
    default:
      return {
        mode: "local",
        text: `Unknown DorkPipe local command: ${cmd}\nTry /help`,
      };
  }
}

function renderChatHtml(webview, state) {
  const nonce = String(Date.now());
  const transcript = state.messages
    .map((m) => {
      const role = m.role === "assistant" ? "DorkPipe" : "You";
      return `<article class="msg ${m.role}"><div class="role">${role}</div><div class="body">${escapeHtml(m.text)}</div></article>`;
    })
    .join("");
  return `<!doctype html>
<html>
  <head>
    <meta charset="UTF-8" />
    <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-${nonce}';" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <style>
      :root {
        color-scheme: dark light;
      }
      * { box-sizing: border-box; }
      body {
        font-family: var(--vscode-font-family);
        margin: 0;
        background:
          radial-gradient(circle at top, color-mix(in srgb, var(--vscode-button-background) 10%, transparent) 0%, transparent 40%),
          var(--vscode-editor-background);
        color: var(--vscode-editor-foreground);
      }
      .wrap {
        display: grid;
        grid-template-rows: auto 1fr auto;
        height: 100vh;
      }
      .header {
        padding: 14px 16px 12px;
        border-bottom: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-sideBar-background) 92%, transparent);
        backdrop-filter: blur(10px);
      }
      .eyebrow {
        font-size: 11px;
        letter-spacing: 0.12em;
        text-transform: uppercase;
        color: var(--vscode-descriptionForeground);
        margin-bottom: 6px;
      }
      .title {
        font-size: 18px;
        font-weight: 700;
        margin: 0 0 4px;
      }
      .sub {
        font-size: 12px;
        color: var(--vscode-descriptionForeground);
        line-height: 1.45;
      }
      .transcript {
        padding: 18px 16px 24px;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 14px;
      }
      .quick {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
        padding: 0 16px 10px;
      }
      .quick button {
        min-width: 0;
        padding: 8px 12px;
        border-radius: 999px;
        background: color-mix(in srgb, var(--vscode-button-background) 14%, transparent);
        color: var(--vscode-editor-foreground);
        border: 1px solid var(--vscode-panel-border);
      }
      .msg {
        max-width: 100%;
        padding: 12px 14px;
        border-radius: 16px;
        border: 1px solid var(--vscode-panel-border);
        box-shadow: 0 10px 24px rgba(0,0,0,0.08);
      }
      .msg.user {
        background: linear-gradient(180deg, color-mix(in srgb, var(--vscode-button-background) 20%, transparent), color-mix(in srgb, var(--vscode-button-background) 9%, transparent));
        border-bottom-right-radius: 6px;
      }
      .msg.assistant {
        background: linear-gradient(180deg, color-mix(in srgb, var(--vscode-editorWidget-background) 94%, transparent), color-mix(in srgb, var(--vscode-editorWidget-background) 80%, transparent));
        border-bottom-left-radius: 6px;
      }
      .role {
        font-size: 11px;
        opacity: 0.82;
        margin-bottom: 8px;
        text-transform: uppercase;
        letter-spacing: 0.08em;
      }
      .body {
        white-space: pre-wrap;
        line-height: 1.55;
        font-size: 13px;
      }
      .composerWrap {
        border-top: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-sideBar-background) 92%, transparent);
        padding: 12px 14px 14px;
      }
      .status {
        padding: 0 2px 10px;
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
      }
      .composer {
        display: grid;
        grid-template-columns: 1fr;
        gap: 10px;
      }
      textarea {
        resize: vertical;
        min-height: 108px;
        max-height: 240px;
        width: 100%;
        background: var(--vscode-input-background);
        color: var(--vscode-input-foreground);
        border: 1px solid var(--vscode-input-border);
        border-radius: 14px;
        padding: 12px 14px;
        font: inherit;
        line-height: 1.45;
      }
      .actions {
        display: flex;
        justify-content: flex-end;
      }
      button {
        border: 0;
        border-radius: 12px;
        padding: 10px 16px;
        min-width: 112px;
        background: var(--vscode-button-background);
        color: var(--vscode-button-foreground);
        font: inherit;
        font-weight: 600;
        cursor: pointer;
      }
      button:disabled { opacity: 0.6; cursor: default; }
      @media (max-width: 420px) {
        .header { padding: 12px; }
        .transcript { padding: 14px 12px 20px; }
        .composerWrap { padding: 10px 12px 12px; }
      }
    </style>
  </head>
  <body>
    <div class="wrap">
      <header class="header">
        <div class="eyebrow">DorkPipe</div>
        <h1 class="title">Workspace Chat</h1>
        <div class="sub">Ask about this repo, local signals, workflows, and environment. DorkPipe will ground answers in the context bundle when available. You can also use local commands like <code>/test</code>, <code>/bundle</code>, or <code>/edit fix the README intro</code>.</div>
      </header>
      <div class="quick">
        <button type="button" data-command="/bundle">Bundle</button>
        <button type="button" data-command="/context">Context</button>
        <button type="button" data-command="/test">Test</button>
        <button type="button" data-command="/ci">CI</button>
      </div>
      <main class="transcript" id="transcript">${transcript || `<article class="msg assistant"><div class="role">DorkPipe</div><div class="body">Ask about this workspace. DorkPipe will use the local context bundle when available.</div></article>`}</main>
      <div class="composerWrap">
        <div class="status">${escapeHtml(state.status)}</div>
        <form class="composer" id="composer">
          <textarea id="prompt" placeholder="Ask DorkPipe about this repo... or use /help for local commands"></textarea>
          <div class="actions">
            <button id="send" type="submit">Send</button>
          </div>
        </form>
      </div>
    </div>
    <script nonce="${nonce}">
      const vscode = acquireVsCodeApi();
      const form = document.getElementById("composer");
      const prompt = document.getElementById("prompt");
      const send = document.getElementById("send");
      for (const button of document.querySelectorAll(".quick button")) {
        button.addEventListener("click", () => {
          send.disabled = true;
          vscode.postMessage({ type: "ask", text: button.dataset.command });
        });
      }
      form.addEventListener("submit", (event) => {
        event.preventDefault();
        const text = prompt.value.trim();
        if (!text) return;
        send.disabled = true;
        vscode.postMessage({ type: "ask", text });
      });
      window.addEventListener("message", (event) => {
        const msg = event.data || {};
        if (msg.type === "done") {
          prompt.value = "";
          send.disabled = false;
          prompt.focus();
        }
      });
      prompt.focus();
    </script>
  </body>
</html>`;
}

function escapeHtml(text) {
  return String(text)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

class PipeonChatViewProvider {
  constructor(channel) {
    this.channel = channel;
    this.view = null;
    this.state = {
      messages: [],
      status: "Waiting for workspace...",
    };
  }

  focus() {
    vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`);
  }

  refresh() {
    if (this.view) {
      this.view.webview.html = renderChatHtml(this.view.webview, this.state);
    }
  }

  async ask(root, text) {
    const host = process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST;
    const model = process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL;
    this.state.messages.push({ role: "user", text });
    this.state.status = `Thinking with ${model}...`;
    this.refresh();
    try {
      const local = await handleLocalCommand(root, text);
      if (local) {
        this.state.messages.push({ role: "assistant", text: local.text });
        this.state.status = "Local DorkPipe command executed";
        this.refresh();
        this.view?.webview.postMessage({ type: "done" });
        return;
      }
      const { text: contextText, path: contextPath } = await readContextBundle(root);
      this.state.messages.push({ role: "assistant", text: "" });
      const assistantIndex = this.state.messages.length - 1;
      const answer = await ollamaChatStream({
        host,
        model,
        system: systemPrompt(contextText),
        user: text,
        onToken: (_piece, fullText) => {
          this.state.messages[assistantIndex].text = fullText;
          this.refresh();
        },
      });
      this.state.messages[assistantIndex].text = answer || "(No response text returned.)";
      this.state.status = contextPath
        ? `Model: ${model}  |  Ollama: ${host}  |  Context: ${path.relative(root, contextPath)}`
        : `Model: ${model}  |  Ollama: ${host}  |  No context bundle found`;
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      this.state.messages.push({ role: "assistant", text: `DorkPipe error: ${message}` });
      this.state.status = `Error talking to Ollama`;
      this.channel.error(message);
    }
    this.refresh();
    this.view?.webview.postMessage({ type: "done" });
  }

  resolveWebviewView(webviewView) {
    this.view = webviewView;
    webviewView.webview.options = { enableScripts: true };
    const root = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
    if (root) {
      const host = process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST;
      const model = process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL;
      this.state.status = `Model: ${model}  |  Ollama: ${host}`;
    } else {
      this.state.status = "Open a workspace folder to chat with DorkPipe.";
    }
    this.refresh();
    webviewView.webview.onDidReceiveMessage(async (msg) => {
      const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
      if (!workspaceRoot) {
        vscode.window.showWarningMessage("DorkPipe: open a workspace folder first.");
        return;
      }
      if (msg?.type === "ask" && msg.text) {
        await this.ask(workspaceRoot, msg.text);
      }
    });
  }
}

/** @param {vscode.ExtensionContext} context */
function activate(context) {
  const channel = vscode.window.createOutputChannel("DorkPipe", { log: true });
  const chatProvider = new PipeonChatViewProvider(channel);

  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(CHAT_VIEW_ID, chatProvider, {
      webviewOptions: { retainContextWhenHidden: true },
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("pipeon.openChat", async () => {
      await vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`);
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("pipeon.openContextBundle", async () => {
      const root = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
      if (!root) {
        vscode.window.showWarningMessage("DorkPipe: open a workspace folder first.");
        return;
      }
      try {
        const ctxPath = await resolveContextBundlePath(root);
        if (!ctxPath) {
          throw new Error("missing");
        }
        const text = await fs.readFile(ctxPath, "utf8");
        channel.clear();
        channel.appendLine(text);
        channel.show(true);
      } catch {
        vscode.window.showInformationMessage("DorkPipe: no pipeon-context bundle found — run `pipeon bundle` first.");
      }
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("pipeon.showReadme", async () => {
      const root = vscode.workspace.workspaceFolders?.[0]?.uri;
      if (!root) {
        vscode.window.showWarningMessage("DorkPipe: open the dockpipe repository as a workspace folder first.");
        return;
      }
      const doc = vscode.Uri.joinPath(
        root,
        "packages",
        "pipeon",
        "resolvers",
        "pipeon",
        "assets",
        "docs",
        "pipeon-vscode-fork.md"
      );
      try {
        await vscode.workspace.fs.stat(doc);
        const docShow = await vscode.workspace.openTextDocument(doc);
        await vscode.window.showTextDocument(docShow, { preview: true });
      } catch {
        vscode.window.showInformationMessage(
          "DorkPipe: fork doc not found in this workspace — open the dockpipe repo root."
        );
      }
    })
  );
}

function deactivate() {}

module.exports = { activate, deactivate };
