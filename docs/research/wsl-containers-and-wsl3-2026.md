# WSL Containers, Docker Desktop, And The WSL3 Question

Date: 2026-07-05

Scope: Windows Subsystem for Linux container support, how it differs from Docker Desktop and traditional WSL
distribution workflows, performance implications for developer/runtime tooling, and what "WSL3" likely means for
DockPipe planning.

## Executive Summary

WSL Containers (`wslc`) is now a real Microsoft platform feature, but it is still a public preview. The feature
appeared in the WSL `2.9.3` pre-release on 2026-06-29 and adds native Linux container lifecycle, image, network,
volume, GPU, session, CLI, SDK, and management surfaces directly under WSL. It is not a replacement for Docker
Desktop yet. It is a new Windows-owned container substrate that could eventually reduce the need for Docker Desktop
for simple local Linux container runs, embedded Windows apps that need Linux containers, and managed enterprise
developer environments.

For DockPipe, the practical position is:

- Keep Docker as the stable default Windows container backend for now.
- Track `wslc` as a future first-class runtime/resolver target once it reaches general availability and proves stable.
- Expect better Windows-host file sharing over time because WSL is moving from older cross-OS file plumbing toward
  `virtiofs` and newer networking modes.
- Treat "WSL3" as a real roadmap signal, especially around NPU passthrough and local AI workloads, but separate that
  from the currently verifiable released WSL artifact stream. The official release/docs I found still show WSL 2.x,
  with `2.9.3` carrying the WSLC public preview.

## Confirmed State

Microsoft's user-facing WSL docs still define WSL 2 as the default architecture. WSL 2 runs a Microsoft-built Linux
kernel inside a managed lightweight VM, with Linux distributions running as isolated containers inside that managed
VM. Microsoft documents that those distributions share the same network namespace, device tree, CPU/kernel/memory,
swap, and `/init`, while keeping separate PID, mount, user, cgroup namespaces, and init processes.

The new container feature is separate from normal WSL distributions. The WSL `2.9.3` pre-release release notes call
WSL Containers public preview and describe it as native Linux container support through a new `wslc` command line tool
and projected SDKs for C++, C#/WinRT, and lower-level C APIs.

The public preview includes:

- Container lifecycle: create, run, start, kill, export, prune, inspect, stop signals, resource limits, `--shm-size`,
  `--ulimit`, CPU, and memory.
- Image operations: build, pull, push, import, save, inspect, list, delete, prune, labels, and multi-image tar save.
- Networking: network create/manage, multi-network attachment, aliases, container network mode, custom network types,
  published ports, and network prune.
- Volumes: create, list, prune, remove, VHD-backed volumes, and UID/GID/fixed driver options.
- GPU: GPU-enabled containers using CDI and mounted GPU executables/libraries.
- Sessions: named sessions, default session on demand, configurable storage location, and a default 32 GB session
  store.
- SDK/tooling: C++, C#/WinRT, C APIs, NuGet packages, plugin API, logs, stats, structured CLI output, MSBuild/CMake
  integration, and ADMX group policy support.

The WSL open-source docs now expose API references for the `wslc` surface. The C end-to-end example shows the shape:
check missing components, initialize session settings, create a session, pull an image, configure a container, start
it, wait for the init process, then stop/delete/release the container and terminate the session.

The SDK is not complete parity across languages. The documented known gaps say some C API capabilities, such as
raw-handle image import/load and raw process callback/IO handles, are hidden or not exposed in the C++/WinRT and C#
projections. That matters if DockPipe ever wants to integrate directly with the SDK instead of shelling out to `wslc`.

## How WSLC Differs From Docker Desktop

Docker Desktop on Windows is a product stack. It includes Docker CLI integration, Docker Engine behavior, Compose,
BuildKit/buildx, image management, Docker Desktop UI, Docker extensions, registry/login flows, Docker Scout, Docker
Model Runner, Kubernetes-related tooling, organization/admin features, and optional hardening features such as
Enhanced Container Isolation.

WSLC is a platform substrate. Its preview focuses on the ability to create, run, inspect, network, store, and manage
Linux containers directly through Windows/WSL, with a CLI and SDK. It does not automatically imply Docker's whole
developer ecosystem:

