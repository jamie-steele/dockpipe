#!/usr/bin/env node
const fs = require("fs");
const vm = require("vm");
const path = require("path");

const sourcePath = path.resolve(__dirname, "..", "webview", "chat.js");
const scriptText = fs.readFileSync(sourcePath, "utf8");

class FakeElement {
  constructor(id) {
    this.id = id;
    this.value = "";
    this.disabled = false;
    this.checked = false;
    this.innerHTML = "";
    this.textContent = "";
    this.style = {};
    this.scrollHeight = 100;
    this.scrollTop = 0;
    this.clientHeight = 80;
    this.dataset = {};
    this.className = "";
    this.listeners = new Map();
    this.attributes = new Map();
    const syncClassName = (set) => {
      this.className = Array.from(set).join(" ");
    };
    const classSet = new Set();
    this.classList = {
      add: (...tokens) => {
        for (const token of tokens) {
          if (token) classSet.add(token);
        }
        syncClassName(classSet);
      },
      remove: (...tokens) => {
        for (const token of tokens) {
          classSet.delete(token);
        }
        syncClassName(classSet);
      },
      toggle: (token, force) => {
        if (!token) return false;
        let present = classSet.has(token);
        if (typeof force === "boolean") {
          if (force) classSet.add(token);
          else classSet.delete(token);
          present = force;
        } else if (present) {
          classSet.delete(token);
          present = false;
        } else {
          classSet.add(token);
          present = true;
        }
        syncClassName(classSet);
        return present;
      },
    };
  }

  addEventListener(type, handler) {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, []);
    }
    this.listeners.get(type).push(handler);
  }

  focus() {}

  closest() {
    return null;
  }

  getAttribute(name) {
    return this.attributes.get(name) || this[name] || null;
  }

  setAttribute(name, value) {
    this.attributes.set(name, value);
  }
}

const elements = new Map();
function getElement(id) {
  if (!elements.has(id)) {
    elements.set(id, new FakeElement(id));
  }
  return elements.get(id);
}

const windowListeners = new Map();
const posted = [];

const context = {
  console,
  acquireVsCodeApi() {
    return {
      postMessage(message) {
        posted.push(message);
      },
      getState() {
        return null;
      },
      setState() {},
    };
  },
  document: {
    getElementById(id) {
      return getElement(id);
    },
    querySelector(selector) {
      if (selector === ".composerWrap") {
        return getElement("composerWrap");
      }
      if (selector === ".header") {
        return getElement("header");
      }
      if (selector === ".headerActions") {
        return getElement("headerActions");
      }
      return null;
    },
    querySelectorAll(selector) {
      if (selector === ".primitiveTile") {
        const dockpipe = getElement("primitive-dockpipe");
        dockpipe.setAttribute("data-primitive-type", "dockpipe");
        const model = getElement("primitive-model");
        model.setAttribute("data-primitive-type", "model");
        const loop = getElement("primitive-loop");
        loop.setAttribute("data-primitive-type", "loop");
        return [dockpipe, model, loop];
      }
      return [];
    },
    body: new FakeElement("body"),
  },
  window: {
    addEventListener(type, handler) {
      if (!windowListeners.has(type)) {
        windowListeners.set(type, []);
      }
      windowListeners.get(type).push(handler);
    },
  },
  HTMLElement: FakeElement,
};

vm.createContext(context);
getElement("pipeon-initial-state").textContent = JSON.stringify({
  shellVersion: "reasoning-templates-v1",
  messages: [{ id: "m1", role: "assistant", html: "<p>hi</p>" }],
  reasoningTemplates: [{ id: "dockpipe.default", name: "DockPipe Default", locked: true, builtIn: true }],
  activeTemplate: { id: "dockpipe.default", name: "DockPipe Default" },
  activeTemplateId: "dockpipe.default",
  modelStore: { entries: [{ id: "ollama.default", label: "Ollama Default" }] },
});
new vm.Script(scriptText, { filename: "pipeon-webview-inline.js" }).runInContext(context);

const stages = posted.filter((item) => item && item.type === "diag").map((item) => item.stage);
const ready = posted.some((item) => item && item.type === "webviewReady");
const clientError = posted.find((item) => item && item.type === "clientError");

