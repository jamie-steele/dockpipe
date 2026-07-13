# Host Sandbox Runtime Contract And Roadmap

Date: 2026-07-10

Status: proposed authored/runtime contract and delivery plan. Read with
[Lightweight Native Host Sandbox Runtime](host-sandbox-runtime-2026.md), which contains the executive
recommendation, platform research, and guarantee semantics. No production implementation is part of
this design change.

> **Authoritative audit note:** Start with the
> [architecture decision](host-sandbox-runtime-design-decision-2026.md) and
> [design-audit addendum](host-sandbox-runtime-audit-addendum-2026.md). They supersede conflicting
> assurance, cloud-agent dogfooding, Windows filesystem, YAML migration, Git, and terminology
> examples in this proposed contract.

## Proposed Authored YAML

This surface is not accepted by the current schema. Implementation must update the Go domain model,
JSON Schema, DockPipe Language Support, workflow documentation, compiled runtime manifest, and tests
in one change.

DockPipe already uses a runtime profile string, and `runtime.type` already means `execution`, `ide`,
or `agent`. The aligned selector is therefore `runtime: host-sandbox`, not a new mapping with
`type: host-sandbox`.

### Minimal MVP

```yaml
name: sandboxed-test
runtime: host-sandbox

security:
  profile: host-sandbox-default
  filesystem:
    workspace: .
    read:
      - .
      - ~/.nuget/packages
    write:
      - .
      - ../SharedProject
    deny:
      - ~/.ssh
      - ~/.aws
      - ~/.azure
      - ~/.config/gcloud
      - glob: "**/.env"
        match: existing
    temp:
      mode: private
      size: 2GiB
  network:
    mode: offline
  process:
    allow: [git, dotnet, node, npm, pwsh]
    deny: [ssh, scp, docker]
    environment:
      inherit: [LANG, LC_ALL, TERM, NO_COLOR]
    resources:
      wall_time: 30m
      cpu: "4"
      memory: 8GiB
      processes: 256
      open_files: 4096
      output_bytes: 100MiB
  credentials:
    ambient: deny

requirements:
  assurance: [production]
  enforcement:
    filesystem_write_scope: required
    filesystem_read_scope: required
    filesystem_deny_exact: required
    filesystem_deny_continuous: optional
    network_offline: required
    credentials_ambient_isolation: required
    process_descendant_inheritance: required
    resource_wall_time: required
    teardown_process_tree: required
    process_executable_policy: preferred

approvals:
  scopes: [once, session]
  sandbox_escape: deny
```

The `process.allow` list is an admission and audit control. It does not make PowerShell, Node,
MSBuild, `dotnet`, compilers, or shells safe interpreters and does not prevent arbitrary code within
the OS sandbox.

### Later network-allowlist shape

```yaml
security:
  network:
    mode: allowlist
    dns: brokered
    allow:
      - host: api.nuget.org
        protocol: tcp
        ports: [443]
      - host: registry.npmjs.org
        protocol: tcp
        ports: [443]
      - loopback: true
        protocol: tcp
        ports: [5432, 5001]
      - cidr: 10.40.0.0/16
        protocol: tcp
        ports: [443]
```

This remains portable intent. The Linux MVP reports it unsupported. A later Linux proxy can enforce
hostnames for proxyable protocols only. A Windows WFP broker can enforce numeric address/port policy
but hostname rules remain partial. A required hostname guarantee blocks either driver unless the
exact declared semantics are available.

### Field model

| Field | Values/shape | Contract |
| --- | --- | --- |
| `runtime` | profile string | `host-sandbox`; remains separate from resolver and `runtime.type` |
| `security.profile` | core preset | Defaults bounded by trusted machine/user policy |
| `filesystem.workspace` | path | Logical workspace resolved before untrusted execution |
| `filesystem.read` | path list | Explicit read-only roots; write implies read |
| `filesystem.write` | path list | Persistent writable roots |
| `filesystem.deny` | string or `{glob, match}` | Exact path or `existing|continuous` pattern intent |
| `filesystem.temp` | private temp object | Optional size/persistence; real host temp is not implied |
| `filesystem.caches` | objects | Path/id, access mode, `session|shared` scope |
| `filesystem.special` | typed objects | Devices, remote mounts, sockets; absent by default |
| `network.mode` | `offline|allowlist|restricted|internet` | Retains current policy modes |
| `network.dns` | `disabled|brokered|system` | `offline` requires disabled |
| `network.allow/block` | endpoint rules | Host/IP/CIDR/loopback, protocol, ports; legacy strings may normalize |
| `security.ipc` | typed rules | Unix sockets, named pipes, D-Bus, COM, Mach, devices, brokers |
| `process.allow/deny` | executable selectors | Resolution/admission/approval/audit, not primary isolation |
| `process.executable_roots` | path/mode list | Optional direct-exec restriction with reported bypass limits |
| `process.environment` | inherit/set/deny | Final scrub occurs after resolver/vault merge |
| `process.resources` | typed limits | CPU, memory, process count, wall time, FDs, output, writes |
| `credentials.ambient` | `deny|declared` | Defaults to deny for `host-sandbox` |
| `credentials.expose` | secret-reference objects | Scope, transport, target, TTL; never plaintext values |
| `requirements.enforcement` | guarantee to level | `required|preferred|optional|disabled` |
| `requirements.assurance` | assurance list | Defaults to `production`; previews require explicit opt-in |
| `approvals.scopes` | scope list | Which scopes the trusted UI may offer |
| `approvals.sandbox_escape` | `deny|prompt` | Never silently enabled for unattended work |

Tilde and relative paths expand only during trusted resolution. Effective manifests contain absolute
canonical object identities. Conflicting aliases are validation errors. A denied glob below a
writable root cannot satisfy `filesystem_deny_continuous` unless the driver actually mediates future
names.

### Enforcement report

