# Host Sandbox Runtime Design Audit Addendum

Date: 2026-07-10

Status: architecture correction; no runtime behavior is implemented by this document.

This addendum records the independent design-audit corrections to the host-sandbox research set. It
is authoritative wherever it conflicts with the earlier drafts:

- [platform research and recommendation](host-sandbox-runtime-2026.md)
- [contract, lifecycle, threat model, and roadmap](host-sandbox-runtime-contract-and-roadmap-2026.md)
- [platform mechanism appendix](host-sandbox-runtime-platform-appendix-2026.md)

The Linux-first recommendation remains unchanged. The corrections narrow Windows claims, preserve
DockPipe's existing public policy surface, and make the dogfooding topology executable.

## 1. Assurance Is Part Of The Fail-Closed Decision

An enforcement status and a driver-assurance level are separate facts. A mechanism may pass all of
its canaries while its driver remains `preview`; a workflow that accepts only `production` must still
be denied.

The earlier example that combined `accepted_assurance: [production]`, `driver_assurance: preview`,
and `decision: allow` is invalid. Its correct decision is:

```yaml
accepted_assurance: [production]
driver_assurance: preview
decision: deny
reason: driver_assurance_not_accepted
```

An explicitly opted-in prototype or early dogfood workflow may instead declare:

```yaml
requirements:
  assurance: [preview]
```

That opt-in does not turn preview evidence into a production guarantee. The Linux driver may be
promoted to `production` only after the security review, compatibility matrix, crash/reboot cleanup,
and adversarial conformance gates in the roadmap pass. Defaults remain `production`, so an omitted
assurance list cannot accidentally authorize a preview driver.

## 2. Cloud Agent Dogfooding Requires A Control-Plane Split

The earlier dogfood example placed `resolver: codex` inside an offline runtime while denying ambient
credentials. A cloud-backed Codex process cannot operate under those conditions. Direct provider
access would also contradict the Linux MVP's only strong network mode: a completely disconnected
network namespace.

There are three honest topologies:

| Topology | Provider process | Tool execution | MVP status |
| --- | --- | --- | --- |
| Local model | Inside `host-sandbox` | Inside the same sandbox | Supported when the model and dependencies are already local |
| Split controller/executor | Trusted DockPipe model loop outside the workload sandbox | Every repository read, write, and command goes through the sandbox runtime | Recommended dogfood topology |
| In-sandbox cloud CLI | Inside `host-sandbox` with a narrow provider broker | Inside the same sandbox | Deferred until a broker can be enforced and tested |

### Recommended MVP topology

The top-level orchestrator owns a small trusted model-transport loop. It may call an approved model
provider, but it does not execute model-produced shell text, load repository code, or expose a
general host tool. Model-produced tool requests cross a typed, request-only DockPipe boundary. The
runtime canonicalizes the request, applies the compiled policy and approval overlay, then starts the
tool in `host-sandbox`. Results return as bounded, escaped data.

This yields two distinct trust boundaries:

1. **Provider control plane.** Holds only the provider credential and model-session state. It is part
   of DockPipe's trusted computing base. It has no model-controlled general command interface.
2. **Sandboxed executor.** Holds the managed worktree and local developer tools. It has no ambient
   credentials and no network. All spawned descendants inherit the runtime boundary.

The provider necessarily receives whatever source, diagnostics, or artifacts the orchestrator puts
in model requests. That is an explicit data-disclosure capability, not a consequence hidden under
`network_offline`. DockPipe must record provider identity, data classification/policy, redaction,
request size, and content/artifact hashes without logging secrets. A repository whose policy forbids
provider disclosure must use an approved local model or a stronger approved environment.

`network_offline: enforced` therefore means the sandboxed executor and its descendants cannot use
Internet, LAN, DNS, or host loopback. It says nothing about the separately declared trusted provider
transport. Provider transport must never be used as an arbitrary HTTP, file, or shell proxy for the
agent.