if (clientError) {
  throw new Error(`Webview reported client error: ${clientError.kind}: ${clientError.message}`);
}

if (!ready) {
  throw new Error(`Webview did not report ready. Diag stages: ${stages.join(", ")}`);
}

const requiredStages = [
  "script-start",
  "vscode-api-ready",
  "dom-ready",
  "initial-state-parsed",
  "listeners-attached",
  "initial-render-complete",
  "ready-sent",
];

for (const stage of requiredStages) {
  if (!stages.includes(stage)) {
    throw new Error(`Missing diag stage: ${stage}. Seen: ${stages.join(", ")}`);
  }
}

const initialStateDiag = posted.find((item) => item && item.type === "diag" && item.stage === "initial-state-parsed");
if (!initialStateDiag || !initialStateDiag.extra || initialStateDiag.extra.messages !== 1 || initialStateDiag.extra.templates !== 1) {
  throw new Error(`Initial state was not parsed as the expected object. Diag: ${JSON.stringify(initialStateDiag)}`);
}

const readyMessage = posted.find((item) => item && item.type === "webviewReady");
if (!readyMessage || readyMessage.shellVersion !== "reasoning-templates-v1") {
  throw new Error(`Webview ready message did not carry the expected shell version. Got: ${JSON.stringify(readyMessage)}`);
}

const prompt = getElement("prompt");
const mode = getElement("modeSelect");
const profile = getElement("modelProfileSelect");
prompt.value = "hello";
mode.value = "ask";
profile.value = "balanced";
const promptKeydown = prompt.listeners.get("keydown") || [];
for (const handler of promptKeydown) {
  handler({
    key: "Enter",
    isComposing: false,
    altKey: false,
    shiftKey: false,
    ctrlKey: false,
    metaKey: false,
    preventDefault() {},
    stopPropagation() {},
  });
}

const askMessage = posted.find((item) => item && item.type === "ask");
if (!askMessage) {
  throw new Error("Enter key path did not emit an ask message.");
}

const settingsBtn = getElement("settingsBtn");
const header = getElement("header");
const studioSurface = getElement("studioSurface");
const settingsStudio = getElement("settingsStudio");
const openModelManagerBtn = getElement("openModelManagerBtn");
const studioBackBtn = getElement("studioBackBtn");
const headerActions = getElement("headerActions");
if (!settingsBtn.listeners.has("click")) {
  throw new Error("Settings button did not attach a click handler.");
}
for (const handler of settingsBtn.listeners.get("click") || []) {
  handler({});
}
const openStudioMessage = posted.find((item) => item && item.type === "openReasoningStudio");
if (!openStudioMessage || openStudioMessage.mode !== "settings") {
  throw new Error(`Settings click did not request the main settings studio. Got: ${JSON.stringify(openStudioMessage)}`);
}
const messageHandlers = windowListeners.get("message") || [];
for (const handler of messageHandlers) {
  handler({ data: { type: "forceOpenSettings", mode: "settings" } });
}
if (studioSurface.className.includes("hidden")) {
  throw new Error("Force-opening settings did not reveal the main studio surface.");
}
if (settingsStudio.className.includes("hidden")) {
  throw new Error("Force-opening settings did not reveal the settings summary surface.");
}
if (!header.className.includes("hidden")) {
  throw new Error("Settings mode should hide the top chat header.");
}
if (!headerActions.className.includes("hidden")) {
  throw new Error("Settings mode should hide the chat header actions.");
}
if (!studioBackBtn.className.includes("hidden")) {
  throw new Error("Settings mode should hide the back button.");
}
if (!openModelManagerBtn.listeners.has("click")) {
  throw new Error("Model manager button did not attach a click handler.");
}
for (const handler of openModelManagerBtn.listeners.get("click") || []) {
  handler({});
}
if (studioSurface.className.includes("hidden")) {
  throw new Error("Model manager action did not switch into the studio surface.");
}
if (!settingsStudio.className.includes("hidden")) {
  throw new Error("Opening the model manager should switch away from the settings summary surface.");
}
if (studioBackBtn.className.includes("hidden")) {
  throw new Error("Model manager view should show the back button.");
}

console.log("webview smoke passed");
