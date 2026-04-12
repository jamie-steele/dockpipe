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

  function deepClone(value) {
    try {
      return JSON.parse(JSON.stringify(value));
    } catch {
      return value;
    }
  }

  function clampNumber(value, min, max, fallback) {
    const num = Number(value);
    if (!Number.isFinite(num)) {
      return fallback;
    }
    return Math.min(max, Math.max(min, num));
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
      // ignore bridge failures while reporting renderer failures
    }
    renderFatalError(message);
  }

  function postDiag(stage, extra) {
    try {
      vscode.postMessage({ type: "diag", stage, extra: extra || null });
    } catch {
      // ignore
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

  function templateById(templates, id) {
    const items = Array.isArray(templates) ? templates : [];
    return items.find((template) => template.id === id) || items[0] || null;
  }

  function findNodeMeta(nodes, nodeId, parentNode, container) {
    const items = Array.isArray(nodes) ? nodes : [];
    for (let index = 0; index < items.length; index += 1) {
      const node = items[index];
      if (node.id === nodeId) {
        return { node, index, parentNode: parentNode || null, container, siblings: items };
      }
      if (node.type === "loop" && Array.isArray(node.children)) {
        const found = findNodeMeta(node.children, nodeId, node, node.children);
        if (found) {
          return found;
        }
      }
    }
    return null;
  }

  function removeNodeFromTree(nodes, nodeId) {
    const items = Array.isArray(nodes) ? nodes : [];
    const next = [];
    let removed = null;
    for (const node of items) {
      if (node.id === nodeId) {
        removed = node;
        continue;
      }
      if (node.type === "loop" && Array.isArray(node.children)) {
        const result = removeNodeFromTree(node.children, nodeId);
        if (result.removed) {
          removed = result.removed;
          next.push({ ...node, children: result.nodes });
          continue;
        }
      }
      next.push(node);
    }
    return { nodes: next, removed };
  }

  function addNodeToTarget(template, targetId, node) {
    const draft = deepClone(template);
    if (!targetId || targetId === "root") {
      draft.nodes = [...(Array.isArray(draft.nodes) ? draft.nodes : []), node];
      return draft;
    }
    const meta = findNodeMeta(draft.nodes, targetId, null, draft.nodes);
    if (!meta || meta.node.type !== "loop") {
      draft.nodes = [...(Array.isArray(draft.nodes) ? draft.nodes : []), node];
      return draft;
    }
    meta.node.children = [...(Array.isArray(meta.node.children) ? meta.node.children : []), node];
    return draft;
  }

  function moveNode(template, nodeId, direction) {
    const draft = deepClone(template);
    const meta = findNodeMeta(draft.nodes, nodeId, null, draft.nodes);
    if (!meta || !Array.isArray(meta.siblings)) {
      return draft;
    }
    const offset = direction === "up" ? -1 : 1;
    const targetIndex = meta.index + offset;
    if (targetIndex < 0 || targetIndex >= meta.siblings.length) {
      return draft;
    }
    const siblings = meta.siblings;
    const temp = siblings[meta.index];
    siblings[meta.index] = siblings[targetIndex];
    siblings[targetIndex] = temp;
    return draft;
  }

  function renderTemplateOptions(templates, activeTemplateId) {
    return (Array.isArray(templates) ? templates : []).map((template) => {
      const selected = template.id === activeTemplateId ? " selected" : "";
      const suffix = template.locked ? " (Locked)" : "";
      return '<option value="' + escapeHtml(template.id) + '"' + selected + '>' + escapeHtml(template.name + suffix) + "</option>";
    }).join("");
  }

  function renderModelOptions(modelStore, selectedId) {
    const entries = Array.isArray(modelStore?.entries) ? modelStore.entries : [];
    return entries.map((entry) => {
      const selected = entry.id === selectedId ? " selected" : "";
      return '<option value="' + escapeHtml(entry.id) + '"' + selected + '>' + escapeHtml(entry.label) + "</option>";
    }).join("");
  }

  function renderDesignerNode(node, selectedNodeId) {
    const isSelected = node.id === selectedNodeId;
    const badges = [];
    if (node.type === "model" && node.config?.modelId) {
      badges.push(node.config.modelId);
    }
    if (node.type === "loop") {
      badges.push("x" + String(node.config?.maxIterations || 1));
    }
    if (node.type === "dockpipe" && node.config?.phase) {
      badges.push(node.config.phase);
    }
    return [
      '<div class="designerNode' + (isSelected ? " selected" : "") + '" draggable="true" data-node-id="' + escapeHtml(node.id) + '">',
      '<div class="designerNodeHead">',
      '<button class="nodeMain" type="button" data-node-select="' + escapeHtml(node.id) + '">',
      '<span class="nodeType">' + escapeHtml(node.type) + "</span>",
      '<span class="nodeLabel">' + escapeHtml(node.label || node.type) + "</span>",
      "</button>",
      '<div class="nodeActions">',
      '<button class="nodeAction" type="button" data-node-move="up" data-node-id="' + escapeHtml(node.id) + '">↑</button>',
      '<button class="nodeAction" type="button" data-node-move="down" data-node-id="' + escapeHtml(node.id) + '">↓</button>',
      "</div>",
      "</div>",
      node.notes ? '<div class="nodeNotes">' + escapeHtml(node.notes) + "</div>" : "",
      badges.length ? '<div class="nodeBadges">' + badges.map((badge) => '<span class="nodeBadge">' + escapeHtml(badge) + "</span>").join("") + "</div>" : "",
      node.type === "loop"
        ? '<div class="loopBody">'
          + '<div class="dropZone" data-drop-target="' + escapeHtml(node.id) + '">Drop into loop</div>'
          + ((Array.isArray(node.children) && node.children.length)
              ? node.children.map((child) => renderDesignerNode(child, selectedNodeId)).join("")
              : '<div class="loopEmpty">Loop body is empty.</div>')
          + "</div>"
        : "",
      "</div>",
    ].join("");
  }

  function renderModelStore(modelStore) {
    const entries = Array.isArray(modelStore?.entries) ? modelStore.entries : [];
    if (!entries.length) {
      return '<div class="emptyBlock">No models are registered yet.</div>';
    }
    return entries.map((entry) => {
      const capabilities = Array.isArray(entry.capabilities) && entry.capabilities.length
        ? '<div class="modelCapabilities">' + entry.capabilities.map((item) => '<span class="nodeBadge">' + escapeHtml(item) + "</span>").join("") + "</div>"
        : "";
      const actions = entry.locked
        ? '<div class="modelMeta muted">Built-in and locked</div>'
        : '<button class="btn ghost compact danger" type="button" data-delete-model-id="' + escapeHtml(entry.id) + '">Delete</button>';
      return [
        '<article class="modelCard">',
        '<div class="modelCardHead"><div><div class="modelLabel">' + escapeHtml(entry.label) + '</div><div class="modelMeta">' + escapeHtml(entry.provider + " · " + entry.model) + '</div></div>' + actions + '</div>',
        capabilities,
        entry.notes ? '<div class="modelMeta">' + escapeHtml(entry.notes) + '</div>' : "",
        "</article>",
      ].join("");
    }).join("");
  }

  function readCapabilities(value) {
    return String(value || "")
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
  }

  function runChatWebview() {
    if (typeof acquireVsCodeApi === "function") {
      try {
        vscode = acquireVsCodeApi();
      } catch {
        // keep stub
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
    const headerMeta = document.getElementById("headerMeta");
    const settingsBtn = document.getElementById("settingsBtn");
    const settingsDrawer = document.getElementById("settingsDrawer");
    const settingsCloseBtn = document.getElementById("settingsCloseBtn");
    const templateSelect = document.getElementById("templateSelect");
    const copyTemplateBtn = document.getElementById("copyTemplateBtn");
    const newTemplateBtn = document.getElementById("newTemplateBtn");
    const deleteTemplateBtn = document.getElementById("deleteTemplateBtn");
    const saveTemplateBtn = document.getElementById("saveTemplateBtn");
    const designerCanvas = document.getElementById("designerCanvas");
    const inspectorTarget = document.getElementById("inspectorTarget");
    const templateNameInput = document.getElementById("templateNameInput");
    const templateDescriptionInput = document.getElementById("templateDescriptionInput");
    const templateSafetyRulesInput = document.getElementById("templateSafetyRulesInput");
    const templateOutputRequirementsInput = document.getElementById("templateOutputRequirementsInput");
    const templateConfidenceInput = document.getElementById("templateConfidenceInput");
    const templateExecutionConstraintsInput = document.getElementById("templateExecutionConstraintsInput");
    const templateRoutingPreferenceInput = document.getElementById("templateRoutingPreferenceInput");
    const templateOrchestrationGuidanceInput = document.getElementById("templateOrchestrationGuidanceInput");
    const templateModelGuidanceInput = document.getElementById("templateModelGuidanceInput");
    const nodeLabelInput = document.getElementById("nodeLabelInput");
    const nodeNotesInput = document.getElementById("nodeNotesInput");
    const nodeDecisionInput = document.getElementById("nodeDecisionInput");
    const nodeOrchestrationInput = document.getElementById("nodeOrchestrationInput");
    const nodeModelInput = document.getElementById("nodeModelInput");
    const nodeModelSelect = document.getElementById("nodeModelSelect");
    const nodeTaskInput = document.getElementById("nodeTaskInput");
    const nodeOutputInput = document.getElementById("nodeOutputInput");
    const nodeLoopIterationsInput = document.getElementById("nodeLoopIterationsInput");
    const nodeModelField = document.getElementById("nodeModelField");
    const nodeTaskField = document.getElementById("nodeTaskField");
    const nodeOutputField = document.getElementById("nodeOutputField");
    const nodeLoopIterationsField = document.getElementById("nodeLoopIterationsField");
    const removeNodeBtn = document.getElementById("removeNodeBtn");
    const modelStoreSummary = document.getElementById("modelStoreSummary");
    const modelStoreList = document.getElementById("modelStoreList");
    const modelEntryIdInput = document.getElementById("modelEntryIdInput");
    const modelEntryLabelInput = document.getElementById("modelEntryLabelInput");
    const modelEntryProviderInput = document.getElementById("modelEntryProviderInput");
    const modelEntryModelInput = document.getElementById("modelEntryModelInput");
    const modelEntryCapabilitiesInput = document.getElementById("modelEntryCapabilitiesInput");
    const modelEntryContextWindowInput = document.getElementById("modelEntryContextWindowInput");
    const modelEntryNotesInput = document.getElementById("modelEntryNotesInput");
    const saveModelEntryBtn = document.getElementById("saveModelEntryBtn");
    const hasSettingsSurface = !!(
      settingsBtn &&
      settingsDrawer &&
      settingsCloseBtn &&
      templateSelect &&
      designerCanvas &&
      modelStoreList
    );

    if (bootSentinel) {
      bootSentinel.style.display = "none";
    }

    postDiag("dom-ready", {
      hasForm: !!form,
      hasPrompt: !!prompt,
      hasTranscript: !!transcript,
      hasSettings: hasSettingsSurface,
    });

    const initialState = readInitialState();
    postDiag("initial-state-parsed", {
      keys: Object.keys(initialState || {}),
      messages: Array.isArray(initialState?.messages) ? initialState.messages.length : 0,
      templates: Array.isArray(initialState?.reasoningTemplates) ? initialState.reasoningTemplates.length : 0,
    });

    let viewState = (() => {
      const raw = vscode.getState();
      if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
        return {
          draft: "",
          pinnedToBottom: true,
          mode: "ask",
          modelProfile: "balanced",
          settingsOpen: false,
          selectedNodeId: "",
        };
      }
      return {
        draft: typeof raw.draft === "string" ? raw.draft : "",
        pinnedToBottom: raw.pinnedToBottom !== false,
        mode: ["ask", "agent", "plan"].includes(String(raw.mode || "").toLowerCase()) ? String(raw.mode).toLowerCase() : "ask",
        modelProfile: ["fast", "balanced", "deep", "max"].includes(String(raw.modelProfile || "").toLowerCase()) ? String(raw.modelProfile).toLowerCase() : "balanced",
        settingsOpen: !!raw.settingsOpen,
        selectedNodeId: typeof raw.selectedNodeId === "string" ? raw.selectedNodeId : "",
      };
    })();

    let currentState = initialState && typeof initialState === "object" ? initialState : {};
    let dragPayload = null;
    let workingTemplate = null;

    function saveViewState(extra) {
      viewState = { ...viewState, ...extra };
      vscode.setState(viewState);
    }

    function currentTemplate() {
      const template = templateById(currentState.reasoningTemplates || [], currentState.activeTemplateId);
      return workingTemplate && template && workingTemplate.id === template.id ? workingTemplate : deepClone(template);
    }

    function currentNode() {
      if (!workingTemplate || !viewState.selectedNodeId) {
        return null;
      }
      const meta = findNodeMeta(workingTemplate.nodes || [], viewState.selectedNodeId, null, workingTemplate.nodes || []);
      return meta ? meta.node : null;
    }

    function setSettingsOpen(open) {
      if (!hasSettingsSurface) {
        return;
      }
      saveViewState({ settingsOpen: !!open });
      settingsDrawer.classList.toggle("hidden", !open);
      settingsDrawer.setAttribute("aria-hidden", open ? "false" : "true");
      settingsBtn.classList.toggle("active", !!open);
    }

    function syncTemplateFromState() {
      workingTemplate = currentTemplate();
      if (!workingTemplate) {
        return;
      }
      if (viewState.selectedNodeId) {
        const node = findNodeMeta(workingTemplate.nodes || [], viewState.selectedNodeId, null, workingTemplate.nodes || []);
        if (!node) {
          saveViewState({ selectedNodeId: "" });
        }
      }
    }

    function renderSessions(nextState) {
      const sessions = nextState.sessionList || [];
      sessionSelect.innerHTML = sessions.map((session) => {
        const selected = session.id === nextState.activeSessionId ? " selected" : "";
        return '<option value="' + escapeHtml(session.id) + '"' + selected + ">" + escapeHtml(session.title) + "</option>";
      }).join("");
    }

    function renderSettings(nextState) {
      if (!hasSettingsSurface) {
        if (headerMeta) {
          const templatePart = nextState.activeTemplate ? `Template: ${nextState.activeTemplate.name}` : "";
          const shellPart = nextState.shellVersion ? `Shell: ${nextState.shellVersion}` : "";
          headerMeta.textContent = [templatePart, shellPart].filter(Boolean).join("  •  ");
        }
        return;
      }
      syncTemplateFromState();
      const template = workingTemplate;
      const locked = !!template?.locked;
      const node = currentNode();

      {
        const templatePart = nextState.activeTemplate ? `Template: ${nextState.activeTemplate.name}` : "Template: DockPipe Default";
        const shellPart = nextState.shellVersion ? `Shell: ${nextState.shellVersion}` : "";
        headerMeta.textContent = [templatePart, shellPart].filter(Boolean).join("  •  ");
      }
      templateSelect.innerHTML = renderTemplateOptions(nextState.reasoningTemplates || [], nextState.activeTemplateId);
      deleteTemplateBtn.disabled = locked;
      saveTemplateBtn.disabled = locked || !template;
      copyTemplateBtn.disabled = !template;

      if (!template) {
        designerCanvas.innerHTML = '<div class="emptyBlock">No reasoning template is active.</div>';
        inspectorTarget.textContent = "Template";
        return;
      }

      templateNameInput.value = template.name || "";
      templateDescriptionInput.value = template.description || "";
      templateSafetyRulesInput.value = template.globalModifiers?.safetyRules || "";
      templateOutputRequirementsInput.value = template.globalModifiers?.outputRequirements || "";
      templateConfidenceInput.value = String(template.globalModifiers?.confidenceThreshold ?? 0.72);
      templateExecutionConstraintsInput.value = template.globalModifiers?.executionConstraints || "";
      templateRoutingPreferenceInput.value = template.globalModifiers?.routingPreference || "";
      templateOrchestrationGuidanceInput.value = template.guidance?.orchestration || "";
      templateModelGuidanceInput.value = template.guidance?.model || "";

      templateNameInput.disabled = locked;
      templateDescriptionInput.disabled = locked;
      templateSafetyRulesInput.disabled = locked;
      templateOutputRequirementsInput.disabled = locked;
      templateConfidenceInput.disabled = locked;
      templateExecutionConstraintsInput.disabled = locked;
      templateRoutingPreferenceInput.disabled = locked;
      templateOrchestrationGuidanceInput.disabled = locked;
      templateModelGuidanceInput.disabled = locked;

      const canvasBody = Array.isArray(template.nodes) && template.nodes.length
        ? template.nodes.map((item) => renderDesignerNode(item, viewState.selectedNodeId)).join("")
        : '<div class="emptyBlock">Drop a primitive here to start designing a custom flow.</div>';
      designerCanvas.innerHTML = '<div class="dropZone root" data-drop-target="root">Drop on root canvas</div>' + canvasBody;

      const entries = Array.isArray(nextState.modelStore?.entries) ? nextState.modelStore.entries : [];
      modelStoreSummary.textContent = entries.length
        ? `${entries.length} registered models · active template uses ${template.name}`
        : "No models registered yet.";
      modelStoreList.innerHTML = renderModelStore(nextState.modelStore || { entries: [] });

      if (!node) {
        inspectorTarget.textContent = locked ? "Locked template" : "Template";
        nodeLabelInput.value = "";
        nodeNotesInput.value = "";
        nodeDecisionInput.value = "";
        nodeOrchestrationInput.value = "";
        nodeModelInput.value = "";
        nodeTaskInput.value = "";
        nodeOutputInput.value = "";
        nodeLoopIterationsInput.value = "2";
        nodeModelSelect.innerHTML = renderModelOptions(nextState.modelStore || { entries: [] }, "");
        nodeLabelInput.disabled = true;
        nodeNotesInput.disabled = true;
        nodeDecisionInput.disabled = true;
        nodeOrchestrationInput.disabled = true;
        nodeModelInput.disabled = true;
        nodeModelSelect.disabled = true;
        nodeTaskInput.disabled = true;
        nodeOutputInput.disabled = true;
        nodeLoopIterationsInput.disabled = true;
        removeNodeBtn.disabled = true;
        nodeModelField.classList.add("hidden");
        nodeTaskField.classList.add("hidden");
        nodeOutputField.classList.add("hidden");
        nodeLoopIterationsField.classList.add("hidden");
        return;
      }

      inspectorTarget.textContent = node.label || node.type;
      nodeLabelInput.value = node.label || "";
      nodeNotesInput.value = node.notes || "";
      nodeDecisionInput.value = node.decision || "";
      nodeOrchestrationInput.value = node.guidance?.orchestration || "";
      nodeModelInput.value = node.guidance?.model || "";
      nodeTaskInput.value = node.config?.task || node.config?.phase || "";
      nodeOutputInput.value = node.config?.output || node.config?.artifactMode || "";
      nodeLoopIterationsInput.value = String(node.config?.maxIterations || 2);
      nodeModelSelect.innerHTML = renderModelOptions(nextState.modelStore || { entries: [] }, node.config?.modelId || "");

      const disableNodeFields = locked;
      nodeLabelInput.disabled = disableNodeFields;
      nodeNotesInput.disabled = disableNodeFields;
      nodeDecisionInput.disabled = disableNodeFields;
      nodeOrchestrationInput.disabled = disableNodeFields;
      nodeModelInput.disabled = disableNodeFields;
      nodeModelSelect.disabled = disableNodeFields;
      nodeTaskInput.disabled = disableNodeFields;
      nodeOutputInput.disabled = disableNodeFields;
      nodeLoopIterationsInput.disabled = disableNodeFields;
      removeNodeBtn.disabled = disableNodeFields;

      nodeModelField.classList.toggle("hidden", node.type !== "model");
      nodeTaskField.classList.remove("hidden");
      nodeOutputField.classList.toggle("hidden", node.type === "loop");
      nodeLoopIterationsField.classList.toggle("hidden", node.type !== "loop");
    }

    function render(nextState) {
      try {
        currentState = nextState || {};
        syncTemplateFromState();
        const previousBottomOffset = transcript.scrollHeight - transcript.scrollTop;
        const stickToBottom = previousBottomOffset - transcript.clientHeight <= 24 || !!viewState.pinnedToBottom;
        transcript.innerHTML = renderMessages(currentState.messages || []);
        trace.innerHTML = renderTrace(currentState.trace || []);
        status.textContent = currentState.status || "";
        renderSessions(currentState);
        renderSettings(currentState);
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

    function persistWorkingTemplate() {
      if (!workingTemplate) {
        return;
      }
      vscode.postMessage({ type: "saveTemplate", template: workingTemplate });
    }

    function updateWorkingTemplate(updater) {
      if (!workingTemplate || workingTemplate.locked) {
        return;
      }
      const next = deepClone(workingTemplate);
      updater(next);
      next.updatedAt = new Date().toISOString();
      workingTemplate = next;
      renderSettings(currentState);
    }

    function updateCurrentNode(updater) {
      if (!workingTemplate || workingTemplate.locked || !viewState.selectedNodeId) {
        return;
      }
      updateWorkingTemplate((draft) => {
        const meta = findNodeMeta(draft.nodes || [], viewState.selectedNodeId, null, draft.nodes || []);
        if (meta?.node) {
          updater(meta.node);
        }
      });
    }

    function handleDrop(targetId) {
      if (!workingTemplate || workingTemplate.locked || !dragPayload) {
        return;
      }
      updateWorkingTemplate((draft) => {
        let nodeToInsert = null;
        if (dragPayload.kind === "primitive") {
          nodeToInsert = dragPayload.factory();
        } else if (dragPayload.kind === "node" && dragPayload.nodeId) {
          const removed = removeNodeFromTree(draft.nodes || [], dragPayload.nodeId);
          draft.nodes = removed.nodes;
          nodeToInsert = removed.removed;
        }
        if (!nodeToInsert) {
          return;
        }
        const updated = addNodeToTarget(draft, targetId, nodeToInsert);
        draft.nodes = updated.nodes;
      });
      dragPayload = null;
    }

    function submitPrompt() {
      const text = prompt.value.trim();
      postDiag("submit-attempt", { chars: text.length, mode: modeSelect.value, profile: modelProfileSelect.value });
      if (!text) return;
      prompt.value = "";
      saveViewState({ draft: "", pinnedToBottom: true, mode: modeSelect.value, modelProfile: modelProfileSelect.value });
      vscode.postMessage({ type: "ask", text, mode: modeSelect.value, modelProfile: modelProfileSelect.value });
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
        saveViewState({ modelProfile: modelProfileSelect.value });
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

      if (hasSettingsSurface) {
        settingsBtn.addEventListener("click", () => {
          setSettingsOpen(!viewState.settingsOpen);
        });
        settingsCloseBtn.addEventListener("click", () => {
          setSettingsOpen(false);
        });
        templateSelect.addEventListener("change", () => {
          vscode.postMessage({ type: "setActiveTemplate", templateId: templateSelect.value });
          saveViewState({ selectedNodeId: "" });
        });
        copyTemplateBtn.addEventListener("click", () => {
          vscode.postMessage({ type: "createTemplate", templateId: templateSelect.value });
        });
        newTemplateBtn.addEventListener("click", () => {
          vscode.postMessage({ type: "createTemplate", templateId: "__blank__" });
        });
        deleteTemplateBtn.addEventListener("click", () => {
          if (!workingTemplate || workingTemplate.locked) {
            return;
          }
          vscode.postMessage({ type: "deleteTemplate", templateId: workingTemplate.id });
          saveViewState({ selectedNodeId: "" });
        });
        saveTemplateBtn.addEventListener("click", () => {
          persistWorkingTemplate();
        });

        designerCanvas.addEventListener("click", (event) => {
          const target = event.target instanceof HTMLElement ? event.target : null;
          if (!target) return;
          const selectBtn = target.closest("[data-node-select]");
          if (selectBtn) {
            saveViewState({ selectedNodeId: selectBtn.getAttribute("data-node-select") || "" });
            renderSettings(currentState);
            return;
          }
          const moveBtn = target.closest("[data-node-move]");
          if (moveBtn) {
            const nodeId = moveBtn.getAttribute("data-node-id");
            const direction = moveBtn.getAttribute("data-node-move");
            updateWorkingTemplate((draft) => {
              const moved = moveNode(draft, nodeId, direction);
              draft.nodes = moved.nodes;
            });
          }
        });
        designerCanvas.addEventListener("dragstart", (event) => {
          const target = event.target instanceof HTMLElement ? event.target.closest("[data-node-id]") : null;
          if (!target) return;
          dragPayload = { kind: "node", nodeId: target.getAttribute("data-node-id") || "" };
        });
        designerCanvas.addEventListener("dragover", (event) => {
          if (!dragPayload) return;
          event.preventDefault();
        });
        designerCanvas.addEventListener("drop", (event) => {
          const target = event.target instanceof HTMLElement ? event.target.closest("[data-drop-target]") : null;
          if (!target) return;
          event.preventDefault();
          handleDrop(target.getAttribute("data-drop-target"));
        });
        for (const primitive of document.querySelectorAll(".primitiveTile")) {
          primitive.addEventListener("dragstart", () => {
            const type = primitive.getAttribute("data-primitive-type") || "dockpipe";
            dragPayload = {
              kind: "primitive",
              factory() {
                if (type === "model") {
                  return {
                    id: "node_" + Date.now().toString(36),
                    type: "model",
                    label: "Model Step",
                    notes: "",
                    decision: "",
                    guidance: { orchestration: "", model: "" },
                    config: {
                      modelId: (currentState.modelStore?.entries || [])[0]?.id || "ollama.default",
                      task: "reason",
                      output: "artifact-or-answer",
                    },
                  };
                }
                if (type === "loop") {
                  return {
                    id: "node_" + Date.now().toString(36),
                    type: "loop",
                    label: "Loop Control",
                    notes: "",
                    decision: "",
                    guidance: { orchestration: "", model: "" },
                    config: { maxIterations: 2, stopCondition: "artifact-valid" },
                    children: [],
                  };
                }
                return {
                  id: "node_" + Date.now().toString(36),
                  type: "dockpipe",
                  label: "DockPipe Step",
                  notes: "",
                  decision: "",
                  guidance: { orchestration: "", model: "" },
                  config: { phase: "route-and-prepare", localFirst: true, artifactMode: "strict" },
                };
              },
            };
          });
        }

        modelStoreList.addEventListener("click", (event) => {
          const target = event.target instanceof HTMLElement ? event.target.closest("[data-delete-model-id]") : null;
          if (!target) return;
          vscode.postMessage({ type: "deleteModelEntry", modelId: target.getAttribute("data-delete-model-id") || "" });
        });
        saveModelEntryBtn.addEventListener("click", () => {
          const entry = {
            id: modelEntryIdInput.value.trim(),
            label: modelEntryLabelInput.value.trim(),
            provider: modelEntryProviderInput.value.trim() || "ollama",
            model: modelEntryModelInput.value.trim(),
            capabilities: readCapabilities(modelEntryCapabilitiesInput.value),
            contextWindow: Number(modelEntryContextWindowInput.value || 8192),
            notes: modelEntryNotesInput.value.trim(),
          };
          if (!entry.id || !entry.label || !entry.model) {
            return;
          }
          vscode.postMessage({ type: "upsertModelEntry", entry });
          modelEntryIdInput.value = "";
          modelEntryLabelInput.value = "";
          modelEntryModelInput.value = "";
          modelEntryCapabilitiesInput.value = "";
          modelEntryContextWindowInput.value = "8192";
          modelEntryNotesInput.value = "";
        });

        templateNameInput.addEventListener("input", () => updateWorkingTemplate((draft) => { draft.name = templateNameInput.value; }));
        templateDescriptionInput.addEventListener("input", () => updateWorkingTemplate((draft) => { draft.description = templateDescriptionInput.value; }));
        templateSafetyRulesInput.addEventListener("input", () => updateWorkingTemplate((draft) => { draft.globalModifiers.safetyRules = templateSafetyRulesInput.value; }));
        templateOutputRequirementsInput.addEventListener("input", () => updateWorkingTemplate((draft) => { draft.globalModifiers.outputRequirements = templateOutputRequirementsInput.value; }));
        templateConfidenceInput.addEventListener("input", () => updateWorkingTemplate((draft) => { draft.globalModifiers.confidenceThreshold = clampNumber(templateConfidenceInput.value, 0, 1, 0.72); }));
        templateExecutionConstraintsInput.addEventListener("input", () => updateWorkingTemplate((draft) => { draft.globalModifiers.executionConstraints = templateExecutionConstraintsInput.value; }));
        templateRoutingPreferenceInput.addEventListener("input", () => updateWorkingTemplate((draft) => { draft.globalModifiers.routingPreference = templateRoutingPreferenceInput.value; }));
        templateOrchestrationGuidanceInput.addEventListener("input", () => updateWorkingTemplate((draft) => { draft.guidance.orchestration = templateOrchestrationGuidanceInput.value; }));
        templateModelGuidanceInput.addEventListener("input", () => updateWorkingTemplate((draft) => { draft.guidance.model = templateModelGuidanceInput.value; }));

        nodeLabelInput.addEventListener("input", () => updateCurrentNode((node) => { node.label = nodeLabelInput.value; }));
        nodeNotesInput.addEventListener("input", () => updateCurrentNode((node) => { node.notes = nodeNotesInput.value; }));
        nodeDecisionInput.addEventListener("input", () => updateCurrentNode((node) => { node.decision = nodeDecisionInput.value; }));
        nodeOrchestrationInput.addEventListener("input", () => updateCurrentNode((node) => { node.guidance.orchestration = nodeOrchestrationInput.value; }));
        nodeModelInput.addEventListener("input", () => updateCurrentNode((node) => { node.guidance.model = nodeModelInput.value; }));
        nodeModelSelect.addEventListener("change", () => updateCurrentNode((node) => {
          node.config = node.config || {};
          node.config.modelId = nodeModelSelect.value;
        }));
        nodeTaskInput.addEventListener("input", () => updateCurrentNode((node) => {
          node.config = node.config || {};
          if (node.type === "dockpipe") {
            node.config.phase = nodeTaskInput.value;
          } else {
            node.config.task = nodeTaskInput.value;
          }
        }));
        nodeOutputInput.addEventListener("input", () => updateCurrentNode((node) => {
          node.config = node.config || {};
          if (node.type === "dockpipe") {
            node.config.artifactMode = nodeOutputInput.value;
          } else {
            node.config.output = nodeOutputInput.value;
          }
        }));
        nodeLoopIterationsInput.addEventListener("input", () => updateCurrentNode((node) => {
          node.config = node.config || {};
          node.config.maxIterations = clampNumber(nodeLoopIterationsInput.value, 1, 8, 2);
        }));
        removeNodeBtn.addEventListener("click", () => {
          if (!workingTemplate || workingTemplate.locked || !viewState.selectedNodeId) {
            return;
          }
          updateWorkingTemplate((draft) => {
            const removed = removeNodeFromTree(draft.nodes || [], viewState.selectedNodeId);
            draft.nodes = removed.nodes;
          });
          saveViewState({ selectedNodeId: "" });
          renderSettings(currentState);
        });
      }

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
      setSettingsOpen(!!viewState.settingsOpen);
      postDiag("initial-render-complete", {
        messages: Array.isArray(currentState?.messages) ? currentState.messages.length : 0,
      });
      vscode.postMessage({ type: "webviewReady", shellVersion: currentState?.shellVersion || "" });
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
