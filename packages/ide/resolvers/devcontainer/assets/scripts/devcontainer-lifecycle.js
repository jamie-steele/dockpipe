#!/usr/bin/env node
'use strict';

// Package-owned, fixture-only native Dev Container discovery/status seam.
// This module intentionally does not import child_process and never invokes
// Docker, the Dev Container CLI, an editor, lifecycle hooks, or a provider.

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

const CONTRACT_VERSION = 'devcontainer.lifecycle.v1';
const SESSION_LABEL = 'com.dockpipe.devcontainer.session';
const allowedOperations = new Set(['discover', 'status']);

function fail(message) {
  throw new Error(message);
}

function parseArgs(argv) {
  let args = argv.slice();
  if (args[0] === '--args-json') {
    if (args.length !== 2) fail('--args-json requires exactly one JSON array');
    try {
      args = JSON.parse(args[1]);
    } catch (_) {
      fail('--args-json must be a JSON array');
    }
    if (!Array.isArray(args) || !args.every((value) => typeof value === 'string')) {
      fail('--args-json must be a JSON array of strings');
    }
  }
  const out = { operation: '', workspace: '.', definitionRef: '', requestId: '', readFixture: '', dockerFixture: '', sessionsFixture: '' };
  for (let index = 0; index < args.length; index += 1) {
    const token = args[index];
    if (token === 'discover' || token === 'status') {
      if (out.operation) fail('only one operation is allowed');
      out.operation = token;
      continue;
    }
    if (token === '--operation') {
      out.operation = args[++index] || '';
      continue;
    }
    const options = new Map([
      ['--workspace', 'workspace'],
      ['--definition-ref', 'definitionRef'],
      ['--request-id', 'requestId'],
      ['--read-configuration-fixture', 'readFixture'],
      ['--docker-inspect-fixture', 'dockerFixture'],
      ['--managed-session-fixture', 'sessionsFixture'],
    ]);
    if (options.has(token)) {
      const value = args[++index];
      if (!value) fail(`${token} requires a value`);
      out[options.get(token)] = value;
      continue;
    }
    if (token === '--help' || token === '-h') {
      process.stdout.write('usage: devcontainer-lifecycle discover|status --workspace <dir> [--definition-ref <workspace-relative-path>] [fixture adapter options]\n');
      process.exit(0);
    }
    fail(`unknown argument ${JSON.stringify(token)}`);
  }
  if (!allowedOperations.has(out.operation)) fail('operation must be discover or status');
  out.readFixture ||= process.env.DEVCONTAINER_READ_CONFIGURATION_FIXTURE || '';
  out.dockerFixture ||= process.env.DEVCONTAINER_DOCKER_INSPECT_FIXTURE || '';
  out.sessionsFixture ||= process.env.DEVCONTAINER_MANAGED_SESSION_FIXTURE || '';
  return out;
}

function workspaceRoot(input) {
  const root = path.resolve(input || '.');
  let stat;
  try { stat = fs.statSync(root); } catch (_) { fail('workspace does not exist'); }
  if (!stat.isDirectory()) fail('workspace must be a directory');
  return root;
}

function normalizedRef(value) {
  const raw = String(value || '').replace(/\\/g, '/').trim();
  if (!raw || path.posix.isAbsolute(raw) || raw === '.' || raw.startsWith('../') || raw.includes('/../')) {
    fail('definition_ref must be a workspace-relative file path');
  }
  const normalized = path.posix.normalize(raw);
  if (normalized === '..' || normalized.startsWith('../')) fail('definition_ref must stay under the workspace');
  return normalized;
}