```yaml
schema: dockpipe.runtime_enforcement_report.v1
runtime_profile: host-sandbox
driver: linux-userns
platform:
  os: linux
  kernel: 6.8.0
assurance: preview
policy_fingerprint: sha256:...
decision: allow

enforcement:
  filesystem_write_scope:
    status: enforced
    mechanism: [mount_namespace, rw_bind]
    evidence: [canary.write.workspace, canary.write.outside_denied]
  filesystem_read_scope:
    status: enforced
    mechanism: [tmpfs_root, explicit_bind_set]
  filesystem_deny_exact:
    status: enforced
    mechanism: [mask_mount]
    coverage:
      resolved_paths: 6
  filesystem_deny_continuous:
    status: partially_enforced
    mechanism: [prepare_time_glob, mask_mount]
    coverage:
      missing: [future_names_under_writable_roots]
  network_offline:
    status: enforced
    mechanism: [network_namespace]
    evidence: [canary.ipv4, canary.ipv6, canary.dns, canary.host_loopback]
  resource_memory_limit:
    status: enforced
    mechanism: [cgroup_v2_memory_max]
  teardown_process_tree:
    status: enforced
    mechanism: [cgroup_v2_kill, pid_namespace]
  process_executable_policy:
    status: best_effort
    mechanism: [path_resolution, admission_check]
    coverage:
      missing: [interpreters, shell_builtins, dynamic_loading, generated_code]
```

A required guarantee with any status other than `enforced`, or with an assurance outside the accepted
set, changes `decision` to `deny`. No workload process starts. The report is finalized after execution
with observed resource and teardown evidence.

## Runtime Lifecycle And Resolver Integration

### Generic driver boundary

The implementation needs a reusable runtime backend, not host-sandbox branches scattered through
agent or host-step code:

```text
Probe(host facts) -> platform feature inventory
Plan(resolved command, requested policy, requirements) -> immutable runtime plan
Prepare(plan) -> runtime-owned session and policy artifacts
Validate(session) -> enforcement report and allow/deny decision
Execute(session, argv, cwd, env, stdio) -> observed process result
Teardown(session) -> orphan and cleanup result
```

Docker, VM, remote, and future substrates can progressively implement the same policy/report
contract. Generic code must not branch on Bubblewrap, AppContainer, QEMU, Kubernetes, Codex, or
DorkPipe names. Runtimes own the substrate; resolvers own tool invocation.

### End-to-end flow

```text
workflow + resolver + trusted policy ceiling
  -> resolve profile, command, workspace, paths, endpoints, secret refs, limits
  -> compile immutable requested policy and fingerprint
  -> select and probe candidate runtime driver
  -> prepare runtime-owned workspace, state, mounts/ACLs/jobs/cgroups
  -> run active enforcement canaries
  -> validate every required guarantee
       -> deny and report if any requirement fails
  -> execute exact resolved command under prepared boundary
  -> observe result, limits, denials, and structured requests
  -> an approval may create a scoped overlay and fresh execution
  -> collect bounded artifacts through trusted parent
  -> kill/teardown, prove no descendants, reconcile platform state
  -> finalize capability report and operation events
```

Preparation may create trusted OS objects before final validation, but no untrusted command executes
until the report allows it. Preparation failure invokes trusted teardown.

### Resolve and trust precedence

- Resolve workflow, runtime and resolver profiles, strategy, workspace, cwd/scopes, command, and
  initial executable identity.
- Treat repository workflow/package declarations as **requests**, not authority. A hostile repository
  cannot widen its policy by editing YAML.
- Resolve paths through existing DockPipe workspace/package/store helpers and retain OS object ids.
- Resolve tool roots using a controlled PATH and record implicit runtime/system grants.
- Keep secret references separate from values and redact credential-bearing argv.
- Reject ambiguous host hooks, unsupported special paths, and overlapping aliases before mutation.

Effective authority is an intersection:

```text
platform capability
  intersect organization/machine ceiling
  intersect trusted user/project policy
  intersect workflow/package request
  plus narrowly scoped approved overlays
```

Current security layering is value precedence; sandbox authority needs this additional trust ceiling.
Untrusted content may narrow policy or request a grant, never silently widen it.

Runtime control state, approvals, session metadata, and canonical audit logs must not be writable by
the worker. Existing state-root helpers remain mandatory. Keep control data outside the visible
workspace or mount it absent/read-only, and copy only bounded results after teardown.

### Prepare and validate

- Create/open a runtime-owned managed workspace before the sandbox.
- Prepare synthetic home/temp/session caches, an explicit environment, bounded stdio or PTY, and a
  request-only unprivileged control channel.
- Create Linux namespace/mount/cgroup state or Windows AppContainer/ACL/Job state.
- Apply restrictions before the child opens sensitive resources; inherit no generic handles/FDS.
- Run positive and negative canaries with the same launcher and identity as the workload.
- Emit the full report and stop if a requirement fails.

Version checks are hints. Active canaries prove the actual machine configuration.

### Execute, collect, and teardown

The resolver supplies tool argv and requested environment. The runtime owns final filtering, cwd,
limits, process tree, platform policy, and teardown. A resolver cannot switch to host execution.

Prefer argv over shell reparsing. If a shell is required, record the resolved shell and exact original
command. Treat output as hostile: cap it, sanitize terminal control sequences in summaries, and retain
raw bytes only in a clearly marked bounded artifact.

Artifact collection is a trusted-parent operation. It opens only declared roots, rejects escaping
links/reparse points and special files, enforces count/size limits, and never runs file handlers, Git
hooks, package scripts, or generated executables.

Teardown must:

- stop new child work and requests;
- kill the cgroup/job/session tree with the strongest mechanism;
- wait for and prove empty process ownership;
- remove mounts, profiles, session SID grants, BFS/WFP objects, temp state, and control sockets using a
  crash-recoverable cleanup journal;
- preserve policy/report/events/recovery artifacts;
- mark the run failed or degraded if teardown fails, even when the command exited zero.

### Host hooks and strategies

Current `kind: host`, top-level host pre-scripts, `act`, and strategy hooks sit outside container
policy. A `host-sandbox` workflow must not leak through those paths. The MVP should reject ambiguous
host hooks. Untrusted setup/teardown scripts run inside the selected runtime; runtime-owned lifecycle
helpers run in the trusted control plane with narrow typed inputs; truly unrestricted host behavior
uses a separate `sandbox_escape: true` operation.

