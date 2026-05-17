# windows-vm

First-party dogfood workflow for the generic `vm` runtime on a Linux host,
using the packaged `qemu` resolver as the concrete VM product.

What it does:

- lets you choose between a bootable guest disk and an installer ISO
- gets its concrete VM implementation defaults from the packaged `qemu` resolver
- defaults to Windows 11-friendly VM requirements with TPM and secure boot enabled
- defaults the guest disk to `sata` so Windows setup sees the install disk without manual driver loading
- defaults the guest network adapter to `e1000e` so Windows setup sees networking without manual driver loading
- prompts for missing disk / firmware / ISO paths through the shared prompt primitive
- prompts to reset writable UEFI firmware vars during installer runs when DockPipe detects stale boot-state reuse for the selected disk
- runs a guest command over SSH inside the VM when you boot an existing image
- keeps an interactive installer VM session alive when you install from ISO
- tears the VM down when the DockPipe run exits

Prompting behavior:

- every VM prompt is overrideable from workflow YAML or `--var`
- DockPipe only prompts for a value when the corresponding `DOCKPIPE_VM_*` field is still empty
- if you fully specify the VM in YAML, the workflow can run without any configuration prompts except explicit safety confirmations

Important boundaries:

- DockPipe does not ship Windows media, licenses, or driver ISOs
- you supply any guest disk, installer ISO, firmware files, and license rights yourself
- DockPipe does not bypass Windows 11 checks; it configures virtual TPM and secure boot instead
- if you choose `Boot existing disk image`, the guest still needs to be reachable over SSH
- if you choose `Install from ISO`, DockPipe keeps the installer session interactive

Useful variables:

- `DOCKPIPE_VM_BOOT_SOURCE=image|installer-iso`
- `DOCKPIPE_VM_DISK`
- `DOCKPIPE_VM_DISK_BUS=auto|virtio|sata|ide|nvme`
- `DOCKPIPE_VM_NET_DEVICE=auto|virtio|e1000e|e1000|rtl8139`
- `DOCKPIPE_VM_DISK_SIZE` (leave empty to be prompted, with `64G` suggested)
- `DOCKPIPE_VM_TPM=required|optional|off`
- `DOCKPIPE_VM_SECURE_BOOT=required|optional|off`
- `DOCKPIPE_VM_SSH_USER`
- `DOCKPIPE_VM_FIRMWARE_CODE`
- `DOCKPIPE_VM_FIRMWARE_VARS`
- `DOCKPIPE_VM_CDROM`
- `DOCKPIPE_VM_VIRTIO_ISO`
- `DOCKPIPE_VM_DISPLAY`
- `DOCKPIPE_VM_PERSISTENCE=ephemeral|persistent`
- `DOCKPIPE_VM_HOSTFWD`
- `DOCKPIPE_VM_GUEST_COMMAND`

Resolver-owned defaults:

- packaged resolver: `qemu`
- default backend: `qemu-kvm`
- default disk bus: `sata`
- default network adapter: `e1000e`
- default TPM mode: `required`
- default secure boot mode: `required`
- default CPUs / memory: `4` / `8G`

If you want to avoid prompts entirely, set the fields you already know in YAML. For example, this suppresses the image-vs-ISO prompt and the disk-size prompt:

```yaml
vars:
  DOCKPIPE_VM_BOOT_SOURCE: installer-iso
  DOCKPIPE_VM_CDROM: /home/jamie/Downloads/windows.iso
  DOCKPIPE_VM_DISK: /home/jamie/VMs/windows11.qcow2
  DOCKPIPE_VM_DISK_SIZE: 96G
  DOCKPIPE_VM_TPM: required
  DOCKPIPE_VM_SECURE_BOOT: required
  DOCKPIPE_VM_FIRMWARE_CODE: /usr/share/OVMF/OVMF_CODE_4M.fd
  DOCKPIPE_VM_FIRMWARE_VARS: /home/jamie/VMs/OVMF_VARS_windows.fd
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
  --var DOCKPIPE_VM_PERSISTENCE=persistent \
  --var DOCKPIPE_VM_HOSTFWD='tcp::3389-:3389' --
```
