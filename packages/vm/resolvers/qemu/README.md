# qemu

Packaged QEMU resolver for the `vm` runtime family.

What it owns:

- concrete VMM backend selection for the VM substrate
- Windows-friendly QEMU defaults such as disk bus and NIC model
- baseline guest hardware defaults for the first-party Windows flow
- optional host-to-guest sync settings consumed by the generic vmimage runner before the guest command starts
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

If the guest image is not yet SSH-ready, DockPipe now ships a built-in guest-side provisioning helper at `src/core/assets/scripts/provision-windows-ssh.ps1`.

For first-run setup before SSH exists, the VM runner now auto-attaches guest-readable bootstrap media containing that helper. If you also set `Advanced.BootstrapPath` / `DOCKPIPE_VM_BOOTSTRAP_PATH`, DockPipe merges your extra host file or directory into the same bootstrap media. This is the intended path for provisioning scripts and one-off setup assets when clipboard or sync are not available yet.

When `Network.SshPassword` / `DOCKPIPE_VM_SSH_PASSWORD` is set on a Windows host, DockPipe can use a non-interactive PuTTY transport (`plink` / `pscp`) instead of the default key-oriented OpenSSH path. This is intended as a pragmatic bootstrap mode; key auth still remains supported.

For visible Windows-host sessions, DockPipe now defaults the display backend to `gtk,grab-on-hover=on,window-close=on` because the plain QEMU window default tends to release the pointer too easily at the edges.

For visible sessions, DockPipe leaves clipboard sharing off by default so the baseline QEMU input stack stays stable. If you explicitly want to test the experimental clipboard path, enable `Advanced.Clipboard` / `DOCKPIPE_VM_CLIPBOARD=true`.

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

For package-specific defaults and mappings, prefer typed `inputs:` over raw `DOCKPIPE_VM_*` workflow vars. That keeps the resolver contract authored as field paths such as `General.ExecMode` or `Advanced.KeepAlive`, while still letting users bind their own globals through `from:`.

This resolver exports its typed model surface itself, so consumer workflows do not need to add a manual `types:` path just to use `inputs:`.
