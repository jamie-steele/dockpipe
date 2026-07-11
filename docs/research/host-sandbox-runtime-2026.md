# Lightweight Native Host Sandbox Runtime

Date: 2026-07-10

Status: research and architecture recommendation; no production implementation is implied by this document.

> **Authoritative audit note:** Start with the
> [architecture decision](host-sandbox-runtime-design-decision-2026.md) and
> [design-audit addendum](host-sandbox-runtime-audit-addendum-2026.md). They supersede conflicting
> assurance, cloud-agent dogfooding, Windows filesystem, YAML migration, Git, and terminology
> examples in this supporting research.

## Executive Summary

DockPipe should **prototype**, not yet broadly ship, a new runtime profile named **`host-sandbox`**.
It should provide a common declarative policy and capability-report contract, with independent
operating-system drivers. It must remain visibly distinct from unrestricted host execution and must
never fall back to it.

The recommended platform order is:

1. **Linux prototype and MVP.** A useful rootless boundary is feasible with user, mount, PID, IPC,
   UTS, and network namespaces; an explicitly constructed filesystem view; `no_new_privs`; dropped
   capabilities; a parent-owned cgroup v2; and optional Landlock/seccomp defense in depth. Bubblewrap
   is a practical namespace/mount constructor, but DockPipe must own the policy and probes. Linux is
   the best combination of enforcement strength, implementation effort, and usefulness.
