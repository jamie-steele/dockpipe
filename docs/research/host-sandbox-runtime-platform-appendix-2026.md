# Host Sandbox Platform Compatibility Appendix

Date: 2026-07-10

This appendix records mechanism and developer-tool details supporting the recommendation in
[host-sandbox-runtime-2026.md](host-sandbox-runtime-2026.md). It does not change the common contract or
the roadmap in
[host-sandbox-runtime-contract-and-roadmap-2026.md](host-sandbox-runtime-contract-and-roadmap-2026.md).

> **Authoritative audit note:** Read this appendix with the
> [architecture decision](host-sandbox-runtime-design-decision-2026.md) and
> [design-audit addendum](host-sandbox-runtime-audit-addendum-2026.md), which narrow the stable
> Windows filesystem claims and qualify Job Object and macOS inheritance behavior.

## Linux Mechanism Notes

- Bubblewrap is only the namespace/mount constructor. An empty root plus explicit read-only and
  writable binds is the dependable filesystem boundary; the arguments and active canaries are the
  policy proof.
- User namespaces give setup capabilities only over resources governed by that user namespace. They
  do not give privilege in the initial host namespace, but they increase exposed kernel attack
  surface and can be disabled by distro policy.
- Mount propagation must be private. Nested mounts, NFS/SMB/FUSE, device files, `/proc` descriptors,
  Unix sockets, and pre-opened descriptors are distinct exposure paths.
- PID namespaces hide host processes and provide a reaping init. IPC/UTS namespaces reduce ambient
  shared state. A network namespace provides strong offline behavior. None of those imposes CPU or
  memory limits by itself.
- `no_new_privs` and empty capability sets/bounding set prevent setuid/file-cap privilege gain.
  Nested user namespaces must be disabled after setup.
- Seccomp is inherited by normal children and reduces syscall attack surface. It cannot dereference
  pathname/socket-address pointers and is not a standalone sandbox. Use a conservative denylist
  compatible with compilers and debuggers rather than a large syscall allowlist.
- Landlock is unprivileged and inherited, but it is hierarchy allowlisting rather than a glob-deny
  engine. Pre-opened FDs remain usable; ABI 1/2 cannot deny truncation; current metadata and special
  filesystem gaps must remain explicit. Its TCP/UDP controls are port-based, not IP/hostname based.
- A parent-owned cgroup v2 leaf can enforce process, memory, and CPU limits and use `cgroup.kill` for
  race-safe teardown only when the relevant controllers are available/delegated. A cgroup namespace
  alone is view isolation, not resource control.
- AppArmor/SELinux can be valuable administrator-installed defense in depth. Loading/transitioning
  arbitrary policy is not a portable rootless MVP and must not be required silently.

Ordinary host shells, compilers, SDKs, and offline package managers work when their executable,
library, config, temp, and cache roots are explicitly present. System roots should be read-only.
Shared package caches should be read-only, with writes redirected to a private session cache. GUI,
GPU, KVM, Docker/Podman, local databases, D-Bus/systemd services, and host loopback require separate
explicit capabilities and are excluded from the MVP.

## macOS Developer-Tool Compatibility

Custom Seatbelt/SBPL can technically run ordinary development commands with inherited kernel policy,
but the launcher must maintain an OS-version-sensitive allowlist of files, sysctls, Mach services,
IOKit, sockets, and IPC. The custom profile interface is deprecated/unsupported, so these practical
observations do not make it a supported DockPipe product boundary.

- **Shells and command-line tools:** `sh`/`zsh`, Git, Clang, Node, and modern .NET can often run after
  system libraries, SDKs, temp, PTY, config, and executable roots are granted. Shell startup files and
  the real home must remain absent.
- **Generated code:** executing tests/compiler output requires execute access to a writable output
  root. That necessarily permits hostile generated binaries there; a command allowlist does not
  repair this.
- **Homebrew:** read-only use of existing `/opt/homebrew` or `/usr/local` installations is plausible
  under Seatbelt. `brew install`, upgrade, or package lifecycle work needs broad writes, network, temp,
  and many subprocesses and is outside the MVP.
- **npm/NuGet and similar package managers:** offline operations can use read-only shared caches plus
  session temp/cache. Online installation needs a network broker and explicit short-lived auth; all
  lifecycle scripts inherit the sandbox.
- **Xcode:** basic compiler invocations may be made to work, but `xcodebuild`, Simulator/device tests,
  code signing, Swift build services, DerivedData, CoreSimulator, keychains, and developer services use
  XPC/Mach services and long-lived daemons. Granting them materially broadens the boundary, while
  delegated service work does not simply become a descendant. Basic compilation and full Xcode
  testing would require distinct profiles and conformance suites.