### Runtime selection

1. Compile policy and requirements independently of a candidate driver.
2. Probe available drivers.
3. Discard any driver missing a required guarantee at accepted assurance.
4. Rank survivors by trusted preference, compatibility, startup cost, and preferred guarantees.
5. Record the selected driver and every rejected-candidate reason.
6. If none qualify, fail with stronger-runtime recommendations. Never select `host` as fallback.

The same workflow can then use `host-sandbox`, `dockerimage`, `vm`, or a remote-backed profile. QEMU,
WSL, Kubernetes, and providers remain backend/resolver composition details consistent with DockPipe's
architecture rather than new special-purpose workflow semantics.

## Approval And Escalation

### Structured request

```yaml
schema: dockpipe.approval_request.v1
request_id: apr_01...
run_id: run_01...
session_id: session_01...
policy_fingerprint: sha256:...

command:
  executable: /usr/bin/git
  argv: [git, push, origin, feature/example]
  cwd: /work

requested_capability:
  type: network
  access: connect
  destination:
    host: github.com
    protocol: tcp
    port: 443

denied_by:
  rule_id: network.mode.offline
  mechanism: network_namespace

reason: Publish the runtime-owned session branch.
scope_options: [once, session]
additional_blast_radius:
  - Allows the approved process to transmit any readable data to github.com:443.
effective_enforcement_after_grant:
  status: partially_enforced
  reason: HTTPS broker policy is not an OS hostname identity.
sandbox_escape: false
```

Requests preserve exact structured argv and a separately redacted display. They bind to canonical
path object, endpoint, AppContainer SID, or runtime-operation identity. They do not invent a command
after denial.

| Scope | Effect |
| --- | --- |
| `once` | One exact command/resource execution with a fresh fingerprint |
| `session` | Future commands in the current session until expiry |
| `workflow` | Trusted run/workflow overlay; does not silently edit repository YAML |
| `persisted` | Explicit policy change stored by trusted UI after separate confirmation |

The agent may request but never approve. Most OS policies are immutable or unsafe to widen in place,
so a grant starts a fresh execution. It does not resume a denied syscall. A driver refuses an
approval it cannot enforce honestly; no wildcard is inferred.

### Sandbox escape

Unrestricted host execution is a distinct request:

```yaml
requested_capability:
  type: runtime
  runtime: host
sandbox_escape: true
scope_options: [once]
additional_blast_radius:
  - Removes filesystem, network, credential, process, and teardown guarantees.
```

It has a highly visible UI, separate event unit, and explicit human approval. It is disabled for
unattended work unless trusted external policy pre-authorizes the exact operation. An agent may not
translate a denied operation into SSH, HTTP, a shell script, or another host equivalent.

Typed runtime-owned helpers are preferable where their authority is narrower. For example,
`session.publish` can receive one remote, branch, and short-lived credential without giving the
worker raw Git/network credentials. It emits its own policy, result, and audit events.

## Observability And Audit

Use the existing operation-result envelope and append-only `dockpipe.operation_event.v1` JSONL ledger.
Do not create a second authoritative sandbox event database. Resolved policy and enforcement report
are immutable artifacts referenced from the same event stream; indexes remain rebuildable.

| Unit/event | Required data |
| --- | --- |
| `sandbox.resolve` | Profiles, command/workspace ids, requested fingerprint |
| `sandbox.prepare` | Driver/platform and runtime-owned resource ids |
| `sandbox.policy.resolve` | Canonical resources, sources, implicit grants, redactions |
| `sandbox.capabilities.validate` | All statuses, mechanisms, assurance, canaries, decision |
| `sandbox.process.start` | Process/token id, parent, executable id, cwd, policy |
| `sandbox.process.child` | Parent/child where declared observer coverage exists |
| `sandbox.process.exit` | Exit/signal/limit reason, duration, resource summary |
| `sandbox.violation.filesystem` | Operation/path/rule/mechanism when actually observable |
| `sandbox.violation.network` | Protocol/destination/rule/mechanism when observable |
| `sandbox.violation.process` | Exec/broker/mitigation denial when observable |
| `sandbox.limit` | Limit id, observed value, action, affected process/session |
| `sandbox.approval.request/decision` | Request, scope, expiry, fingerprints; no secret values |
| `sandbox.escape.request` | Exact command and blast-radius summary |
| `sandbox.policy.changed` | Old/new fingerprint and approval binding |
| `sandbox.artifacts.collect` | Root, count, bytes, rejected special/escaping files |
| `sandbox.teardown` | Kill mechanism, empty-tree proof, cleanup result |
| `sandbox.orphan.detected` | Process/object identity and recovery disposition |

```json
{
  "schema": "dockpipe.operation_event.v1",
  "type": "operation_result",
  "ts": "2026-07-10T14:22:11Z",
  "unit": "sandbox.capabilities.validate",
  "status": "done",
  "ids": {
    "run": "run-1842",
    "session": "session-7f3a",
    "policy": "sha256:..."
  },
  "data": {
    "decision": "allow",
    "required_enforced": 8,
    "preferred_missing": 1,
    "report": "<runtime-scoped-report-path>"
  }
}
```

Interactive output shows driver selection, short grants, failed requirements, approvals, limits,
command result, and teardown. It does not flood context with every child or file open. Detailed local
artifacts retain policy provenance, canaries, fingerprints, bounded process/resource data, observed
violations, approvals, and cleanup journal. Raw secrets and full inherited environments never appear.

Namespaces and AppContainer often return `ENOENT`, `EACCES`, or network failure without a complete
unprivileged violation stream. Rootless Linux mount namespaces do not attribute every denied path;
stable AppContainer lacks a complete unprivileged file-denial feed; Seatbelt logs are not a structured
control channel. `audit_policy_violations` is therefore partial/unsupported in the MVP. DockPipe must
not fabricate denial details or widen policy for diagnostics.