| Area | Docker Desktop | WSL Containers preview |
| --- | --- | --- |
| Primary purpose | Complete Docker developer platform on Windows | Built-in WSL-backed Linux container substrate |
| CLI | `docker`, `docker compose`, buildx, context ecosystem | `wslc` preview CLI |
| API shape | Docker Engine API, Docker Desktop APIs, ecosystem tooling | WSL container SDK/API for Windows apps |
| Compose | Mature, widely used | Not shown as first-class in preview notes |
| Build ecosystem | BuildKit/buildx, cache exporters, bake, multi-platform workflows | Image build/pull/push/save/import basics in preview |
| UI | Docker Desktop dashboard | No equivalent dashboard in the cited preview |
| Enterprise controls | Docker org policy plus Desktop settings/ECI | ADMX/group policy and Intune-style WSL controls emerging |
| Integration target | General container development | Windows apps and tools that need Linux containers without shipping Docker |
| Stability | Production product | Public preview in WSL pre-release |

The important architectural difference is ownership. Docker Desktop currently uses WSL 2 as a backend on Windows,
storing its engine data under Docker's WSL-managed location and running inside its own `docker-desktop` WSL
distribution. WSLC moves more of that responsibility into Microsoft's WSL layer itself.

That does not mean Docker becomes obsolete. It means Docker may no longer be the only practical way to run Linux
containers locally on Windows. Docker still carries the workflow ecosystem, compatibility expectations, Compose,
BuildKit depth, registry UX, Desktop UI, policy, security tooling, and cross-platform developer muscle memory.

## How WSLC Differs From Running Docker Inside A WSL Distro

Running Docker Engine inside a normal WSL distro is Linux-native but operationally awkward:

- The user owns daemon install, startup, upgrades, socket permissions, systemd behavior, storage, and conflicts.
- Docker Desktop docs explicitly advise uninstalling Docker Engine/CLI installed directly in WSL before enabling
  Docker Desktop's WSL backend because running both can conflict.
- The container runtime lives inside a user distro, so it is tied to that distro's package state and service model.

WSLC moves the container session out of the user's distro. That should reduce "which distro owns the daemon" problems
and make Windows apps able to request Linux containers programmatically without telling the user to bootstrap Docker
inside Ubuntu. The tradeoff is that `wslc` is new and may lack ecosystem features that Docker users assume.

## Performance Notes

### Linux-on-Linux filesystem work remains the fast path

Microsoft's WSL guidance still recommends storing files on the same side as the tools doing the work. If Linux tools
are building or scanning the repo, put the repo in the WSL/Linux filesystem. If Windows tools are operating on the
repo, keep it on NTFS. The cross-filesystem boundary is where historical WSL performance problems show up.

Microsoft's WSL version comparison says early WSL 2 was up to 20x faster than WSL 1 for unpacking a zipped tarball
and around 2-5x faster for `git clone`, `npm install`, and `cmake` on various projects. The same doc still calls out
one exception: WSL 1 can be faster when a project must live on the Windows filesystem and be accessed by Linux tools.

### Docker Desktop already benefits from WSL 2

Docker's WSL backend docs say Docker Desktop with WSL 2 gets Linux workspaces, avoids maintaining duplicate Windows
and Linux build scripts, improves file-system sharing, improves cold-start times, and benefits from dynamic resource
allocation. They also call out `autoMemoryReclaim` as useful after container image builds because it helps Windows
reclaim unused memory from the WSL VM.

### WSLC may reduce stack overhead, but benchmark it

WSLC removes some product-layer dependency for simple runs, but the underlying mechanics are still Linux containers
on a Windows-managed virtualization substrate. Expect the performance question to split by workload:

| Workload | Expected best backend | Why |
| --- | --- | --- |
| Linux package installs/builds against Linux-resident source | WSL 2 distro, Docker Desktop WSL backend, or WSLC | Avoid cross-OS file access |
| Windows checkout bind-mounted into Linux container | Depends on `virtiofs`/sharing mode and file count | Metadata-heavy workloads are sensitive |
| Small CLI tools with short startup | WSLC could be competitive | Fewer product layers may help, but preview must be measured |
| Compose-heavy integration stack | Docker Desktop | Mature Compose/network/volume workflow |
| GPU container development | Docker Desktop now; WSLC later if CDI path stabilizes | Preview says GPU support exists, but ecosystem maturity matters |
| Enterprise locked-down dev boxes | TBD | WSL policy controls are emerging; Docker has its own mature admin surface |

The WSL `2.9.3` release notes also mention lower-level fixes and improvements related to `virtiofs`, `Consomme`
networking, DNS tunneling, VirtioNet tracing, VHD handling, and mirrored networking. Public reporting around the
preview says Microsoft is positioning `virtiofs` as a significantly faster Windows-file access path and `Consomme`
as the newer default networking mode. Treat those as promising platform signals, not DockPipe performance facts until
we run our own workload benchmarks.

### What DockPipe Should Benchmark

DockPipe should avoid deciding this by vibes. A useful Windows benchmark matrix would include:

- `git status`, `rg`, and tree walk over a large Windows checkout bind-mounted into a Linux container.
- Same repo stored inside WSL ext4/VHD and mounted from there.
- `go test ./...`, `npm install`, `cmake`, and archive unpack workloads.
- Container cold start and repeated hot start for tiny commands.
- Bind mount correctness for symlinks, case sensitivity, file watches, chmod bits, and long paths.
- Volume growth/cleanup behavior under repeated Pipeon/DorkPipe runs.
- Network service reachability from Windows host to container and container to Windows host.
- Memory return after large image builds or package installs.

## Security And Isolation

WSL 2 distributions are not full separate VMs per distro. Microsoft documents that WSL 2 distros share major VM
resources and some namespaces, while keeping process/mount/user/cgroup isolation. Docker Desktop's docs are similarly
plain: its WSL 2 integration follows WSL's existing security model, runs inside its own `docker-desktop` distro, and
does not interact with other distros unless integration is enabled. For stricter isolation, Docker recommends Hyper-V
mode or Enhanced Container Isolation.

For WSLC, the preview adds policy management through ADMX/group policy and exposes per-container limits. That is
useful, but preview status matters. Until the security model, CVE handling, update cadence, and enterprise controls
are proven, DockPipe should not treat `wslc` as a stronger isolation boundary than Docker Desktop or a dedicated VM.

Practical DockPipe stance:

- Docker Desktop remains the known Windows container backend.
- Hyper-V or dedicated VM remains the answer for hard isolation.
- WSLC may become attractive for "good local dev isolation with less installed product surface."
- Secrets and host mounts should keep DockPipe's existing explicit-reference and approval rules.

## WSL3 And NPU Passthrough

Correction from the first pass: WSL3 should not be dismissed. Build 2026 reporting explicitly references WSL 3 in the
context of NPU passthrough, where Linux environments would get access to Windows PC NPU hardware for on-device AI
inference. That is a real enough platform signal for DockPipe planning.

The careful distinction is release status. As of 2026-07-05, the official Microsoft Learn docs and Microsoft WSL
GitHub releases I could verify still describe WSL 2 as the default architecture and show the public release stream as
WSL 2.x, with `2.9.3` carrying the WSLC public preview. I did not find an official WSL 3 GitHub release, Microsoft
Learn architecture page, or WSL open-source API page that describes WSL 3 as a released package.

| Layer | Evidence | DockPipe stance |
| --- | --- | --- |
| Released WSL package stream | Official WSL GitHub releases show WSL `2.9.3` pre-release, not a 3.x package | Do not require WSL3 in runtime code yet |
| Current public container feature | WSLC public preview is in WSL `2.9.3` | Benchmark and track as the near-term container backend |
| Build-era platform direction | Reporting says Build 2026 sessions cover NPU passthrough in WSL 3 | Track separately for local AI/model runtime planning |

That distinction matters because WSL3 is not just a container story. If NPU passthrough lands as described, the more
important DockPipe impact is local AI execution:

- Windows laptops with NPUs become more credible local inference hosts for Linux-side tooling.
- Linux agent workers could use Windows AI hardware without requiring cloud fallback for every model lane.
- Container and model scheduling may need to account for CPU, GPU, and NPU availability as separate runtime
  capabilities.
- DockPipe may eventually need capability detection for `wslc`, GPU CDI, NPU exposure, model runtimes, and Windows
  AI Runtime rather than a single "Docker available" check.

WSL2 was mostly a kernel and virtualization boundary shift from syscall translation to a real Linux kernel in a
managed VM. WSL3, based on the available reporting, looks more like a device, AI, and Windows/Linux substrate shift:
better access to host accelerators, better file/network behavior, and more first-party container/runtime APIs. That
could make Windows a stronger local agent/runtime platform even when Docker remains the compatibility default.

The biggest future-landscape impact is not that Docker disappears. It is that Docker becomes one choice on top of a
more capable Windows Linux/container substrate. Docker can keep winning where its ecosystem wins. WSLC can win where
the job is "run this Linux container locally on Windows with minimal product dependency." WSL3/NPU passthrough can win
where the job is "run local Linux AI workloads against Windows accelerator hardware."

## DockPipe Implications

### Near term

Do not switch defaults. Keep Docker Desktop as the Windows path in public docs and support flows. It is mature,
available, and compatible with the Compose-heavy and Docker CLI-heavy workflows users already expect.

### Medium term

Add a tracked experimental runtime target for `wslc` when it is reasonable to test. The right abstraction is not
"Docker alternative" in engine code; it is a runtime capability that can run an image/container with mounts, env,
ports, resource limits, logs, and cleanup. Keep the engine generic and put `wslc` specifics in a Windows/runtime
resolver layer.