If the product requirement is literal containment of the entire cloud CLI process, regular Codex
dogfooding is not an MVP feature. It remains blocked until the provider broker has a narrow protocol,
credential isolation, source-disclosure policy, request/response limits, direct-connect denial, and
adversarial tests. A hostname allowlist alone is insufficient.

The corrected conceptual dogfood selection is:

```yaml
orchestrator:
  agent_transport: governed-provider-control-plane

worker:
  runtime: host-sandbox
  security:
    network:
      mode: offline
    credentials:
      ambient: deny
  requirements:
    assurance: [preview]
    enforcement:
      network_offline: required
      credentials_ambient_isolation: required
```

These keys illustrate the topology and are not yet a ratified public schema. The schema design must
name the provider-disclosure policy before implementation. A fully local agent can keep the original
single-runtime shape.

## 3. Windows Stable And Experimental Guarantees Must Be Separate

Regular AppContainer is useful, but it is not a constructed filesystem view. It retains platform
grants to selected files, registry keys, COM surfaces, and named objects. An ACL grant to a dedicated
worktree proves access to that worktree; sampled negative canaries cannot prove the universal claim
that every other host object is absent.

The stable Windows preview must report these narrower facts:

| Guarantee | Regular AppContainer + ACL + Job | LPAC profile | Experimental Bound File System API |
| --- | --- | --- | --- |
| Explicit dedicated workspace access | `enforced` after positive/negative ACL and reparse tests | `enforced` after the same tests | `enforced` at `experimental` assurance after BFS conformance |
| Exact sensitive-path denial | Per resolved object: `enforced` only after effective-access and runtime canaries; otherwise `partially_enforced` | Potentially narrower, still object-tested | Potentially `enforced` after complete-view tests |
| Universal `filesystem_read_scope` | `partially_enforced`; implicit platform grants must be enumerated | `partially_enforced` until a complete effective-view proof exists | May be `enforced`, but only at `experimental` assurance |
| Universal `filesystem_write_scope` | `partially_enforced`; runtime-owned AppContainer storage and any platform grants are implicit writable roots | `partially_enforced` until complete proof | May be `enforced`, but only at `experimental` assurance |
| Network offline | `enforced` only with no network capability or loopback exemption | Same | May be `enforced` at `experimental` assurance |
| Normal descendant containment | `enforced` for descendants that remain in the token and non-breakaway Job; broker surfaces denied | Same | Subject to API conformance |

The report must list the AppContainer profile directory, runtime-owned temp/storage, tested system
roots, common principal grants that affect the process, registry/COM profile, and any explicit
capabilities as `implicit_grants`. `filesystem_read_scope` or `filesystem_write_scope` cannot be
promoted based only on a finite canary sample.

Microsoft's `Experimental_CreateProcessInSandbox` and its Bound File System support remain a
high-value spike, but they are a distinct driver/mechanism with `experimental` assurance. Stable
AppContainer ACL behavior must never be described as BFS behavior. LPAC is another distinct profile;
failure must not fall back to regular AppContainer.

Consequently, Windows cannot satisfy a workflow that requires the broad MVP filesystem scopes with
`production` assurance today. It can support an explicitly opted-in preview using narrow exact-path
guarantees, or a separately labelled experimental BFS prototype. Stronger workloads continue to use
a VM, container, or remote worker.

### Job profile correction

Windows build compatibility and strong UI restrictions are separate profiles. A headless/UI-limited
Job profile must reject tools that need incompatible nested-job behavior. A build-compatible profile
may allow tested nested jobs but must not claim the incompatible UI-limit set. The profile and every
dropped mitigation or limit appear in the enforcement report; there is no retry with a weaker Job.

### Ordinary Windows Firewall rules

Ordinary Windows Firewall rules are not the MVP boundary. They are normally machine policy, commonly
identify a program/path rather than a unique DockPipe session identity, require administrative or
managed-policy authority to install reliably, and create setup/cleanup race and stale-rule risks.
Mutable executable paths also make image-scoped policy a poor session identity.