- **Broker escapes:** `open`, `osascript`, Apple Events, LaunchServices, `launchctl`, Keychain/securityd,
  Docker sockets, and Simulator spawning must be denied or modeled as explicit external operations.

Supported App Sandbox does not solve these cases: its app-bundle entitlement and security-scoped
bookmark model is too static, network access is coarse, and arbitrary developer executables/build
outputs do not fit its supported helper model. Endpoint Security plus Network Extension is a possible
installed privileged security product, not a lightweight command runtime.

## Windows Mechanism Notes

### Restricted tokens and Low Integrity

`CreateRestrictedToken` can remove privileges, mark SIDs deny-only, and add restricting SIDs so both
the normal and restricting access checks must pass. It becomes a strong path boundary only when every
file, DLL, registry object, and IPC object has compatible ACLs. A restricted-token-only mode must
therefore remain `best_effort` for the requested broad contract.

Low Integrity's normal mandatory policy prevents writes to medium/unlabelled objects but generally
does not prevent reads. It cannot express repository roots or network destinations. AppContainer
already combines Low Integrity with its stronger dual-principal model.

### AppContainer and child processes

Stable AppContainer is the primary boundary. Create a unique session profile/SID, grant only the
dedicated NTFS worktree and tested read-only tool/cache roots, grant no network capabilities, and
launch via `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES`. Ordinary children created by PowerShell,
`cmd.exe`, native tools, Node, .NET, and MSBuild inherit the AppContainer token. The non-breakaway Job
Object independently owns the normal process tree and resources.

Regular AppContainer retains access to selected common system files, registry keys, and COM objects;
the resolved report must enumerate that platform base rather than claim a literal filesystem-only
view. LPAC reduces that surface but needs explicit registry/COM capabilities and may break SDK,
legacy .NET, and tool discovery. Never silently fall back from LPAC to regular AppContainer.

### Job Objects and desktop/session controls

Create the child suspended or assign the Job at creation, set neither breakaway flag, and use
kill-on-close. Jobs can enforce active-process, job/process memory, CPU time/rate, and selected UI
limits and provide completion-port lifecycle notifications. A parent watchdog supplies wall time and
bounded pipes/ConPTY supply output limits.

Job UI limits can restrict clipboard, foreign USER handles, global atoms, display/system settings,
desktop switching, and exit operations. Win32k system-call disablement gives a stronger headless
profile but breaks GUI-dependent tooling. Restricted-token documentation also recommends a separate
desktop to avoid window-message attacks. Interactive GUI isolation is therefore a separate partial
profile, not MVP behavior.

Some build tools create nested jobs; modern Windows supports nested jobs, but each toolchain must pass
compatibility tests. WMI-created processes and work delegated to COM, BITS, Task Scheduler, SCM, or a
service are not safely assumed to stay in the Job. Those broker surfaces must remain unavailable.

### Named objects, registry, services, and devices

AppContainer has a private named-object namespace and ACL-mediated access to named pipes. Do not grant
Docker/container-engine pipes, Pageant/Git credential pipes, compiler servers, arbitrary COM, WMI,
BITS, Task Scheduler, SCM, browser automation, or host-service endpoints. Regular AppContainer common
registry/COM access is broader than LPAC and must be reported.