### What to avoid

Do not bake `wslc` names or WSL-version assumptions into core engine paths. The existing DockPipe architecture wants
runtime = where, resolver = tool/profile, strategy = lifecycle wrapper. WSLC should fit that model if it is added.

Do not assume `wslc` has Docker Compose, BuildKit cache behavior, registry auth parity, image scanning, or Docker
Desktop admin/security features until confirmed.

Do not assume cross-OS bind mounts are cheap enough for all workflows just because `virtiofs` is improving. Benchmark
DockPipe's actual source trees and agent workloads.

### Likely opportunities

- A lightweight Windows local runtime for simple tool containers without Docker Desktop.
- Cleaner Pipeon/DorkPipe launch flows on locked-down machines where Docker Desktop install policy is painful.
- SDK-level integration from a Windows launcher or future PipeDeck app to create managed Linux task containers.
- Better cleanup and observability if WSLC sessions expose predictable storage/session handles.
- Performance wins for Windows-hosted source bind mounts if `virtiofs` materially improves metadata-heavy workloads.

## Recommendation

Treat WSLC as the most important Windows runtime development to watch for DockPipe, but not as a default backend yet.
The platform direction supports DockPipe's model: a generic engine that can target multiple runtimes while package
assets own platform-specific behavior.

Recommended next steps:

1. Add a `wslc` tracking task once the preview is installable on our Windows dev machines.
2. Build a benchmark script that compares Docker Desktop, WSLC, and direct WSL for DockPipe-like workloads.
3. Prototype a package-owned `wslc` resolver/runtime shim outside `src/lib` once the CLI stabilizes.
4. Keep Docker Desktop docs current as the production path.
5. Track WSL3/NPU passthrough as a separate local-AI runtime task, while keeping WSLC container support as the
   near-term measurable work.

## Sources

- Microsoft Learn, "What is the Windows Subsystem for Linux?" WSL 2 architecture, distro isolation model, WSL open
  source links. https://learn.microsoft.com/en-us/windows/wsl/about
- Microsoft Learn, "Comparing WSL Versions." WSL 1 vs WSL 2 tradeoffs, filesystem performance, WSL 2 Linux kernel,
  Store-serviced WSL updates. https://learn.microsoft.com/en-us/windows/wsl/compare-versions
- Microsoft Learn, "Working across Windows and Linux file systems." Same-side filesystem guidance for performance.
  https://learn.microsoft.com/en-us/windows/wsl/filesystems
- Microsoft WSL GitHub releases, `2.9.3` pre-release. WSLC public preview feature list, `virtiofs`/Consomme/networking
  fixes and improvements. https://github.com/microsoft/WSL/releases
- WSL open-source docs, API reference and C end-to-end example. WSLC session/container/image/process/storage API
  shape. https://wsl.dev/ and https://wsl.dev/api-reference/c/end-to-end-example/
- WSL open-source docs, C++ and C# known gaps. Projection differences from lower-level C APIs.
  https://wsl.dev/api-reference/cpp/not-yet-implemented-and-known-gaps/ and
  https://wsl.dev/api-reference/csharp/known-gaps/
- Docker Docs, "Docker Desktop WSL 2 backend on Windows." Docker Desktop WSL backend behavior, WSL integration,
  storage location, security model, and Enhanced Container Isolation guidance.
  https://docs.docker.com/desktop/features/wsl/
- The Verge, "Microsoft's new developer-optimized Windows embraces Linux even more" (Build 2026 reporting). Public
  preview timing and Microsoft positioning of WSL Containers. https://www.theverge.com/news/941314/microsoft-windows-11-developer-optimized-experience-linux
- TechRadar, "Microsoft just made a huge Linux move..." (2026-07-01 reporting). Notes on public preview availability,
  `virtiofs`, `Consomme`, and interaction with Docker/Podman/Rancher Desktop. https://www.techradar.com/pro/microsoft-just-made-a-huge-linux-move-that-developers-and-container-fans-everywhere-will-love
- TechRadar, "50 Microsoft tools you can use for free just in time for Build 2026" (Build 2026 reporting). Mentions
  Windows AI Runtime sessions and NPU passthrough in WSL 3 for on-device AI inference.
  https://www.techradar.com/pro/50-microsoft-tools-you-can-use-for-free-just-in-time-for-build-2026
- TechRadar, "Windows 12 at Build 2026: What to expect" (Build 2026 reporting). Notes expected WSL performance,
  file-access, networking, onboarding, and enterprise policy focus.
  https://www.techradar.com/pro/windows-12-at-build-2026-what-to-expect
