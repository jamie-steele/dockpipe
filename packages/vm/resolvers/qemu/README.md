# qemu

Packaged QEMU resolver for the `vm` runtime family.

What it owns:

- concrete VMM backend selection for the VM substrate
- Windows-friendly QEMU defaults such as disk bus and NIC model
- baseline guest hardware defaults for the first-party Windows flow

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
