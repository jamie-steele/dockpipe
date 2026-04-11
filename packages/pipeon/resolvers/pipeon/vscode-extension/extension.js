/**
 * DorkPipe — VS Code extension (install into a Code OSS fork or stock VS Code).
 * @see pipeon resolver assets/docs/pipeon-vscode-fork.md (maintainer IDE pack)
 */
// @ts-check
const vscode = require("vscode");
const fs = require("fs/promises");
const path = require("path");
const http = require("http");
const https = require("https");
const cp = require("child_process");
const os = require("os");

const DEFAULT_OLLAMA_HOST = "http://127.0.0.1:11434";
const DEFAULT_MODEL = "llama3.2";
const CHAT_VIEW_ID = "pipeon.chatView";
const WELCOME_PANEL_ID = "pipeon.welcome";
const PANEL_BOTTOM_MIGRATION_KEY = "pipeon.panelBottomMigrated.v1";
const CHAT_STATE_KEY = "pipeon.chatState.v2";
const MAX_SAVED_SESSIONS = 20;
const MAX_HISTORY_MESSAGES = 14;
const MAX_CONTEXT_CHARS = 18000;
const MAX_SELECTION_CHARS = 2400;
const MAX_MESSAGE_CHARS = 32000;
const MODEL_PROFILES = ["fast", "balanced", "deep", "max"];

async function ensureDir(dir) {
  await fs.mkdir(dir, { recursive: true });
}

function shellQuote(value) {
  return `'${String(value).replace(/'/g, `'\\''`)}'`;
}

function nowIso() {
  return new Date().toISOString();
}

function makeId(prefix) {
  return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

function clampText(text, maxChars) {
  if (!text) return "";
  const value = String(text);
  if (value.length <= maxChars) {
    return value;
  }
  return `${value.slice(0, maxChars)}\n\n[truncated]`;
}

function relativeToRoot(root, target) {
  if (!root || !target) return null;
  const rel = path.relative(root, target);
  if (!rel || rel.startsWith("..")) {
    return target;
  }
  return rel;
}

function summarizeSessionTitle(text) {
  const oneLine = String(text || "")
    .replace(/\s+/g, " ")
    .replace(/^\/\S+\s*/g, "")
    .trim();
  if (!oneLine) {
    return "New chat";
  }
  if (oneLine.length <= 44) {
    return oneLine;
  }
  return `${oneLine.slice(0, 41)}...`;
}

function createSession(seedText = "") {
  const createdAt = nowIso();
  return {
    id: makeId("chat"),
    title: summarizeSessionTitle(seedText),
    createdAt,
    updatedAt: createdAt,
    messages: [],
  };
}

function createInitialChatState() {
  const session = createSession();
  return {
    activeSessionId: session.id,
    sessions: [session],
    composerMode: "ask",
    autoApplyEdits: false,
    modelProfile: "balanced",
  };
}

function normalizeModelProfile(value) {
  const next = String(value || "").toLowerCase();
  return MODEL_PROFILES.includes(next) ? next : "balanced";
}

function resolveNumCtxForProfile(profile) {
  const memGiB = os.totalmem() / (1024 ** 3);
  switch (normalizeModelProfile(profile)) {
    case "fast":
      return memGiB >= 24 ? 4096 : 3072;
    case "deep":
      return memGiB >= 48 ? 24576 : memGiB >= 24 ? 16384 : 8192;
    case "max":
      return memGiB >= 64 ? 32768 : memGiB >= 32 ? 24576 : memGiB >= 24 ? 16384 : 8192;
    default:
      return memGiB >= 32 ? 16384 : memGiB >= 16 ? 8192 : 4096;
  }
}

function modelProfileLabel(profile) {
  switch (normalizeModelProfile(profile)) {
    case "fast":
      return "Fast";
    case "deep":
      return "Deep";
    case "max":
      return "Max";
    default:
      return "Balanced";
  }
}

function sanitizePendingAction(value) {
  if (!value || typeof value !== "object") {
    return null;
  }
  const kind = value.kind === "command" ? "command" : value.kind === "edit" ? "edit" : "";
  if (!kind) {
    return null;
  }
  return {
    kind,
    title: String(value.title || ""),
    artifactDir: value.artifactDir ? String(value.artifactDir) : "",
    patchPath: value.patchPath ? String(value.patchPath) : "",
    diffPreview: value.diffPreview ? clampText(String(value.diffPreview), 12000) : "",
    helperScriptPath: value.helperScriptPath ? String(value.helperScriptPath) : "",
    helperScriptPurpose: value.helperScriptPurpose ? String(value.helperScriptPurpose) : "",
    helperScriptRuntime: value.helperScriptRuntime ? String(value.helperScriptRuntime) : "",
    helperScriptPreview: value.helperScriptPreview ? clampText(String(value.helperScriptPreview), 12000) : "",
    targetFiles: Array.isArray(value.targetFiles) ? value.targetFiles.map((item) => String(item)).slice(0, 8) : [],
    requestText: value.requestText ? String(value.requestText) : "",
    mode: ["ask", "agent", "plan"].includes(String(value.mode || "").toLowerCase()) ? String(value.mode).toLowerCase() : "ask",
  };
}

function sanitizeMessage(message) {
  return {
    id: String(message?.id || makeId("msg")),
    role: message?.role === "user" ? "user" : "assistant",
    text: clampText(message?.text || "", MAX_MESSAGE_CHARS),
    format: message?.format === "plain" ? "plain" : "markdown",
    createdAt: String(message?.createdAt || nowIso()),
    pendingAction: sanitizePendingAction(message?.pendingAction),
    diffPreview: message?.diffPreview ? clampText(String(message.diffPreview), 12000) : "",
    liveStatus: message?.liveStatus ? String(message.liveStatus) : "",
    liveTrace: Array.isArray(message?.liveTrace) ? message.liveTrace.map((item) => String(item)).slice(-5) : [],
  };
}

function summarizeRequestActivity(label, mode) {
  const normalizedMode = ["ask", "agent", "plan"].includes(String(mode || "").toLowerCase()) ? String(mode).toLowerCase() : "ask";
  const lower = String(label || "").toLowerCase();
  if (!lower) {
    return normalizedMode === "agent" ? "Working through the request" : "Thinking";
  }
  if (lower.includes("received")) {
    return "Reading your request";
  }
  if (lower.includes("breaking the request into primitives")) {
    return "Breaking the job into primitives";
  }
  if (lower.includes("inspecting workspace context")) {
    return "Scanning the workspace";
  }
  if (lower.includes("package scaffold primitive")) {
    return "Scaffolding the package locally";
  }
  if (lower.includes("collected candidate files")) {
    return "Picking likely files";
  }
  if (lower.includes("planning edit strategy") || lower.includes("routing to planner model")) {
    return "Planning the edit";
  }
  if (lower.includes("planner selected edit targets")) {
    return "Chose edit targets";
  }
  if (lower.includes("ranking likely target files")) {
    return "Ranking candidate files";
  }
  if (lower.includes("building retrieval bundle")) {
    return "Assembling retrieval context";
  }
  if (lower.includes("generating bounded helper script")) {
    return "Generating a bounded helper";
  }
  if (lower.includes("running bounded helper script")) {
    return "Running the bounded helper";
  }
  if (lower.includes("helper script produced a valid patch")) {
    return "Helper generated a valid patch";
  }
  if (lower.includes("helper script fell back")) {
    return "Falling back to direct patch generation";
  }
  if (lower.includes("deterministic")) {
    return "Using a fast local edit path";
  }
  if (lower.includes("routing to model") || lower.includes("generating patch artifact")) {
    return "Handing off to the model";
  }
  if (lower.includes("repairing invalid patch artifact")) {
    return "Repairing the patch artifact";
  }
  if (lower.includes("re-checking repaired patch")) {
    return "Re-validating the repaired patch";
  }
  if (lower.includes("streaming from")) {
    return "Generating a response";
  }
  if (lower.includes("prepared a validated patch artifact")) {
    return "Prepared a validated edit";
  }
  if (lower.includes("applying")) {
    return "Applying the change";
  }
  if (lower.includes("validating")) {
    return "Validating the result";
  }
  return normalizedMode === "agent" ? "Working through the request" : "Thinking";
}

function buildRequestErrorStatus(message) {
  const lower = String(message || "").toLowerCase();
  if (lower.includes("ollama")) {
    return "Error talking to Ollama";
  }
  if (lower.includes("patch") || lower.includes("edit")) {
    return "Edit request failed";
  }
  return "DorkPipe request failed";
}

function normalizeStoredChatState(raw) {
  if (!raw || !Array.isArray(raw.sessions) || raw.sessions.length === 0) {
    return createInitialChatState();
  }

  const sessions = raw.sessions
    .map((session) => ({
      id: String(session?.id || makeId("chat")),
      title: summarizeSessionTitle(session?.title || "New chat"),
      createdAt: String(session?.createdAt || nowIso()),
      updatedAt: String(session?.updatedAt || session?.createdAt || nowIso()),
      messages: Array.isArray(session?.messages) ? session.messages.map(sanitizeMessage).slice(-MAX_HISTORY_MESSAGES * 4) : [],
    }))
    .slice(0, MAX_SAVED_SESSIONS);

  const activeSession = sessions.find((session) => session.id === raw.activeSessionId) || sessions[0];
  return {
    activeSessionId: activeSession.id,
    sessions,
    composerMode: ["ask", "agent", "plan"].includes(String(raw.composerMode || "").toLowerCase())
      ? String(raw.composerMode).toLowerCase()
      : "ask",
    autoApplyEdits: !!raw.autoApplyEdits,
    modelProfile: normalizeModelProfile(raw.modelProfile),
  };
}

function sortSessionsByUpdate(sessions) {
  return [...sessions].sort((a, b) => String(b.updatedAt).localeCompare(String(a.updatedAt)));
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
    return { text: "", path: null, mtime: null };
  }
  const stat = await fs.stat(ctxPath);
  return {
    text: clampText(await fs.readFile(ctxPath, "utf8"), MAX_CONTEXT_CHARS),
    path: ctxPath,
    mtime: stat.mtime.toISOString(),
  };
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

function buildOllamaUrl(host) {
  return new URL("/api/chat", host.endsWith("/") ? host : `${host}/`);
}

function escapeHtml(text) {
  return String(text)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function renderInlineMarkdown(text) {
  const placeholders = [];
  let html = escapeHtml(text);

  html = html.replace(/`([^`]+)`/g, (_match, code) => {
    const token = `@@CODE${placeholders.length}@@`;
    placeholders.push(`<code>${escapeHtml(code)}</code>`);
    return token;
  });

  html = html.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, (_match, label, href) => {
    return `<a href="${escapeHtml(href)}">${escapeHtml(label)}</a>`;
  });
  html = html.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  html = html.replace(/\*([^*]+)\*/g, "<em>$1</em>");

  for (let i = 0; i < placeholders.length; i += 1) {
    html = html.replace(`@@CODE${i}@@`, placeholders[i]);
  }
  return html;
}