## Threat Model And Trust Boundaries

### Trusted control plane

The trusted computing base is deliberately small:

- the host kernel and active OS security mechanisms;
- the installed/verified DockPipe engine, policy compiler, launcher, supervisor, and artifact
  collector;
- the selected platform constructor/helper and its pinned version/provenance;
- trusted machine/user policy and approval UI/channel;
- narrow runtime-owned Git/network/install helpers when explicitly used.

Untrusted inputs include agent output, repository content, prompts, workflow/package requests,
resolver scripts executed as workload, build/test/package lifecycle scripts, dependencies, generated
files, logs, terminal bytes, and collected artifacts. A same-UID process already running outside the
sandbox remains outside this trust boundary: host-sandbox protects the host from the workload, not
the workload from the rest of the developer session.

### Threat analysis

| Threat | Host-sandbox mitigation | Residual risk / stronger-runtime trigger |
| --- | --- | --- |
| Malicious agent or prompt injection | OS policy is independent of model instructions; untrusted YAML cannot widen trusted ceiling | Agent can corrupt anything intentionally writable |
| Hostile build/package scripts or dependencies | Inherit filesystem/network/process boundary | Kernel exploit, allowed endpoint tunnelling, intentionally exposed secrets |
| Symlink/junction/traversal attacks | Canonical object identity, constructed view/AppContainer checks, escaping-link rejection | Hard-linked/copied secret already in allowed tree; implementation races must be tested |
| Mount/reparse races | Handle/object-based binding, dedicated worktree, no string-prefix authorization | Linux bubblewrap path reopening or Windows reparse bugs require fail-closed canaries/security review |
| Child, shell, or interpreter escape | Namespace/AppContainer inheritance plus cgroup/Job ownership | Allowed interpreters execute arbitrary code inside boundary; external brokers must stay denied |
| Credential discovery | Synthetic home, scrubbed env/handles, hidden agents/keychains/sockets | Any explicitly exposed credential is readable and exfiltratable through allowed egress |
| Docker/Kubernetes authority | Sockets/config absent; CLI deny is secondary | Granting Docker socket is effectively a sandbox escape |
| Local service/localhost attacks | Host loopback denied in offline MVP; IPC absent | Explicit service grants enlarge blast radius and may expose broker vulnerabilities |
| DNS/network tunnelling | Offline namespace/AppContainer; later forced broker with direct-IP denial | Allowed servers can relay data; hostname is not a universal OS identity |
| Persistence via startup, tasks, services, hooks | Synthetic home, no broker APIs, process-tree kill; `.git` hidden/runtime-owned | Changes inside writable source persist for review; shared writable caches permit cross-session persistence |
| Git hook modification | Worker lacks writable Git metadata; runtime helper disables repository hooks | Source files can still propose malicious hooks for later human execution; review required |
| Writes outside workspace | OS filesystem boundary and negative canaries | Unsupported path classes cause preflight failure, never best-guess execution |
| Terminal escape/output flood | Parent PTY/pipes, byte limits, rendered-control sanitization | Raw log viewers must remain explicitly marked and careful |
| Runtime/approval tampering | Control state/channel outside worker view, fingerprints and scoped records | Compromise of DockPipe binary, approval UI, or same-user host session is outside workload boundary |
| Validation-to-use race | Object ids/handles retained through launch; immutable fingerprint | Any driver unable to bind identity race-free reports partial/unsupported |
| Runaway or orphan process | PID namespace+cgroup kill or AppContainer+non-breakaway Job, watchdog, empty-tree proof | Delegation to external services is forbidden; unsupported teardown blocks required workflow |
| GPU/device/kernel attack | Devices absent in MVP, minimal `/dev`, AppContainer device denial | Same kernel remains exposed; device/GPU/driver workloads require VM/remote disposable host |

The runtime mitigates common file, credential, egress, persistence, and runaway-process risks. It does
not provide a separate kernel, protect against kernel vulnerabilities, make allowed tools trustworthy,
make builds reproducible, inspect the semantics of allowed traffic, or safely expose broad host
devices/services. Those cases require Docker where sufficient, or a VM/QEMU, Kubernetes-backed
disposable worker, Windows Sandbox, or remote machine for a stronger boundary.

## Runtime-Selection Decision Matrix

Ratings are relative and configuration-dependent. "Strong" host-sandbox rows refer only to a driver
that passed its declared canaries; macOS currently has no production driver.

| Choice | Isolation | Startup | Host tooling | Reproducibility | GPU/hardware | Legacy/native OS | Network control | Ambient credentials | Teardown | Complexity |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Unrestricted host | None | Excellent | Excellent | Low | Excellent | Excellent | Weak | High exposure | Weak | Low |
| `host-sandbox` | Medium, OS/platform-dependent | Excellent/Good | High | Low/Medium | High only when explicitly granted | High; Windows compatibility requires probes | Strong offline; allowlists later/limited | Denied by default | Strong on qualified Linux/Windows | High |
| Docker profile | Medium, shared kernel | Good | Low/Medium; duplicate toolchain | High | Medium with explicit devices | Low for Windows-only/legacy tools | Strong offline; endpoint policy needs extra machinery | Low unless mounted | Strong | Medium |
| WSL-backed profile | Medium only when host interop/mounts are constrained | Good/Medium | High for Linux tools, lower for Windows tools | Medium | Medium | Mixed | Medium | Medium | Medium | Medium |
| QEMU/VM profile | High, separate kernel | Slow | Low without guest provisioning | High | Low/Medium with passthrough | High when correct guest OS is provisioned | Strong | Low | Strong | High |
| Kubernetes pod-backed profile | Medium/high according to node/runtime policy, shared node kernel | Medium | Low | High | Medium | Low for desktop/legacy host tools | Strong with suitable CNI/policy | Low but service-account risk | Strong | High |
| Remote disposable worker | High separation from developer host | Medium/Slow | Low unless image is prepared | High | Depends on worker | Depends on worker OS | Strong | Low with brokered short-lived auth | Strong | High |

