(function () {
  const status = document.getElementById("status");
  function set(text) {
    if (status) {
      status.textContent = text;
    }
  }

  set("canary: script-start");
  let vscode = null;
  try {
    if (typeof acquireVsCodeApi !== "function") {
      set("canary: acquireVsCodeApi missing");
      return;
    }
    vscode = acquireVsCodeApi();
    set("canary: vscode-api-ready");
    vscode.postMessage({ type: "canaryReady" });
  } catch (error) {
    set("canary: boot-failed " + String(error && (error.message || error)));
    return;
  }

  window.addEventListener("message", (event) => {
    const msg = event.data || {};
    if (msg.type === "pong") {
      set("canary: host-message-ok");
    }
  });

  const ping = document.getElementById("ping");
  if (ping) {
    ping.addEventListener("click", () => {
      set("canary: ping");
      vscode.postMessage({ type: "ping" });
    });
  }
})();
