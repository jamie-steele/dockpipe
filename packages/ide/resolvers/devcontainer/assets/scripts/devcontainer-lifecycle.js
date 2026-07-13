#!/usr/bin/env node
'use strict';

// Package-owned native Dev Container lifecycle seam. The only live adapter in
// this slice is a pinned read-configuration call for an explicit definition.
// It never directly invokes Docker, up, exec, build, hooks, an editor, or a
// provider. Dev Container CLI 0.87.0 performs its own label-filtered, read-only
// `docker ps` during read-configuration; that indirect query is expressly allowed.

const crypto = require('crypto');
const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const CONTRACT_VERSION = 'devcontainer.lifecycle.v1';
const SESSION_LABEL = 'com.dockpipe.devcontainer.session';
const PINNED_DEVCONTAINER_CLI_VERSION = '0.87.0';
const ADAPTER_TIMEOUT_MS = 10000;
const allowedOperations = new Set(['discover', 'status', 'up']);
const APPROVAL_RISKS = ['image_pull', 'build', 'compose_create_start', 'feature_install', 'lifecycle_hooks'];

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
  const out = {
    operation: '', workspace: '.', definitionRef: '', requestId: '', readFixture: '', dockerFixture: '', sessionsFixture: '',
    approvalFixture: '', upFixture: '', sessionOutput: '', liveReadConfiguration: false, devcontainerBin: '',
  };
  for (let index = 0; index < args.length; index += 1) {
    const token = args[index];
    if (token === 'discover' || token === 'status' || token === 'up') {
      if (out.operation) fail('only one operation is allowed');
      out.operation = token;
      continue;
    }
    if (token === '--operation') {
      out.operation = args[++index] || '';
      continue;
    }
    if (token === '--live-read-configuration') {
      out.liveReadConfiguration = true;
      continue;
    }
    const options = new Map([
      ['--workspace', 'workspace'],
      ['--definition-ref', 'definitionRef'],
      ['--request-id', 'requestId'],
      ['--read-configuration-fixture', 'readFixture'],
      ['--docker-inspect-fixture', 'dockerFixture'],
      ['--managed-session-fixture', 'sessionsFixture'],
      ['--approval-fixture', 'approvalFixture'],
      ['--up-result-fixture', 'upFixture'],
      ['--managed-session-output', 'sessionOutput'],
      ['--devcontainer-bin', 'devcontainerBin'],
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
  if (!allowedOperations.has(out.operation)) fail('operation must be discover, status, or up');
  if (out.liveReadConfiguration && out.readFixture) fail('live read-configuration cannot be combined with a fixture');
  out.readFixture ||= process.env.DEVCONTAINER_READ_CONFIGURATION_FIXTURE || '';
  out.dockerFixture ||= process.env.DEVCONTAINER_DOCKER_INSPECT_FIXTURE || '';
  out.sessionsFixture ||= process.env.DEVCONTAINER_MANAGED_SESSION_FIXTURE || '';
  return out;
}

function executableOnPath(command) {
  if (path.isAbsolute(command)) return fs.existsSync(command) ? command : '';
  const extensions = process.platform === 'win32' ? ['.cmd', '.exe', ''] : [''];
  for (const directory of String(process.env.PATH || '').split(path.delimiter)) {
    if (!directory) continue;
    for (const extension of extensions) {
      const candidate = path.join(directory, `${command}${extension}`);
      if (fs.existsSync(candidate)) return candidate;
    }
  }
  return '';
}

function installedDevcontainerInvocation(command) {
  const executable = executableOnPath(command || 'devcontainer');
  if (!executable) fail('installed Dev Container CLI is unavailable');
  if (process.platform === 'win32' && path.extname(executable).toLowerCase() === '.cmd') {
    const entrypoint = path.join(path.dirname(executable), 'node_modules', '@devcontainers', 'cli', 'devcontainer.js');
    if (!fs.existsSync(entrypoint)) fail('installed Dev Container CLI entrypoint is unavailable');
    return { program: process.execPath, prefix: [entrypoint] };
  }
  return { program: executable, prefix: [] };
}

function runAdapter(invocation, args, runner) {
  const result = runner(invocation.program, [...invocation.prefix, ...args], {
    encoding: 'utf8',
    shell: false,
    windowsHide: true,
    timeout: ADAPTER_TIMEOUT_MS,
    maxBuffer: 1024 * 1024,
  });
  if (!result || result.error || result.signal || result.status !== 0) fail('installed Dev Container CLI adapter failed');
  return String(result.stdout || '').trim();
}

function configurationReference(payload) {
  if (!payload || Array.isArray(payload) || typeof payload !== 'object') return '';
  const configuration = payload.configuration && typeof payload.configuration === 'object' ? payload.configuration : {};
  for (const value of [payload.definition_ref, payload.config_file, payload.configFile, payload.config_path, payload.configPath,
    configuration.definition_ref, configuration.config_file, configuration.configFile, configuration.config_path, configuration.configPath]) {
    if (typeof value === 'string' && value) return value;
  }
  return '';
}

function readLiveConfiguration(input, root, candidate, options = {}) {
  const runner = options.runner || spawnSync;
  const invocation = options.invocation || installedDevcontainerInvocation(input.devcontainerBin);
  const version = runAdapter(invocation, ['--version'], runner);
  if (version !== PINNED_DEVCONTAINER_CLI_VERSION) fail('installed Dev Container CLI version does not match the package pin');
  const selected = path.join(root, ...candidate.definition_ref.split('/'));
  const text = runAdapter(invocation, ['read-configuration', '--workspace-folder', root, '--config', selected], runner);
  let payload;
  try { payload = JSON.parse(text); } catch (_) { fail('installed Dev Container CLI returned invalid read-configuration JSON'); }
  if (!payload || Array.isArray(payload) || typeof payload !== 'object') fail('installed Dev Container CLI returned an invalid read-configuration result');
  const returnedReference = configurationReference(payload);
  if (returnedReference && fixtureRef(returnedReference, root) !== candidate.definition_ref) {
    fail('installed Dev Container CLI read-configuration result does not match the selected definition');
  }
  return payload;
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
  const emit = (event, fields = {}) => {
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
      approval_id: fields.approval_id,
      session_id: fields.session_id,
      session_label: fields.session_label,
      summary: fields.summary,
      log_ref: fields.log_ref,
      next_actions: fields.next_actions || [],
      candidates: fields.candidates,
    };
    for (const key of Object.keys(output)) if (output[key] == null || output[key] === '') delete output[key];
    process.stdout.write(`${JSON.stringify(output)}\n`);
  };
  emit.requestID = requestId;
  return emit;
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
  if (!ref) fail('this operation requires --definition-ref');
  const normalized = normalizedRef(ref);
  const candidate = candidates.find((item) => item.definition_ref === normalized);
  if (!candidate) fail('definition_ref is not a discovered Dev Container candidate');
  if (!candidate.valid) fail('selected definition is malformed JSON/JSONC');
  return candidate;
}

function workspaceOutputPath(root, value) {
  if (!value) fail('up requires --managed-session-output');
  const raw = String(value).replace(/\\/g, '/').trim();
  if (!raw || path.posix.isAbsolute(raw) || raw === '.' || raw.startsWith('../') || raw.includes('/../')) {
    fail('managed_session_output must be a workspace-relative file path');
  }
  const output = path.resolve(root, ...path.posix.normalize(raw).split('/'));
  if (output === root || !output.startsWith(`${root}${path.sep}`)) fail('managed_session_output must stay under the workspace');
  return output;
}

function approvalForUp(payload, candidate, requestID, root) {
  if (!payload || payload.approved !== true) fail('explicit approval is required before up');
  if (String(payload.operation || '') !== 'up') fail('approval fixture must approve the up operation');
  if (String(payload.request_id || '') !== requestID) fail('approval fixture does not match the request_id');
  if (fixtureRef(payload.workspace_ref, root) !== '.') fail('approval fixture does not match the workspace');
  if (String(payload.definition_ref || '') !== candidate.definition_ref || String(payload.definition_fingerprint || '') !== candidate.definition_fingerprint) {
    fail('approval fixture does not match the selected definition identity');
  }
  const risks = new Set(Array.isArray(payload.risks) ? payload.risks : []);
  if (APPROVAL_RISKS.some((risk) => !risks.has(risk))) fail('approval fixture must cover every up lifecycle risk');
  const approvalID = String(payload.approval_id || '');
  if (!approvalID) fail('approval fixture requires approval_id');
  return approvalID;
}

function managedRecordFromUp(payload, candidate, root) {
  if (!payload || String(payload.outcome || '') !== 'success') fail('up result fixture must report outcome success');
  const adapter = payload.adapter;
  if (!adapter || String(adapter.distribution || '') !== 'installed_pinned_devcontainer_cli' || String(adapter.command || '') !== 'devcontainer') {
    fail('up result fixture must use the installed/pinned Dev Container CLI adapter');
  }
  if (!String(adapter.version || '') || String(adapter.version) !== String(adapter.pinned_version || '')) {
    fail('up result fixture must prove the installed Dev Container CLI version is pinned');
  }
  if (fixtureRef(payload.workspace_ref, root) !== '.' || String(payload.definition_ref || '') !== candidate.definition_ref || String(payload.definition_fingerprint || '') !== candidate.definition_fingerprint) {
    fail('up result fixture does not match the selected definition identity');
  }
  const containerID = String(payload.container_id || '');
  const sessionID = String(payload.session_id || '');
  const labels = payload.labels && typeof payload.labels === 'object' ? payload.labels : {};
  if (!containerID || !sessionID || String(labels[SESSION_LABEL] || '') !== sessionID) {
    fail('up result fixture must bind container_id, session_id, and the DockPipe session label');
  }
  return {
    container_id: containerID,
    environment_ref: opaqueEnvironmentRef({ Id: containerID }),
    session_id: sessionID,
    workspace_ref: '.',
    definition_ref: candidate.definition_ref,
    definition_fingerprint: candidate.definition_fingerprint,
    session_label: SESSION_LABEL,
    session_label_value: sessionID,
  };
}

function writeManagedSession(output, record) {
  let sessions = [];
  if (fs.existsSync(output)) {
    let existing;
    try { existing = JSON.parse(fs.readFileSync(output, 'utf8')); } catch (_) { fail('managed_session_output must contain valid JSON when it already exists'); }
    sessions = Array.isArray(existing) ? existing : existing && Array.isArray(existing.sessions) ? existing.sessions : null;
    if (!sessions) fail('managed_session_output must contain a sessions array when it already exists');
    if (sessions.some((item) => item && (item.container_id === record.container_id || item.session_id === record.session_id))) {
      fail('managed_session_output already contains this managed session identity');
    }
  }
  fs.mkdirSync(path.dirname(output), { recursive: true });
  const temporary = `${output}.tmp-${process.pid}`;
  try {
    fs.writeFileSync(temporary, `${JSON.stringify({ schema_version: 'devcontainer.managed-session.v1', sessions: [...sessions, record] }, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 });
    fs.renameSync(temporary, output);
  } catch (error) {
    try { if (fs.existsSync(temporary)) fs.unlinkSync(temporary); } catch (_) { /* best effort */ }
    throw error;
  }
}

function runUp(input, root, emit, candidate) {
  const requested = resultFields(candidate, 'approval_required', 'ambiguous', 'A managed Dev Container up request requires explicit approval.', ['approve_up']);
  emit('up_requested', requested);
  if (!input.approvalFixture) {
    emit('approval_required', requested);
    emit('failed', requested);
    return 3;
  }
  let approvalID;
  try {
    approvalID = approvalForUp(readJSON(input.approvalFixture, 'approval'), candidate, emit.requestID, root);
  } catch (error) {
    const fields = { ...requested, summary: error.message, next_actions: ['approve_up'] };
    emit('approval_required', fields);
    emit('failed', fields);
    return 3;
  }
  let record;
  try {
    record = managedRecordFromUp(readJSON(input.upFixture, 'up result'), candidate, root);
    writeManagedSession(workspaceOutputPath(root, input.sessionOutput), record);
  } catch (error) {
    const fields = resultFields(candidate, 'failed', 'ambiguous', error.message, ['repair_managed_session']);
    emit('failed', { ...fields, approval_id: approvalID });
    return 2;
  }
  const fields = resultFields(candidate, 'running', 'managed', 'An explicitly approved managed Dev Container was recorded.', ['status'], record.environment_ref);
  emit('up_result', { ...fields, approval_id: approvalID, session_id: record.session_id, session_label: record.session_label });
  emit('completed', { ...fields, approval_id: approvalID, session_id: record.session_id, session_label: record.session_label });
  return 0;
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
  if (input.operation === 'up') return runUp(input, root, emit, candidate);
  let readConfiguration;
  let dockerInspect;
  let sessions;
  try {
    readConfiguration = input.liveReadConfiguration
      ? readLiveConfiguration(input, root, candidate)
      : readJSON(input.readFixture, 'read-configuration');
    dockerInspect = readJSON(input.dockerFixture, 'docker inspect');
    sessions = input.sessionsFixture ? readJSON(input.sessionsFixture, 'managed session') : { sessions: [] };
    if (!input.liveReadConfiguration) {
      const fixtureDefinitionRef = fixtureRef(configurationReference(readConfiguration), root);
      if (fixtureDefinitionRef !== candidate.definition_ref) fail('read-configuration fixture does not match the selected definition');
    }
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

if (require.main === module) {
  let exitCode = 2;
  try {
    exitCode = run(parseArgs(process.argv.slice(2)));
  } catch (error) {
    // Parsing failures occur before an operation envelope exists. Keep stderr
    // bounded and do not include workspace paths, raw adapter payloads, or commands.
    process.stderr.write(`devcontainer lifecycle: ${error.message}\n`);
  }
  process.exit(exitCode);
}

module.exports = { PINNED_DEVCONTAINER_CLI_VERSION, readLiveConfiguration };
