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

  const read = fixture('read-standard', { definition_ref: '.devcontainer/devcontainer.json' });
  const noContainer = invoke(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-none-standard', { containers: [] })]);
  assert.strictEqual(noContainer.status, 0, noContainer.stderr);
  assert.deepStrictEqual(noContainer.events.map((event) => event.event), ['status', 'completed']);
  assert.strictEqual(noContainer.events[0].state, 'not_created');
  assert.strictEqual(noContainer.events[0].ownership, 'external');

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
  // generic dockpipe.run path and emits the same event envelope.
  const bash = process.env.DOCKPIPE_HOST_BASH_BIN || 'bash';
  const wrapped = spawnSync(bash, [wrapper], { encoding: 'utf8', env: { ...process.env, DOCKPIPE_ARGS_JSON: JSON.stringify(['status', '--workspace', workspace, '--definition-ref', '.devcontainer/devcontainer.json', '--read-configuration-fixture', read, '--docker-inspect-fixture', fixture('docker-wrapper', { containers: [] })]) } });
  assert.strictEqual(wrapped.status, 0, wrapped.stderr);
  const wrappedEvents = wrapped.stdout.trim().split(/\r?\n/).map((line) => JSON.parse(line));
  assert.strictEqual(wrappedEvents[0].contract_version, 'devcontainer.lifecycle.v1');
  assert.deepStrictEqual(wrappedEvents.map((event) => event.sequence), [1, 2]);
  assert.ok(!/docker\s+(run|start|stop|exec)|devcontainer\s+(up|exec|build)/i.test(fs.readFileSync(script, 'utf8')), 'read-only resolver must not contain lifecycle invocation');
  process.stdout.write('ide devcontainer fixture contract tests ok\n');
} finally {
  fs.rmSync(workspace, { recursive: true, force: true });
}