function stripJsonc(text) {
  let result = '';
  let quoted = false;
  let escaped = false;
  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    const next = text[index + 1];
    if (quoted) {
      result += char;
      if (escaped) escaped = false;
      else if (char === '\\') escaped = true;
      else if (char === '"') quoted = false;
      continue;
    }
    if (char === '"') { quoted = true; result += char; continue; }
    if (char === '/' && next === '/') {
      index += 1;
      while (index + 1 < text.length && text[index + 1] !== '\n' && text[index + 1] !== '\r') index += 1;
      continue;
    }
    if (char === '/' && next === '*') {
      index += 2;
      while (index < text.length && !(text[index] === '*' && text[index + 1] === '/')) index += 1;
      if (index >= text.length) fail('unterminated JSONC block comment');
      index += 1;
      continue;
    }
    result += char;
  }
  // JSONC permits trailing commas. This conservative pass only removes a comma
  // followed by whitespace and a closing object/array delimiter outside strings.
  let withoutTrailingCommas = '';
  quoted = false;
  escaped = false;
  for (let index = 0; index < result.length; index += 1) {
    const char = result[index];
    if (quoted) {
      withoutTrailingCommas += char;
      if (escaped) escaped = false;
      else if (char === '\\') escaped = true;
      else if (char === '"') quoted = false;
      continue;
    }
    if (char === '"') { quoted = true; withoutTrailingCommas += char; continue; }
    if (char === ',') {
      let cursor = index + 1;
      while (cursor < result.length && /\s/.test(result[cursor])) cursor += 1;
      if (result[cursor] === '}' || result[cursor] === ']') continue;
    }
    withoutTrailingCommas += char;
  }
  return withoutTrailingCommas;
}

function safeConfiguration(text) {
  try {
    const parsed = JSON.parse(stripJsonc(text));
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') return { valid: false };
    const name = typeof parsed.name === 'string' && parsed.name.trim() ? parsed.name.trim().slice(0, 200) : '';
    return { valid: true, name };
  } catch (_) {
    return { valid: false };
  }
}

function discover(root) {
  const direct = [];
  const standard = ['.devcontainer/devcontainer.json', '.devcontainer.json'];
  for (const ref of standard) {
    const absolute = path.join(root, ...ref.split('/'));
    if (fs.existsSync(absolute) && fs.statSync(absolute).isFile()) direct.push(ref);
  }
  const dir = path.join(root, '.devcontainer');
  if (fs.existsSync(dir) && fs.statSync(dir).isDirectory()) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isFile() || !/\.jsonc?$/i.test(entry.name)) continue;
      const ref = `.devcontainer/${entry.name}`;
      if (!direct.includes(ref)) direct.push(ref);
    }
  }
  // Default UTF-16 string ordering is specified by JavaScript and avoids
  // locale/host-dependent collation for the contract's candidate sequence.
  const refs = [...new Set(direct)].sort();
  return refs.map((ref) => {
    const absolute = path.join(root, ...ref.split('/'));
    const content = fs.readFileSync(absolute);
    const config = safeConfiguration(content.toString('utf8'));
    return {
      definition_ref: ref,
      definition_fingerprint: `sha256:${crypto.createHash('sha256').update(content).digest('hex')}`,
      display_name: config.name || undefined,
      valid: config.valid,
    };
  });
}

function readJSON(file, label) {
  if (!file) fail(`${label} fixture is required in this fixture-only slice`);
  let text;
  try { text = fs.readFileSync(path.resolve(file), 'utf8'); } catch (_) { fail(`${label} fixture cannot be read`); }
  try { return JSON.parse(text); } catch (_) { fail(`${label} fixture must be valid JSON`); }
}

function eventEmitter(input, root) {
  let sequence = 0;
  const workspaceRef = path.relative(root, root) || '.';
  const requestId = input.requestId || `dc-${crypto.createHash('sha256').update(`${input.operation}\0${root}\0${input.definitionRef}`).digest('hex').slice(0, 20)}`;
  return (event, fields = {}) => {
    sequence += 1;
    const output = {
      contract_version: CONTRACT_VERSION,
      request_id: requestId,
      sequence,
      event,
      operation: input.operation,
      workspace_ref: workspaceRef,
      definition_ref: fields.definition_ref,
      definition_fingerprint: fields.definition_fingerprint,
      state: fields.state,
      ownership: fields.ownership,
      environment_ref: fields.environment_ref,
      summary: fields.summary,
      log_ref: fields.log_ref,
      next_actions: fields.next_actions || [],
      candidates: fields.candidates,
    };
    for (const key of Object.keys(output)) if (output[key] == null || output[key] === '') delete output[key];
    process.stdout.write(`${JSON.stringify(output)}\n`);
  };
}