Orchestrator guidance:

- use unrestricted host only for trusted commands requiring full local/GUI integration and active
  human ownership;
- use `host-sandbox` for high-frequency supervised edit/build/test work needing host SDKs, caches,
  legacy tools, or hardware and whose required guarantees the driver proves;
- use Docker for reproducible same-kernel workloads that fit container images;
- use WSL for Linux compatibility on Windows, not as an automatic security boundary;
- use a VM/QEMU for hostile native code, separate-kernel requirements, driver/device risk, or legacy
  guest OS needs;
- use Kubernetes for scalable disposable container workloads with cluster policy;
- use remote workers for unattended/high-risk work, clean-machine guarantees, or when the local OS
  cannot enforce requirements.

## DockPipe Dogfooding Design

### Flow

1. The top-level orchestrator selects a backlog item and compiles a bounded task/policy.
2. The runtime creates a managed session branch/worktree through runtime-owned Git lifecycle.
3. It prepares `host-sandbox`, resolves tools/caches, runs canaries, and emits an enforcement report.
4. The coding agent starts only when all required guarantees pass.
5. The agent reads/edits the worktree and runs already-authorized build/test commands without repeated
   prompts.
6. Network, extra repositories, dependency installation, and local services require scoped requests.
   Git metadata writes, checkpoints, sync, and publish remain runtime operations.
7. The trusted collector returns patch/findings/questions/test results/proposed next work and the
   complete run report.
8. The human reviews architecture, diffs, tests, approvals, checkpoint/publish, and acceptance.

### Example policy

```yaml
name: dockpipe-backlog-worker
runtime: host-sandbox
resolver: codex

workspace:
  repo: .
  mode: managed
  storage: worktree
  lifecycle:
    branch_prefix: ai
    checkpoint: manual
    publish: none

security:
  profile: host-sandbox-default
  filesystem:
    workspace: .
    read: [., ~/.cache/go-build, ~/go/pkg/mod]
    write: [.]
    deny:
      - .git
      - ~/.ssh
      - ~/.aws
      - ~/.azure
      - ~/.config/gcloud
      - glob: "**/.env"
        match: existing
    caches:
      - path: ~/.cache/go-build
        access: read_only
        scope: shared
      - id: go-build-session
        access: read_write
        scope: session
    temp:
      mode: private
      size: 4GiB
  network:
    mode: offline
  process:
    allow: [go, make, pwsh, bash, rg, dockpipe]
    deny: [git, ssh, docker, kubectl]
    environment:
      inherit: [LANG, LC_ALL, TERM, NO_COLOR]
    resources:
      wall_time: 2h
      memory: 12GiB
      processes: 512
      output_bytes: 250MiB
  credentials:
    ambient: deny

requirements:
  enforcement:
    filesystem_read_scope: required
    filesystem_write_scope: required
    filesystem_deny_exact: required
    network_offline: required
    credentials_ambient_isolation: required
    process_descendant_inheritance: required
    resource_wall_time: required
    teardown_process_tree: required
    audit_process_lifecycle: preferred

approvals:
  scopes: [once, session]
  sandbox_escape: deny
```

Shared caches are read-only; a session cache absorbs writes and is discarded or promoted only after
review. The worker receives a trusted installed DockPipe binary read-only, never a workspace-built
launcher it can replace. `.git` and runtime control state are not writable or visible.

### Minimum regular-use safety bar

Dogfooding is not safe enough for regular use until all of these are true:

- Linux user/mount/PID/IPC/UTS/network namespace and cgroup-v2 teardown canaries pass on the supported
  development host; no `try`/fallback semantics are used.
- A dedicated clean managed worktree is the only persistent writable source root.
- Sensitive-read and outside-write sentinels fail for the root process, shells, and grandchildren.
- IPv4, IPv6, DNS, LAN, and host-loopback canaries all fail in offline mode.
- Every descendant remains in the boundary and parent-owned cgroup/job; forced launcher crashes leave
  no process.
- Environment, FDs/handles, home, agents, credential stores, Docker/Kubernetes authority, and proxy
  configuration pass absence audits.
- `.git` is protected and runtime-owned checkpoint/publish never runs repository hooks.
- Shared caches are read-only and session cache cleanup is idempotent.
- `go build`, relevant tests, PowerShell/bash helpers, interactive PTY use, output limits, and artifact
  collection pass representative compatibility tests.
- Required guarantee decisions come from active probes and reports, not intended configuration.
- Approval records are scoped/fingerprinted; unattended runs cannot approve or escape.
- Teardown and cleanup reconciliation survive timeout, kill, terminal close, DockPipe crash, and reboot.
- An independent security review accepts the launcher, path binding, policy ceiling, and canary suite.

Initial dogfooding should remain network-off and use preinstalled dependencies. Dependency installs,
pushes, external repositories, GPUs, local databases, Docker sockets, and broad MCP access are later
typed capabilities, not MVP exceptions.

## Narrow MVP Scope

### In scope

- Linux only, marked preview until security review and dogfood gates pass.
- `runtime: host-sandbox` profile and a generic driver/probe/report interface.
- Dedicated managed worktree or explicit canonical local roots.
- Empty mount view; explicit read-only tool/system/cache roots; workspace and explicit writable roots.
- Exact existing deny-path masking; reject required continuous globs under writable ancestors.
- Private home/temp, minimal devices/proc, no authority-bearing IPC sockets.
- Explicit namespaces, `no_new_privs`, empty capability sets, nested userns disabled, conservative
  seccomp, negotiated Landlock defense in depth.
- `network.mode: offline` only.
- Scrubbed environment and explicit secret-reference rejection/absence; no ambient credentials.
- Wall time/output cap and cgroup CPU/memory/process/teardown when delegated and canary-proven.
- Immutable policy fingerprint, enforcement report, operation events, bounded artifacts.
- Structured preflight/request approvals with once/session scopes; no silent host fallback.
- Complete cgroup-based teardown and orphan proof as a required dogfood guarantee.

### Explicitly out of scope