function renderMarkdown(text) {
  const lines = String(text || "").replace(/\r\n/g, "\n").split("\n");
  const out = [];
  let paragraph = [];
  let listItems = [];
  let inCode = false;
  let codeFence = "";
  let codeLang = "";
  let codeLines = [];
  let quoteLines = [];

  const flushParagraph = () => {
    if (!paragraph.length) return;
    out.push(`<p>${renderInlineMarkdown(paragraph.join(" "))}</p>`);
    paragraph = [];
  };

  const flushList = () => {
    if (!listItems.length) return;
    out.push(`<ul>${listItems.map((item) => `<li>${renderInlineMarkdown(item)}</li>`).join("")}</ul>`);
    listItems = [];
  };

  const flushQuote = () => {
    if (!quoteLines.length) return;
    out.push(`<blockquote>${renderMarkdown(quoteLines.join("\n"))}</blockquote>`);
    quoteLines = [];
  };

  const flushCode = () => {
    const langClass = codeLang ? ` class="language-${escapeHtml(codeLang)}"` : "";
    out.push(`<pre><code${langClass}>${escapeHtml(codeLines.join("\n"))}</code></pre>`);
    codeLines = [];
    codeLang = "";
    codeFence = "";
  };

  for (const line of lines) {
    if (inCode) {
      if (line.startsWith(codeFence)) {
        inCode = false;
        flushCode();
      } else {
        codeLines.push(line);
      }
      continue;
    }

    const codeMatch = line.match(/^(```+|~~~+)\s*(\S+)?\s*$/);
    if (codeMatch) {
      flushParagraph();
      flushList();
      flushQuote();
      inCode = true;
      codeFence = codeMatch[1];
      codeLang = codeMatch[2] || "";
      continue;
    }

    if (/^\s*> ?/.test(line)) {
      flushParagraph();
      flushList();
      quoteLines.push(line.replace(/^\s*> ?/, ""));
      continue;
    }
    flushQuote();

    if (/^\s*$/.test(line)) {
      flushParagraph();
      flushList();
      continue;
    }

    const headingMatch = line.match(/^(#{1,6})\s+(.+)$/);
    if (headingMatch) {
      flushParagraph();
      flushList();
      const level = headingMatch[1].length;
      out.push(`<h${level}>${renderInlineMarkdown(headingMatch[2])}</h${level}>`);
      continue;
    }

    if (/^---+$/.test(line.trim()) || /^___+$/.test(line.trim())) {
      flushParagraph();
      flushList();
      out.push("<hr />");
      continue;
    }

    const listMatch = line.match(/^\s*[-*]\s+(.+)$/);
    if (listMatch) {
      flushParagraph();
      listItems.push(listMatch[1]);
      continue;
    }

    paragraph.push(line.trim());
  }

  flushParagraph();
  flushList();
  flushQuote();
  if (inCode) {
    flushCode();
  }
  return out.join("");
}

function renderMessageBody(message) {
  if (message.format === "plain") {
    return `<pre class="plain">${escapeHtml(message.text)}</pre>`;
  }
  return renderMarkdown(message.text);
}

async function collectWorkspaceSignals(root) {
  const editor = vscode.window.activeTextEditor;
  const activePath = editor?.document?.uri?.scheme === "file" ? editor.document.uri.fsPath : null;
  const selectionText =
    editor && !editor.selection.isEmpty
      ? clampText(editor.document.getText(editor.selection), MAX_SELECTION_CHARS)
      : "";
  const openFiles = [];
  for (const group of vscode.window.tabGroups?.all || []) {
    for (const tab of group.tabs || []) {
      const input = tab.input;
      const uri = input && typeof input === "object" && "uri" in input ? input.uri : null;
      if (uri?.scheme === "file" && uri.fsPath) {
        openFiles.push(relativeToRoot(root, uri.fsPath));
      }
    }
  }

  const context = await readContextBundle(root);
  return {
    rootName: path.basename(root),
    activeFile: activePath ? relativeToRoot(root, activePath) : null,
    languageId: editor?.document?.languageId || null,
    selectionText,
    openFiles: [...new Set(openFiles)].slice(0, 8).filter(Boolean),
    contextText: context.text,
    contextPath: context.path,
    contextMtime: context.mtime,
  };
}

function buildSystemPrompt(signals) {
  return [
    "You are DorkPipe, a local-first repo-aware IDE assistant inside VS Code.",
    "Ground your answers in the provided workspace context when relevant.",
    "If you provide code, use fenced code blocks with a language tag when possible.",
    "Prefer concise, practical guidance and state uncertainty plainly.",
    signals.activeFile ? `Active file: ${signals.activeFile}` : "Active file: none",
    signals.selectionText ? `Selected text:\n${signals.selectionText}` : "Selected text: none",
    signals.openFiles.length ? `Open files:\n- ${signals.openFiles.join("\n- ")}` : "Open files: none",
    signals.contextPath
      ? `Repository context bundle (${signals.contextMtime || "unknown time"}):\n\n${signals.contextText}`
      : "Repository context bundle: unavailable. Say so if grounding would help.",
  ].join("\n\n");
}

function buildConversationMessages(session, signals) {
  const system = { role: "system", content: buildSystemPrompt(signals) };
  const history = session.messages
    .filter((message) => (message.role === "user" || message.role === "assistant") && String(message.text || "").trim())
    .slice(-MAX_HISTORY_MESSAGES)
    .map((message) => ({
      role: message.role,
      content: clampText(message.text, 6000),
    }));
  return [system, ...history];
}

function ollamaChat({ host, model, messages, numCtx }) {
  return new Promise((resolve, reject) => {
    let url;
    try {
      url = buildOllamaUrl(host);
    } catch {
      reject(new Error(`Invalid OLLAMA_HOST: ${host}`));
      return;
    }
    const payload = JSON.stringify({
      model,
      stream: false,
      messages,
      options: numCtx ? { num_ctx: numCtx } : undefined,
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
          } catch {
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

function ollamaChatStream({ host, model, messages, onToken, numCtx }) {
  return new Promise((resolve, reject) => {
    let url;
    try {
      url = buildOllamaUrl(host);
    } catch {
      reject(new Error(`Invalid OLLAMA_HOST: ${host}`));
      return;
    }
    const payload = JSON.stringify({
      model,
      stream: true,
      messages,
      options: numCtx ? { num_ctx: numCtx } : undefined,
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
              // ignore partial lines
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

function fileExists(target) {
  return fs
    .stat(target)
    .then(() => true)
    .catch(() => false);
}

function spawnStreamingCommand(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    const child = cp.spawn(command, args, {
      cwd: options.cwd,
      env: { ...process.env, ...(options.env || {}) },
      stdio: ["ignore", "pipe", "pipe"],
    });

    let stdoutBuffer = "";
    let stderrBuffer = "";
    let stdoutText = "";
    let stderrText = "";

    child.stdout.on("data", (chunk) => {
      const text = String(chunk);
      stdoutText += text;
      stdoutBuffer += text;
      const lines = stdoutBuffer.split("\n");
      stdoutBuffer = lines.pop() || "";
      for (const line of lines) {
        if (options.onStdoutLine) {
          options.onStdoutLine(line);
        }
      }
    });

    child.stderr.on("data", (chunk) => {
      const text = String(chunk);
      stderrText += text;
      stderrBuffer += text;
      const lines = stderrBuffer.split("\n");
      stderrBuffer = lines.pop() || "";
      for (const line of lines) {
        if (options.onStderrLine) {
          options.onStderrLine(line);
        }
      }
    });

    child.on("error", (error) => reject(error));
    child.on("close", (code) => {
      if (stdoutBuffer && options.onStdoutLine) {
        options.onStdoutLine(stdoutBuffer);
      }
      if (stderrBuffer && options.onStderrLine) {
        options.onStderrLine(stderrBuffer);
      }
      resolve({
        code: typeof code === "number" ? code : 0,
        stdout: stdoutText,
        stderr: stderrText,
      });
    });
  });
}

async function resolveDorkpipeInvocation(root) {
  const binary = path.join(root, "packages", "dorkpipe", "bin", "dorkpipe");
  if (await fileExists(binary)) {
    const probe = await runCommand(`${shellQuote(binary)} --help`, root);
    const helpText = `${probe.stdout}\n${probe.stderr}`;
    if (probe.ok && /\brequest\b/.test(helpText) && /\bapply-edit\b/.test(helpText)) {
      return {
        command: binary,
        argsPrefix: [],
        cwd: root,
        mode: "binary",
      };
    }
  }
  return {
    command: "go",
    argsPrefix: ["run", "./cmd/dorkpipe"],
    cwd: path.join(root, "packages", "dorkpipe", "lib"),
    mode: "go-run",
  };
}

async function executeNaturalLanguageRequest(root, text, signals, options = {}) {
  const invocation = await resolveDorkpipeInvocation(root);
  const requestMode = ["ask", "agent", "plan"].includes(String(options.mode || "").toLowerCase())
    ? String(options.mode).toLowerCase()
    : "ask";
  const modelProfile = normalizeModelProfile(options.modelProfile);
  const numCtx = resolveNumCtxForProfile(modelProfile);
  const args = [
    ...invocation.argsPrefix,
    "request",
    "--execute",
    "--workdir",
    root,
    "--mode",
    requestMode,
    "--message",
    text,
    "--num-ctx",
    String(numCtx),
  ];
  if (signals.activeFile) {
    args.push("--active-file", signals.activeFile);
  }
  if (signals.selectionText) {
    args.push("--selection-text", signals.selectionText);
  }

  let finalEvent = null;
  let readyToApply = null;
  const stderrLines = [];
  let streamedText = "";
  const result = await spawnStreamingCommand(invocation.command, args, {
    cwd: invocation.cwd,
    env: {
      DOCKPIPE_WORKDIR: root,
    },
    onStdoutLine: (line) => {
      const trimmed = line.trim();
      if (!trimmed) return;
      try {
        const event = JSON.parse(trimmed);
        if (event.display_text && options.onEvent) {
          options.onEvent(event.display_text);
        }
        if (event.type === "model_stream") {
          const piece = event.metadata?.text || "";
          if (piece) {
            streamedText += piece;
            options.onToken?.(piece, streamedText);
          }
        }
        if (event.type === "ready_to_apply") {
          readyToApply = event.metadata || {};
        }
        if (event.type === "done" || event.type === "error") {
          finalEvent = event;
        }
      } catch {
        if (options.onEvent) {
          options.onEvent(trimmed);
        }
      }
    },
    onStderrLine: (line) => {
      const trimmed = line.trim();
      if (!trimmed) return;
      stderrLines.push(trimmed);
    },
  });

  if (stderrLines.length) {
    options.channel?.appendLine(stderrLines.join("\n"));
  }
  if (finalEvent?.type === "error") {
    throw new Error(finalEvent.error?.user_message || "DorkPipe request failed.");
  }
  if (result.code !== 0) {
    throw new Error(stderrLines.join("\n") || "DorkPipe request failed.");
  }
  if (!finalEvent) {
    throw new Error("DorkPipe request did not return a final event.");
  }
  return {
    kind: finalEvent.metadata?.route || "chat",
    text: finalEvent.user_message || streamedText || "(No response text returned.)",
    format: "markdown",
    readyToApply,
    metadata: finalEvent.metadata || {},
    status: buildDorkpipeStatus({ ...(finalEvent.metadata || {}), model_profile: modelProfile, num_ctx: numCtx }, requestMode),
  };
}

function buildDorkpipeStatus(metadata, fallbackMode = "ask") {
  const route = metadata.route || "request";
  const mode = metadata.mode || fallbackMode;
  const profile = modelProfileLabel(metadata.model_profile || "balanced");
  if (route === "chat") {
    const parts = [
      `Mode: ${String(mode).replace(/^./, (c) => c.toUpperCase())}`,
      `Model: ${metadata.model || process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL}`,
      `Profile: ${profile}`,
      `Ollama: ${process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST}`,
    ];
    if (metadata.num_ctx) {
      parts.push(`Context: ${metadata.num_ctx}`);
    }
    if (metadata.context_path) {
      parts.push(`Context: ${metadata.context_path}`);
    }
    if (metadata.active_file) {
      parts.push(`Active file: ${metadata.active_file}`);
    }
    return parts.join("  |  ");
  }
  if (route === "inspect") {
    const parts = [
      `Mode: ${String(mode).replace(/^./, (c) => c.toUpperCase())}`,
      `Action: ${metadata.action || "inspect"}`,
    ];
    if (metadata.workflow) {
      parts.push(`Workflow: ${metadata.workflow}`);
    }
    if (metadata.target) {
      parts.push(`Target: ${metadata.target}`);
    }
    if (metadata.context_path) {
      parts.push(`Context: ${metadata.context_path}`);
    }
    if (metadata.validation_status) {
      parts.push(`Validation: ${metadata.validation_status}`);
    }
    return parts.join("  |  ");
  }
  if (route === "edit") {
    const parts = [`Mode: ${String(mode).replace(/^./, (c) => c.toUpperCase())}`, "Route: edit", `Profile: ${profile}`];
    if (metadata.num_ctx) {
      parts.push(`Context: ${metadata.num_ctx}`);
    }
    if (typeof metadata.files_touched === "number") {
      parts.push(`Files: ${metadata.files_touched}`);
    }
    if (metadata.helper_script_used) {
      parts.push("Primitive: sidecar");
    }
    if (metadata.validation_status) {
      parts.push(`Validation: ${metadata.validation_status}`);
    }
    if (metadata.artifact_dir) {
      parts.push(`Artifact: ${metadata.artifact_dir}`);
    }
    return parts.join("  |  ");
  }
  return "Request handled by DorkPipe";
}

async function applyPreparedEdit(root, artifactDir, options = {}) {
  const invocation = await resolveDorkpipeInvocation(root);
  const args = [
    ...invocation.argsPrefix,
    "apply-edit",
    "--workdir",
    root,
    "--artifact-dir",
    artifactDir,
  ];
  let finalEvent = null;
  const stderrLines = [];
  const result = await spawnStreamingCommand(invocation.command, args, {
    cwd: invocation.cwd,
    env: {
      DOCKPIPE_WORKDIR: root,
    },
    onStdoutLine: (line) => {
      const trimmed = line.trim();
      if (!trimmed) return;
      try {
        const event = JSON.parse(trimmed);
        if (event.display_text && options.onEvent) {
          options.onEvent(event.display_text);
        }
        if (event.type === "done" || event.type === "error") {
          finalEvent = event;
        }
      } catch {
        if (options.onEvent) {
          options.onEvent(trimmed);
        }
      }
    },
    onStderrLine: (line) => {
      const trimmed = line.trim();
      if (!trimmed) return;
      stderrLines.push(trimmed);
    },
  });
  if (stderrLines.length) {
    options.channel?.appendLine(stderrLines.join("\n"));
  }
  if (finalEvent?.type === "error") {
    throw new Error(finalEvent.error?.user_message || "Apply edit failed.");
  }
  if (result.code !== 0) {
    throw new Error(stderrLines.join("\n") || "Apply edit failed.");
  }
  return {
    kind: "edit",
    text: finalEvent?.user_message || "Applied prepared edit.",
    format: "markdown",
    status: "Structured edit applied",
  };
}

async function readPatchPreview(root, readyToApply) {
  const relPatch = String(readyToApply?.patch_path || "").trim();
  const relArtifact = String(readyToApply?.artifact_dir || "").trim();
  const candidates = [];
  if (relPatch) {
    candidates.push(path.isAbsolute(relPatch) ? relPatch : path.join(root, relPatch));
  }
  if (relArtifact) {
    candidates.push(path.join(root, relArtifact, "patch.diff"));
  }
  for (const candidate of candidates) {
    try {
      const text = await fs.readFile(candidate, "utf8");
      const lines = text.split(/\r?\n/).slice(0, 120).join("\n");
      return clampText(lines, 12000);
    } catch {
      // Try next candidate.
    }
  }
  return "";
}

async function readHelperScriptPreview(root, readyToApply) {
  const relHelper = String(readyToApply?.helper_script_path || "").trim();
  if (!relHelper) {
    return "";
  }
  const target = path.isAbsolute(relHelper) ? relHelper : path.join(root, relHelper);
  try {
    const text = await fs.readFile(target, "utf8");
    return clampText(text.split(/\r?\n/).slice(0, 120).join("\n"), 12000);
  } catch {
    return "";
  }
}

function looksLikeShellCommand(text) {
  const trimmed = String(text || "").trim();
  if (!trimmed || /\n/.test(trimmed)) {
    return false;
  }
  const lower = trimmed.toLowerCase();
  return /^(make|npm|pnpm|yarn|bun|go|cargo|git|docker|docker-compose|podman|node|npx|python|python3|bash|sh|cmake|dockpipe|dorkpipe|ollama)\b/.test(lower);
}

function getCliConfirmationRequest(text, mode = "ask") {
  const trimmed = String(text || "").trim();
  const lower = trimmed.toLowerCase();
  let title = "";
  if (lower === "/test") {
    title = "Run the `test` workflow?";
  } else if (lower === "/ci") {
    title = "Run the `ci-emulate` workflow?";
  } else if (lower.startsWith("/workflow ")) {
    title = `Run ${trimmed.slice("/workflow ".length).trim()}?`;
  } else if (lower.startsWith("/validate ")) {
    title = `Validate ${trimmed.slice("/validate ".length).trim()}?`;
  } else if (lower.startsWith("/workflow-validate ")) {
    title = `Validate ${trimmed.slice("/workflow-validate ".length).trim()}?`;
  } else {
    const deterministic = getDeterministicIntent(trimmed);
    if (deterministic?.command === "/test") {
      title = "Run the `test` workflow?";
    } else if (deterministic?.command === "/ci") {
      title = "Run the `ci-emulate` workflow?";
    }
  }
  if (!title && looksLikeShellCommand(trimmed)) {
    title = `Run \`${trimmed}\` in a terminal?`;
  }
  if (!title) {
    return null;
  }
  return {
    kind: "command",
    title,
    requestText: trimmed,
    mode,
  };
}

function shouldDelegateSlashToDorkpipe(text) {
  const trimmed = String(text || "").trim().toLowerCase();
  return (
    trimmed === "/context" ||
    trimmed === "/status" ||
    trimmed === "/bundle" ||
    trimmed === "/test" ||
    trimmed === "/ci" ||
    trimmed.startsWith("/workflow ") ||
    trimmed.startsWith("/validate ") ||
    trimmed.startsWith("/workflow-validate ") ||
    trimmed.startsWith("/edit ")
  );
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

function summarizeCommandTitle(text) {
  const trimmed = String(text || "").trim();
  if (!trimmed) {
    return "DorkPipe command";
  }
  const parts = trimmed.split(/\s+/).slice(0, 3);
  return `DorkPipe: ${parts.join(" ")}`;
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
        kind: "local",
        text: [
          "# DorkPipe local commands",
          "",
          "- `/help`",
          "- `/status`",
          "- `/bundle`",
          "- `/context`",
          "- `/test`",
          "- `/ci`",
          "- `/validate [path]`",
          "- `/plan <task>`",
          "- `/edit <task>`",
          "- `/workflow <name>`",
          "- `/workflow-validate <path-to-config.yml>`",
        ].join("\n"),
        status: "Local command help",
      };
    case "/status": {
      const result = await runCommand("./packages/pipeon/resolvers/pipeon/bin/pipeon status", root, {
        DOCKPIPE_WORKDIR: root,
        DOCKPIPE_PIPEON: process.env.DOCKPIPE_PIPEON || "1",
        DOCKPIPE_PIPEON_ALLOW_PRERELEASE: process.env.DOCKPIPE_PIPEON_ALLOW_PRERELEASE || "1",
      });
      return {
        kind: "local",
        text: ["```text", (result.stdout || result.stderr || "No output").trim(), "```"].join("\n"),
        status: "Local status command executed",
      };
    }
    case "/bundle": {
      const result = await runCommand("./packages/pipeon/resolvers/pipeon/bin/pipeon bundle", root, {
        DOCKPIPE_WORKDIR: root,
        DOCKPIPE_PIPEON: process.env.DOCKPIPE_PIPEON || "1",
        DOCKPIPE_PIPEON_ALLOW_PRERELEASE: process.env.DOCKPIPE_PIPEON_ALLOW_PRERELEASE || "1",
      });
      return {
        kind: "local",
        text: ["```text", (result.stdout || result.stderr || "No output").trim(), "```"].join("\n"),
        status: "Context bundle refreshed",
      };
    }
    case "/context": {
      const ctxPath = await resolveContextBundlePath(root);
      return {
        kind: "local",
        text: ctxPath ? `Current context bundle: \`${path.relative(root, ctxPath)}\`` : "No DorkPipe context bundle found yet. Run `/bundle` first.",
        status: ctxPath ? "Context bundle available" : "No context bundle found",
      };
    }
    case "/workflow": {
      if (args.length === 0) {
        return { kind: "local", text: "Usage: `/workflow <name>`", status: "Workflow command needs a name" };
      }
      const workflow = args[0];
      launchTerminalCommand(root, `DorkPipe workflow: ${workflow}`, `./src/bin/dockpipe --workflow ${workflow} --workdir . --`);
      return {
        kind: "local",
        text: `Started workflow \`${workflow}\` in a terminal.`,
        status: `Started workflow ${workflow}`,
      };
    }
    case "/test":
      launchTerminalCommand(root, "DorkPipe test", "./src/bin/dockpipe --workflow test --workdir . --");
      return { kind: "local", text: "Started `test` workflow in a terminal.", status: "Started test workflow" };
    case "/ci":
      launchTerminalCommand(root, "DorkPipe ci-emulate", "./src/bin/dockpipe --workflow ci-emulate --workdir . --");
      return { kind: "local", text: "Started `ci-emulate` workflow in a terminal.", status: "Started ci-emulate workflow" };
    case "/validate": {
      if (args.length > 0) {
        const target = args.join(" ");
        const result = await runCommand(`./src/bin/dockpipe workflow validate ${shellQuote(target)}`, root);
        return {
          kind: "local",
          text: ["```text", (result.stdout || result.stderr || "No output").trim(), "```"].join("\n"),
          status: "Workflow validation finished",
        };
      }
      launchTerminalCommand(root, "DorkPipe validate", "./src/bin/dockpipe --workflow test --workdir . --");
      return { kind: "local", text: "Started validation via the `test` workflow in a terminal.", status: "Started validation workflow" };
    }
    case "/plan": {
      if (args.length === 0) {
        return { kind: "local", text: `Usage: \`${cmd} <task description>\``, status: "Task scaffold command needs a description" };
      }
      const task = args.join(" ");
      const file = await writeTaskFile(root, cmd.slice(1), task);
      const doc = await vscode.workspace.openTextDocument(file);
      await vscode.window.showTextDocument(doc, { preview: false });
      return {
        kind: "local",
        text: `Created ${cmd.slice(1)} task scaffold at \`${path.relative(root, file)}\`.\n\nNext: run \`/test\`, \`/ci\`, or \`/workflow <name>\` after you wire the execution step you want.`,
        status: `Created ${cmd.slice(1)} task scaffold`,
      };
    }
    case "/workflow-validate": {
      if (args.length === 0) {
        return { kind: "local", text: "Usage: `/workflow-validate <path-to-config.yml>`", status: "Workflow validation needs a path" };
      }
      const result = await runCommand(`./src/bin/dockpipe workflow validate ${shellQuote(args.join(" "))}`, root);
      return {
        kind: "local",
        text: ["```text", (result.stdout || result.stderr || "No output").trim(), "```"].join("\n"),
        status: "Workflow validation finished",
      };
    }
    default:
      return {
        kind: "local",
        text: `Unknown DorkPipe local command: \`${cmd}\`\n\nTry \`/help\`.`,
        status: "Unknown local command",
      };
  }
}

function getDeterministicIntent(text) {
  const value = text.trim().toLowerCase();
  if (!value) return null;
  if (/^(show|open|what is).*(context bundle|context)\??$/.test(value) || /\bwhat context\b/.test(value)) {
    return { command: "/context", reason: "workspace context question" };
  }
  if (/\b(refresh|rebuild|bundle)\b.*\bcontext\b/.test(value)) {
    return { command: "/bundle", reason: "context refresh request" };
  }
  if (/^(show|check|what is).*\bstatus\b/.test(value) || value === "status") {
    return { command: "/status", reason: "status request" };
  }
  if (/\b(run|start)\b.*\btests?\b/.test(value)) {
    return { command: "/test", reason: "test workflow request" };
  }
  if (/\b(run|start)\b.*\bci\b/.test(value)) {
    return { command: "/ci", reason: "ci workflow request" };
  }
  return null;
}

async function executeDorkpipeRequest(root, session, text, options = {}) {
  const onToken = typeof options.onToken === "function" ? options.onToken : null;
  const onEvent = typeof options.onEvent === "function" ? options.onEvent : null;
  const mode = ["ask", "agent", "plan"].includes(String(options.mode || "").toLowerCase())
    ? String(options.mode).toLowerCase()
    : "ask";
  const modelProfile = normalizeModelProfile(options.modelProfile);
  const numCtx = resolveNumCtxForProfile(modelProfile);

  const emitEvent = (label) => {
    if (onEvent) {
      onEvent(label);
    }
  };

  emitEvent("Received request");
  const signals = await collectWorkspaceSignals(root);

  if (!text.trim().startsWith("/") || shouldDelegateSlashToDorkpipe(text)) {
    return executeNaturalLanguageRequest(root, text, signals, {
      onEvent,
      channel: options.channel,
      onToken,
      mode,
      modelProfile,
    });
  }

  const local = await handleLocalCommand(root, text);
  if (local) {
    emitEvent("Routing to safe local action");
    return {
      kind: "local",
      text: local.text,
      format: "markdown",
      host,
      model,
      status: local.status || "Local DorkPipe command executed",
      contextPath: null,
    };
  }

  emitEvent("Inspecting workspace context");
  emitEvent(signals.contextPath ? `Loaded ${path.relative(root, signals.contextPath)}` : "No context bundle found");
  if (signals.activeFile) {
    emitEvent(`Active file: ${signals.activeFile}`);
  }

  const deterministicIntent = getDeterministicIntent(text);
  if (deterministicIntent) {
    emitEvent(`Confident route: ${deterministicIntent.reason}`);
    const orchestrated = await handleLocalCommand(root, deterministicIntent.command);
    if (orchestrated) {
      return {
        kind: "local",
        text: `${orchestrated.text}\n\n_Handled locally without calling the model._`,
        format: "markdown",
        status: orchestrated.status || "Handled locally",
        contextPath: signals.contextPath,
      };
    }
  }

  emitEvent("Routing to model");
  const messages = buildConversationMessages(session, signals);

  let answer = "";
  if (onToken) {
    emitEvent(`Streaming from ${model}`);
    answer = await ollamaChatStream({
      host,
      model,
      messages,
      onToken,
      numCtx,
    });
  } else {
    emitEvent(`Waiting for ${model}`);
    answer = await ollamaChat({
      host,
      model,
      messages,
      numCtx,
    });
  }

  return {
    kind: "model",
    text: answer || "(No response text returned.)",
    format: "markdown",
    contextPath: signals.contextPath,
    status: signals.contextPath
      ? `Model: ${process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL}  |  Profile: ${modelProfileLabel(modelProfile)}  |  num_ctx: ${numCtx}  |  Ollama: ${process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST}  |  Context: ${path.relative(root, signals.contextPath)}`
      : `Model: ${process.env.PIPEON_OLLAMA_MODEL || process.env.DOCKPIPE_OLLAMA_MODEL || DEFAULT_MODEL}  |  Profile: ${modelProfileLabel(modelProfile)}  |  num_ctx: ${numCtx}  |  Ollama: ${process.env.OLLAMA_HOST || DEFAULT_OLLAMA_HOST}  |  No context bundle found`,
  };
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
          radial-gradient(circle at top, color-mix(in srgb, var(--vscode-button-background) 12%, transparent) 0%, transparent 40%),
          linear-gradient(180deg, color-mix(in srgb, var(--vscode-sideBar-background) 95%, transparent), var(--vscode-editor-background));
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
      .titleRow {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
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
        margin-top: 2px;
      }
      .headerActions {
        display: flex;
        gap: 8px;
        align-items: center;
      }
      select, textarea, button {
        font: inherit;
      }
      select {
        min-width: 176px;
        max-width: 240px;
        background: var(--vscode-dropdown-background, var(--vscode-input-background));
        color: var(--vscode-dropdown-foreground, var(--vscode-input-foreground));
        border: 1px solid var(--vscode-dropdown-border, var(--vscode-input-border));
        border-radius: 10px;
        padding: 8px 10px;
      }
      .btn {
        border: 0;
        border-radius: 10px;
        padding: 9px 13px;
        min-width: 88px;
        background: var(--vscode-button-background);
        color: var(--vscode-button-foreground);
        font-weight: 600;
        cursor: pointer;
      }
      .btn.ghost {
        background: color-mix(in srgb, var(--vscode-button-background) 14%, transparent);
        color: var(--vscode-editor-foreground);
        border: 1px solid var(--vscode-panel-border);
      }
      .btn:disabled { opacity: 0.6; cursor: default; }
      .trace {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
        margin-top: 10px;
      }
      .traceItem {
        padding: 5px 9px;
        border-radius: 999px;
        border: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-editorWidget-background) 92%, transparent);
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
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
        background: linear-gradient(180deg, color-mix(in srgb, var(--vscode-button-background) 18%, transparent), color-mix(in srgb, var(--vscode-button-background) 9%, transparent));
        border-bottom-right-radius: 6px;
      }
      .msg.assistant {
        background: linear-gradient(180deg, color-mix(in srgb, var(--vscode-editorWidget-background) 96%, transparent), color-mix(in srgb, var(--vscode-editorWidget-background) 84%, transparent));
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
        line-height: 1.58;
        font-size: 13px;
        word-break: break-word;
      }
      .body p:first-child,
      .body h1:first-child,
      .body h2:first-child,
      .body h3:first-child { margin-top: 0; }
      .body p:last-child,
      .body ul:last-child,
      .body pre:last-child,
      .body blockquote:last-child { margin-bottom: 0; }
      .body pre {
        margin: 12px 0;
        padding: 12px;
        overflow-x: auto;
        background: color-mix(in srgb, var(--vscode-textCodeBlock-background, #111) 92%, transparent);
        border: 1px solid var(--vscode-panel-border);
        border-radius: 12px;
      }
      .body code {
        font-family: var(--vscode-editor-font-family, monospace);
        font-size: 0.95em;
      }
      .body :not(pre) > code {
        padding: 2px 6px;
        border-radius: 6px;
        background: color-mix(in srgb, var(--vscode-textCodeBlock-background, #111) 68%, transparent);
      }
      .body ul {
        padding-left: 20px;
      }
      .body blockquote {
        margin: 12px 0;
        padding-left: 12px;
        border-left: 3px solid var(--vscode-button-background);
        color: var(--vscode-descriptionForeground);
      }
      .body a {
        color: var(--vscode-textLink-foreground);
      }
      .pendingCard {
        margin-top: 12px;
        padding: 12px;
        border-radius: 12px;
        border: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-editorWidget-background) 92%, transparent);
      }
      .pendingTitle {
        font-size: 12px;
        font-weight: 700;
        margin-bottom: 8px;
      }
      .pendingMeta {
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
        margin-bottom: 8px;
      }
      .diffPreview {
        margin: 10px 0 0;
        padding: 10px;
        overflow-x: auto;
        border-radius: 10px;
        border: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-textCodeBlock-background, #111) 92%, transparent);
        font-family: var(--vscode-editor-font-family, monospace);
        font-size: 12px;
        line-height: 1.45;
        white-space: pre;
      }
      .pendingActions {
        display: flex;
        gap: 8px;
        margin-top: 10px;
      }
      .liveCard {
        margin-top: 12px;
        padding: 12px;
        border-radius: 12px;
        border: 1px solid var(--vscode-panel-border);
        background: color-mix(in srgb, var(--vscode-editorWidget-background) 94%, transparent);
      }
      .liveRow {
        display: flex;
        align-items: center;
        gap: 10px;
        margin-bottom: 8px;
      }
      .liveDot {
        width: 10px;
        height: 10px;
        border-radius: 999px;
        background: var(--vscode-button-background);
        box-shadow: 0 0 0 0 color-mix(in srgb, var(--vscode-button-background) 45%, transparent);
        animation: pulse 1.4s ease-in-out infinite;
      }
      .liveTitle {
        font-size: 12px;
        font-weight: 700;
      }
      .liveMeta {
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
      }
      .liveTrace {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        margin-top: 8px;
      }
      .liveChip {
        padding: 4px 8px;
        border-radius: 999px;
        font-size: 11px;
        border: 1px solid var(--vscode-panel-border);
        color: var(--vscode-descriptionForeground);
        background: color-mix(in srgb, var(--vscode-sideBar-background) 92%, transparent);
      }
      .toggleWrap {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
      }
      .toggleWrap input {
        margin: 0;
      }
      .plain {
        white-space: pre-wrap;
        margin: 0;
        font-family: var(--vscode-editor-font-family, monospace);
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
        line-height: 1.45;
      }
      .actions {
        display: flex;
        justify-content: space-between;
        gap: 10px;
        align-items: center;
      }
      .hint {
        font-size: 11px;
        color: var(--vscode-descriptionForeground);
      }
      .sendWrap {
        display: flex;
        gap: 8px;
      }
      @media (max-width: 520px) {
        .titleRow, .actions {
          flex-direction: column;
          align-items: stretch;
        }
        .headerActions {
          flex-wrap: wrap;
        }
        select {
          max-width: none;
        }
      }
      @keyframes pulse {
        0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--vscode-button-background) 45%, transparent); }
        70% { box-shadow: 0 0 0 9px transparent; }
        100% { box-shadow: 0 0 0 0 transparent; }
      }
    </style>
  </head>
  <body>
    <div class="wrap">
      <header class="header">
        <div class="eyebrow">DorkPipe</div>
        <div class="titleRow">
          <div>
            <h1 class="title">Workspace Chat</h1>
            <div class="sub">Persistent chats, repo context, safe local actions, and streamed model responses.</div>
          </div>
          <div class="headerActions">
            <select id="sessionSelect" aria-label="Chat session"></select>
            <button class="btn ghost" id="clearBtn" type="button">Clear</button>
            <button class="btn" id="newChatBtn" type="button">New chat</button>
          </div>
        </div>
        <div class="trace" id="trace"></div>
      </header>
      <main class="transcript" id="transcript"></main>
      <div class="composerWrap">
        <div class="status" id="status"></div>
        <form class="composer" id="composer">
          <textarea id="prompt" placeholder="Ask DorkPipe about this repo... or use /help for local commands"></textarea>
          <div class="actions">
            <div class="hint">Uses the workspace bundle when available. Safe local routing can skip the model for obvious actions.</div>
            <div class="sendWrap">
              <label class="toggleWrap"><input id="autoApplyEdits" type="checkbox" /> Auto-apply edits</label>
              <select id="modelProfileSelect" aria-label="Model profile">
                <option value="fast">Fast</option>
                <option value="balanced">Balanced</option>
                <option value="deep">Deep</option>
                <option value="max">Max</option>
              </select>
              <select id="modeSelect" aria-label="Request mode">
                <option value="ask">Ask</option>
                <option value="agent">Agent</option>
                <option value="plan">Plan</option>
              </select>
              <button class="btn ghost" id="newFromComposer" type="button">New</button>
              <button class="btn" id="send" type="submit">Send</button>
            </div>
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
      const clearBtn = document.getElementById("clearBtn");
      const newChatBtn = document.getElementById("newChatBtn");
      const newFromComposer = document.getElementById("newFromComposer");
      const sessionSelect = document.getElementById("sessionSelect");
      const modeSelect = document.getElementById("modeSelect");
      const autoApplyEdits = document.getElementById("autoApplyEdits");
      const modelProfileSelect = document.getElementById("modelProfileSelect");
      const transcript = document.getElementById("transcript");
      const status = document.getElementById("status");
      const trace = document.getElementById("trace");
      let viewState = vscode.getState() || { draft: "", pinnedToBottom: true, mode: "ask", modelProfile: "balanced" };
      let currentState = initialState;

      function escapeHtml(value) {
        return String(value || "")
          .replace(/&/g, "&amp;")
          .replace(/</g, "&lt;")
          .replace(/>/g, "&gt;")
          .replace(/"/g, "&quot;")
          .replace(/'/g, "&#39;");
      }

      function renderPendingAction(message) {
        const pending = message.pendingAction;
        if (!pending) {
          return "";
        }
        const meta = [];
        if (pending.kind === "edit" && Array.isArray(pending.targetFiles) && pending.targetFiles.length) {
          meta.push("Files: " + pending.targetFiles.join(", "));
        }
        if (pending.kind === "edit" && pending.helperScriptRuntime) {
          meta.push("Sidecar: " + pending.helperScriptRuntime);
        }
        if (pending.kind === "command" && pending.requestText) {
          meta.push("Command: " + pending.requestText);
        }
        const helper = pending.helperScriptPreview
          ? [
              '<div class="pendingTitle">Bounded helper script</div>',
              pending.helperScriptPurpose ? '<div class="pendingMeta">' + escapeHtml(pending.helperScriptPurpose) + "</div>" : "",
              '<pre class="diffPreview">' + escapeHtml(pending.helperScriptPreview) + "</pre>",
            ].join("")
          : "";
        const diff = pending.diffPreview
          ? '<pre class="diffPreview">' + escapeHtml(pending.diffPreview) + "</pre>"
          : "";
        return [
          '<div class="pendingCard">',
          '<div class="pendingTitle">' + escapeHtml(pending.title || (pending.kind === "edit" ? "Review edit" : "Confirm command")) + "</div>",
          meta.length ? '<div class="pendingMeta">' + escapeHtml(meta.join("  |  ")) + "</div>" : "",
          helper,
          diff,
          '<div class="pendingActions">',
          '<button class="btn" type="button" data-pending-action="approve" data-message-id="' + escapeHtml(message.id) + '">' + (pending.kind === "edit" ? "Apply" : "Run") + "</button>",
          '<button class="btn ghost" type="button" data-pending-action="dismiss" data-message-id="' + escapeHtml(message.id) + '">Dismiss</button>',
          "</div>",
          "</div>",
        ].join("");
      }

      function renderDiffPreview(message) {
        if (!message.diffPreview) {
          return "";
        }
        return [
          '<div class="pendingCard">',
          '<div class="pendingTitle">Diff preview</div>',
          '<pre class="diffPreview">' + escapeHtml(message.diffPreview) + "</pre>",
          "</div>",
        ].join("");
      }

      function renderLiveCard(message) {
        if (!message.liveStatus) {
          return "";
        }
        const chips = Array.isArray(message.liveTrace) && message.liveTrace.length
          ? '<div class="liveTrace">' + message.liveTrace.map((item) => '<div class="liveChip">' + escapeHtml(item) + '</div>').join("") + "</div>"
          : "";
        return [
          '<div class="liveCard">',
          '<div class="liveRow"><div class="liveDot"></div><div class="liveTitle">' + escapeHtml(message.liveStatus) + "</div></div>",
          '<div class="liveMeta">DorkPipe is routing, gathering context, and deciding the safest next step.</div>',
          chips,
          "</div>",
        ].join("");
      }

      function renderMessages(messages) {
        if (!messages.length) {
          return '<article class="msg assistant"><div class="role">DorkPipe</div><div class="body"><p>Ask about this workspace. DorkPipe will use the local context bundle when available, keep chat history per workspace, and route obvious safe actions locally first.</p></div></article>';
        }
        return messages.map((message) => {
          const role = message.role === "assistant" ? "DorkPipe" : "You";
          return '<article class="msg ' + message.role + '"><div class="role">' + role + '</div><div class="body">' + (message.html || "") + renderLiveCard(message) + renderDiffPreview(message) + renderPendingAction(message) + '</div></article>';
        }).join("");
      }

      function renderTrace(items) {
        if (!items || !items.length) {
          return "";
        }
        return items.map((item) => '<div class="traceItem">' + item + '</div>').join("");
      }

      function renderSessions(nextState) {
        const sessions = nextState.sessionList || [];
        sessionSelect.innerHTML = sessions.map((session) => {
          const selected = session.id === nextState.activeSessionId ? " selected" : "";
          return '<option value="' + session.id + '"' + selected + '>' + session.title + '</option>';
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
        trace.innerHTML = renderTrace(currentState.trace || []);
        status.textContent = currentState.status || "";
        renderSessions(currentState);
        modeSelect.value = currentState.composerMode || viewState.mode || "ask";
        autoApplyEdits.checked = !!currentState.autoApplyEdits;
        modelProfileSelect.value = currentState.modelProfile || "balanced";
        send.disabled = !!currentState.isBusy;
        clearBtn.disabled = !!currentState.isBusy;
        newChatBtn.disabled = !!currentState.isBusy;
        newFromComposer.disabled = !!currentState.isBusy;
        modeSelect.disabled = !!currentState.isBusy;
        autoApplyEdits.disabled = !!currentState.isBusy;
        modelProfileSelect.disabled = !!currentState.isBusy;
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
      modeSelect.value = viewState.mode || currentState.composerMode || "ask";
      modelProfileSelect.value = viewState.modelProfile || currentState.modelProfile || "balanced";
      prompt.addEventListener("input", () => {
        saveViewState({ draft: prompt.value });
      });
      modeSelect.addEventListener("change", () => {
        saveViewState({ mode: modeSelect.value });
        vscode.postMessage({ type: "setComposerMode", mode: modeSelect.value });
      });
      autoApplyEdits.addEventListener("change", () => {
        vscode.postMessage({ type: "setAutoApplyEdits", value: autoApplyEdits.checked });
      });
      modelProfileSelect.addEventListener("change", () => {
        vscode.postMessage({ type: "setModelProfile", value: modelProfileSelect.value });
      });
      prompt.addEventListener("keydown", (event) => {
        if (event.key !== "Enter") {
          return;
        }
        if (event.altKey) {
          return;
        }
        if (event.shiftKey || event.ctrlKey || event.metaKey) {
          return;
        }
        event.preventDefault();
        form.requestSubmit();
      });

      sessionSelect.addEventListener("change", () => {
        vscode.postMessage({ type: "switchSession", sessionId: sessionSelect.value });
      });

      clearBtn.addEventListener("click", () => {
        vscode.postMessage({ type: "clearSession" });
      });

      function startNewChat() {
        vscode.postMessage({ type: "newSession", seed: prompt.value.trim() });
      }

      newChatBtn.addEventListener("click", startNewChat);
      newFromComposer.addEventListener("click", startNewChat);

      form.addEventListener("submit", (event) => {
        event.preventDefault();
        const text = prompt.value.trim();
        if (!text) return;
        prompt.value = "";
        saveViewState({ draft: "", pinnedToBottom: true, mode: modeSelect.value, modelProfile: modelProfileSelect.value });
        vscode.postMessage({ type: "ask", text, mode: modeSelect.value, modelProfile: modelProfileSelect.value });
      });

      transcript.addEventListener("click", (event) => {
        const target = event.target instanceof HTMLElement ? event.target.closest("[data-pending-action]") : null;
        if (!target) {
          return;
        }
        vscode.postMessage({
          type: "resolvePendingAction",
          messageId: target.getAttribute("data-message-id"),
          decision: target.getAttribute("data-pending-action"),
        });
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
  /** @param {vscode.ExtensionContext} context */
  constructor(context, channel) {
    this.context = context;
    this.channel = channel;
    this.view = null;
    this.rendered = false;
    this.chatStore = normalizeStoredChatState(this.context.workspaceState.get(CHAT_STATE_KEY));
    this.state = {
      sessionList: [],
      activeSessionId: this.chatStore.activeSessionId,
      composerMode: this.chatStore.composerMode || "ask",
      autoApplyEdits: !!this.chatStore.autoApplyEdits,
      modelProfile: normalizeModelProfile(this.chatStore.modelProfile),
      messages: [],
      trace: [],
      status: "Waiting for workspace...",
      isBusy: false,
    };
    this.syncViewState();
  }

  get activeSession() {
    return this.chatStore.sessions.find((session) => session.id === this.chatStore.activeSessionId) || this.chatStore.sessions[0];
  }

  async persistChatStore() {
    await this.context.workspaceState.update(CHAT_STATE_KEY, this.chatStore);
  }

  syncViewState() {
    const session = this.activeSession || createSession();
    this.state.activeSessionId = session.id;
    this.state.composerMode = this.chatStore.composerMode || "ask";
    this.state.autoApplyEdits = !!this.chatStore.autoApplyEdits;
    this.state.modelProfile = normalizeModelProfile(this.chatStore.modelProfile);
    this.state.sessionList = sortSessionsByUpdate(this.chatStore.sessions).map((item) => ({
      id: item.id,
      title: item.title || "New chat",
    }));
    this.state.messages = session.messages.map((message) => ({
      ...message,
      html: renderMessageBody(message),
    }));
  }

  async focus() {
    await vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`);
  }

  refresh() {
    this.syncViewState();
    if (this.view) {
      if (!this.rendered) {
        this.view.webview.html = renderChatHtml(this.view.webview, this.state);
        this.rendered = true;
      } else {
        this.view.webview.postMessage({ type: "state", state: this.state });
      }
    }
  }

  async saveAndRefresh() {
    await this.persistChatStore();
    this.refresh();
  }

  async newSession(seedText = "") {
    const session = createSession(seedText);
    this.chatStore.sessions = [session, ...this.chatStore.sessions].slice(0, MAX_SAVED_SESSIONS);
    this.chatStore.activeSessionId = session.id;
    this.state.trace = [];
    this.state.status = "Started a new chat.";
    await this.saveAndRefresh();
  }

  async clearActiveSession() {
    const session = this.activeSession;
    session.messages = [];
    session.updatedAt = nowIso();
    session.title = "New chat";
    this.state.trace = [];
    this.state.status = "Cleared current chat.";
    await this.saveAndRefresh();
  }

  async switchSession(sessionId) {
    const found = this.chatStore.sessions.find((session) => session.id === sessionId);
    if (!found) {
      return;
    }
    this.chatStore.activeSessionId = found.id;
    this.state.trace = [];
    this.state.status = `Viewing chat: ${found.title}`;
    await this.saveAndRefresh();
  }

  async setComposerMode(mode) {
    const nextMode = ["ask", "agent", "plan"].includes(String(mode || "").toLowerCase())
      ? String(mode).toLowerCase()
      : "ask";
    this.chatStore.composerMode = nextMode;
    this.state.composerMode = nextMode;
    await this.saveAndRefresh();
  }

  async setAutoApplyEdits(value) {
    this.chatStore.autoApplyEdits = !!value;
    this.state.autoApplyEdits = !!value;
    await this.saveAndRefresh();
  }

  async setModelProfile(value) {
    this.chatStore.modelProfile = normalizeModelProfile(value);
    this.state.modelProfile = normalizeModelProfile(value);
    await this.saveAndRefresh();
  }

  pushTrace(label) {
    const items = [...this.state.trace, label].slice(-6);
    this.state.trace = items;
    this.refresh();
  }

  async ask(root, text, mode = "ask", modelProfile = "balanced") {
    const session = this.activeSession;
    const createdAt = nowIso();
    const userMessage = sanitizeMessage({
      id: makeId("msg"),
      role: "user",
      text,
      format: "markdown",
      createdAt,
    });
    session.messages.push(userMessage);
    session.updatedAt = createdAt;
    if (session.messages.filter((message) => message.role === "user").length === 1) {
      session.title = summarizeSessionTitle(text);
    }

    const normalizedMode = ["ask", "agent", "plan"].includes(String(mode || "").toLowerCase())
      ? String(mode).toLowerCase()
      : (this.chatStore.composerMode || "ask");
    this.chatStore.composerMode = normalizedMode;
    this.state.composerMode = normalizedMode;
    const normalizedProfile = normalizeModelProfile(modelProfile || this.chatStore.modelProfile);
    this.chatStore.modelProfile = normalizedProfile;
    this.state.modelProfile = normalizedProfile;

    const commandConfirmation = getCliConfirmationRequest(text, normalizedMode);
    if (commandConfirmation) {
      const assistantMessage = sanitizeMessage({
        id: makeId("msg"),
        role: "assistant",
        text: `${commandConfirmation.title}\n\nI’ll wait for your approval before running this command.`,
        format: "markdown",
        createdAt: nowIso(),
        pendingAction: { ...commandConfirmation, modelProfile: normalizedProfile },
      });
      session.messages.push(assistantMessage);
      session.updatedAt = nowIso();
      this.state.trace = [];
      this.state.status = "Awaiting command confirmation";
      this.state.isBusy = false;
      await this.saveAndRefresh();
      this.view?.webview.postMessage({ type: "done" });
      return;
    }

    this.state.trace = [];
    this.state.status = `Routing ${normalizedMode} request...`;
    this.state.isBusy = true;
    await this.saveAndRefresh();

    const assistantMessage = sanitizeMessage({
      id: makeId("msg"),
      role: "assistant",
      text: "",
      format: "markdown",
      createdAt: nowIso(),
      liveStatus: summarizeRequestActivity("", normalizedMode),
      liveTrace: [],
    });
    session.messages.push(assistantMessage);
    await this.saveAndRefresh();

    try {
      const result = await executeDorkpipeRequest(root, session, text, {
        onEvent: (label) => {
          this.pushTrace(label);
          assistantMessage.liveStatus = summarizeRequestActivity(label, normalizedMode);
          assistantMessage.liveTrace = [...(assistantMessage.liveTrace || []), label].slice(-5);
          this.refresh();
        },
        onToken: (_piece, fullText) => {
          assistantMessage.text = fullText;
          assistantMessage.liveStatus = "Generating a response";
          this.refresh();
        },
        channel: this.channel,
        mode: normalizedMode,
        modelProfile: normalizedProfile,
      });
      assistantMessage.liveStatus = "";
      assistantMessage.liveTrace = [];
      assistantMessage.text = result.text;
      assistantMessage.format = result.format || "markdown";
      session.updatedAt = nowIso();
      this.state.status = result.status;
        if (result.readyToApply?.artifact_dir) {
          const diffPreview = await readPatchPreview(root, result.readyToApply);
          const helperScriptPreview = await readHelperScriptPreview(root, result.readyToApply);
          assistantMessage.diffPreview = diffPreview;
          assistantMessage.pendingAction = sanitizePendingAction({
            kind: "edit",
            title: "Review this code edit",
            artifactDir: result.readyToApply.artifact_dir,
            patchPath: result.readyToApply.patch_path,
            diffPreview,
            helperScriptPath: result.readyToApply.helper_script_path,
            helperScriptPurpose: result.readyToApply.helper_script_purpose,
            helperScriptRuntime: result.readyToApply.helper_script_runtime,
            helperScriptPreview,
            targetFiles: result.readyToApply.target_files,
          });
        if (this.chatStore.autoApplyEdits) {
          this.pushTrace("Applying confirmed edit");
          assistantMessage.liveStatus = "Applying the change";
          assistantMessage.liveTrace = ["Prepared a validated patch artifact", "Applying confirmed edit"];
          this.refresh();
        const applied = await applyPreparedEdit(root, result.readyToApply.artifact_dir, {
            onEvent: (label) => this.pushTrace(label),
            channel: this.channel,
          });
          assistantMessage.liveStatus = "";
          assistantMessage.liveTrace = [];
          assistantMessage.text = `${result.text}\n\n---\n\n${applied.text}`;
          assistantMessage.format = applied.format || "markdown";
          assistantMessage.pendingAction = null;
          this.state.status = applied.status;
        } else {
          this.state.status = "Edit prepared; review diff below";
        }
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      assistantMessage.liveStatus = "";
      assistantMessage.liveTrace = [];
      assistantMessage.text = `DorkPipe error: ${message}`;
      assistantMessage.format = "markdown";
      this.state.status = buildRequestErrorStatus(message);
      this.channel.error(message);
    }

    session.messages = session.messages.slice(-MAX_HISTORY_MESSAGES * 4);
    this.state.isBusy = false;
    await this.saveAndRefresh();
    this.view?.webview.postMessage({ type: "done" });
  }

  async resolvePendingAction(root, messageId, decision) {
    const session = this.activeSession;
    const assistantMessage = session.messages.find((message) => message.id === messageId);
    if (!assistantMessage?.pendingAction) {
      return;
    }
    const pending = assistantMessage.pendingAction;
    if (decision !== "approve") {
      assistantMessage.pendingAction = null;
      assistantMessage.text = `${assistantMessage.text}\n\n_${pending.kind === "edit" ? "Edit" : "Command"} dismissed._`;
      this.state.status = pending.kind === "edit" ? "Edit dismissed" : "Command dismissed";
      await this.saveAndRefresh();
      return;
    }

    this.state.isBusy = true;
    this.state.trace = [];
    this.state.status = pending.kind === "edit" ? "Applying prepared edit..." : "Running confirmed command...";
    await this.saveAndRefresh();

    try {
      if (pending.kind === "edit") {
        this.pushTrace("Applying confirmed edit");
        assistantMessage.liveStatus = "Applying the change";
        assistantMessage.liveTrace = ["Applying confirmed edit"];
        this.refresh();
        const applied = await applyPreparedEdit(root, pending.artifactDir, {
          onEvent: (label) => {
            this.pushTrace(label);
            assistantMessage.liveStatus = summarizeRequestActivity(label, pending.mode);
            assistantMessage.liveTrace = [...(assistantMessage.liveTrace || []), label].slice(-5);
            this.refresh();
          },
          channel: this.channel,
        });
        assistantMessage.liveStatus = "";
        assistantMessage.liveTrace = [];
        assistantMessage.text = `${assistantMessage.text}\n\n---\n\n${applied.text}`;
        assistantMessage.format = applied.format || "markdown";
        assistantMessage.pendingAction = null;
        this.state.status = applied.status;
      } else {
        this.pushTrace("Running confirmed command");
        assistantMessage.liveStatus = "Running the command";
        assistantMessage.liveTrace = ["Running confirmed command"];
        this.refresh();
        launchTerminalCommand(root, summarizeCommandTitle(pending.requestText), pending.requestText);
        assistantMessage.liveStatus = "";
        assistantMessage.liveTrace = [];
        assistantMessage.text = `Started \`${pending.requestText}\` in a terminal.`;
        assistantMessage.format = "markdown";
        assistantMessage.pendingAction = null;
        this.state.status = "Command started in terminal";
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      assistantMessage.liveStatus = "";
      assistantMessage.liveTrace = [];
      assistantMessage.text = `DorkPipe error: ${message}`;
      assistantMessage.format = "markdown";
      assistantMessage.pendingAction = null;
      this.state.status = buildRequestErrorStatus(message);
      this.channel.error(message);
    }

    session.updatedAt = nowIso();
    this.state.isBusy = false;
    await this.saveAndRefresh();
    this.view?.webview.postMessage({ type: "done" });
  }

  resolveWebviewView(webviewView) {
    this.view = webviewView;
    this.rendered = false;
    webviewView.webview.options = { enableScripts: true };
    const root = getWorkspaceRoot();
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
      const workspaceRoot = getWorkspaceRoot();
      if (msg?.type === "switchSession" && msg.sessionId) {
        await this.switchSession(msg.sessionId);
        return;
      }
      if (msg?.type === "newSession") {
        await this.newSession(msg.seed || "");
        return;
      }
      if (msg?.type === "clearSession") {
        await this.clearActiveSession();
        return;
      }
      if (msg?.type === "setComposerMode") {
        await this.setComposerMode(msg.mode);
        return;
      }
      if (msg?.type === "setAutoApplyEdits") {
        await this.setAutoApplyEdits(!!msg.value);
        return;
      }
      if (msg?.type === "setModelProfile") {
        await this.setModelProfile(msg.value);
        return;
      }
      if (!workspaceRoot) {
        vscode.window.showWarningMessage("DorkPipe: open a workspace folder first.");
        return;
      }
      if (msg?.type === "resolvePendingAction" && msg.messageId) {
        await this.resolvePendingAction(workspaceRoot, msg.messageId, msg.decision);
        return;
      }
      if (msg?.type === "ask" && msg.text) {
        await this.ask(workspaceRoot, msg.text, msg.mode, msg.modelProfile);
      }
    });
  }
}

/** @param {vscode.ExtensionContext} context */
function activate(context) {
  const channel = vscode.window.createOutputChannel("DorkPipe", { log: true });
  const chatProvider = new PipeonChatViewProvider(context, channel);

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
    vscode.commands.registerCommand("pipeon.newChat", async () => {
      await chatProvider.newSession();
      await revealDorkpipePanel();
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("pipeon.clearChat", async () => {
      await chatProvider.clearActiveSession();
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
      const root = getWorkspaceRoot();
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
