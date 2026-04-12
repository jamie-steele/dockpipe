(function () {
  let vscode = {
    postMessage() {},
    getState() { return null; },
    setState() {},
  };

  function escapeHtml(value) {
    return String(value || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function renderFatalError(message) {
    const safe = String(message || "Unknown webview error");
    document.body.innerHTML = '<div style="padding:16px;font-family:var(--vscode-font-family);color:var(--vscode-editor-foreground);background:var(--vscode-editor-background);height:100vh;box-sizing:border-box;">'
      + '<article style="max-width:760px;border:1px solid var(--vscode-panel-border);border-radius:14px;padding:16px;background:color-mix(in srgb, var(--vscode-editorWidget-background) 94%, transparent);">'
      + '<div style="font-size:11px;opacity:.8;text-transform:uppercase;letter-spacing:.08em;margin-bottom:8px;">DorkPipe</div>'
      + '<h2 style="margin:0 0 10px;font-size:18px;">The chat UI hit a client-side error.</h2>'
      + '<p style="margin:0 0 10px;line-height:1.5;">The panel stayed alive, but the webview renderer failed.</p>'
      + '<pre style="margin:0;padding:12px;border-radius:10px;overflow:auto;background:color-mix(in srgb, var(--vscode-textCodeBlock-background, #111) 92%, transparent);border:1px solid var(--vscode-panel-border);white-space:pre-wrap;">'
      + safe.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      + "</pre></article></div>";
  }

  function reportClientError(kind, error) {
    const message = error instanceof Error ? (error.stack || error.message) : String(error);
    try {
      vscode.postMessage({ type: "clientError", kind, message });
    } catch {
      // Ignore bridge failures while reporting a renderer error.
    }
    renderFatalError(message);
  }

  function postDiag(stage, extra) {
    try {
      vscode.postMessage({ type: "diag", stage, extra: extra || null });
    } catch {
      // Ignore bridge failures while emitting diagnostics.
    }
  }

  function readInitialState() {
    const node = document.getElementById("pipeon-initial-state");
    if (!node) {
      return {};
    }
    try {
      return JSON.parse(node.textContent || "{}");
    } catch (error) {
      reportClientError("initial-state", error);
      return {};
    }
  }

  function renderCompactDiff(diffText) {
    const raw = String(diffText || "").trim();
    if (!raw) {
      return "";
    }
    const sections = raw.split(/\n{2,}/).map((block) => block.trim()).filter(Boolean);
    const cards = sections.map((section) => {
      const lines = section.split("\n");
      const header = lines.shift() || "";
      const fileMatch = header.match(/^#\s+(.+?)\s+\+(\d+)\s+\-(\d+)$/);
      const fileName = fileMatch ? fileMatch[1] : header.replace(/^#\s+/, "");
      const adds = fileMatch ? fileMatch[2] : "0";
      const removes = fileMatch ? fileMatch[3] : "0";
      const body = lines
        .map((line) => {
          const cls = line.startsWith("+") ? " add" : line.startsWith("-") ? " remove" : "";
          return '<div class="diffLine' + cls + '">' + escapeHtml(line) + "</div>";
        })
        .join("");
      return [
        '<div class="diffFile">',
        '<div class="diffFileHead"><div class="diffFileName">' + escapeHtml(fileName || "file") + "</div><div>+" + adds + " -" + removes + "</div></div>",
        '<div class="diffLines">' + body + "</div>",
        "</div>",
      ].join("");
    });
    return '<div class="diffFiles">' + cards.join("") + "</div>";
  }

  function renderPendingAction(message) {
    const pending = message.pendingAction;
    if (!pending) {
      return "";
    }
    const meta = [];
    if (pending.kind === "edit" && Array.isArray(pending.targetFiles) && pending.targetFiles.length) {
      meta.push(pending.targetFiles.length + " file" + (pending.targetFiles.length === 1 ? "" : "s"));
    }
    if (pending.kind === "edit" && pending.helperScriptRuntime) {
      meta.push("Sidecar: " + pending.helperScriptRuntime);
    }
    if (pending.kind === "command" && pending.requestText) {
      meta.push("Command: " + pending.requestText);
    }
    const files = pending.kind === "edit" && Array.isArray(pending.targetFiles) && pending.targetFiles.length
      ? '<div class="pendingFiles">' + pending.targetFiles.slice(0, 3).map((file) => '<div class="pendingFile">' + escapeHtml(file) + "</div>").join("") + "</div>"
      : "";
    const helper = pending.helperScriptPreview
      ? '<div class="pendingMeta">Uses bounded helper' + (pending.helperScriptPurpose ? ": " + escapeHtml(pending.helperScriptPurpose) : "") + "</div>"
      : "";
    const diff = pending.diffPreview ? renderCompactDiff(pending.diffPreview) : "";
    return [
      '<div class="pendingCard">',
      '<div class="pendingTitle">' + escapeHtml(pending.title || (pending.kind === "edit" ? "Review edit" : "Confirm command")) + "</div>",
      meta.length ? '<div class="pendingMeta">' + escapeHtml(meta.join("  |  ")) + "</div>" : "",
      files,
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
    if (!message.diffPreview || (message.pendingAction && message.pendingAction.kind === "edit")) {
      return "";
    }
    return '<div class="pendingCard"><div class="pendingTitle">Diff preview</div>' + renderCompactDiff(message.diffPreview) + "</div>";
  }

  function renderLiveCard(message) {
    if (!message.liveStatus) {
      return "";
    }
    const chips = Array.isArray(message.liveTrace) && message.liveTrace.length
      ? '<div class="liveTrace">' + message.liveTrace.map((item) => '<div class="liveChip">' + escapeHtml(item) + "</div>").join("") + "</div>"
      : "";
    return '<div class="liveCard"><div class="liveRow"><div class="liveDot"></div><div class="liveTitle">' + escapeHtml(message.liveStatus) + "</div></div>" + chips + "</div>";
  }

  function renderMessages(messages) {
    if (!messages.length) {
      return '<article class="msg assistant"><div class="role">DorkPipe</div><div class="body"><p>Ask about this workspace. DorkPipe will use the local context bundle when available, keep chat history per workspace, and route obvious safe actions locally first.</p></div></article>';
    }
    return messages.map((message) => {
      const role = message.role === "assistant" ? "DorkPipe" : "You";
      return '<article class="msg ' + message.role + '"><div class="role">' + role + '</div><div class="body">' + (message.html || "") + renderLiveCard(message) + renderDiffPreview(message) + renderPendingAction(message) + "</div></article>";
    }).join("");
  }

  function renderTrace(items) {
    if (!items || !items.length) {
      return "";
    }
    return items.map((item) => '<div class="traceItem">' + escapeHtml(item) + "</div>").join("");
  }

  function runChatWebview() {
    if (typeof acquireVsCodeApi === "function") {
      try {
        vscode = acquireVsCodeApi();
      } catch {
        // Keep the stub so the UI can still render a visible error.
      }
    }

    if (typeof acquireVsCodeApi !== "function") {
      reportClientError("boot", "acquireVsCodeApi is unavailable in this webview");
      return;
    }

    postDiag("script-start");
    postDiag("vscode-api-ready", { hasBridge: typeof acquireVsCodeApi === "function" });

    window.addEventListener("error", (event) => {
      reportClientError("error", event?.error || event?.message || "Unknown webview error");
    });

    window.addEventListener("unhandledrejection", (event) => {
      reportClientError("unhandledrejection", event?.reason || "Unhandled promise rejection");
    });

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
    const bootSentinel = document.getElementById("bootSentinel");
    if (bootSentinel) {
      bootSentinel.style.display = "none";
    }

    postDiag("dom-ready", {
      hasForm: !!form,
      hasPrompt: !!prompt,
      hasTranscript: !!transcript,
    });

    const initialState = readInitialState();
    postDiag("initial-state-parsed", {
      keys: Object.keys(initialState || {}),
      messages: Array.isArray(initialState?.messages) ? initialState.messages.length : 0,
    });

    let viewState = (() => {
      const raw = vscode.getState();
      if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
        return { draft: "", pinnedToBottom: true, mode: "ask", modelProfile: "balanced" };
      }
      return {
        draft: typeof raw.draft === "string" ? raw.draft : "",
        pinnedToBottom: raw.pinnedToBottom !== false,
        mode: ["ask", "agent", "plan"].includes(String(raw.mode || "").toLowerCase()) ? String(raw.mode).toLowerCase() : "ask",
        modelProfile: ["fast", "balanced", "deep", "max"].includes(String(raw.modelProfile || "").toLowerCase())
          ? String(raw.modelProfile).toLowerCase()
          : "balanced",
      };
    })();

    let currentState = initialState && typeof initialState === "object" ? initialState : {};

    function renderSessions(nextState) {
      const sessions = nextState.sessionList || [];
      sessionSelect.innerHTML = sessions.map((session) => {
        const selected = session.id === nextState.activeSessionId ? " selected" : "";
        return '<option value="' + escapeHtml(session.id) + '"' + selected + ">" + escapeHtml(session.title) + "</option>";
      }).join("");
    }

    function saveViewState(extra) {
      viewState = { ...viewState, ...extra };
      vscode.setState(viewState);
    }

    function submitPrompt() {
      const text = prompt.value.trim();
      postDiag("submit-attempt", { chars: text.length, mode: modeSelect.value, profile: modelProfileSelect.value });
      if (!text) return;
      prompt.value = "";
      saveViewState({ draft: "", pinnedToBottom: true, mode: modeSelect.value, modelProfile: modelProfileSelect.value });
      vscode.postMessage({ type: "ask", text, mode: modeSelect.value, modelProfile: modelProfileSelect.value });
    }

    function render(nextState) {
      try {
        currentState = nextState || {};
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
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        transcript.innerHTML = '<article class="msg assistant"><div class="role">DorkPipe</div><div class="body"><p>UI render failed.</p><p><code>' + escapeHtml(message) + "</code></p></div></article>";
        status.textContent = "DorkPipe UI hit a render error";
      }
    }

    try {
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
        if (event.isComposing || event.altKey || event.shiftKey || event.ctrlKey || event.metaKey) {
          return;
        }
        event.preventDefault();
        event.stopPropagation();
        submitPrompt();
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
        submitPrompt();
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
        postDiag("host-message", { type: msg.type || "" });
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

      postDiag("listeners-attached");
      render(currentState);
      postDiag("initial-render-complete", {
        messages: Array.isArray(currentState?.messages) ? currentState.messages.length : 0,
      });
      vscode.postMessage({ type: "webviewReady" });
      postDiag("ready-sent");
      prompt.focus();
      postDiag("focus-complete");
    } catch (error) {
      reportClientError("boot", error);
    }
  }

  try {
    runChatWebview();
  } catch (error) {
    renderFatalError(error instanceof Error ? (error.stack || error.message) : String(error));
  }
})();
