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
    this.listeners = new Map();
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
    return this[name] || null;
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

console.log("webview smoke passed");
