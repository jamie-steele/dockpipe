# windows-vm

First-party dogfood workflow for the generic `vm` runtime, using the packaged
`qemu` resolver as the concrete VM product.

What it does:

- lets you choose between a bootable guest disk and an installer ISO
- gets its concrete VM implementation defaults from the packaged `qemu` resolver
- auto-detects the host backend: `qemu-kvm` on Linux, `qemu-windows` on Windows
- defaults to Windows 11-friendly VM requirements with TPM and secure boot enabled
- defaults the guest disk to `sata` so Windows setup sees the install disk without manual driver loading
- defaults the guest network adapter to `e1000e` so Windows setup sees networking without manual driver loading
- supports host PCI / GPU passthrough through the packaged `qemu` resolver on Linux hosts when devices are already isolated for `vfio-pci`
- prompts for missing disk / firmware / ISO paths through the shared prompt primitive
- prompts to reset writable UEFI firmware vars during installer runs when DockPipe detects stale boot-state reuse for the selected disk
- runs a guest command over SSH inside the VM when you boot an existing image
- can use a DockPipe-owned guest agent service for structured readiness, observability, and graceful shutdown after the guest is provisioned
- can optionally open an interactive authenticated SSH shell instead of running a one-shot guest command
- can optionally copy a host file tree into the guest over SCP before the guest command runs
- automatically attaches guest-readable bootstrap media containing the built-in `provision-windows-ssh.ps1` helper before the VM starts
- supports an agent-backed best-effort plain-text clipboard bridge for Windows-host visible VM sessions once the guest agent is provisioned
- can optionally merge extra host-provided bootstrap files into that media before the guest starts
- can optionally keep the VM running after the guest command completes for manual SSH setup work
- keeps an interactive installer VM session alive when you install from ISO
- tears the VM down when the DockPipe run exits

Prompting behavior:

- every VM prompt is overrideable from workflow YAML or `--var`
- DockPipe only prompts for a value when the corresponding `DOCKPIPE_VM_*` field is still empty
- DockPipe infers `DOCKPIPE_VM_BOOT_SOURCE=image` when `DOCKPIPE_VM_DISK` is already set, and infers `installer-iso` when `DOCKPIPE_VM_CDROM` is set
- if you fully specify the VM in YAML, the workflow can run without configuration prompts or repeat safety confirmations

Authoring surfaces:

- use `inputs:` when you want to set typed package fields such as `General.ExecMode` or `Advanced.KeepAlive`
- use `inputs.<field>.from` when you want to map your own env or vault-backed variable into a package field without renaming it to `DOCKPIPE_VM_*`
- keep `vars:` for raw workflow-global env that is not package-specific
- you do not need to add a local `types:` entry just to consume the packaged `qemu` field model

Important boundaries:

- DockPipe does not ship Windows media, licenses, or driver ISOs
- you supply any guest disk, installer ISO, firmware files, and license rights yourself
- DockPipe does not bypass Windows 11 checks; it configures virtual TPM and secure boot instead
- if you choose `Boot existing disk image`, the guest still needs to be reachable over SSH
- if you choose `Install from ISO`, DockPipe keeps the installer session interactive
- the current Windows-host backend does not support PCI passthrough, and TPM emulation still requires Linux-host tooling

Workflow contract:

- this workflow intentionally uses `runtime: vm` with `resolver: qemu`
- the resolver owns the host-side VM bridge and guest SSH handoff
- steps in workflows built on this pattern should not add `kind: host`

Minimal example shape:

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

Do not do this:

```yaml
steps:
  - id: guest-command
    kind: host
    cmd: echo wrong-shape
```

If you need a plain host step, make it a separate host-oriented workflow step that does not use or inherit the `qemu` resolver.

Mixed host + guest example:

```yaml
steps:
  - id: prep
    kind: host
    cmd: Write-Host "prepare local state"

  - id: guest-command
    runtime: vm
    resolver: qemu
    vm:
      guest_path: C:\uh
      keepalive: true
    cmd: Set-Location C:\uh; whoami; hostname

  - id: report
    kind: host
    cmd: Write-Host "print connection info"
```

`vm.guest_path` uses the current DockPipe workdir as the default host sync source. Set `vm.host_context` only when you want to mount a different host folder into the guest.

When you need more than one host-to-guest mapping, use `vm.mounts`. Each `host` -> `guest` pair is applied in order before the guest command runs. The older `vm.host_context` + `vm.guest_path` shape remains as sugar for one default mapping.

Useful variables:

