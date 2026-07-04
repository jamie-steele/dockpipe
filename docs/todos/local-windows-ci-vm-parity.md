# TODO-011 Local Windows CI VM Parity And Guest Tooling

## Current State

- `workflows/ci/ci-emulate` now treats Linux and Windows differently on purpose:
  - Linux hosts run the Ubuntu CI job through `act` and run the Windows CI subset through the
    first-party `windows-vm` workflow.
  - Windows hosts run the Windows CI subset directly on the host and keep using `act` for the
    Linux CI job.
- The Linux VM path currently assumes a valid Windows guest image is already provisioned and
  reachable over SSH.

## Still Open

- Improve the Windows guest bootstrap and guest-agent path so local CI validation needs less manual
  VM preparation.
- Add stronger repo-owned docs or helper flows for standing up the Windows CI guest image the first
  time.
- Decide whether more of the GitHub `windows-latest` tool/image contract should be mirrored inside
  the local Windows VM.
- Revisit whether additional Windows-local checks should move into the VM path once guest tooling
  and setup time improve.
