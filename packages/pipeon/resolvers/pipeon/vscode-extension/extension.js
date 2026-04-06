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
const WELCOME_PANEL_ID = "pipeon.welcome";
const PANEL_BOTTOM_MIGRATION_KEY = "pipeon.panelBottomMigrated.v1";

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

function getWorkspaceRoot() {
  return vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || null;
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

async function executeDorkpipeRequest(root, text, options = {}) {
  const host = process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST;
  const model = process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL;
  const onToken = typeof options.onToken === "function" ? options.onToken : null;
  const local = await handleLocalCommand(root, text);
  if (local) {
    return {
      kind: "local",
      text: local.text,
      host,
      model,
      status: "Local DorkPipe command executed",
      contextPath: null,
    };
  }

  const { text: contextText, path: contextPath } = await readContextBundle(root);
  let answer = "";
  if (onToken) {
    answer = await ollamaChatStream({
      host,
      model,
      system: systemPrompt(contextText),
      user: text,
      onToken,
    });
  } else {
    answer = await ollamaChat({
      host,
      model,
      system: systemPrompt(contextText),
      user: text,
    });
  }

  return {
    kind: "model",
    text: answer || "(No response text returned.)",
    host,
    model,
    contextPath,
    status: contextPath
      ? `Model: ${model}  |  Ollama: ${host}  |  Context: ${path.relative(root, contextPath)}`
      : `Model: ${model}  |  Ollama: ${host}  |  No context bundle found`,
  };
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
  const initialState = JSON.stringify(state).replace(/</g, "\\u003c");
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
        <div class="sub">Ask about this workspace, codebase, or local environment. DorkPipe will use the local context bundle when available.</div>
      </header>
      <main class="transcript" id="transcript"></main>
      <div class="composerWrap">
        <div class="status" id="status"></div>
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
      const initialState = ${initialState};
      const form = document.getElementById("composer");
      const prompt = document.getElementById("prompt");
      const send = document.getElementById("send");
      const transcript = document.getElementById("transcript");
      const status = document.getElementById("status");
      let viewState = vscode.getState() || { draft: "", pinnedToBottom: true };
      let currentState = initialState;

      function escapeHtml(text) {
        return String(text)
          .replace(/&/g, "&amp;")
          .replace(/</g, "&lt;")
          .replace(/>/g, "&gt;");
      }

      function renderMessages(messages) {
        if (!messages.length) {
          return '<article class="msg assistant"><div class="role">DorkPipe</div><div class="body">Ask about this workspace. DorkPipe will use the local context bundle when available.</div></article>';
        }
        return messages.map((message) => {
          const role = message.role === "assistant" ? "DorkPipe" : "You";
          return '<article class="msg ' + message.role + '"><div class="role">' + role + '</div><div class="body">' + escapeHtml(message.text) + '</div></article>';
        }).join("");
      }

      function saveViewState(extra) {
        viewState = { ...viewState, ...extra };
        vscode.setState(viewState);
      }

      function render(nextState) {
        currentState = nextState;
        const previousBottomOffset = transcript.scrollHeight - transcript.scrollTop;
        const stickToBottom = previousBottomOffset - transcript.clientHeight <= 24 || !!viewState.pinnedToBottom;
        transcript.innerHTML = renderMessages(currentState.messages || []);
        status.textContent = currentState.status || "";
        send.disabled = !!currentState.isBusy;
        if (stickToBottom) {
          transcript.scrollTop = transcript.scrollHeight;
        } else {
          transcript.scrollTop = Math.max(0, transcript.scrollHeight - previousBottomOffset);
        }
        saveViewState({ pinnedToBottom: stickToBottom });
      }

      transcript.addEventListener("scroll", () => {
        const bottomGap = transcript.scrollHeight - transcript.scrollTop - transcript.clientHeight;
        saveViewState({ pinnedToBottom: bottomGap <= 24 });
      });

      prompt.value = viewState.draft || "";
      prompt.addEventListener("input", () => {
        saveViewState({ draft: prompt.value });
      });

      form.addEventListener("submit", (event) => {
        event.preventDefault();
        const text = prompt.value.trim();
        if (!text) return;
        saveViewState({ draft: prompt.value, pinnedToBottom: true });
        vscode.postMessage({ type: "ask", text });
      });
      window.addEventListener("message", (event) => {
        const msg = event.data || {};
        if (msg.type === "state" && msg.state) {
          render(msg.state);
        }
        if (msg.type === "done") {
          prompt.value = "";
          saveViewState({ draft: "", pinnedToBottom: true });
          send.disabled = false;
          prompt.focus();
        }
      });
      render(currentState);
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

function renderWelcomeHtml(webview, extensionUri) {
  const nonce = String(Date.now());
  const iconUri = webview.asWebviewUri(vscode.Uri.joinPath(extensionUri, "images", "icon.png"));
  return `<!doctype html>
<html>
  <head>
    <meta charset="UTF-8" />
    <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${webview.cspSource} https: data:; style-src 'unsafe-inline'; script-src 'nonce-${nonce}';" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <style>
      body {
        margin: 0;
        font-family: var(--vscode-font-family);
        color: var(--vscode-editor-foreground);
        background:
          radial-gradient(circle at top left, color-mix(in srgb, var(--vscode-button-background) 22%, transparent), transparent 45%),
          linear-gradient(180deg, color-mix(in srgb, var(--vscode-sideBar-background) 94%, transparent), var(--vscode-editor-background));
      }
      .wrap {
        min-height: 100vh;
        display: grid;
        grid-template-columns: minmax(320px, 560px) 1fr;
        gap: 32px;
        align-items: center;
        padding: 48px;
      }
      .copy {
        max-width: 520px;
      }
      .eyebrow {
        text-transform: uppercase;
        letter-spacing: 0.14em;
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
        margin-bottom: 12px;
      }
      h1 {
        margin: 0 0 16px;
        font-size: 42px;
        line-height: 1.05;
      }
      p {
        margin: 0 0 16px;
        font-size: 17px;
        line-height: 1.6;
        color: var(--vscode-descriptionForeground);
      }
      .actions {
        display: flex;
        gap: 12px;
        flex-wrap: wrap;
        margin-top: 20px;
      }
      button {
        border: 0;
        border-radius: 12px;
        padding: 12px 18px;
        background: var(--vscode-button-background);
        color: var(--vscode-button-foreground);
        font: inherit;
        font-weight: 600;
        cursor: pointer;
      }
      button.secondary {
        background: color-mix(in srgb, var(--vscode-button-background) 15%, transparent);
        color: var(--vscode-editor-foreground);
        border: 1px solid var(--vscode-panel-border);
      }
      .hero {
        display: flex;
        justify-content: center;
      }
      .card {
        width: min(100%, 560px);
        aspect-ratio: 1.2 / 1;
        border-radius: 28px;
        border: 1px solid color-mix(in srgb, var(--vscode-button-background) 22%, var(--vscode-panel-border));
        background:
          radial-gradient(circle at 30% 25%, color-mix(in srgb, var(--vscode-button-background) 24%, transparent), transparent 30%),
          linear-gradient(180deg, color-mix(in srgb, var(--vscode-editorWidget-background) 96%, transparent), color-mix(in srgb, var(--vscode-editorWidget-background) 88%, transparent));
        box-shadow: 0 24px 60px rgba(0,0,0,0.22);
        display: grid;
        place-items: center;
      }
      .card img {
        width: min(62%, 320px);
        height: auto;
        filter: drop-shadow(0 18px 30px rgba(0,0,0,0.35));
      }
      @media (max-width: 960px) {
        .wrap {
          grid-template-columns: 1fr;
          padding: 28px;
        }
        h1 { font-size: 34px; }
      }
    </style>
  </head>
  <body>
    <div class="wrap">
      <section class="copy">
        <div class="eyebrow">Pipeon</div>
        <h1>Get Started with Pipeon</h1>
        <p>Local-first coding with a proper app shell, DorkPipe in-editor help, DockPipe workflows, and your repo context already wired in.</p>
        <p>Open chat, inspect the local bundle, or jump straight into the workspace without the stock web welcome getting in the way.</p>
        <div class="actions">
          <button data-command="chat">Open DorkPipe Chat</button>
          <button class="secondary" data-command="context">Open Context Bundle</button>
          <button class="secondary" data-command="docs">Open Docs</button>
        </div>
      </section>
      <section class="hero">
        <div class="card">
          <img src="${iconUri}" alt="Pipeon" />
        </div>
      </section>
    </div>
    <script nonce="${nonce}">
      const vscode = acquireVsCodeApi();
      for (const button of document.querySelectorAll('button[data-command]')) {
        button.addEventListener('click', () => {
          vscode.postMessage({ type: 'command', command: button.dataset.command });
        });
      }
    </script>
  </body>
</html>`;
}

function openPipeonWelcome(context) {
  const panel = vscode.window.createWebviewPanel(
    WELCOME_PANEL_ID,
    "Pipeon",
    vscode.ViewColumn.Active,
    {
      enableScripts: true,
      retainContextWhenHidden: true,
    }
  );
  panel.webview.html = renderWelcomeHtml(panel.webview, context.extensionUri);
  panel.webview.onDidReceiveMessage(async (msg) => {
    switch (msg?.command) {
      case "chat":
        await vscode.commands.executeCommand("pipeon.openChat");
        break;
      case "context":
        await vscode.commands.executeCommand("pipeon.openContextBundle");
        break;
      case "docs":
        await vscode.commands.executeCommand("pipeon.showReadme");
        break;
      default:
        break;
    }
  });
  return panel;
}

function looksLikeStockWelcomeTab(tab) {
  const label = String(tab?.label || "").toLowerCase();
  return label.includes("welcome") || label.includes("get started");
}

async function closeStockWelcomeTabs() {
  const groups = vscode.window.tabGroups?.all || [];
  const tabsToClose = [];

  for (const group of groups) {
    for (const tab of group.tabs) {
      if (looksLikeStockWelcomeTab(tab)) {
        tabsToClose.push(tab);
      }
    }
  }

  if (tabsToClose.length === 0) {
    return false;
  }

  try {
    await vscode.window.tabGroups.close(tabsToClose);
    return true;
  } catch {
    try {
      await vscode.commands.executeCommand("workbench.action.closeAllEditors");
      return true;
    } catch {
      return false;
    }
  }
}

async function replaceStockWelcomeWithPipeon(context) {
  const groups = vscode.window.tabGroups?.all || [];
  let foundWelcome = false;
  let foundNonWelcome = false;

  for (const group of groups) {
    for (const tab of group.tabs) {
      if (looksLikeStockWelcomeTab(tab)) {
        foundWelcome = true;
      } else {
        foundNonWelcome = true;
      }
    }
  }

  if (!foundWelcome || foundNonWelcome) {
    return;
  }

  await closeStockWelcomeTabs();
  openPipeonWelcome(context);
}

async function redirectStockWelcomeToPipeon(context) {
  const groups = vscode.window.tabGroups?.all || [];
  let foundWelcome = false;

  for (const group of groups) {
    for (const tab of group.tabs) {
      if (looksLikeStockWelcomeTab(tab)) {
        foundWelcome = true;
        break;
      }
    }
    if (foundWelcome) {
      break;
    }
  }

  if (!foundWelcome) {
    return;
  }

  const closed = await closeStockWelcomeTabs();
  if (closed) {
    openPipeonWelcome(context);
  }
}

async function revealDorkpipePanel() {
  await vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`);
}

async function resetPanelToBottomOnce(context) {
  if (context.globalState.get(PANEL_BOTTOM_MIGRATION_KEY)) {
    return;
  }

  await context.globalState.update(PANEL_BOTTOM_MIGRATION_KEY, true);

  try {
    await vscode.commands.executeCommand("workbench.action.positionPanelBottom");
  } catch {
    // Best-effort cleanup for the old broken layout migration.
  }
}

class PipeonChatViewProvider {
  constructor(channel) {
    this.channel = channel;
    this.view = null;
    this.rendered = false;
    this.state = {
      messages: [],
      status: "Waiting for workspace...",
      isBusy: false,
    };
  }

  focus() {
    vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`);
  }

  refresh() {
    if (this.view) {
      if (!this.rendered) {
        this.view.webview.html = renderChatHtml(this.view.webview, this.state);
        this.rendered = true;
      } else {
        this.view.webview.postMessage({ type: "state", state: this.state });
      }
    }
  }

  async ask(root, text) {
    this.state.messages.push({ role: "user", text });
    this.state.status = "Thinking...";
    this.state.isBusy = true;
    this.refresh();
    try {
      this.state.messages.push({ role: "assistant", text: "" });
      const assistantIndex = this.state.messages.length - 1;
      const result = await executeDorkpipeRequest(root, text, {
        onToken: (_piece, fullText) => {
          this.state.messages[assistantIndex].text = fullText;
          this.refresh();
        },
      });
      this.state.messages[assistantIndex].text = result.text;
      this.state.status = result.status;
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      this.state.messages.push({ role: "assistant", text: `DorkPipe error: ${message}` });
      this.state.status = `Error talking to Ollama`;
      this.channel.error(message);
    }
    this.state.isBusy = false;
    this.refresh();
    this.view?.webview.postMessage({ type: "done" });
  }

  resolveWebviewView(webviewView) {
    this.view = webviewView;
    this.rendered = false;
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
    webviewView.onDidDispose(() => {
      this.view = null;
      this.rendered = false;
    });
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
      await revealDorkpipePanel();
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("pipeon.openWelcome", async () => {
      openPipeonWelcome(context);
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

  context.subscriptions.push(
    vscode.window.tabGroups.onDidChangeTabs(() => {
      setTimeout(() => {
        void redirectStockWelcomeToPipeon(context);
      }, 50);
    })
  );

  setTimeout(() => {
    void replaceStockWelcomeWithPipeon(context);
    void resetPanelToBottomOnce(context);
  }, 400);
}

function deactivate() {}

module.exports = { activate, deactivate };