- `DOCKPIPE_VM_BOOT_SOURCE=image|installer-iso`
- `DOCKPIPE_VM_DISK`
- `DOCKPIPE_VM_DISK_BUS=auto|virtio|sata|ide|nvme`
- `DOCKPIPE_VM_NET_DEVICE=auto|virtio|e1000e|e1000|rtl8139`
- `DOCKPIPE_VM_DISK_SIZE` (leave empty to be prompted, with `64G` suggested)
- `DOCKPIPE_VM_TPM=required|optional|off`
- `DOCKPIPE_VM_SECURE_BOOT=required|optional|off`
- `DOCKPIPE_VM_SSH_USER`
- `DOCKPIPE_VM_SSH_PASSWORD`
- `DOCKPIPE_VM_SSH_PORT`
- `DOCKPIPE_VM_FIRMWARE_CODE`
- `DOCKPIPE_VM_FIRMWARE_VARS`
- `DOCKPIPE_VM_CDROM`
- `DOCKPIPE_VM_VIRTIO_ISO`
- `DOCKPIPE_VM_DISPLAY`
- `DOCKPIPE_VM_PCI_DEVICES` (comma-separated BDFs such as `0000:01:00.0,0000:01:00.1`)
- `DOCKPIPE_VM_GPU_PRIMARY=true|false`
- `DOCKPIPE_VM_ALLOW_BOOT_VGA=true|false`
- `DOCKPIPE_VM_PERSISTENCE=ephemeral|persistent`
- `DOCKPIPE_VM_HOSTFWD`
- `DOCKPIPE_VM_CONFIRM_PROMPTS`
- `DOCKPIPE_VM_BOOTSTRAP_PATH`
- `DOCKPIPE_VM_AGENT=true|false`
- `DOCKPIPE_VM_AGENT_PORT`
- `DOCKPIPE_VM_SYNC_HOST_PATH`
- `DOCKPIPE_VM_SYNC_GUEST_PATH`
- `DOCKPIPE_VM_KEEPALIVE=true|false`
- `DOCKPIPE_VM_KEEPALIVE_SECONDS`
- `DOCKPIPE_VM_GUEST_COMMAND`

Preparing a Windows guest for SSH:

- use `vm.interactive_debug: true` for the first manual boot when the image is not yet automation-ready
- use `vm.interactive_ssh: true` when the guest is SSH-ready and you want DockPipe to log you into a live shell instead of running a single scripted command
- once you are inside the guest, open an elevated PowerShell session
- the DockPipe VM runner itself now stages `provision-windows-ssh.ps1` and `README.txt` onto guest-readable bootstrap media
- that same bootstrap media also carries `dockpipe-guest-agent.exe`, and `provision-windows-ssh.ps1` installs it as a LocalSystem startup task by default
- if you want extra files available on that same media, point `Advanced.BootstrapPath` / `DOCKPIPE_VM_BOOTSTRAP_PATH` at a host file or directory and DockPipe will merge it into the staged bootstrap payload
- `provision-windows-ssh.ps1`, `dockpipe-guest-agent.exe`, `dockpipe-guest-agent.ps1`, and `README.txt` are reserved bootstrap filenames; DockPipe always restages its built-in copies last so a custom payload cannot replace them by accident
- the script installs OpenSSH Server, enables the firewall rule for TCP 22, configures `sshd` to start automatically, and can create or update a local `dockpipe` user

Example bootstrap path:

```yaml
inputs:
  Advanced.BootstrapPath: C:\vm\bootstrap
```

If `C:\vm\bootstrap` contains extra provisioning files, DockPipe merges them into the same guest-readable media that already includes `provision-windows-ssh.ps1`.

If you do not set `Advanced.BootstrapPath`, DockPipe still attaches the built-in provisioning script automatically.

Guest agent notes:

- `Advanced.Agent: true` enables the host-side forward and allows DockPipe to use the guest agent when it is available
- the guest agent gives DockPipe a better status/control path than raw SSH alone
- after provisioning, DockPipe can use the agent for readiness checks and graceful shutdown even when SSH is flaky

Clipboard notes:

- once `provision-windows-ssh.ps1` installs the DockPipe guest agent, DockPipe automatically bridges plain-text clipboard contents between the Windows host and guest during visible sessions
- that agent-backed clipboard path does not require SPICE guest tools or a workflow flag
- clipboard sharing still depends on guest support and display/backend timing, so treat it as a convenience layer rather than a hard guarantee

Display notes:

- visible Windows-host sessions now default to `gtk,grab-on-hover=on,window-close=on` because that behaves better than the plain QEMU window default for pointer capture
- override `Compute.Display` or `DOCKPIPE_VM_DISPLAY` if you want a different backend or GTK tuning

