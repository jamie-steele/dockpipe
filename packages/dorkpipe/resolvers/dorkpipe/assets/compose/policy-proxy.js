#!/usr/bin/env node
"use strict";

const http = require("http");
const https = require("https");
const net = require("net");
const { URL } = require("url");

const listenPort = Number(process.env.DOCKPIPE_POLICY_PROXY_PORT || "3128");

function decodePolicy(req) {
  const value = String(req.headers["proxy-authorization"] || "").trim();
  if (!value.toLowerCase().startsWith("basic ")) {
    return null;
  }
  try {
    const decoded = Buffer.from(value.slice(6), "base64").toString("utf8");
    const sep = decoded.indexOf(":");
    const username = sep >= 0 ? decoded.slice(0, sep) : decoded;
    const json = Buffer.from(username, "base64url").toString("utf8");
    const policy = JSON.parse(json);
    if (!policy || policy.version !== "dockpipe-proxy-v1") {
      return null;
    }
    policy.allow = Array.isArray(policy.allow) ? policy.allow : [];
    policy.block = Array.isArray(policy.block) ? policy.block : [];
    policy.mode = String(policy.mode || "").trim().toLowerCase();
    return policy;
  } catch {
    return null;
  }
}

function normalizeHost(host) {
  host = String(host || "").trim().toLowerCase();
  if (host.startsWith("[") && host.endsWith("]")) {
    host = host.slice(1, -1);
  }
  if (host.endsWith(".")) {
    host = host.slice(0, -1);
  }
  return host;
}

function hostMatchesPattern(host, pattern) {
  host = normalizeHost(host);
  pattern = normalizeHost(pattern);
  if (!host || !pattern) {
    return false;
  }
  if (pattern.startsWith("*.")) {
    const suffix = pattern.slice(2);
    return host === suffix || host.endsWith(`.${suffix}`);
  }
  if (pattern.startsWith(".")) {
    const suffix = pattern.slice(1);
    return host === suffix || host.endsWith(`.${suffix}`);
  }
  return host === pattern;
}

function policyAllowsHost(policy, host) {
  host = normalizeHost(host);
  if (!host || !policy) {
    return false;
  }
  const blocked = policy.block.some((pattern) => hostMatchesPattern(host, pattern));
  if (blocked) {
    return false;
  }
  switch (policy.mode) {
    case "internet":
      return true;
    case "restricted":
      return true;
    case "allowlist":
      return policy.allow.some((pattern) => hostMatchesPattern(host, pattern));
    case "offline":
    default:
      return false;
  }
}

function destinationHostFromHttpRequest(req) {
  try {
    const parsed = new URL(req.url);
    return normalizeHost(parsed.hostname);
  } catch {
    const host = String(req.headers.host || "").split(":")[0];
    return normalizeHost(host);
  }
}

function writeDenied(res, message) {
  res.writeHead(403, { "content-type": "text/plain; charset=utf-8" });
  res.end(message);
}

function sanitizeHeaders(headers) {
  const out = { ...headers };
  delete out["proxy-authorization"];
  delete out["proxy-connection"];
  return out;
}

const server = http.createServer((req, res) => {
  if (req.method === "GET" && req.url === "/healthz") {
    res.writeHead(200, { "content-type": "text/plain; charset=utf-8" });
    res.end("ok\n");
    return;
  }

  const policy = decodePolicy(req);
  if (!policy) {
    res.writeHead(407, { "Proxy-Authenticate": "Basic realm=\"dockpipe-policy\"" });
    res.end("dockpipe policy token required\n");
    return;
  }

  const targetHost = destinationHostFromHttpRequest(req);
  if (!policyAllowsHost(policy, targetHost)) {
    writeDenied(res, `blocked by DockPipe network policy: ${targetHost}\n`);
    return;
  }

  let parsed;
  try {
    parsed = new URL(req.url);
  } catch {
    writeDenied(res, "invalid proxy request URL\n");
    return;
  }

  const transport = parsed.protocol === "https:" ? https : http;
  const upstream = transport.request(
    {
      protocol: parsed.protocol,
      hostname: parsed.hostname,
      port: parsed.port || (parsed.protocol === "https:" ? 443 : 80),
      method: req.method,
      path: `${parsed.pathname}${parsed.search}`,
      headers: sanitizeHeaders(req.headers)
    },
    (upstreamRes) => {
      res.writeHead(upstreamRes.statusCode || 502, upstreamRes.headers);
      upstreamRes.pipe(res);
    }
  );
  upstream.on("error", (error) => {
    res.writeHead(502, { "content-type": "text/plain; charset=utf-8" });
    res.end(`proxy error: ${error && error.message ? error.message : String(error)}\n`);
  });
  req.pipe(upstream);
});

server.on("connect", (req, clientSocket, head) => {
  const policy = decodePolicy(req);
  const [hostPart, portPart] = String(req.url || "").split(":");
  const host = normalizeHost(hostPart);
  const port = Number(portPart || "443");

  if (!policy) {
    clientSocket.write("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"dockpipe-policy\"\r\n\r\n");
    clientSocket.destroy();
    return;
  }
  if (!policyAllowsHost(policy, host)) {
    clientSocket.write(`HTTP/1.1 403 Forbidden\r\nContent-Type: text/plain\r\n\r\nblocked by DockPipe network policy: ${host}\n`);
    clientSocket.destroy();
    return;
  }

  const upstreamSocket = net.connect(port, host, () => {
    clientSocket.write("HTTP/1.1 200 Connection Established\r\n\r\n");
    if (head && head.length > 0) {
      upstreamSocket.write(head);
    }
    upstreamSocket.pipe(clientSocket);
    clientSocket.pipe(upstreamSocket);
  });
  upstreamSocket.on("error", (error) => {
    clientSocket.write(`HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\n\r\nproxy error: ${error && error.message ? error.message : String(error)}\n`);
    clientSocket.destroy();
  });
});

server.listen(listenPort, "0.0.0.0", () => {
  console.error(`dockpipe-policy-proxy listening on 0.0.0.0:${listenPort}`);
});
