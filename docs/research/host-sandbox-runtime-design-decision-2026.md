# Host Sandbox Runtime Architecture Decision

Date: 2026-07-10

Decision status: approved for a Linux architecture prototype; not approved for production runtime
implementation or cross-platform parity claims.

## Decision

DockPipe should use the name `host-sandbox` and preserve its existing runtime-profile syntax:

```yaml
runtime: host-sandbox
```

Build Phase 0 contract/probes and a Linux-only offline proof of enforcement. Do not start with a
production implementation, network allowlisting, Windows parity, or unsupported macOS Seatbelt
integration.

The Linux prototype is justified because an ordinary unprivileged process can, on compatible hosts,
combine Bubblewrap-constructed user/mount/PID/IPC/UTS/network namespaces, an explicit mount view,
empty capabilities, `no_new_privs`, FD and environment isolation, and parent-owned cgroup-v2 cleanup.
Every required primitive must be actively probed. A blocked user namespace, missing controller
delegation, failed canary, or weaker assurance causes a pre-execution denial.

Windows remains a separate technical preview. Stable AppContainer, ACLs, and a non-breakaway Job
Object can provide useful exact-path, network-off, descendant, and resource guarantees, but regular
AppContainer does not provide a universal constructed filesystem view. Microsoft's Bound File
System API should be spiked only as an `experimental` driver. macOS has no supported public mechanism
that meets the arbitrary-developer-tool contract; production-required workflows must select another
runtime.

## Non-Negotiable Contract Rules

- The unrestricted `kind: host` path remains visibly separate and unchanged.
- A sandbox failure never retries on unrestricted host.
- Desired policy and observed enforcement are separate compiled artifacts.
- Required guarantees fail before workload execution when unavailable.
- The only normative result statuses are `enforced`, `partially_enforced`, `best_effort`,
  `unsupported`, `requires_elevation`, and `disabled_by_policy`. Shorter prose labels in supporting
  drafts are non-normative.
- Driver assurance (`experimental`, `preview`, or `production`) is checked independently of a
  guarantee's status. Preview is never accepted by a production-only workflow.
- Child restrictions are inherited and teardown owns the complete descendant tree.
- Path decisions use canonical object identity and reject ambiguous symlink, junction, reparse,
  mount, device, UNC, and race-prone roots.
- Network-off is the only Linux MVP network mode. Hostname allowlisting is not represented as a
  universal OS-enforced identity.
- Process allowlists are command-shaping and audit controls, not the primary security boundary.
- Ambient environment credentials, agents, keychains, Docker/Kubernetes authority, and provider
  credentials are absent by default.
- Git writes, checkpoint, sync, and publish remain typed runtime-owned session operations.
- Current public security YAML remains backward compatible. Expanded path/credential/requirement
  fields are an additive, versioned contract with explicit normalization and conflict failures.

## MVP Boundary

The first executable prototype includes only:

- Linux preflight and guarantee reporting;
- one managed worktree plus explicit canonical additional roots;
- explicit read-only system, SDK, and cache roots;
- workspace/additional-root writes and private runtime-owned home/temp;
- exact existing sensitive-path masking where mechanically possible;
- a completely disconnected network namespace;
- scrubbed environment and closed inherited descriptors;
- descendant ownership, wall timeout, bounded output, and cgroup limits/kill when delegated;
- structured once/session approval requests without live policy mutation;
- concise operation events plus a detailed local audit artifact;
- idempotent teardown, launcher-crash tests, and orphan detection.

Deny globs created later beneath writable roots, hostname or package-registry allowlists, direct cloud
agent egress, shared writable caches, GPU/device access, local databases, and broad credential
providers are explicitly outside the MVP.

## Dogfooding Decision

A local offline model may run entirely inside `host-sandbox`. A cloud-backed coding CLI cannot run
inside the network-off MVP as previously illustrated.