Example from an elevated PowerShell session inside the guest:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\provision-windows-ssh.ps1 -UserName dockpipe -PasswordPlain 'ChangeMe123!' -GrantAdministrators $true
```

If you prefer key-based auth:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\provision-windows-ssh.ps1 `
  -UserName dockpipe `
  -PasswordPlain 'ChangeMe123!' `
  -AuthorizedKey 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... your-key-comment' `
  -GrantAdministrators $true
```

After the script succeeds:

- reboot once and verify `sshd` still starts automatically
- test locally inside the guest with `Get-Service sshd`
- test through DockPipe with `ssh -p 2222 dockpipe@localhost`

Resolver-owned defaults:

- packaged resolver: `qemu`
- default backend: `auto` (`qemu-kvm` on Linux, `qemu-windows` on Windows)
- default disk bus: `sata`
- default network adapter: `e1000e`
- default TPM mode: `required`
- default secure boot mode: `required`
- default CPUs / memory: `4` / `12G`
- optional host PCI passthrough via `vfio-pci`

GPU / PCI passthrough notes:

- DockPipe does not bundle VFIO kernel support or IOMMU firmware settings for you
- PCI passthrough is currently Linux-host-only
- DockPipe validates that the selected devices exist and are using `vfio-pci`
- DockPipe also validates that each selected device is isolated in its IOMMU group, or that you deliberately selected the whole group
- if they are not yet on `vfio-pci`, DockPipe can now prompt to help rebind them for the current host session
- if a selected device is the host boot/display adapter, DockPipe asks for explicit confirmation unless you set `DOCKPIPE_VM_ALLOW_BOOT_VGA=true`
- common GPU passthrough pairs include the display function and HDMI/DP audio function, for example `0000:01:00.0,0000:01:00.1`
- if your GPU shares an IOMMU group with unrelated hardware, passthrough usually needs a different PCIe slot, different firmware settings, or an ACS override that you explicitly accept

If you want to avoid prompts entirely, set the fields you already know in YAML. For example, this suppresses the image-vs-ISO prompt and the disk-size prompt:

```yaml
inputs:
  General.BootSource: installer-iso
  Storage.Cdrom: /home/jamie/Downloads/windows.iso
  Storage.Disk: /home/jamie/VMs/windows11.qcow2
  Storage.DiskSize: 96G
  Security.Tpm: required
  Security.SecureBoot: required
  Firmware.FirmwareCode: /usr/share/OVMF/OVMF_CODE_4M.fd
  Firmware.FirmwareVars: /home/jamie/VMs/OVMF_VARS_windows.fd
```

If you want to keep your own global or vault-fed names and map them cleanly into the package contract:

```yaml
vars:
  UH_VM_KEEPALIVE: op://uh/windows-vm/keepalive
  UH_VM_SYNC_ROOT: /home/jamie/src/app

inputs:
  Advanced.KeepAlive:
    from: UH_VM_KEEPALIVE
    value: false
  Advanced.SyncHostPath:
    from: UH_VM_SYNC_ROOT
  Advanced.SyncGuestPath: C:\app
```

Examples:

```bash
dockpipe --workflow windows-vm --
```

```bash
dockpipe --workflow windows-vm \
  --var DOCKPIPE_VM_BOOT_SOURCE=installer-iso \
  --var DOCKPIPE_VM_CDROM=/home/jamie/Downloads/windows.iso \
  --var DOCKPIPE_VM_DISK=/home/jamie/VMs/windows11.qcow2 --
```

```bash
dockpipe --workflow windows-vm --var DOCKPIPE_VM_GUEST_COMMAND='Get-ComputerInfo | Select-Object WindowsProductName, WindowsVersion' --
```

```bash
dockpipe --workflow windows-vm \
  --var DOCKPIPE_VM_SYNC_HOST_PATH=/home/jamie/src/app \
  --var DOCKPIPE_VM_SYNC_GUEST_PATH='C:\app' \
  --var DOCKPIPE_VM_GUEST_COMMAND='Get-ChildItem C:\app' --
```

```bash
dockpipe --workflow windows-vm \
  --var DOCKPIPE_VM_KEEPALIVE=true \
  --var DOCKPIPE_VM_KEEPALIVE_SECONDS=28800 --
```

```bash
dockpipe --workflow windows-vm \
  --var DOCKPIPE_VM_CONFIRM_PROMPTS=true \
  --var DOCKPIPE_VM_PERSISTENCE=persistent \
  --var DOCKPIPE_VM_HOSTFWD='tcp::3389-:3389' --
```

```bash
dockpipe --workflow windows-vm \
  --var DOCKPIPE_VM_MEMORY=16G \
  --var DOCKPIPE_VM_PCI_DEVICES='0000:01:00.0,0000:01:00.1' \
  --var DOCKPIPE_VM_GPU_PRIMARY=true --
```