Device, GPU, camera/microphone, raw disk, `\\.\`, `GLOBALROOT`, mount-point, UNC, mapped-drive, and
removable-media access are excluded from the preview. Reparse points, junctions, hard links, ADS, 8.3
names, case variants, and `\\?\` paths require adversarial tests against the final object identity.

### Process mitigation policies

Creation-time DEP, ASLR-related policy, strict handle checks, Win32k disablement, image-load/signature
policy, dynamic-code prohibition, and child-process policy are exploit hardening rather than the
filesystem/network boundary.

- Win32k disablement is suitable for a headless strict profile.
- Dynamic-code prohibition breaks JIT-based Node and .NET workloads.
- Microsoft-signed-only/image restrictions break ordinary SDKs and developer tools.
- Child-process prohibition conflicts with builds and tests.

Expose mitigations as separately reported optional profiles after compatibility probes; never claim
they replace AppContainer or Job containment.

### ACL-scoped temporary users

A temporary or pre-provisioned local account provides a separate profile and useful NTFS ownership,
but account creation normally requires elevation, profile creation/cleanup is heavier, broad
`Users`/`Authenticated Users` ACLs still exist, networking stays unrestricted without WFP, and COM,
services, scheduled tasks, public pipes, and persistence remain. Existing SDK/caches need grants and
account/password lifecycle becomes sensitive state. It can be an enterprise-managed stronger profile,
not the lightweight default. A unique AppContainer SID is the better MVP execution identity.

### Network enforcement

No AppContainer network capability gives strong Internet/LAN/host-loopback denial. `internetClient`
is broad Internet access, not an allowlist. A loopback exemption is broad, not per-port.

WFP can filter at ALE layers by AppContainer/package SID, executable, user, protocol, address, and
port, but installing filters normally needs administrative authority. A future privileged DockPipe
broker can own dynamic filters scoped to the unique AppContainer SID. Numeric IP/CIDR/port rules may
then be `enforced`; FQDN rules remain partial because DNS visibility, caches, DoH, proxies, VPNs, and
address rotation break a durable hostname identity.

### Windows Sandbox applicability

Windows Sandbox is a Hyper-V-backed disposable environment with a separate kernel. It is valuable as
a stronger future DockPipe runtime, but it is explicitly a VM, does not reuse installed host
applications as native processes, and violates this runtime's no-VM objective. It is not an API layer
for constraining one arbitrary host process.

### Experimental composable sandbox API

Microsoft's June 2026 `Experimental_CreateProcessInSandbox` API is the highest-value Windows spike:
AppContainer, Bound File System read-only/read-write roots, default-deny networking/proxy, integrity,
Win32k, and Job UI controls closely match the contract and fail unsupported combinations. It is still
experimental, has no public header, dynamically loads from `processmodel.dll`, and uses FlatBuffer
schema `0.1.0`. Treat it as `experimental` assurance until Microsoft supplies stable availability,
schema, servicing, and compatibility commitments.

## Platform Resource/Teardown Snapshot

| Guarantee | Linux qualified MVP | Windows preview | Supported macOS lightweight driver |
| --- | --- | --- | --- |
| Workspace/extra write roots | `enforced` by mount view | `enforced` by AppContainer SID ACL/BFS after canaries | `unsupported` |
| Sensitive reads outside roots | `enforced` by absent mounts | `enforced` by AppContainer dual principal after canaries | `unsupported` |
| Continuous deny glob under writable root | `unsupported` | `unsupported` | `unsupported` |
| Network offline | `enforced` by net namespace | `enforced` with no network capability | `unsupported` for requested arbitrary-tool shape |
| Hostname allowlist | `unsupported` MVP | `partially_enforced` with later WFP/broker | `unsupported` |
| Child inheritance | `enforced` | `enforced` for ordinary children; external brokers denied | App Sandbox/Seatbelt inherits, but launcher unsupported |
| CPU/memory/process count | `enforced` with delegated cgroup controllers | `enforced` with Job Object | `partially_enforced` POSIX limits only |
| Open-file limit | Per-process `enforced`/reported; not aggregate | `unsupported` as Job-wide hard count | Per-process `partially_enforced` |
| Aggregate persistent write quota | `unsupported` on arbitrary host FS | `unsupported` | `unsupported` |
| Wall time/output | `enforced` by parent | `enforced` by parent/Job | `best_effort` process-group parent |
| Complete contained-tree teardown | `enforced` with cgroup kill | `enforced` for Job tree; brokers denied | `best_effort` |
| Complete denial telemetry | `unsupported` from namespaces alone | `unsupported` without privileged monitoring | `unsupported` in lightweight design |

## Primary References

- [Linux Landlock](https://docs.kernel.org/userspace-api/landlock.html)
- [Linux seccomp](https://docs.kernel.org/userspace-api/seccomp_filter.html)
- [Linux cgroup v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- [Bubblewrap](https://github.com/containers/bubblewrap/blob/main/README.md)
- [Apple custom sandbox guidance](https://developer.apple.com/forums/thread/661939)
- [Apple App Sandbox entitlements](https://developer.apple.com/library/archive/documentation/Miscellaneous/Reference/EntitlementKeyReference/Chapters/EnablingAppSandbox.html)
- [Apple Endpoint Security](https://developer.apple.com/documentation/endpointsecurity)
- [Apple Network Extension content filters](https://developer.apple.com/documentation/networkextension/content-filter-providers)
- [Microsoft AppContainer launch](https://learn.microsoft.com/en-us/windows/win32/secauthz/implementing-an-appcontainer)
- [Microsoft restricted tokens](https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-createrestrictedtoken)
- [Microsoft Mandatory Integrity Control](https://learn.microsoft.com/en-us/windows/win32/secauthz/mandatory-integrity-control)
- [Microsoft Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
- [Microsoft process mitigation policies](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-setprocessmitigationpolicy)
- [Microsoft Windows Filtering Platform](https://learn.microsoft.com/en-us/windows/win32/fwp/about-windows-filtering-platform)
- [Microsoft Windows Sandbox](https://learn.microsoft.com/en-us/windows/security/application-security/application-isolation/windows-sandbox/)
- [Microsoft experimental sandbox API](https://learn.microsoft.com/en-us/windows/win32/secauthz/createprocessinsandbox)
