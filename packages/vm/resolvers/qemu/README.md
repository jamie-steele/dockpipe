# qemu

Packaged QEMU resolver for the `vm` runtime family.

What it owns:

- concrete VMM backend selection for the VM substrate
- Windows-friendly QEMU defaults such as disk bus and NIC model
- baseline guest hardware defaults for the first-party Windows flow
- optional host-to-guest sync settings consumed by the generic vmimage runner before the guest command starts
- optional DockPipe guest agent forwarding for structured status and shutdown on provisioned Windows guests
- host-side delegation for the `vm` runtime; workflows should select `runtime: vm` and `resolver: qemu` and let the resolver own the host bridge

What it does not own:

- Windows media or licenses
- DockPipe core VM lifecycle primitives
- workflow-specific Windows setup values like locale, admin user, or guest command

Current resolver defaults:

- backend: `qemu-kvm`
- disk bus: `sata`
- network adapter: `e1000e`
- TPM: `required`
- secure boot: `required`
- CPUs / memory: `4` / `8G`

Workflow shape:

- use `runtime: vm`
- use `resolver: qemu`
- do not set `kind: host` on steps that use or inherit this resolver

Minimal pattern:

```yaml
runtime: vm
resolver: qemu

steps:
  - id: guest-command
    vm:
      guest_path: C:\uh
      keepalive: true
    cmd: Set-Location C:\uh; whoami; hostname
```

Multiple mount pattern:

```yaml
runtime: vm
resolver: qemu

steps:
  - id: guest-command
    vm:
      mounts:
        - host: C:\Source\worktrees\uh1
          guest: C:\uh
        - host: C:\tmp\artifacts
          guest: C:\artifacts
      keepalive: true
    cmd: Set-Location C:\uh; whoami; hostname
```

Interactive SSH pattern:

```yaml
runtime: vm
resolver: qemu

steps:
  - id: guest-shell
    vm:
      interactive_ssh: true
      guest_path: C:\uh
      keepalive: true
```

Interactive debug pattern:

```yaml
runtime: vm
resolver: qemu

steps:
  - id: guest-debug
    vm:
      interactive_debug: true
    cmd: echo skipped while you inspect the guest manually
```

When `vm.interactive_debug: true` is set, DockPipe tells the VM runner to launch a visible guest session and skip the normal guest-command-over-SSH automation path. Use it when you need to finish manual Windows setup before turning automation back on.

When `vm.interactive_ssh: true` is set, DockPipe waits for guest SSH readiness and then opens a live authenticated shell instead of executing `cmd:` as a one-shot guest command. On Windows guests, DockPipe currently uses a minimal `cmd.exe /d /q /k prompt $P$G` shell even when `ExecMode` is `powershell`, because that path is more stable over the password-authenticated `plink` terminal. Bash mode still opens `bash -li`.

If the guest image is not yet SSH-ready, DockPipe now ships a built-in guest-side provisioning helper at `src/core/assets/scripts/provision-windows-ssh.ps1`.

For first-run setup before SSH exists, the VM runner now auto-attaches guest-readable bootstrap media containing that helper plus `dockpipe-guest-agent.exe`. If you also set `Advanced.BootstrapPath` / `DOCKPIPE_VM_BOOTSTRAP_PATH`, DockPipe merges your extra host file or directory into the same bootstrap media. The filenames `provision-windows-ssh.ps1`, `dockpipe-guest-agent.exe`, `dockpipe-guest-agent.ps1`, and `README.txt` are reserved by DockPipe and are always restaged last so a custom payload cannot accidentally replace them. This is the intended path for provisioning scripts and one-off setup assets when clipboard or sync are not available yet. The provisioning script installs the guest agent as a LocalSystem startup task.

When `Advanced.Agent` / `DOCKPIPE_VM_AGENT=true` is enabled, DockPipe also forwards a guest agent port and can use that control plane for structured readiness and graceful shutdown once the guest has been provisioned.

When `Network.SshPassword` / `DOCKPIPE_VM_SSH_PASSWORD` is set on a Windows host, DockPipe can use a non-interactive PuTTY transport (`plink` / `pscp`) instead of the default key-oriented OpenSSH path. This is intended as a pragmatic bootstrap mode; key auth still remains supported.

For visible Windows-host sessions, DockPipe now defaults the display backend to `gtk,grab-on-hover=on,window-close=on` because the plain QEMU window default tends to release the pointer too easily at the edges.

When the DockPipe guest agent is enabled and provisioned on a Windows guest, DockPipe bridges plain-text clipboard behavior through that agent on Windows hosts. That path does not need SPICE guest tools or an extra workflow flag; it starts automatically once the guest agent is reachable.

Incorrect pattern:

```yaml
runtime: vm
resolver: qemu

steps:
  - id: guest-command
    kind: host
    cmd: echo should-not-be-host
```

`qemu` already delegates through the resolver/runtime bridge. Marking the step as `kind: host` conflicts with that contract and DockPipe will reject it.

When `vm.guest_path` is set, DockPipe treats the effective workdir as the default host context and maps it into the guest before the guest command runs. Override that with `vm.host_context` when you want to sync a different host directory.

When you need more than one host-to-guest mapping, use `vm.mounts`. Each `host` -> `guest` pair is applied in order before the guest command runs. The older `vm.host_context` + `vm.guest_path` shape remains as sugar for a single default mapping.

For package-specific defaults and mappings, prefer typed `inputs:` over raw `DOCKPIPE_VM_*` workflow vars. That keeps the resolver contract authored as field paths such as `General.ExecMode` or `Advanced.KeepAlive`, while still letting users bind their own globals through `from:`.

This resolver exports its typed model surface itself, so consumer workflows do not need to add a manual `types:` path just to use `inputs:`.