Initial cloud-assisted dogfooding therefore uses a governed split:

- a small trusted orchestration/model-transport control plane holds the provider credential and
  exchanges bounded model requests;
- every repository read, write, executable, and descendant runs through `host-sandbox`;
- model output has no general host execution interface;
- provider/source disclosure is a separately declared and audited trust decision;
- the worker remains network-off and credential-free.

If literal containment of the full cloud CLI is required, dogfooding is deferred until a narrow
provider broker exists. Linux dogfooding also opts into `preview` explicitly until the independent
security-review promotion gate passes.

## Architecture Placement

`host-sandbox` is a generic runtime driver/profile. It is not an agent-only feature, resolver, or
strategy. The lifecycle remains:

```text
resolve -> prepare -> sandbox.enforcement.validate -> execute -> observe
        -> request approval/escalation -> collect artifacts -> teardown
```

Resolvers continue to select tools and argv. Docker, WSL, QEMU, Kubernetes, and remote workers are
runtime backends, never resolver details. The compiled runtime manifest records the normalized
policy, sources, canonical resources, mechanisms, implicit grants, assurance, canary evidence,
decision, and fingerprints. Existing operation-result/event infrastructure remains authoritative.

## Long-Horizon: DockPipe-Owned Native Sandbox Substrate

The public runtime remains `host-sandbox`. In the longer term, DockPipe may replace external
constructors and platform glue with a signed, DockPipe-owned launcher and supervisor for each
supported operating system. This is an implementation evolution behind the common driver contract,
not a new workflow runtime type and not a claim that one portable enforcement mechanism exists.

Cross-platform means one versioned launch-policy protocol, one guarantee vocabulary, one event and
approval model, and one conformance suite. Enforcement remains native and operating-system-specific:

- **Linux:** a small rootless launcher can use user/mount/PID/IPC/UTS/network namespaces directly,
  bind mounts or modern mount APIs, `openat2`/FD-based path binding, Landlock, seccomp, pidfds, and
  cgroup v2 without requiring a separately installed Bubblewrap executable.
- **Windows:** a native launcher can call supported AppContainer/LPAC, restricted-token, Job Object,
  explicit handle-list, ConPTY, and mitigation APIs directly. It may communicate through a narrow
  protocol with a separately installed/elevated WFP broker; the unprivileged launcher never acquires
  that authority itself. Bound File System remains a separate experimental backend until Microsoft
  stabilizes it.
- **macOS:** an owned binary does not create a supported sandbox API. DockPipe should keep a detector
  and fail required guarantees until Apple exposes a supported mechanism, or treat a privileged
  Endpoint Security/Network Extension product as a different deployment class.

The architecture boundary remains conventional: domain code owns immutable normalized policy,
guarantee, launch-plan, and report values; application code owns generic driver selection and the
runtime lifecycle; infrastructure code owns OS API bindings and launch behavior. Resolver and plugin
behavior never enters the helper.

The engine should compile an immutable launch specification and pass it over a narrow binary IPC or
inherited handle/FD boundary. Launchers accept structured paths, handles, limits, and capability ids;
they never accept shell command strings as policy, parse workflow YAML, resolve profiles, or load
plugins. A one-shot unprivileged helper is the safest default. Any setuid helper, persistent
capability, machine service, or elevated broker is a separate driver and assurance tier.

A pre-warmed per-user supervisor may be explored later, but reuse is restricted to the same user,
worktree, immutable policy fingerprint, and trust domain. Every session receives a fresh OS identity,
filesystem view, canary run, descendant owner, and teardown proof; policy changes invalidate it.
The control endpoint stays outside worker reach, and no state is pooled across repositories or users.
A resident service must not become a broad privileged command broker.

### Expected value

Owning the launch path could provide:

- one signed DockPipe distribution instead of separately installed/versioned sandbox executables;
- tighter provenance, update, compatibility, and vulnerability-response control;
- fewer process hops and less argument/path translation during setup;
- FD/handle-based path binding that reduces validation/use races;
- compiled policy reuse, batched mount/ACL setup, and direct pidfd/Job lifecycle observation;
- lower cold-start and high-frequency command overhead where measurements prove it.

These are hypotheses, not guarantees. Most agent time may be dominated by tool startup, builds, or
model latency, and a custom launcher expands DockPipe's security-critical code and maintenance load.
The project must measure before replacing a mature constructor.

### Promotion gates

A native launcher is eligible to replace an existing driver only when:

1. it consumes the same compiled policy and produces the same or narrower effective grants;
2. the complete adversarial conformance suite passes with no downgraded required guarantee;
3. crash, cancellation, update, and reboot tests prove descendant cleanup and state reconciliation;
4. fuzzing, memory-safety review, dependency provenance, signing, and an independent security review
   accept the enlarged trusted computing base;
5. A/B benchmarks record cold and warm prepare, first-process, no-op command, teardown, CPU, RSS,
   handle/FD, mount/ACL, and concurrency costs at p50, p95, and p99;
6. the measured dependency, deployability, latency, or throughput benefit is material enough to
   justify owning the security-sensitive implementation.

The matching OS/architecture helper is bundled rather than downloaded during a run. Its version,
digest, signing identity, build provenance, and SBOM are recorded in the runtime manifest. The
reference driver remains explicitly selectable for at least one compatibility release; this is not
permission to retry it after a native launch failure.

The implementation language remains an engineering decision. The Go engine can own compilation and
orchestration, while a minimal Rust or platform-native helper may provide safer pre-exec/syscall
control. Choosing Rust, C, or mixed Go helpers must account for memory safety, binary size,
cross-compilation, supply chain, debugger support, signing, and the cost of introducing another
toolchain. No language choice weakens the same conformance and review gates.

## Delivery Map

The complete research deliverables are split by concern:

| Deliverable | Source |
| --- | --- |
| Executive recommendation, naming, platform comparison, common guarantees | [platform research](host-sandbox-runtime-2026.md) |
| Linux/macOS/Windows mechanism details and compatibility limits | [platform appendix](host-sandbox-runtime-platform-appendix-2026.md) |
| YAML proposal, lifecycle, resolver integration, approval, events, threat model | [contract and roadmap](host-sandbox-runtime-contract-and-roadmap-2026.md) |
| Runtime matrix, dogfooding, MVP, phases, tests, open questions | [contract and roadmap](host-sandbox-runtime-contract-and-roadmap-2026.md) |
| Assurance, cloud-agent, Windows, YAML-compatibility, Git, and terminology corrections | [design-audit addendum](host-sandbox-runtime-audit-addendum-2026.md) |

The design-audit addendum is authoritative over conflicting supporting examples.

## Build / Defer / Prototype Recommendation

- **Build now:** the versioned common guarantee model, Linux probes, policy compiler/manifest shape,
  conformance harness, and an offline Linux proof behind explicit preview opt-in.
- **Prototype in parallel after Linux evidence:** stable AppContainer exact-path/network-off behavior
  and the experimental Windows Bound File System API as distinct drivers.
- **Defer:** production Windows claims, endpoint allowlisting, an in-sandbox cloud CLI, GPU/device and
  local-service brokers, shared writable caches, and a privileged Windows WFP service.
- **Long horizon:** evaluate signed DockPipe-owned native launchers after the Linux preview establishes
  security and performance baselines; retain the same `host-sandbox` policy and driver contract.
- **Do not build as a production runtime:** a macOS driver based on custom Seatbelt profiles or
  `sandbox-exec`.

This runtime is a constrained native-execution option, not a replacement for containers, VMs, or
remote workers. Hostile native code, kernel/device risk, strong clean-machine guarantees, unattended
high-risk work, or unsupported host enforcement should select a stronger runtime.