No AppContainer network capability is the stable network-off mechanism. A future privileged broker
may install Windows Filtering Platform filters keyed to the unique AppContainer SID and numeric
address/protocol/port. Hostname rules remain partial because DNS, DoH, proxies, VPNs, caches, and
address rotation prevent a durable OS hostname identity.

## 4. The Policy Contract Is An Additive, Versioned Evolution

DockPipe's current public security fields are not replaced by the proposed path-oriented contract.
Existing workflows and compiled container policy retain their current meaning.

The schema rollout must be versioned. The exact authored version key should be decided with the
repository's schema conventions, but the design requires these rules:

1. An unversioned current policy parses with current semantics.
2. A new version opts into the expanded common contract.
3. Existing container policies compile to the same desired policy and backend arguments as before.
4. A compiled manifest records schema version, normalized policy, source field for every rule,
   driver interpretation, enforcement result, and fingerprints.
5. No migration may widen a path, network, process, credential, or resource grant.
6. Unsupported cross-runtime semantics fail capability validation; they are not guessed.

### Compatibility mapping

| Existing field | Common-contract treatment |
| --- | --- |
| `network.mode`, `network.allow`, `network.block` | Retained. Drivers report the exact level they can enforce; host-sandbox MVP accepts only `offline`. |
| `filesystem.root` | Retained as root-writability intent for current container drivers. It is not reinterpreted as visibility of the host root. |
| `filesystem.writes: workspace-only` | Normalizes to the resolved workspace as the only authored persistent write root, plus explicitly runtime-owned ephemeral roots. |
| `filesystem.writes: declared` | Uses current declared writable-path semantics; migration to expanded roots must preserve the same resolved set. |
| `filesystem.writable_paths` | Normalizes to explicit writable roots in the same path namespace. |
| `filesystem.temp_paths` | Normalizes to explicit runtime-owned temporary roots; lifetime and cleanup are added metadata, not silently changed. |
| `process.user` | Retained as identity intent. Each driver reports whether it can implement that identity; it is not inferred from filesystem isolation. |
| `process.pid_limit` | Normalizes to the common process-count request when semantics match. |
| `process.resources.cpu`, `process.resources.memory` | Retained and normalized with their current units/meaning. Enforcement status remains driver-specific. |

New `workspace`, `read`, `write`, `deny`, `caches`, credential, executable, and enforcement-requirement
fields are additive. They do not delete legacy fields in the first release.

When legacy and expanded forms address the same dimension, exact duplicates after canonicalization
are allowed and retain both provenance entries. Different values are a validation error. DockPipe
must not union writable roots, choose the broader rule, or silently reinterpret `root: writable`.
For example, `process.pid_limit` and an expanded `resources.processes` must resolve to the same value
or fail. Rules from profiles, workflow, and step scope still use DockPipe's documented precedence,
then pass through this conflict check.

The authored schema, Go types, validation, editor support, compiled manifest, security docs, runtime
docs, and conformance fixtures must change together when implementation begins.

## 5. Runtime Selection And Terminology Corrections

The recommended profile name remains `host-sandbox`, authored as DockPipe's existing runtime-profile
string:

```yaml
runtime: host-sandbox
```

Current `kind: host` behavior remains explicitly unrestricted and unchanged. `host-sandbox` must not
be implemented as a host-step security flag. A future `runtime: host` selector is a separate public
dispatch proposal, not an MVP dependency and not current behavior.

Runtime and resolver terminology remains strict:

- runtime: where execution occurs and which isolation/session lifecycle applies;
- resolver: which tool/profile/argv is selected;
- strategy: lifecycle wrapper;
- workflow/template: what is run.

QEMU, WSL, Kubernetes pods, containers, and remote workers are runtime drivers or runtime backend
configuration. They are never resolver implementations. Generic code may compose a resolver with
any compatible runtime, but it does not put substrate behavior in the resolver.

Use `sandbox.enforcement.validate` for the lifecycle/event unit. The earlier
`sandbox.capabilities.validate` name risks collision with DockPipe's existing resolver capability
vocabulary. The report may still contain a platform-neutral list of enforcement guarantees.