function resultFields(candidate, state, ownership, summary, nextActions, environmentRef) {
  return {
    definition_ref: candidate && candidate.definition_ref,
    definition_fingerprint: candidate && candidate.definition_fingerprint,
    state,
    ownership,
    environment_ref: environmentRef,
    summary,
    log_ref: null,
    next_actions: nextActions,
  };
}

function fixtureRef(value, root) {
  const raw = String(value || '').replace(/\\/g, '/');
  const rootSlash = root.replace(/\\/g, '/');
  if (raw === rootSlash || raw === '.' || raw === './') return '.';
  if (raw.startsWith(`${rootSlash}/`)) return raw.slice(rootSlash.length + 1);
  return raw.replace(/^\.\//, '');
}

function containerList(payload) {
  if (Array.isArray(payload)) return payload;
  if (payload && Array.isArray(payload.containers)) return payload.containers;
  fail('docker inspect fixture must be an array or contain containers');
}

function labelsFor(container) {
  return (container && container.Config && container.Config.Labels) || (container && container.Labels) || {};
}

function labelValue(labels, names) {
  for (const name of names) if (typeof labels[name] === 'string' && labels[name]) return labels[name];
  return '';
}

function statusForContainer(container) {
  const state = (container.State && (container.State.Status || container.State.status)) || container.Status || '';
  if (container.State && container.State.Running === true) return 'running';
  if (String(state).toLowerCase() === 'running') return 'running';
  if (String(state).toLowerCase() === 'created') return 'created';
  return 'stopped';
}

function opaqueEnvironmentRef(container) {
  const id = String((container && (container.Id || container.ID || container.id)) || 'unknown');
  return `devcontainer:${crypto.createHash('sha256').update(id).digest('hex').slice(0, 24)}`;
}

function matchingContainers(payload, root, candidate) {
  const selectedAbsolute = path.join(root, ...candidate.definition_ref.split('/')).replace(/\\/g, '/');
  return containerList(payload).filter((container) => {
    const labels = labelsFor(container);
    const config = labelValue(labels, ['devcontainer.config_file', 'devcontainer.configFile']);
    const workspace = labelValue(labels, ['devcontainer.local_folder', 'devcontainer.localFolder']);
    const normalizedConfig = fixtureRef(config, root);
    const normalizedWorkspace = fixtureRef(workspace, root);
    return (config === selectedAbsolute || normalizedConfig === candidate.definition_ref) && normalizedWorkspace === '.';
  });
}

function classify(candidate, containers, sessionsPayload, root) {
  if (containers.length === 0) return { state: 'not_created', ownership: 'external', summary: 'No matching Dev Container was found.', next_actions: ['select_definition'] };
  if (containers.length > 1) return { state: 'ambiguous', ownership: 'ambiguous', summary: 'More than one container matches the selected definition.', next_actions: ['repair_selection'] };
  const container = containers[0];
  const labels = labelsFor(container);
  const sessionID = String(labels[SESSION_LABEL] || '');
  if (!sessionID) {
    return { state: statusForContainer(container), ownership: 'external', summary: 'A matching external Dev Container was found.', next_actions: ['status_only'], environmentRef: opaqueEnvironmentRef(container) };
  }
  const records = Array.isArray(sessionsPayload) ? sessionsPayload : (sessionsPayload && Array.isArray(sessionsPayload.sessions) ? sessionsPayload.sessions : []);
  const id = String(container.Id || container.ID || container.id || '');
  const record = records.find((value) => value && String(value.container_id || '') === id && String(value.session_id || '') === sessionID);
  if (!record) {
    return { state: statusForContainer(container), ownership: 'orphan_candidate', summary: 'A prior DockPipe-labeled container has no matching managed session record.', next_actions: ['repair_managed_session'], environmentRef: opaqueEnvironmentRef(container) };
  }
  const recordWorkspace = fixtureRef(record.workspace_ref, root);
  if (recordWorkspace !== '.' || record.definition_ref !== candidate.definition_ref || record.definition_fingerprint !== candidate.definition_fingerprint) {
    return { state: 'ambiguous', ownership: 'ambiguous', summary: 'The managed-session record does not prove the selected definition identity.', next_actions: ['repair_managed_session'] };
  }
  return { state: statusForContainer(container), ownership: 'managed', summary: 'A matching managed Dev Container was found.', next_actions: ['status_only'], environmentRef: opaqueEnvironmentRef(container) };
}

function selectedCandidate(candidates, ref) {
  if (!ref) fail('status requires --definition-ref');
  const normalized = normalizedRef(ref);
  const candidate = candidates.find((item) => item.definition_ref === normalized);
  if (!candidate) fail('definition_ref is not a discovered Dev Container candidate');
  if (!candidate.valid) fail('selected definition is malformed JSON/JSONC');
  return candidate;
}

function run(input) {
  const root = workspaceRoot(input.workspace);
  const emit = eventEmitter(input, root);
  const candidates = discover(root);
  if (input.operation === 'discover') {
    if (candidates.length === 0) {
      const fields = resultFields(null, 'unavailable', 'external', 'No Dev Container definition was found.', [], undefined);
      emit('discovered', { ...fields, candidates });
      emit('completed', fields);
      return 0;
    }
    if (candidates.length > 1) {
      const fields = resultFields(null, 'selection_required', 'ambiguous', 'Multiple Dev Container definitions require an explicit workspace-relative definition_ref.', ['select_definition'], undefined);
      emit('discovered', { ...fields, candidates });
      emit('selection_required', { ...fields, candidates });
      emit('failed', fields);
      return 2;
    }
    const candidate = candidates[0];
    const fields = resultFields(candidate, 'available', 'external', 'One Dev Container definition is available for explicit status.', ['status']);
    emit('discovered', { ...fields, candidates });
    emit('completed', fields);
    return 0;
  }

  let candidate;
  try { candidate = selectedCandidate(candidates, input.definitionRef); } catch (error) {
    const fields = resultFields(null, 'ambiguous', 'ambiguous', error.message, ['select_definition']);
    emit('failed', fields);
    return 2;
  }
  let readConfiguration;
  let dockerInspect;
  let sessions;
  try {
    readConfiguration = readJSON(input.readFixture, 'read-configuration');
    dockerInspect = readJSON(input.dockerFixture, 'docker inspect');
    sessions = input.sessionsFixture ? readJSON(input.sessionsFixture, 'managed session') : { sessions: [] };
    const fixtureDefinitionRef = fixtureRef(readConfiguration.definition_ref || readConfiguration.config_file || readConfiguration.configFile, root);
    if (fixtureDefinitionRef !== candidate.definition_ref) fail('read-configuration fixture does not match the selected definition');
  } catch (error) {
    const fields = resultFields(candidate, 'unavailable', 'ambiguous', error.message, ['supply_fixture_adapter']);
    emit('failed', fields);
    return 2;
  }
  const result = classify(candidate, matchingContainers(dockerInspect, root, candidate), sessions, root);
  const fields = resultFields(candidate, result.state, result.ownership, result.summary, result.next_actions, result.environmentRef);
  emit('status', fields);
  emit('completed', fields);
  return 0;
}

let exitCode = 2;
try {
  exitCode = run(parseArgs(process.argv.slice(2)));
} catch (error) {
  // Parsing failures occur before an operation envelope exists. Keep stderr
  // bounded and do not include workspace paths, raw adapter payloads, or commands.
  process.stderr.write(`devcontainer lifecycle: ${error.message}\n`);
}
process.exit(exitCode);