- macOS production enforcement and broad Windows support.
- Network hostname, IP/CIDR, registry, LAN, localhost, or MCP allowlists.
- Package-manager network installs and ambient package credentials.
- Shared writable dependency caches.
- GPU, KVM, USB, display, audio, keychain, Docker/Podman, Kubernetes, or arbitrary device/socket access.
- UNC/remote/network filesystems and Windows reparse/device path classes.
- Strong executable identity/arbitrary-code prevention.
- Continuous deny globs inside writable roots.
- Aggregate persistent disk-write or network-byte quotas.
- Complete rootless filesystem/network violation attribution.
- Automatic elevation, machine policy installation, or persistent approvals.
- GUI desktop isolation.

## Phased Implementation Roadmap

### Phase 0: contract and executable probes

- Ratify guarantee ids/status/assurance semantics and trust-ceiling precedence.
- Decide the generic runtime-driver boundary and migration from `kind: host` to explicit effective
  `host`/`host-sandbox` runtimes without breaking current workflows.
- Extend the effective runtime manifest and operation-event mapping generically.
- Build a standalone conformance probe suite before a workload launcher.
- Threat-model and security-review path identity, control-state placement, approvals, and artifacts.

Exit: Linux host facts produce a truthful report; no commands execute.

### Phase 1: Linux offline proof of enforcement

- Pin/provenance-check Bubblewrap as the reference constructor. Start a native Linux launcher before
  Phase 6 only if path-race review proves Bubblewrap cannot satisfy a required preview guarantee; in
  that case the launcher is a security prerequisite rather than a performance optimization.
- Implement empty-root mount policy, namespace set, FD/env scrub, minimal home/temp/dev/proc.
- Add cgroup-v2 ownership/limits/kill, Landlock negotiation, and conservative seccomp.
- Implement required-guarantee validation, immutable reports, operation events, and fail-closed CLI.
- Run adversarial conformance tests across supported Linux distributions/kernels.

Exit: synthetic hostile tests cannot read/write outside roots, network, escape descendants, or remain
after teardown.

### Phase 2: supervised DockPipe dogfooding

- Integrate runtime-owned managed worktrees and protect Git/control state.
- Add interactive agent PTY, bounded artifact collection, once/session requests, and recovery journal.
- Validate DockPipe Go/PowerShell/bash build/test compatibility with network disabled.
- Run a limited backlog-item cohort with human review and collect compatibility/security metrics.

Exit: minimum dogfood safety bar passes and an independent review approves regular supervised use.

### Phase 3: narrow brokers, not broad exceptions

- Prototype a sole-path HTTP(S) egress broker, IP/CIDR controls, DNS semantics, byte accounting, and
  short-lived package credentials.
- Add typed local-service/IPC grants one transport at a time.
- Add dependency install as a separate policy/operation, not ordinary build permission.
- Keep unsupported protocols fail-closed and report broker versus OS enforcement distinctly.

### Phase 4: Windows parallel spike and preview

- Immediately probe `Experimental_CreateProcessInSandbox`, BFS semantics, ConPTY, nested jobs, proxy,
  cleanup, and exact Windows build availability.
- Independently implement/evaluate stable AppContainer + Job Object on a dedicated NTFS worktree.
- Test LPAC versus regular AppContainer and .NET/Node/PowerShell/MSBuild compatibility.
- Add WFP only through a separately reviewed elevated policy broker; keep preview offline first.

Exit: Windows preview passes its own conformance/safety bar. Experimental API remains disabled for
production until Microsoft stabilizes it.

### Phase 5: macOS capability detector, not false parity

- Ship detection/reporting that marks the required lightweight guarantees unsupported.
- Optionally maintain an explicit version-pinned `experimental-seatbelt` research adapter that cannot
  satisfy production assurance by default.
- Reconsider production only if Apple publishes a supported dynamic arbitrary-process sandbox API, or
  treat Endpoint Security + Network Extension as a separately installed enterprise product.

### Phase 6: DockPipe-owned native launchers

This is a long-horizon optimization and dependency-reduction phase, not an MVP prerequisite. Keep the
public `runtime: host-sandbox` profile and common guarantee contract unchanged while substituting an
explicitly selected implementation driver.

- Freeze a versioned structured launch specification and IPC/handle protocol shared by all drivers.
- Establish Bubblewrap-based Linux and stable Windows preview security/performance baselines first.
- Build a small rootless Linux launcher around direct namespace, mount/FD, Landlock, seccomp, pidfd,
  and cgroup operations; do not add a setuid fallback.
- Build a Windows launcher around supported AppContainer/LPAC, Job Object, handle-list, ConPTY, and
  mitigation APIs. Keep elevated WFP and experimental BFS in separately reviewed components.
- Ship each launcher as a signed, provenance-recorded DockPipe component with an independent update
  and rollback path; record its version, digest, signing identity, reproducible-build provenance, and
  SBOM in the runtime manifest. Do not download or compile helpers during a workflow run.
- Evaluate a pre-warmed per-user supervisor only after the one-shot launcher passes. It must create a
  fresh OS identity and containment state per session, restrict reuse to one user/worktree/policy
  fingerprint/trust domain, and expose no general privileged command API. Never pool repositories or
  users.
- Keep macOS at capability detection until a supported enforcement API exists; owning the launcher
  does not change the platform guarantee.

Exit: the native driver passes the same adversarial conformance and cleanup suite, completes an
independent security review, and demonstrates a material measured deployability, startup-latency, or
throughput benefit without weakening any required guarantee. Driver choice and fallback remain
explicit; a native-driver failure never selects Bubblewrap, host, or another substrate silently.
Keep the reference driver explicitly selectable for at least one compatibility release without
making it an automatic fallback.

## Testing Strategy

### Policy/compiler tests

- YAML/schema validation, defaults, legacy normalization, status and assurance matching.
- Trusted ceiling intersection: untrusted workflow/profile edits can only narrow or request.
- Canonical path/object identity, case-sensitive directories, aliases, overlaps, nonexistent children,
  symlink/junction/reparse/mount/remote filesystem detection.