## 6. Git Approval Uses A Typed Runtime Operation

Raw `git push` is not a normal sandbox network overlay. DockPipe owns managed-workspace Git writes,
checkpoints, sync, and publish. The corrected approval shape is:

```yaml
schema: dockpipe.approval_request.v1
requested_operation:
  type: session.publish
  session_id: session_01...
  remote: origin
  branch: feature/example
requested_capability:
  type: runtime_operation
  operation: session.publish
scope_options: [once]
sandbox_escape: false
```

The trusted runtime operation receives only the named session, remote, branch, and a short-lived
credential. It disables repository hooks, validates the managed branch and remote policy, and emits
its own operation events. Raw lifecycle Git from the worker is rejected. If a user insists on raw
host Git, it is a separate visible `sandbox_escape: true`, not a capability overlay.

## 7. macOS Qualification

The macOS conclusion remains: there is no supported public, lightweight API for dynamically
sandboxing arbitrary host developer tools with this contract.

App Sandbox inheritance applies to appropriately signed embedded helpers carrying the inheritance
entitlement and compatible parent entitlements; it is not a supported general-purpose wrapper for
arbitrary Homebrew, Xcode, shell, compiler, and package-manager trees. The
`com.apple.security.files.user-selected.executable` entitlement enables a narrower user-selected
executable use case, but it does not supply DockPipe's dynamic path, network, descendant, and
developer-tool contract. Custom Seatbelt profiles remain an unsupported experimental prototype, not
a production driver.

## 8. Corrected MVP And Dogfood Gates

The implementation recommendation is now more precise:

- Build a Linux-only `host-sandbox` prototype first.
- Require user/mount/PID/IPC/UTS/network namespaces, a constructed mount view, no-new-privileges,
  empty capabilities, FD cleanup, and parent-owned descendant teardown.
- Require cgroup-v2 guarantees only when the needed controllers and kill semantics are delegated and
  proven; otherwise a workflow requiring them is denied.
- Support network-off only. Do not put general package installation or a cloud agent CLI inside the
  sandbox.
- Preserve current security YAML, add a versioned common IR, and ratify schema changes before runtime
  implementation.
- Dogfood first with a local model or the governed split controller/executor topology and explicit
  preview assurance.
- Keep Windows as a separately labelled technical preview with narrower guarantees; spike BFS in an
  experimental driver. Do not schedule a macOS production driver on unsupported Seatbelt APIs.

Regular DockPipe dogfooding is allowed only after the Linux gates pass and the human explicitly
accepts `preview`. Promotion to production requires the independent security-review gate. If the
team requires an in-sandbox cloud CLI, regular dogfooding is deferred until the provider-broker
phase.

## 9. Remaining Documentation Integration Work

Before implementation, fold these corrections into the canonical architecture/security/runtime
docs, link the platform appendix and this addendum from the research index, and update the
task-index-linked generic software-development backlog item. Remove the superseded short contract
pointer once its useful links have been merged. These are documentation integration tasks, not
permission to implement production code.

## References

- [Bubblewrap README](https://github.com/containers/bubblewrap/blob/main/README.md)
- [Linux Landlock userspace API](https://docs.kernel.org/userspace-api/landlock.html)
- [Linux cgroup v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- [Apple App Sandbox inheritance](https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.inherit)
- [Apple user-selected executable entitlement](https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.files.user-selected.executable)
- [Apple custom sandbox guidance](https://developer.apple.com/forums/thread/661939)
- [Microsoft AppContainer launch](https://learn.microsoft.com/en-us/windows/win32/secauthz/implementing-an-appcontainer)
- [Microsoft Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
- [Microsoft Experimental CreateProcessInSandbox](https://learn.microsoft.com/en-us/windows/win32/secauthz/createprocessinsandbox)
- [Windows Filtering Platform](https://learn.microsoft.com/en-us/windows/win32/fwp/windows-filtering-platform-start-page)