2. **Windows technical preview and API spike.** A stable design is possible with AppContainer plus a
   non-breakaway Job Object, a dedicated local NTFS worktree, explicit AppContainer SID access, a
   clean environment, and no network capabilities. Compatibility and cleanup are materially harder
   than Linux. Microsoft also documented an unusually well-aligned
   [`Experimental_CreateProcessInSandbox`](https://learn.microsoft.com/en-us/windows/win32/secauthz/createprocessinsandbox)
   API on 2026-06-01. It deserves an immediate spike, but it has no public header, uses schema `0.1.0`,
   and is explicitly experimental, so it is not a production dependency.
3. **No production macOS driver with the current public platform.** Seatbelt can enforce the desired
   policy, but the custom SBPL and `sandbox-exec` interface needed for arbitrary tools is deprecated
   and unsupported. App Sandbox is supported but is designed around signed app bundles, static
   entitlements, and user-mediated file access; it does not fit arbitrary host SDKs and build output.
   Endpoint Security plus Network Extension would be a privileged installed security product, not a
   lightweight runtime. macOS must report required guarantees as unsupported and fail closed. A
   version-pinned Seatbelt research adapter may exist only behind an explicit experimental gate.

The MVP should be deliberately narrow:

- explicit, canonical filesystem roots;
- workspace and additional writable-root enforcement;
- sensitive paths outside visible roots unavailable by construction;
- exact existing denied paths masked where the driver can prove it;
- network completely disabled;
- inherited child restrictions;
- wall-time and output caps;
- CPU, memory, process-count, and race-safe teardown only when the platform probe proves them;
- a scrubbed environment, private home/temp, and no ambient credential or authority-bearing sockets;
- structured capability reports and approvals;
- no hostname allowlists, shared writable caches, GPU/device access, or broad local-service access in
  the first release.

This runtime is a **constrained native execution option**, not a replacement for containers, VMs,
Kubernetes-backed workers, or disposable remote machines. It shares the host kernel and deliberately
uses selected host files and tools. Kernel exploits, risky device access, highly hostile native
binaries, broad credentials, and strong reproducibility still require a stronger runtime.

## Recommendation And Naming

Use **`host-sandbox`** as the public runtime profile name.

- `host` clearly identifies the execution substrate.
- `sandbox` communicates an OS-enforced boundary without claiming VM equivalence.
- `native-sandbox` is ambiguous: it can mean native code, a native API, or the target OS.
- `constrained-host` is accurate but too easy to mistake for advisory restrictions.

DockPipe currently authors runtime selection as a **profile string**, and `runtime.type` already means
the behavioral classification `execution`, `ide`, or `agent`. Therefore the aligned form is:

```yaml
runtime: host-sandbox
```

It should **not** be authored as `runtime: { type: host-sandbox }`, because that overloads DockPipe's
existing `runtime.type` terminology.

The steady-state runtime selector should make unrestricted host execution equally explicit as
`runtime: host`, while retaining existing `kind: host` steps as a backward-compatible shorthand.
That migration is a public engine/schema change and is not part of this research-only change. Until
that refactor is implemented, `host-sandbox` must not be implemented as a `kind: host` step: current
host steps intentionally bypass runtime security.

DockPipe's canonical profiles remain substrate-oriented. Container profiles are `dockerimage` and
`dockerfile`; VM-backed execution uses `vm`; QEMU is a VM backend/resolver choice rather than a new
workflow primitive. WSL-, Kubernetes-, and remote-backed execution should likewise compose through
the existing runtime/resolver boundary instead of adding product-specific branches to core.

## Architectural Fit With DockPipe

The design preserves DockPipe's normative model:

| Concern | Ownership |
| --- | --- |
| Workflow | What command or steps run. |
| Runtime | Where execution runs and which isolation lifecycle applies. |
| Resolver | Which tool/profile is invoked. It does not provision the sandbox. |
| Strategy | Reusable before/after lifecycle, subject to the same trust boundaries. |
| Security policy | Portable desired restrictions compiled into runtime-specific enforcement. |
| Enforcement report | Observed platform mechanisms and their actual coverage. |

The package-level `capability` / `requires_capabilities` model remains resolver discovery metadata.
Sandbox guarantees are a different concept and must live under `requirements.enforcement`; they
must not be added to resolver capability ids.

The current compiled runtime manifest already has the right architectural home: it records runtime
profile, policy sources, security policy, fingerprints, rule ids, and enforcement summaries. The
host-sandbox design extends that generic manifest rather than inventing an agent-only policy file.

Two current behaviors require explicit correction before implementation:

1. `security` currently applies only to container steps. The compiled policy must become
   runtime-driver-neutral, and each driver must report which parts it enforces.
2. Host pre-scripts, `act` scripts, and strategy hooks currently execute outside container policy.
   An untrusted script in a `host-sandbox` workflow must run inside the selected runtime. Any truly
   unrestricted host hook must be a separately declared host operation and, when sensitive, a
   `sandbox_escape: true` approval. The MVP should reject ambiguous host hooks rather than leak out of
   the sandbox.

## Platform Feasibility Summary

| Platform | Practical production mechanism | Root/elevation | Filesystem boundary | Network-off | Child inheritance | MVP recommendation |
| --- | --- | --- | --- | --- | --- | --- |
| Linux | Bubblewrap-built namespaces and explicit mounts; cgroup v2; optional Landlock/seccomp | No, if user namespaces and cgroup delegation are available | Strong for constructed view | Strong | Strong | Implement first |
| Windows | AppContainer + Job Object + session SID access on dedicated NTFS workspace | No for baseline; fine-grained WFP needs elevation | Strong after ACL/BFS and token canaries | Strong | Strong for normal children | Technical preview second |
| macOS | No supported lightweight arbitrary-process API; Seatbelt interface is unsupported | Seatbelt prototype no; supported ES/NE product yes | Technically strong only through unsupported Seatbelt | Same | Seatbelt/App Sandbox descendants inherit | Fail capability validation |

The word **strong** in this table is scoped to the named guarantee. None of these mechanisms makes a
same-kernel process equivalent to a VM.

## Linux Research And Design

### Practical mechanisms

Bubblewrap is an unprivileged, low-level sandbox constructor. Upstream explicitly says that it is not
a complete policy and that protection depends on the arguments supplied. Current upstream has
removed setuid mode, so usable unprivileged user namespaces are a hard prerequisite. See the
[bubblewrap security and limitations documentation](https://github.com/containers/bubblewrap/blob/main/README.md).

The smallest dependable rootless stack is:

1. **Probe, do not infer.** Execute a canary that creates the exact user, mount, PID, IPC, UTS, and
   network namespaces required. Do not use `*-try` options for required guarantees. Distro policy can
   block user namespaces even when the kernel supports them; Ubuntu, for example, can gate them with
   [AppArmor user-namespace restrictions](https://documentation.ubuntu.com/security/security-features/privilege-restriction/apparmor/).
2. **Construct an empty filesystem view.** Start with a tmpfs root. Bind only resolved runtime/tool
   roots read-only, bind the workspace and explicit extra roots at their declared modes, create a
   private `/proc` for the PID namespace, provide a minimal `/dev`, create bounded tmpfs temp/home
   directories, and synthesize the minimum `/etc` needed by the toolchain. Do not expose `/sys`, the
   user's home, session D-Bus, systemd sockets, display sockets, SSH agents, Docker/Podman sockets, or
   credential-manager sockets by default.
3. **Create explicit namespaces.** Use user, mount, PID, IPC, UTS, and network namespaces, a private
   mount-propagation tree, a new terminal session, `--die-with-parent`, and nested-user-namespace
   disablement. A new network namespace supplies a dependable offline boundary; its loopback is the
   sandbox's loopback, not host localhost. Linux documents network namespace separation in
   [`network_namespaces(7)`](https://man7.org/linux/man-pages/man7/network_namespaces.7.html).
4. **Remove privilege paths.** Apply `PR_SET_NO_NEW_PRIVS`, drop effective, permitted, inheritable,
   ambient, and bounding capability sets after setup, close every inherited file descriptor except
   explicit stdio/control channels, isolate or deny keyrings, and use a conservative seccomp denylist.
   The kernel explicitly states that [seccomp is not itself a sandbox](https://docs.kernel.org/userspace-api/seccomp_filter.html).
5. **Own resources from outside.** Put the launcher in a parent-owned cgroup v2 leaf before untrusted
   execution. Where the relevant controllers are delegated, use `memory.max`, `pids.max`, and
   `cpu.max`; retain the cgroup handle outside the sandbox; use `cgroup.kill` for teardown; and wait
   for the cgroup to become empty. Delegation is a prerequisite, not something the worker receives.
   See the kernel's [cgroup v2 documentation](https://docs.kernel.org/admin-guide/cgroup-v2.html).
6. **Add Landlock as defense in depth.** Landlock is unprivileged, stackable, monotonic, and inherited
   by descendants. It is useful for reinforcing allowed filesystem hierarchies and, on newer ABIs,
   Unix-socket/signal scope. It does not replace the constructed mount view. Require ABI 3 before
   claiming truncation/write coverage, negotiate every right, and record unsupported operations.
   The kernel lists both inheritance and current limitations in the
   [Landlock userspace API](https://docs.kernel.org/userspace-api/landlock.html).

AppArmor and SELinux can strengthen an administrator-managed installation, but loading dynamic
policy is not a portable rootless primitive. Report an active DockPipe profile as defense in depth;
do not make it the Linux MVP baseline.

### Linux filesystem limits

The mount namespace is the primary boundary: a process cannot name host paths that were never placed
in its filesystem view. Read-only and writable binds provide clear access modes while host DAC still
applies as the invoking user.

Important qualifications:

- All roots must be canonicalized, deduplicated, checked for conflicting overlaps, and bound by a
  race-resistant launcher. A future production launcher should prefer handle-based Linux APIs such
  as `openat2`/mount FDs instead of validating a string and reopening it later.
- Inherited file or directory descriptors bypass pathname visibility. Descriptor closure is part of
  the filesystem guarantee, not optional hygiene.
- Nested mounts and remote filesystems under an allowed root must be detected. NFS/FUSE/SMB access
  can cause host-side or kernel-mediated I/O even when the sandbox has no network interface.
- Symlinks to an unexposed absolute host path resolve in the sandbox namespace and fail, but a hard
  link or copy of sensitive content already inside an allowed tree is allowed content.
- An exact existing denied file below a writable root can be hidden with a nested mask mount. A glob
  such as `**/.env` is not a continuous kernel name policy: new matches can be created after policy
  compilation. Landlock is hierarchy allowlisting, not a subtractive glob engine. Such a rule is
  `partially_enforced` or `unsupported` unless the workflow requests only an enumerated-existing
  guarantee.
- Landlock does not currently mediate every metadata operation and does not revoke pre-opened FDs.
  Its report must list handled ABI rights instead of saying merely "Landlock enabled."

### Linux network limits

The MVP supports only `network.mode: offline`.

- It is `enforced` by a new network namespace and inherited by descendants.
- Host loopback, LAN access, package registries, local databases, and MCP endpoints are unavailable.
- Filesystem Unix sockets are a separate IPC surface and remain absent unless explicitly granted.

A later brokered design can give the network namespace a single user-mode uplink and force HTTP(S)
through a DockPipe proxy. The proxy can resolve names itself, reject direct IP bypass, pin each
connection, reject private/link-local destinations unless requested, and count bytes. That would be
**broker-enforced protocol-scoped hostname policy**, not an OS hostname guarantee.

Landlock's network rules are port-based, not address- or hostname-based. nftables and cgroup BPF can
enforce numeric address/port policy when properly installed, but a hostname resolves to changing IP
sets. Raw protocols, DNS rebinding, DoH, QUIC, redirects, proxies, and allowed endpoints that tunnel
traffic prevent a general claim that the OS enforces a hostname allowlist.

### Linux rootless preflight

The Linux driver must fail before workload execution unless all guarantees marked required pass
active canaries:

- user/mount/PID/IPC/UTS/network namespace creation;
- empty-root and read-only/writable bind behavior;
- nested user namespaces disabled;
- `no_new_privs` and empty capability sets;
- private PID `/proc` and host-process invisibility;
- network denial for IPv4, IPv6, DNS, and host loopback;
- relevant Landlock ABI rights, when required;
- cgroup controller availability, membership, limits, and `cgroup.kill`, when resource/teardown
  guarantees are required;
- descriptor and environment allowlists;
- teardown canary proving no descendant remains.

If a host policy blocks user namespaces, the result is `unsupported` or `requires_elevation`. The
driver must not retry as unrestricted host and should not recommend disabling a machine-wide
security control as an automatic fix.

## macOS Research And Design

### Seatbelt

Seatbelt is an OS-enforced sandbox and direct descendants inherit it, so technically it can express
the core filesystem and network policy. The problem is product support. `sandbox-exec` and the
custom-profile APIs are deprecated, and SBPL is undocumented for third-party use. Apple Developer
Technical Support states that it would be
[unwise to build a product on this interface](https://developer.apple.com/forums/thread/661939).

Chromium demonstrates that Seatbelt can be maintained by a large security team, but it also shows
the burden: private/undeclared interfaces, OS-sensitive policies, Mach-service and IOKit rules, and a
large compatibility test surface. Presence of `/usr/bin/sandbox-exec` on a current machine is not a
support or stability commitment.

### Supported alternatives

Apple's supported [App Sandbox](https://developer.apple.com/documentation/security/app_sandbox) is
strong for applications designed around entitlements and container directories. It is not a general
launcher for arbitrary Homebrew, Xcode, SDK, compiler-output, and repository executables:

- policy is primarily static code-signing entitlements;
- dynamic file access is mediated by user selection/security-scoped bookmarks;
- inherited children receive static rights, not arbitrary later PowerBox grants;
- network entitlements are coarse client/server rights, not endpoint rules;
- executable helpers are expected to be embedded and signed as part of the application model.

Endpoint Security can authorize and observe file/process activity, but it requires a restricted
entitlement, root or an approved system extension, Full Disk Access, and deadline-sensitive external
decision logic. Apple documents an implicit allow when an authorization client misses its deadline.
Network Extension content filters can enforce flows but require a signed, notarized, installed
system extension and user approval. Apple also says [`pf` is not a product API](https://developer.apple.com/documentation/technotes/tn3165-packet-filter-is-not-api).

Endpoint Security plus Network Extension could become a separate enterprise DockPipe security agent.
It is not the lightweight runtime requested here.

### macOS capability disposition

For a supported lightweight implementation that can run arbitrary developer tools:

| Guarantee | Status | Reason |
| --- | --- | --- |
| Workspace/additional write roots | `unsupported` | No supported dynamic per-process path policy with the required tool compatibility |
| Sensitive read denial | `unsupported` | App Sandbox cannot express the needed arbitrary negative overlays; Endpoint Security alone can fail open |
| Full network disable | `unsupported` for this runtime shape | App Sandbox changes the execution model; Network Extension is an installed product |
| Hostname/port allowlist | `unsupported` | No supported lightweight per-process mechanism; hostnames are not reliable flow identities |
| Child inheritance | `enforced` under Seatbelt/App Sandbox | Not enough to make the overall unsupported launcher production-ready |
| CPU/open-file/file-size limits | `partially_enforced` | POSIX limits are per process or per user, not an aggregate job boundary |
| Aggregate memory/process tree | `unsupported` | No public cgroup/Job Object equivalent for this use |
| Complete teardown | `best_effort` | Process groups can be escaped; complete lineage needs a privileged monitor or dedicated identity |

An optional `experimental-seatbelt` adapter may report the kernel mechanism separately from product
support, for example `status: enforced`, `assurance: experimental`, and
`platform_support: deprecated_unsupported`. Required production guarantees must not accept it by
default. It should be build-version allowlisted, disabled by default, and regression-tested on every
macOS update. Regular unattended macOS work should use a VM or remote worker.

## Windows Research And Design

### Stable primitives

AppContainer is the primary Windows security boundary. Microsoft documents a dual-principal access
check: both the user's token and the AppContainer/package SID must receive access. Without a network
capability, an AppContainer cannot use the network; it also isolates credentials, processes,
windows, devices, files, registry, and named objects. See Microsoft's
[AppContainer launch documentation](https://learn.microsoft.com/en-us/windows/win32/secauthz/implementing-an-appcontainer).

A Windows preview should combine:

1. a unique per-session AppContainer identity;
2. a dedicated local NTFS worktree or staging root with inheritable session-SID access;
3. explicit read/execute grants for tested SDK/tool roots and read-only dependency caches;
4. no network capabilities;
5. a clean Unicode environment and no generic handle inheritance;
6. a non-breakaway Job Object assigned at process creation, with kill-on-close, active-process,
   memory, CPU, UI, completion-port, watchdog, and output controls as requested;
7. profile/ACL cleanup recorded in a crash-recoverable journal.

Regular AppContainer has some common system registry/COM access. Less-Privileged AppContainer (LPAC)
is stricter but likely less compatible with SDK discovery and legacy tooling. Treat them as separate
tested profiles and report the regular AppContainer base surface; never silently fall back from LPAC
to a broader token.

Restricted tokens and Low Integrity are useful layers, not substitutes:

- [`CreateRestrictedToken`](https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-createrestrictedtoken)
  can remove privileges, mark SIDs deny-only, and add restricting SIDs, but it needs compatible ACLs
  on every file, DLL, registry object, and IPC resource.
- [Mandatory Integrity Control](https://learn.microsoft.com/en-us/windows/win32/secauthz/mandatory-integrity-control)
  normally prevents Low Integrity writes to medium/unlabelled objects; it does not generally prevent
  sensitive reads.
- [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects) own a normal
  child process tree and resources, but do not restrict filesystem, registry, credentials, IPC, or
  network destinations.

### New experimental Windows sandbox API

The June 2026 `Experimental_CreateProcessInSandbox` API maps directly to several DockPipe needs:

- AppContainer creation and identity;
- Bound File System read-only/read-write roots without persistent recursive ACL edits;
- default-deny AppContainer networking and a proxy policy;
- integrity and Win32k controls;
- Job Object UI restrictions;
- fail-closed validation of incompatible fields.

It is nevertheless explicitly experimental, exported dynamically from `processmodel.dll`, lacks a
public header, declares only "Windows 11 (experimental)" as its minimum, and requires a FlatBuffer
specification whose current version is `0.1.0`. Use it for a parallel prototype and conformance
experiments. Do not make it the baseline until Microsoft publishes a supported API/schema and
DockPipe proves ConPTY, nested jobs, reparse points, BFS lifecycle, and proxy bypass behavior.

### Windows network and filesystem limits

Network-off is strong with no AppContainer capability, including host loopback by default. Endpoint
allowlisting is not an MVP feature:

- WFP can enforce AppContainer SID, application, user, address, protocol, and port at ALE layers.
- Adding filters normally requires elevation. A future installed DockPipe policy broker could own
  dynamic WFP sessions and scope them to a unique AppContainer SID.
- Numeric IP/CIDR/port policy could then be `enforced` after conformance testing.
- FQDN rules depend on DNS observations and are vulnerable to caches, DoH, proxies, VPN behavior,
  and address rotation, so hostname policy remains `partially_enforced`.
- A loopback exemption is too broad; per-port localhost access requires WFP or a narrow broker.

The stable Windows preview should reject UNC paths, mapped/network drives, removable/device paths,
reparse-point roots, alternate device namespaces, and deny globs nested under writable roots. It
should begin with a clean dedicated worktree, not mutate arbitrary user checkout ACLs. Junction and
symlink targets still require AppContainer access, but every path form and race must pass adversarial
tests before `filesystem_symlink_escape` is reported as enforced.

### Windows compatibility boundary

Command-line PowerShell, `cmd`, native tools, modern .NET, Node, npm, and MSBuild may work after
explicit tool, registry, temp, and cache grants. Likely friction includes Visual Studio automation,
legacy .NET Framework/GAC/COM tooling, SDK discovery, PowerShell user modules, credential providers,
compiler servers, MSBuild node reuse, local databases, GPUs/devices, and named-pipe services.

The preview should initially disable compiler-server/node reuse and make shared caches read-only.
Compatibility failure is a normal reported result; it must never trigger unrestricted host execution.

## Cross-Platform Policy And Guarantee Contract

### Desired policy versus observed enforcement

DockPipe must keep three objects distinct:

1. **Requested policy**: portable workflow intent such as readable roots and offline networking.
2. **Required guarantees**: the minimum enforcement a workflow will accept.
3. **Capability report**: observed platform support, actual mechanism, assurance level, and limits.

A driver is selected only when every `required` guarantee is both `enforced` and accepted at the
requested assurance level. `preferred` affects selection and warnings but does not block. `optional`
is reported. `disabled` documents deliberate non-use.

Use this exact enforcement-status vocabulary:

| Status | Meaning | Satisfies `required`? |
| --- | --- | --- |
| `enforced` | The active OS mechanism covers the named, narrowly defined guarantee and passed canaries. | Yes, subject to assurance policy |
| `partially_enforced` | Some cases are enforced and the report enumerates uncovered cases. | No |
| `best_effort` | Admission checks, monitoring, or cleanup reduce risk but are bypassable by the workload. | No |
| `unsupported` | The driver cannot provide the guarantee. | No |
| `requires_elevation` | A mechanism exists but the current unprivileged driver cannot install/use it. | No |
| `disabled_by_policy` | Platform or organization policy forbids the mechanism or grant. | No |

Add orthogonal metadata instead of weakening the status meaning:

- `assurance`: `production`, `preview`, or `experimental`;
- `mechanism`: concrete OS primitive(s);
- `coverage`: structured limitations and negotiated ABI/version;
- `evidence`: canary ids and results;
- `platform_support`: `supported`, `deprecated_unsupported`, or other explicit support state.

An experimental mechanism may be kernel-enforced while still unacceptable for a production-required
workflow. The default accepted assurance is `production`.

### Stable enforcement guarantee ids

These are runtime guarantees, not resolver capability ids:

| Id | Narrow meaning |
| --- | --- |
| `filesystem_read_scope` | Paths outside the resolved visible roots cannot be opened for content reads. |
| `filesystem_write_scope` | Persistent writes are limited to resolved writable roots. |
| `filesystem_deny_exact` | Every resolved concrete deny path is inaccessible. |
| `filesystem_deny_continuous` | Future path names matching deny patterns are also inaccessible. |
| `filesystem_path_identity` | Roots are bound to canonical object identity without validation/use races. |
| `filesystem_special_files` | Device, proc, sys, socket, remote-mount, and other special-file exposure is constrained as reported. |
| `network_offline` | No host, LAN, Internet, DNS, or host-loopback network access is possible. |
| `network_endpoint_allowlist` | Only declared numeric address/protocol/port endpoints are reachable. |
| `network_hostname_allowlist` | Only declared hostname/protocol/port endpoints are reachable under the reported DNS/broker semantics. |
| `network_loopback_scope` | Host loopback is denied or limited to declared ports. |
| `process_descendant_inheritance` | Ordinary descendants retain the sandbox boundary and resource ownership. |
| `process_executable_policy` | The exact reported execution policy is enforced; never infer arbitrary-code prevention from it. |
| `process_count_limit` | Aggregate task/process count is capped. |
| `resource_cpu_limit` | Aggregate CPU rate/time is capped as specified. |
| `resource_memory_limit` | Aggregate memory is capped as specified. |
| `resource_open_files_limit` | Open descriptors/handles are capped with the reported scope. |
| `resource_wall_time` | The trusted supervisor terminates the session at the deadline. |
| `resource_output_bytes` | Parent-owned output channels terminate/truncate at the declared cap. |
| `resource_write_bytes` | Aggregate persistent bytes written are capped, not merely measured. |
| `credentials_ambient_isolation` | Ambient environment, files, agents, keychains, sockets, and handles are unavailable unless declared. |
| `teardown_process_tree` | The runtime can terminate and prove absence of every contained descendant. |
| `audit_process_lifecycle` | Process start/exit coverage matches the reported boundary. |
| `audit_policy_violations` | Denied resource attempts are attributed with the reported completeness. |

If a broad guarantee would only be partially enforced, define/request a narrower guarantee rather
than accepting `partially_enforced` as "good enough." For example, Linux can enforce
`filesystem_deny_exact` for prepared mask mounts while not enforcing `filesystem_deny_continuous` for
new glob matches inside a writable root.

### Filesystem policy semantics

- `workspace` resolves relative to the trusted workflow source/workspace identity, not the current
  shell after untrusted code runs.
- `write` implies read. `read` without `write` is read-only. `deny` overrides visibility, but a deny
  nested below a writable root must be proven by the selected driver or fail the corresponding
  requirement.
- Runtime/tool/system roots added by a driver are part of resolved policy and must be visible in the
  report. "Automatic" never means hidden from audit.
- A private synthetic home and temp are defaults. The real home is not mounted.
- Shared caches default to read-only. Writable caches default to per-session storage and are treated
  as persistent attack surfaces if reused.
- Every root is normalized with OS APIs: symlink/reparse resolution, volume/device identity, case
  behavior, and final object id. String-prefix checks are invalid.
- Nonexistent writable destinations are resolved through their existing canonical parent and created
  by the trusted runtime.
- Conflicting aliases/overlaps are validation errors unless one is an explicit, provably enforceable
  deny child.
- Mounted drives, remote filesystems, UNC paths, devices, Unix sockets, named pipes, and special
  files are denied by default and require separate driver support.
- Artifact collection is relative to approved roots, does not follow escaping links, enforces size
  limits, and never executes collected content.

### Network policy semantics

- `offline` means no Internet, LAN, host loopback, or DNS. Sandbox-private loopback may exist only for
  processes inside the same boundary.
- `allowlist` is deny-by-default and every entry specifies protocol plus ports and exactly one of
  hostname, IP, CIDR, loopback, or brokered resource identity.
- DNS mode is explicit: `disabled`, `brokered`, or `system`. A report states who resolved each name,
  which addresses were used, TTL/update behavior, private-address rejection, and whether direct IP
  bypass was possible.
- Proxies are part of the enforcement mechanism, not ambient environment. Proxy variables inherited
  from the user are removed.
- `internet` is an explicit broad grant, not an automatic package-manager convenience.
- Local databases and TCP MCP servers are loopback/LAN grants. Unix sockets, Windows named pipes, D-Bus,
  Mach services, COM, and similar transports are IPC grants even if a user thinks of them as network.
- Endpoint allowlisting limits connection destinations, not the meaning of traffic. An allowed server
  can relay or tunnel data; secrets exposed to a networked process remain exfiltration-capable.

### Process and executable semantics

Process `allow`/`deny` lists provide intent validation, command resolution, approvals, and audit. They
are not the primary security boundary.

- Resolve the initial executable to an absolute path/object identity before launch and record it.
- Build a controlled `PATH`; clear dynamic-loader and language injection variables; use a known shell
  only when the command requires one.
- A shell built-in has no new executable event.
- An allowed shell, Node, Python, PowerShell, .NET, Java, compiler, linker, MSBuild, or package manager
  can run arbitrary scripts or code.
- Execution denied from writable directories can block direct `exec`, but interpreters, JITs,
  `dlopen`, modules, writable DLL/shared-library search paths, and memory-backed execution remain.
- Downloaded binaries and generated tests should use an explicit executable output root when direct
  execution is needed. A stricter build phase can write without executing, followed by a narrower
  test phase.
- Denying the `docker` CLI is meaningless if a Docker socket is exposed; omitting the socket is the
  real boundary.

### Secrets and credential semantics

Default ambient credential posture is deny:

- do not inherit token/password environment variables;
- do not expose SSH/Pageant agents, cloud credential stores, browser profiles/sessions, OS keychains,
  Docker sockets, Kubernetes configuration, Git credential managers, or user package-manager auth;
- use a synthetic home so shell startup files and user configuration do not leak in;
- keep Git clone/fetch/checkpoint/publish credentials in runtime-owned helpers, not AI workers;
- inject only explicit DockPipe secret references, preferably short-lived and step-scoped, through a
  private environment value, memory/pipe channel, or private temporary file;
- redact values from policy, events, command displays, and artifacts;
- mount package-manager auth only for the install step and combine it with the narrow registry policy;
- remember that a process with both a secret and allowed egress can transmit that secret.

The runtime applies the final environment/handle filter **after** resolver and vault resolution. A
resolver may request credentials; it cannot bypass runtime policy.

### Resource semantics

| Resource | Required interpretation |
| --- | --- |
| Wall time | Parent-controlled deadline, then contained-tree termination |
| CPU | Aggregate session CPU rate/time where supported; per-process fallback is reported separately |
| Memory | Aggregate committed/charged memory where supported |
| Process count | Aggregate descendants/threads according to the platform's documented accounting |
| Open files | Exact scope must say per-process, per-user, or aggregate |
| Temp/disk | Bounded private temp is distinct from aggregate persistent write quota |
| Output | Parent-owned stdout/stderr/PTY byte cap with terminal-sequence sanitization |
| Network bytes | Counted only when every egress path crosses the measuring mechanism |
| Background work | Must remain in the namespace/job/cgroup; brokered external work is disallowed or separately owned |

An unavailable hard limit is `unsupported`, not silently replaced by post-run measurement.