- Stable fingerprints, redaction, deterministic reports, and candidate-selection/no-fallback rules.
- Approval scope/expiry/replay/resource-binding tests.

### Cross-platform conformance suite

Every driver implements identical positive/negative canaries for each guarantee it advertises. Tests
assert the report and behavior together. A driver never claims `enforced` solely because an API call
succeeded.

- read approved file; fail sensitive/unlisted read;
- write approved root; fail outside/readonly/exact-deny writes and truncation;
- root child/grandchild/shell/interpreter tests;
- IPv4/IPv6/TCP/UDP/DNS/DoH/QUIC/raw-IP/loopback/LAN/proxy tests as applicable;
- process-count, memory, CPU, wall-time, output, temp-size tests;
- kill/crash/close/reboot reconciliation and empty-tree proof;
- environment, inherited descriptor/handle, agent/socket/keychain/credential sentinel tests;
- event, report, artifact, and redaction assertions.

### Adversarial filesystem tests

Linux:

- `..`, absolute and relative symlinks, hard links, bind aliases, nested mounts, OverlayFS/FUSE/NFS,
  `/proc/*/fd`, namespace FDs, Unix sockets, device nodes, concurrent path replacement;
- pre-opened read/write/directory descriptors and descriptor passing;
- setuid/file-cap binaries, nested user namespaces, mount/setns/ptrace/keyring/BPF/perf attempts.

Windows:

- case variants, 8.3 names, ADS, `\\?\`, `\\.\`, `GLOBALROOT`, junctions, reparse points, mount points,
  hard links, UNC/mapped drives, concurrent replacement;
- WMI, COM/DCOM, BITS, Task Scheduler, SCM, shell execution, named pipes, compiler servers, breakaway,
  inherited token/process/section/console handles;
- actual token AppContainer SID and Job membership for every observed process.

macOS experimental only:

- build-version gating, Seatbelt inheritance, pre-opened FDs, Mach/XPC/IOKit/Keychain/LaunchServices,
  daemonization, network, and every OS update/beta.

### Compatibility matrix

Run representative real projects and record required grants, failures, and performance:

- Go, Rust, C/C++, Java, Node/npm, Python, modern .NET, PowerShell, Bash;
- Git read-only inspection versus runtime-owned lifecycle;
- package managers offline with warm read-only caches;
- legacy Windows/MSBuild/Visual Studio command-line tooling in the Windows preview;
- PTY/interactive agent sessions, test runners, compiler servers disabled/enabled;
- startup latency, memory overhead, cache hit rates, teardown time, and approval frequency.

Compatibility is never fixed by automatic host retry.

### Native-driver equivalence and performance

Run the external-constructor and DockPipe-native drivers against identical compiled policies,
fixtures, machines, and canary sets. Treat the security result and the performance result as separate
gates.

- Compare effective mount/ACL views, implicit grants, child membership, environment, inherited
  handles/FDs, network reachability, resource limits, events, and teardown evidence.
- Measure cold and warm policy compilation, preparation, first-process/no-op command, steady-state
  spawn, teardown, CPU, peak RSS, binary footprint, handles/FDs, and mount/ACL scaling.
- Record p50, p95, p99, variance, concurrency saturation, and performance across 1, 10, and 100
  policy roots and representative cache counts rather than relying on a single microbenchmark.
- Measure end-to-end build/test/agent sessions as well as launcher microbenchmarks so an optimization
  is not promoted when it is immaterial to user-visible latency.
- Compare unrestricted host, the reference driver, native one-shot, native warm-session, and
  Docker/WSL where available. Include stat-heavy scans, child fanout, interactive PTY latency, and
  warm-cache Go, Node, .NET, Rust, and C/C++ builds/tests.
- Set regression budgets only after reproducible baselines exist. Dependency elimination can be an
  independent adoption benefit, but it never compensates for weaker enforcement or cleanup.
- Re-run the matrix on supported kernel/OS updates and block promotion when security equivalence or
  the stated performance budget regresses.

### Fault injection and security review

- Kill launcher/supervisor/helper at every lifecycle transition.
- Exhaust processes/memory/output/disk temp; fork concurrently during teardown.
- Corrupt cleanup journals and repeat reconciliation to prove idempotence.
- Fuzz policy parsing, path canonicalization, endpoint rules, event rendering, and approval messages.
- Test ANSI/OSC terminal attacks, log injection, artifact bombs, sparse files, FIFOs/sockets/devices.
- Run dependency/SAST/vulnerability review for privileged or namespace-facing code.
- Require independent design/code review and a focused red-team escape pass before production assurance.

## Open Technical Questions

1. What exact generic driver interface best replaces current container/host dispatch without embedding
   platform names or breaking `kind: host` workflows?
2. Should the steady-state public unrestricted profile be `runtime: host`, with legacy `kind: host`
   normalized internally, and on what deprecation timeline?
3. Where should trusted runtime control state live so existing scope helpers remain canonical while a
   worker with workspace writes cannot tamper with it?
4. Which policy sources are trusted, how are user/project ceilings approved/signed, and how does a
   repository request rather than grant wider permissions?
5. Is bubblewrap's pathname interface sufficiently race-resistant for the preview, or should DockPipe
   build a small native `openat2`/mount-FD launcher before any security claim?
6. Which Linux distributions/kernels and cgroup-delegation shapes form the supported baseline? Is a
   user systemd scope sufficient everywhere DockPipe targets?
7. How are implicit runtime/tool roots discovered without mounting an entire home or making host
   toolchain resolution nondeterministic?
8. Should required sensitive patterns inside a writable tree force a clean filtered managed worktree,
   rather than prepare-time masks?
9. How should interactive PTY support coexist with a new session, TIOCSTI prevention, raw output, and
   terminal sanitization?
10. Which compiler servers, local databases, MCP transports, and GPU/device profiles can be mediated
    without converting the runtime into a broad host grant?
11. What exact semantics can a forced network broker promise for DNS, redirects, CDN rotation,
    private-IP answers, proxy auth, QUIC, and allowed-host tunnelling?
12. On Windows, which builds expose the experimental API, how stable is `SandboxSpec.fbs`, and how do
    BFS paths behave across reparse points, nested denies, profiles, ConPTY, and teardown?
13. Is regular AppContainer or LPAC the viable default for .NET/Node/PowerShell/MSBuild, and how can
    session SID ACL cleanup be crash-safe without touching arbitrary checkouts?
14. Can shared caches remain read-only without unacceptable performance, and what promotion/scanning
    model is safe for session cache outputs?
15. Which platform events can honestly satisfy violation-audit guarantees without a privileged global
    monitor or unacceptable overhead?
16. How are approvals authenticated and replay-protected when requested through an interactive agent
    session, and which scopes may ever persist?
17. How should host-sandbox manifests represent no image while remaining compatible with the current
    compiled runtime/image artifact contract?
18. Who owns the recurring security review, OS-update compatibility gates, and response process for a
    same-kernel sandbox boundary?
19. Should the native launcher be a minimal Rust helper, platform-native C/C++, carefully constrained
    Go, or a mixed design, and what new compiler/supply-chain dependency is acceptable?
20. What is the stable versioned launch-spec and IPC/handle ABI, and how are older engine/launcher
    combinations rejected or rolled back without weakening policy?
21. Which measured cold-start, steady-state spawn, throughput, footprint, or deployability improvement
    justifies replacing Bubblewrap or stable platform glue with owned security-critical code?
22. Can a pre-warmed per-user supervisor preserve fresh identity, policy immutability, canaries,
    descendant ownership, and teardown equivalence, or must production remain one-shot?
23. How are signed native helpers distributed and serviced across Linux distributions, Windows
    architectures, and enterprise policies without adding runtime downloads or an unsafe fallback?

## Final Recommendation

**Prototype the runtime. Do not defer the Linux work, and do not claim cross-platform production
parity.**

- Build the common policy/report/driver contract and Linux offline proof first.
- Promote Linux to regular DockPipe dogfooding only after active conformance, failure injection, the
  complete-teardown gate, and independent security review.
- Run the Windows experimental-API spike in parallel, then build a stable AppContainer technical
  preview if compatibility merits it.
- Fail closed on macOS and direct required workloads to VM or remote execution; keep any Seatbelt work
  explicitly experimental.
- Add network/package/local-service capabilities later through narrow brokers. Never weaken the MVP
  by sharing host networking or mounting ambient credentials.
- After production baselines exist, evaluate DockPipe-owned native launchers as an internal driver
  optimization; require security equivalence and measured value before replacing external
  constructors.

This path fills the intended gap: much of host compatibility and latency with materially stronger
OS enforcement than unrestricted host, while remaining honest that containers and especially VMs or
remote disposable workers are the right choice for higher-risk workloads.

## Primary Source Notes

Linux:

- [Bubblewrap upstream documentation and limitations](https://github.com/containers/bubblewrap/blob/main/README.md)
- [Bubblewrap current implementation/options](https://github.com/containers/bubblewrap/blob/main/bubblewrap.c)
- [Linux user namespaces](https://man7.org/linux/man-pages/man7/user_namespaces.7.html)
- [Linux mount namespaces](https://man7.org/linux/man-pages/man7/mount_namespaces.7.html)
- [Linux network namespaces](https://man7.org/linux/man-pages/man7/network_namespaces.7.html)
- [Landlock userspace API](https://docs.kernel.org/userspace-api/landlock.html)
- [Seccomp BPF](https://docs.kernel.org/userspace-api/seccomp_filter.html)
- [Linux capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html)
- [Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- [Ubuntu AppArmor and unprivileged user namespace restrictions](https://documentation.ubuntu.com/security/security-features/privilege-restriction/apparmor/)

macOS:

- [Apple DTS: unsupported custom SBPL / `sandbox-exec` product guidance](https://developer.apple.com/forums/thread/661939)
- [Apple App Sandbox](https://developer.apple.com/documentation/security/app_sandbox)
- [App Sandbox entitlements and child inheritance](https://developer.apple.com/library/archive/documentation/Miscellaneous/Reference/EntitlementKeyReference/Chapters/EnablingAppSandbox.html)
- [Apple Endpoint Security](https://developer.apple.com/documentation/endpointsecurity)
- [WWDC20: Build an Endpoint Security app](https://developer.apple.com/videos/play/wwdc2020/10159/)
- [Network Extension content filters](https://developer.apple.com/documentation/networkextension/content-filter-providers)
- [TN3165: Packet Filter is not API](https://developer.apple.com/documentation/technotes/tn3165-packet-filter-is-not-api)

Windows:

- [`Experimental_CreateProcessInSandbox`](https://learn.microsoft.com/en-us/windows/win32/secauthz/createprocessinsandbox)
- [Launch an AppContainer](https://learn.microsoft.com/en-us/windows/win32/secauthz/implementing-an-appcontainer)
- [AppContainer isolation](https://learn.microsoft.com/en-us/windows/win32/secauthz/appcontainer-isolation)
- [`CreateRestrictedToken`](https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-createrestrictedtoken)
- [Mandatory Integrity Control](https://learn.microsoft.com/en-us/windows/win32/secauthz/mandatory-integrity-control)
- [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
- [Job basic/extended limits](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_limit_information)
- [Windows Filtering Platform](https://learn.microsoft.com/en-us/windows/win32/fwp/about-windows-filtering-platform)
- [Application Layer Enforcement](https://learn.microsoft.com/en-us/windows/win32/fwp/application-layer-enforcement--ale-)
- [Windows Sandbox](https://learn.microsoft.com/en-us/windows/security/application-security/application-isolation/windows-sandbox/)

DockPipe alignment sources:

- [Architecture model](../concepts/architecture-model.md)
- [Isolation layer](../concepts/isolation-layer.md)
- [Workflow YAML](../workflows/workflow-yaml.md)
- [Security policy](../security/security-policy.md)
- [Operation results](../runtime/operation-results.md)
- [Git runtime sessions](../runtime/git-runtime-sessions.md)
