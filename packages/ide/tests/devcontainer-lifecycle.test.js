'use strict';

const assert = require('assert');
const crypto = require('crypto');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const root = path.resolve(__dirname, '..');
const script = path.join(root, 'resolvers', 'devcontainer', 'assets', 'scripts', 'devcontainer-lifecycle.js');
const wrapper = path.join(root, 'resolvers', 'devcontainer', 'assets', 'scripts', 'devcontainer-lifecycle.sh');
const workspace = fs.mkdtempSync(path.join(os.tmpdir(), 'dockpipe-devcontainer-test-'));

function write(relative, value) {
  const target = path.join(workspace, ...relative.split('/'));
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, value);
  return target;
}

function invoke(args, env = {}) {
  const child = spawnSync(process.execPath, [script, ...args], { encoding: 'utf8', env: { ...process.env, ...env } });
  const events = child.stdout.trim() ? child.stdout.trim().split(/\r?\n/).map((line) => JSON.parse(line)) : [];
  return { ...child, events };
}

function fixture(name, value) {
  return write(`fixtures/${name}.json`, JSON.stringify(value, null, 2));
}

function fingerprint(relative) {
  return `sha256:${crypto.createHash('sha256').update(fs.readFileSync(path.join(workspace, ...relative.split('/')))).digest('hex')}`;
}

function container(id, definitionRef, state, labels = {}) {
  return {
    Id: id,
    State: { Status: state, Running: state === 'running' },
    Config: { Labels: { 'devcontainer.local_folder': workspace, 'devcontainer.config_file': path.join(workspace, ...definitionRef.split('/')), ...labels } },
  };
}

