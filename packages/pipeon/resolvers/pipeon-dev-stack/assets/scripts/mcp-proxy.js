#!/usr/bin/env node

const http = require("http");
const fs = require("fs");

const listenPort = Number(process.env.PIPEON_MCP_PROXY_PORT || "8766");
const upstreamBase = String(process.env.PIPEON_MCP_UPSTREAM_URL || "http://dorkpipe-stack:8765").replace(/\/+$/, "");
const keyFile = String(process.env.MCP_HTTP_API_KEY_FILE || "").trim();

if (!keyFile) {
  console.error("pipeon-mcp-proxy: MCP_HTTP_API_KEY_FILE is required");
  process.exit(1);
}

let apiKey = "";
try {
  apiKey = fs.readFileSync(keyFile, "utf8").trim();
} catch (error) {
  console.error(`pipeon-mcp-proxy: could not read MCP api key file: ${error && error.message ? error.message : error}`);
  process.exit(1);
}

if (!apiKey) {
  console.error("pipeon-mcp-proxy: MCP api key file was empty");
  process.exit(1);
}

function targetURL(req) {
  const path = req.url && req.url !== "/" ? req.url : "/mcp";
  return `${upstreamBase}${path}`;
}

const server = http.createServer(async (req, res) => {
  try {
    const body = [];
    for await (const chunk of req) {
      body.push(chunk);
    }
    const payload = Buffer.concat(body);
    const headers = {
      Authorization: `Bearer ${apiKey}`,
    };
    if (req.headers["content-type"]) {
      headers["Content-Type"] = req.headers["content-type"];
    }
    const response = await fetch(targetURL(req), {
      method: req.method || "POST",
      headers,
      body: payload.length ? payload : undefined,
    });
    res.statusCode = response.status;
    const contentType = response.headers.get("content-type");
    if (contentType) {
      res.setHeader("Content-Type", contentType);
    }
    res.end(await response.text());
  } catch (error) {
    res.statusCode = 502;
    res.setHeader("Content-Type", "application/json");
    res.end(JSON.stringify({ error: "proxy_error", message: error && error.message ? error.message : String(error) }));
  }
});

server.listen(listenPort, "0.0.0.0", () => {
  console.error(`pipeon-mcp-proxy listening on 0.0.0.0:${listenPort} -> ${upstreamBase}/mcp`);
});