try {
  // Standard, legacy, direct alternate JSONC, malformed, deterministic ordering.
  write('.devcontainer/devcontainer.json', '{"name":"Standard"}\n');
  write('.devcontainer/zeta.jsonc', '{ // comment\n "name": "Zeta",\n}\n');
  write('.devcontainer/alpha.json', '{"name":"Alpha"}\n');
  write('.devcontainer/broken.json', '{"name":');
  write('.devcontainer.json', '{"name":"Legacy"}\n');
  const multiple = invoke(['discover', '--workspace', workspace]);
  assert.strictEqual(multiple.status, 2, multiple.stderr);
  assert.deepStrictEqual(multiple.events.map((event) => event.event), ['discovered', 'selection_required', 'failed']);
  assert.deepStrictEqual(multiple.events[0].candidates.map((item) => item.definition_ref), ['.devcontainer.json', '.devcontainer/alpha.json', '.devcontainer/broken.json', '.devcontainer/devcontainer.json', '.devcontainer/zeta.jsonc']);
  assert.strictEqual(multiple.events[0].candidates.find((item) => item.definition_ref === '.devcontainer/zeta.jsonc').display_name, 'Zeta');
  assert.strictEqual(multiple.events[0].candidates.find((item) => item.definition_ref === '.devcontainer/broken.json').valid, false);
  const missingSelection = invoke(['status', '--workspace', workspace, '--read-configuration-fixture', fixture('read', { definition_ref: '.devcontainer/devcontainer.json' }), '--docker-inspect-fixture', fixture('docker-none', { containers: [] })]);
  assert.strictEqual(missingSelection.status, 2);
  assert.strictEqual(missingSelection.events[0].event, 'failed');
  assert.strictEqual(missingSelection.events[0].state, 'ambiguous');
  const malformed = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/broken.json', '--read-configuration-fixture', fixture('read-broken', { definition_ref: '.devcontainer/broken.json' }), '--docker-inspect-fixture', fixture('docker-broken', { containers: [] })]);
  assert.strictEqual(malformed.status, 2);
  assert.match(malformed.events[0].summary, /malformed/);

  // A selected status has no live adapter path: both captured adapters are required.
  const noAdapter = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json']);
  assert.strictEqual(noAdapter.status, 2);
  assert.strictEqual(noAdapter.events[0].state, 'unavailable');
  assert.ok(!JSON.stringify(noAdapter.events).includes(workspace), 'workspace path leaked');

  // Up is an explicit, fail-closed request. It never consults an adapter or
  // creates a session record until a request-bound approval exists.
  const noApprovalOutput = path.join(workspace, 'no-approval-session.json');
  const noApproval = invoke(['up', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--request-id', 'up-request-1', '--managed-session-output', 'no-approval-session.json']);
  assert.strictEqual(noApproval.status, 3, noApproval.stderr);
  assert.deepStrictEqual(noApproval.events.map((event) => event.event), ['up_requested', 'approval_required', 'failed']);
  assert.strictEqual(noApproval.events[0].state, 'approval_required');
  assert.ok(!fs.existsSync(noApprovalOutput), 'up must not create a record before approval');

  const read = fixture('read-standard', { definition_ref: '.devcontainer/devcontainer.json' });
  const noContainer = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-none-standard', { containers: [] })]);
  assert.strictEqual(noContainer.status, 0, noContainer.stderr);
  assert.deepStrictEqual(noContainer.events.map((event) => event.event), ['status', 'completed']);
  assert.strictEqual(noContainer.events[0].state, 'not_created');
  assert.strictEqual(noContainer.events[0].ownership, 'external');

  // A fixture-adapted installed/pinned CLI result produces a managed record.
  // The public stream deliberately contains only an opaque environment ref.
  const upApproval = fixture('up-approval', {
    approved: true,
    approval_id: 'approval-1',
    request_id: 'up-request-2',
    operation: 'up',
    workspace_ref: '.',
    definition_ref: '.devcontainer/devcontainer.json',
    definition_fingerprint: fingerprint('.devcontainer/devcontainer.json'),
    risks: ['image_pull', 'build', 'compose_create_start', 'feature_install', 'lifecycle_hooks'],
  });
  const upResult = fixture('up-result', {
    outcome: 'success',
    adapter: { distribution: 'installed_pinned_devcontainer_cli', command: 'devcontainer', version: '0.76.0', pinned_version: '0.76.0' },
    workspace_ref: '.',
    definition_ref: '.devcontainer/devcontainer.json',
    definition_fingerprint: fingerprint('.devcontainer/devcontainer.json'),
    container_id: 'managed-from-up-1',
    session_id: 'session-from-up-1',
    labels: { 'com.dockpipe.devcontainer.session': 'session-from-up-1' },
  });
  const up = invoke(['up', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--request-id', 'up-request-2', '--approval-fixture', upApproval, '--up-result-fixture', upResult, '--managed-session-output', 'managed-session-from-up.json']);
  assert.strictEqual(up.status, 0, up.stderr);
  assert.deepStrictEqual(up.events.map((event) => event.event), ['up_requested', 'up_result', 'completed']);
  assert.strictEqual(up.events[1].ownership, 'managed');
  assert.strictEqual(up.events[1].approval_id, 'approval-1');
  assert.strictEqual(up.events[1].session_id, 'session-from-up-1');
  assert.strictEqual(up.events[1].session_label, 'com.dockpipe.devcontainer.session');
  assert.match(up.events[1].environment_ref, /^devcontainer:[a-f0-9]{24}$/);
  assert.ok(!JSON.stringify(up.events).includes('managed-from-up-1'), 'managed container id leaked');
  const upSessionPath = path.join(workspace, 'managed-session-from-up.json');
  const upSession = JSON.parse(fs.readFileSync(upSessionPath, 'utf8'));
  assert.strictEqual(upSession.schema_version, 'devcontainer.managed-session.v1');
  assert.deepStrictEqual(upSession.sessions[0], {
    container_id: 'managed-from-up-1',
    environment_ref: up.events[1].environment_ref,
    session_id: 'session-from-up-1',
    workspace_ref: '.',
    definition_ref: '.devcontainer/devcontainer.json',
    definition_fingerprint: fingerprint('.devcontainer/devcontainer.json'),
    session_label: 'com.dockpipe.devcontainer.session',
    session_label_value: 'session-from-up-1',
  });
  const incompleteApproval = invoke(['up', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--request-id', 'up-request-3', '--approval-fixture', fixture('up-approval-incomplete', { approved: true, approval_id: 'approval-3', request_id: 'up-request-3', operation: 'up', workspace_ref: '.', definition_ref: '.devcontainer/devcontainer.json', definition_fingerprint: fingerprint('.devcontainer/devcontainer.json'), risks: ['image_pull'] }), '--up-result-fixture', 'missing-up-result.json', '--managed-session-output', 'incomplete-approval-session.json']);
  assert.strictEqual(incompleteApproval.status, 3);
  assert.deepStrictEqual(incompleteApproval.events.map((event) => event.event), ['up_requested', 'approval_required', 'failed']);
  assert.ok(!fs.existsSync(path.join(workspace, 'incomplete-approval-session.json')));

  const unpinnedAdapter = invoke(['up', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--request-id', 'up-request-4', '--approval-fixture', fixture('up-approval-4', { ...JSON.parse(fs.readFileSync(upApproval, 'utf8')), approval_id: 'approval-4', request_id: 'up-request-4' }), '--up-result-fixture', fixture('up-result-unpinned', { ...JSON.parse(fs.readFileSync(upResult, 'utf8')), adapter: { distribution: 'installed_pinned_devcontainer_cli', command: 'devcontainer', version: '0.76.1', pinned_version: '0.76.0' } }), '--managed-session-output', 'unpinned-adapter-session.json']);
  assert.strictEqual(unpinnedAdapter.status, 2);
  assert.deepStrictEqual(unpinnedAdapter.events.map((event) => event.event), ['up_requested', 'failed']);
  assert.match(unpinnedAdapter.events[1].summary, /version is pinned/);
  assert.ok(!fs.existsSync(path.join(workspace, 'unpinned-adapter-session.json')));

  const managedFromUpContainer = container('managed-from-up-1', '.devcontainer/devcontainer.json', 'running', { 'com.dockpipe.devcontainer.session': 'session-from-up-1' });
  const managedFromUp = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-managed-from-up', { containers: [managedFromUpContainer] }), '--managed-session-fixture', upSessionPath]);
  assert.strictEqual(managedFromUp.status, 0, managedFromUp.stderr);
  assert.strictEqual(managedFromUp.events[0].ownership, 'managed');

  const external = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-external', { containers: [container('external-1', '.devcontainer/devcontainer.json', 'running')] })]);
  assert.strictEqual(external.status, 0);
  assert.strictEqual(external.events[0].state, 'running');
  assert.strictEqual(external.events[0].ownership, 'external');
  assert.match(external.events[0].environment_ref, /^devcontainer:[a-f0-9]{24}$/);
  assert.ok(!JSON.stringify(external.events).includes('external-1'), 'container id leaked');

  const sessionLabel = { 'com.dockpipe.devcontainer.session': 'session-1' };
  const managedContainer = container('managed-1', '.devcontainer/devcontainer.json', 'created', sessionLabel);
  const managedSession = fixture('managed-session', { sessions: [{ container_id: 'managed-1', session_id: 'session-1', workspace_ref: '.', definition_ref: '.devcontainer/devcontainer.json', definition_fingerprint: fingerprint('.devcontainer/devcontainer.json') }] });
  const managed = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-managed', { containers: [managedContainer] }), '--managed-session-fixture', managedSession]);
  assert.strictEqual(managed.status, 0);
  assert.strictEqual(managed.events[0].state, 'created');
  assert.strictEqual(managed.events[0].ownership, 'managed');

  const orphan = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-orphan', { containers: [container('orphan-1', '.devcontainer/devcontainer.json', 'exited', sessionLabel)] })]);
  assert.strictEqual(orphan.status, 0);
  assert.strictEqual(orphan.events[0].state, 'stopped');
  assert.strictEqual(orphan.events[0].ownership, 'orphan_candidate');

  const changed = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-changed', { containers: [managedContainer] }), '--managed-session-fixture', fixture('changed-session', { sessions: [{ container_id: 'managed-1', session_id: 'session-1', workspace_ref: '.', definition_ref: '.devcontainer/devcontainer.json', definition_fingerprint: 'sha256:changed' }] })]);
  assert.strictEqual(changed.status, 0);
  assert.strictEqual(changed.events[0].state, 'ambiguous');
  assert.strictEqual(changed.events[0].ownership, 'ambiguous');

  const duplicate = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-duplicate', { containers: [container('one', '.devcontainer/devcontainer.json', 'running'), container('two', '.devcontainer/devcontainer.json', 'stopped')] })]);
  assert.strictEqual(duplicate.status, 0);
  assert.strictEqual(duplicate.events[0].state, 'ambiguous');
  assert.strictEqual(duplicate.events[0].ownership, 'ambiguous');

  // The host-step wrapper consumes the same JSON argv injected by the CLI/MCP
  // generic dockpipe.run path and emits the same event envelope. Windows test
  // hosts may expose only the WSL shim, so require an explicitly supplied host
  // Bash before running this process-level wrapper assertion.
  if (process.env.DOCKPIPE_HOST_BASH_BIN) {
    const wrapped = spawnSync(process.env.DOCKPIPE_HOST_BASH_BIN, [wrapper], { encoding: 'utf8', env: { ...process.env, DOCKPIPE_ARGS_JSON: JSON.stringify(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-wrapper', { containers: [] })]) } });
    assert.strictEqual(wrapped.status, 0, `${wrapped.stderr}\n${wrapped.stdout}`);
    const wrappedEvents = wrapped.stdout.trim().split(/\r?\n/).map((line) => JSON.parse(line));
    assert.strictEqual(wrappedEvents[0].contract_version, 'devcontainer.lifecycle.v1');
    assert.deepStrictEqual(wrappedEvents.map((event) => event.sequence), [1, 2]);
  }
  assert.ok(!/docker\s+(run|start|stop|exec)|devcontainer\s+(up|exec|build)/i.test(fs.readFileSync(script, 'utf8')), 'read-only resolver must not contain lifecycle invocation');
  process.stdout.write('ide devcontainer fixture contract tests ok\n');
} finally {
  fs.rmSync(workspace, { recursive: true, force: true });
}
