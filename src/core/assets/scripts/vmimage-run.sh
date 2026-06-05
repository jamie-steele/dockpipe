#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/dockpipe-sdk.sh"
dockpipe_sdk_refresh

vmimage_die() {
  local prefix="${DOCKPIPE_WORKFLOW_NAME:-dockpipe-vmimage}"
  echo "${prefix}: $*" >&2
  exit 1
}

vmimage_log() {
  printf '[dockpipe vmimage] %s\n' "$*" >&2
}

vmimage_env_or_resolver() {
  local primary="$1"
  local resolver="$2"
  local default_value="${3:-}"
  local value="${!primary:-}"
  if [[ -n "$value" ]]; then
    printf '%s\n' "$value"
    return 0
  fi
  value="${!resolver:-}"
  if [[ -n "$value" ]]; then
    printf '%s\n' "$value"
    return 0
  fi
  printf '%s\n' "$default_value"
}

vmimage_host_os() {
  local uname_s
  uname_s="$(uname -s 2>/dev/null || printf '')"
  case "$uname_s" in
    Linux) printf 'linux\n' ;;
    MINGW*|MSYS*|CYGWIN*)
      printf 'windows\n'
      ;;
    *)
      if [[ -n "${WINDIR:-}" || -n "${OS:-}" && "${OS}" == "Windows_NT" ]]; then
        printf 'windows\n'
      else
        printf 'other\n'
      fi
      ;;
  esac
}

vmimage_is_windows_host() {
  [[ "$(vmimage_host_os)" == "windows" ]]
}

vmimage_default_backend() {
  case "$(vmimage_host_os)" in
    linux) printf 'qemu-kvm\n' ;;
    windows) printf 'qemu-windows\n' ;;
    *)
      vmimage_die "vm runtime currently supports Linux and Windows hosts only"
      ;;
  esac
}

vmimage_backend() {
  local configured
  configured="$(vmimage_env_or_resolver "DOCKPIPE_VM_BACKEND" "DOCKPIPE_RESOLVER_VM_BACKEND" "auto")"
  case "$configured" in
    ""|auto)
      vmimage_default_backend
      ;;
    qemu-kvm|qemu-windows)
      printf '%s\n' "$configured"
      ;;
    *)
      vmimage_die "unsupported DOCKPIPE_VM_BACKEND=${configured}"
      ;;
  esac
}

vmimage_default_qemu_bin() {
  if vmimage_is_windows_host; then
    printf 'qemu-system-x86_64.exe\n'
  else
    printf 'qemu-system-x86_64\n'
  fi
}

vmimage_default_qemu_img_bin() {
  if vmimage_is_windows_host; then
    printf 'qemu-img.exe\n'
  else
    printf 'qemu-img\n'
  fi
}

vmimage_windows_qemu_candidates() {
  local exe_name="$1"
  local root
  local -a roots=(
    "$(vmimage_env_value 'ProgramW6432')"
    "$(vmimage_env_value 'PROGRAMFILES')"
    "$(vmimage_env_value 'PROGRAMFILES(X86)')"
    "$(vmimage_env_value 'LOCALAPPDATA')\\Programs"
  )
  local -a rels=(
    "qemu\\${exe_name}"
    "QEMU\\${exe_name}"
  )
  local rel
  for root in "${roots[@]}"; do
    [[ -n "$root" ]] || continue
    for rel in "${rels[@]}"; do
      printf '%s\n' "${root}\\${rel}"
    done
  done
}

vmimage_resolve_host_executable() {
  local exe_name="$1"
  local resolved candidate
  if resolved="$(command -v "$exe_name" 2>/dev/null)"; then
    printf '%s\n' "$resolved"
    return 0
  fi
  if vmimage_is_windows_host; then
    while IFS= read -r candidate; do
      [[ -n "$candidate" ]] || continue
      if [[ -f "$candidate" ]]; then
        printf '%s\n' "$candidate"
        return 0
      fi
    done < <(vmimage_windows_qemu_candidates "$exe_name")
  fi
  return 1
}

vmimage_qemu_bin() {
  local configured="${DOCKPIPE_VM_QEMU_BIN:-}"
  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi
  vmimage_resolve_host_executable "$(vmimage_default_qemu_bin)" || return 1
}

vmimage_qemu_img_bin() {
  local configured="${DOCKPIPE_VM_QEMU_IMG_BIN:-}"
  if [[ -n "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi
  vmimage_resolve_host_executable "$(vmimage_default_qemu_img_bin)" || return 1
}

vmimage_default_accel() {
  case "$(vmimage_backend)" in
    qemu-kvm) printf 'kvm\n' ;;
    qemu-windows) printf 'whpx:tcg\n' ;;
  esac
}

vmimage_default_cpu_model() {
  case "$(vmimage_backend)" in
    qemu-kvm) printf 'host\n' ;;
    qemu-windows) printf 'max\n' ;;
  esac
}

vmimage_env_value() {
  printenv "$1" 2>/dev/null || true
}

vmimage_shell_path() {
  local path_value="${1:-}"
  [[ -n "$path_value" ]] || return 0
  case "$path_value" in
    [A-Za-z]:\\*|[A-Za-z]:/*|\\\\*)
      if command -v cygpath >/dev/null 2>&1; then
        cygpath -u "$path_value"
      else
        printf '%s\n' "$path_value"
      fi
      ;;
    *)
      printf '%s\n' "$path_value"
      ;;
  esac
}

vmimage_powershell_bin() {
  if [[ -n "${DOCKPIPE_VM_PWSH_BIN:-}" ]]; then
    printf '%s\n' "$DOCKPIPE_VM_PWSH_BIN"
    return 0
  fi
  if command -v pwsh.exe >/dev/null 2>&1; then
    printf 'pwsh.exe\n'
    return 0
  fi
  if command -v pwsh >/dev/null 2>&1; then
    printf 'pwsh\n'
    return 0
  fi
  if command -v powershell.exe >/dev/null 2>&1; then
    printf 'powershell.exe\n'
    return 0
  fi
  if command -v powershell >/dev/null 2>&1; then
    printf 'powershell\n'
    return 0
  fi
  vmimage_die "PowerShell was not found on PATH; install pwsh or powershell and rerun windows-vm"
}

vmimage_windows_qemu_helper() {
  printf '%s\n' "${SCRIPT_DIR}/vmimage-run-qemu-windows.ps1"
}

vmimage_require() {
  local name="$1"
  local value="${!name:-}"
  [[ -n "$value" ]] || vmimage_die "required env ${name} is not set"
}

vmimage_prompt_confirm() {
  local prompt_id="$1" title="$2" message="$3" default_value="${4:-no}" intent="${5:-}" automation_group="${6:-}" auto_approve_value="${7:-}"
  local -a args=(
    prompt confirm
    --id "$prompt_id"
    --title "$title"
    --message "$message"
    --default "$default_value"
  )
  if [[ -n "$intent" ]]; then
    args+=(--intent "$intent")
  fi
  if [[ -n "$automation_group" ]]; then
    args+=(--automation-group "$automation_group")
  fi
  if [[ -n "$auto_approve_value" ]]; then
    args+=(--allow-auto-approve --auto-approve-value "$auto_approve_value")
  fi
  dockpipe_sdk "${args[@]}"
}

vmimage_prompt_choice() {
  local prompt_id="$1" title="$2" message="$3" default_value="$4"
  shift 4 || true
  local -a options=("$@")
  local -a args=(
    prompt choice
    --id "$prompt_id"
    --title "$title"
    --message "$message"
    --default "$default_value"
  )
  local option
  for option in "${options[@]}"; do
    args+=(--option "$option")
  done
  dockpipe_sdk "${args[@]}"
}

vmimage_prompt_required_input() {
  local name="$1" title="$2" message="$3" default_value="${4:-}"
  local current="${!name:-$default_value}"
  local response
  response="$(
    dockpipe_sdk prompt input \
      --id "vmimage.${name,,}" \
      --title "$title" \
      --message "$message" \
      --default "$current"
  )" || vmimage_die "prompt failed for ${name}"
  [[ -n "$response" ]] || vmimage_die "required value ${name} was not provided"
  printf -v "$name" '%s' "$response"
  export "$name"
}

vmimage_resource_mode_from_path_mode() {
  case "${1:-open-file}" in
    save-file)
      printf 'new\n'
      ;;
    *)
      printf 'select\n'
      ;;
  esac
}

vmimage_resource_kind_from_path_mode() {
  case "${1:-open-file}" in
    open-dir)
      printf 'directory\n'
      ;;
    *)
      printf 'file\n'
      ;;
  esac
}

vmimage_prompt_resource_value() {
  local name="$1" title="$2" message="$3" path_mode="${4:-open-file}" file_filter="${5:-All Files (*)}" must_exist="${6:-true}"
  local current="${!name:-}"
  local resource_mode resource_kind
  resource_mode="$(vmimage_resource_mode_from_path_mode "$path_mode")"
  resource_kind="$(vmimage_resource_kind_from_path_mode "$path_mode")"
  local -a args=(
    prompt resource
    --id "vmimage.${name,,}"
    --title "$title"
    --message "$message"
    --default "$current"
    --mode "$resource_mode"
    --selection single
    --kind "$resource_kind"
  )
  local filter old_ifs
  local -a __vmimage_filters=()
  old_ifs="$IFS"
  IFS=';'
  read -r -a __vmimage_filters <<< "${file_filter//;;/;}"
  IFS="$old_ifs"
  for filter in "${__vmimage_filters[@]}"; do
    filter="${filter#"${filter%%[![:space:]]*}"}"
    filter="${filter%"${filter##*[![:space:]]}"}"
    [[ -n "$filter" ]] || continue
    args+=(--filter "$filter")
  done
  if [[ "$must_exist" == "true" ]]; then
    args+=(--must-exist)
  fi
  local response
  response="$(dockpipe_sdk "${args[@]}")" || vmimage_die "prompt failed for ${name}"
  printf -v "$name" '%s' "$response"
  export "$name"
}

vmimage_prompt_file_value() {
  local name="$1" title="$2" message="$3" path_mode="${4:-open-file}" file_filter="${5:-All Files (*)}" must_exist="${6:-true}"
  local response=""
  vmimage_prompt_resource_value "$name" "$title" "$message" "$path_mode" "$file_filter" "$must_exist"
  response="${!name:-}"
  [[ -n "$response" ]] || vmimage_die "required file value ${name} was not provided"
}

vmimage_prompt_optional_file_value() {
  local name="$1" title="$2" message="$3" path_mode="${4:-open-file}" file_filter="${5:-All Files (*)}"
  vmimage_prompt_resource_value "$name" "$title" "$message" "$path_mode" "$file_filter" false
}

vmimage_tpm_mode() {
  local mode
  mode="$(vmimage_env_or_resolver "DOCKPIPE_VM_TPM" "DOCKPIPE_RESOLVER_VM_TPM")"
  case "$mode" in
    required|optional|off)
      printf '%s\n' "$mode"
      ;;
    "")
      printf 'off\n'
      ;;
    *)
      vmimage_die "unsupported DOCKPIPE_VM_TPM=${mode} (use required, optional, or off)"
      ;;
  esac
}

vmimage_secure_boot_mode() {
  local mode
  mode="$(vmimage_env_or_resolver "DOCKPIPE_VM_SECURE_BOOT" "DOCKPIPE_RESOLVER_VM_SECURE_BOOT")"
  case "$mode" in
    required|optional|off)
      printf '%s\n' "$mode"
      ;;
    "")
      printf 'off\n'
      ;;
    *)
      vmimage_die "unsupported DOCKPIPE_VM_SECURE_BOOT=${mode} (use required, optional, or off)"
      ;;
  esac
}

vmimage_prompt_install_host_deps() {
  local missing_desc="$1"
  local answer
  answer="$(
    vmimage_prompt_confirm \
      "vmimage.install-host-deps" \
      "Install VM Host Dependencies?" \
      "DockPipe needs additional host tools before it can run this VM: ${missing_desc}. Allow DockPipe to help launch the install command for your system?" \
      no \
      host-mutation \
      vm-host-deps \
      yes
  )" || vmimage_die "prompt failed for host dependency install"
  [[ "$answer" == "yes" ]]
}

vmimage_confirm_user_supplied_media_rights() {
  [[ -n "${DOCKPIPE_VM_CDROM:-}${DOCKPIPE_VM_VIRTIO_ISO:-}" ]] || return 0
  local answer
  answer="$(
    vmimage_prompt_confirm \
      "vmimage.media-rights" \
      "Use User-Supplied VM Media?" \
      "DockPipe does not ship or download Windows installers, licenses, or vendor driver media. Continue only if you supplied these image files yourself and have the rights to use them." \
      no \
      credential-use \
      vm-media \
      yes
  )" || vmimage_die "prompt failed for VM media rights"
  [[ "$answer" == "yes" ]] || vmimage_die "stopped before using user-supplied VM media"
}

vmimage_confirm_persistent_disk_use() {
  local persistence
  persistence="$(vmimage_env_or_resolver "DOCKPIPE_VM_PERSISTENCE" "DOCKPIPE_RESOLVER_VM_PERSISTENCE" "ephemeral")"
  [[ "$persistence" == "persistent" ]] || return 0
  local answer
  answer="$(
    vmimage_prompt_confirm \
      "vmimage.persistent-disk" \
      "Modify VM Disk Persistently?" \
      "This run is configured for persistent VM storage. DockPipe will boot the guest directly from the selected disk, and guest changes may modify that image permanently." \
      no \
      destructive \
      vm-persistence \
      yes
  )" || vmimage_die "prompt failed for persistent VM disk use"
  [[ "$answer" == "yes" ]] || vmimage_die "stopped before modifying persistent VM disk"
}

vmimage_confirm_host_network_exposure() {
  [[ -n "${DOCKPIPE_VM_HOSTFWD:-}" ]] || return 0
  local answer
  answer="$(
    vmimage_prompt_confirm \
      "vmimage.hostfwd" \
      "Expose VM Ports To Host?" \
      "This VM run is configured to forward additional guest ports onto the host: ${DOCKPIPE_VM_HOSTFWD}. Continue only if you intend to expose those services to the host." \
      no \
      host-mutation \
      vm-network \
      yes
  )" || vmimage_die "prompt failed for VM host port exposure"
  [[ "$answer" == "yes" ]] || vmimage_die "stopped before exposing VM ports on the host"
}

vmimage_pci_devices_csv() {
  vmimage_env_or_resolver "DOCKPIPE_VM_PCI_DEVICES" "DOCKPIPE_RESOLVER_VM_PCI_DEVICES"
}

vmimage_pci_primary_mode() {
  local mode
  mode="$(vmimage_env_or_resolver "DOCKPIPE_VM_GPU_PRIMARY" "DOCKPIPE_RESOLVER_VM_GPU_PRIMARY")"
  if vmimage_truthy "$mode"; then
    printf 'on\n'
  else
    printf 'off\n'
  fi
}

vmimage_allow_boot_vga_passthrough() {
  local allow
  allow="$(vmimage_env_or_resolver "DOCKPIPE_VM_ALLOW_BOOT_VGA" "DOCKPIPE_RESOLVER_VM_ALLOW_BOOT_VGA")"
  vmimage_truthy "$allow"
}

vmimage_normalize_pci_bdf() {
  local raw="${1,,}"
  raw="${raw// /}"
  case "$raw" in
    0000:??:??.?|[0-9a-f][0-9a-f][0-9a-f][0-9a-f]:[0-9a-f][0-9a-f]:[0-9a-f][0-9a-f].[0-7])
      printf '%s\n' "$raw"
      ;;
    ??:??.?|[0-9a-f][0-9a-f]:[0-9a-f][0-9a-f].[0-7])
      printf '0000:%s\n' "$raw"
      ;;
    *)
      vmimage_die "unsupported PCI device id ${1} (use values like 0000:01:00.0 or 01:00.0)"
      ;;
  esac
}

vmimage_confirm_gpu_passthrough() {
  local devices="$1"
  local answer
  answer="$(
    vmimage_prompt_confirm \
      "vmimage.gpu-passthrough" \
      "Attach Host PCI Devices To VM?" \
      "This VM run is configured to pass host PCI device(s) through to the guest: ${devices}. DockPipe expects them to already be isolated for VFIO, and the host may lose access to them while the VM is running." \
      no \
      destructive \
      vm-pci \
      yes
  )" || vmimage_die "prompt failed for GPU passthrough"
  [[ "$answer" == "yes" ]] || vmimage_die "stopped before attaching host PCI devices to the VM"
}

vmimage_confirm_boot_vga_passthrough() {
  local device="$1"
  local answer
  answer="$(
    vmimage_prompt_confirm \
      "vmimage.boot-vga" \
      "Pass Through Boot VGA Device?" \
      "PCI device ${device} appears to be a host boot/display adapter. Passing it through can blank or destabilize the host display unless you intentionally prepared an alternate GPU or console path." \
      no \
      destructive \
      vm-boot-vga \
      yes
  )" || vmimage_die "prompt failed for boot VGA passthrough"
  [[ "$answer" == "yes" ]] || vmimage_die "stopped before passing through host boot VGA device ${device}"
}

vmimage_prompt_prepare_pci_passthrough() {
  local devices="$1"
  local answer
  answer="$(
    vmimage_prompt_confirm \
      "vmimage.prepare-pci" \
      "Prepare Host PCI Devices For Passthrough?" \
      "DockPipe found PCI device(s) that are not yet bound to vfio-pci: ${devices}. Allow DockPipe to help rebind them for passthrough now?" \
      no \
      host-mutation \
      vm-pci-prepare \
      yes
  )" || vmimage_die "prompt failed for PCI passthrough preparation"
  [[ "$answer" == "yes" ]]
}

vmimage_pci_prepare_script() {
  local devices_csv="$1"
  local script="" raw dev path vendor device
  script+='set -euo pipefail\n'
  script+='modprobe vfio-pci\n'
  IFS=',' read -r -a prep_devices <<< "$devices_csv"
  for raw in "${prep_devices[@]}"; do
    raw="$(printf '%s' "$raw" | xargs)"
    [[ -n "$raw" ]] || continue
    dev="$(vmimage_normalize_pci_bdf "$raw")"
    path="/sys/bus/pci/devices/${dev}"
    vendor="$(cat "${path}/vendor")"
    device="$(cat "${path}/device")"
    script+="printf 'Preparing ${dev} for vfio-pci...\\n'\n"
    script+="if [ -L '${path}/driver' ]; then echo '${dev}' > '${path}/driver/unbind'; fi\n"
    script+="printf 'vfio-pci' > '${path}/driver_override'\n"
    script+="printf '%s' '${vendor} ${device}' > /sys/bus/pci/drivers/vfio-pci/new_id 2>/dev/null || true\n"
    script+="echo '${dev}' > /sys/bus/pci/drivers/vfio-pci/bind\n"
  done
  script+='printf "\\nDockPipe PCI passthrough prep complete.\\n"\n'
  printf '%b' "$script"
}

vmimage_prepare_pci_passthrough_now() {
  local devices_csv="$1"
  local script
  script="$(vmimage_pci_prepare_script "$devices_csv")"
  if [[ "$(id -u)" == "0" ]]; then
    bash -lc "$script"
    return 0
  fi
  if [[ -t 0 && -t 1 ]]; then
    sudo bash -lc "$script"
    return 0
  fi
  if vmimage_launch_install_terminal "sudo bash -lc $(vmimage_single_quote "$script")"; then
    vmimage_die "host PCI passthrough preparation terminal launched. After it completes, rerun windows-vm."
  fi
  vmimage_die "cannot prepare PCI passthrough devices without root access. Run as root or rebind the devices to vfio-pci manually."
}

vmimage_validate_pci_passthrough() {
  local devices_csv="$1"
  [[ -n "$devices_csv" ]] || return 0
  [[ -d /sys/kernel/iommu_groups ]] || vmimage_die "PCI passthrough requires IOMMU support; /sys/kernel/iommu_groups is missing on this host"
  vmimage_confirm_gpu_passthrough "$devices_csv"
  local raw dev path driver driver_name
  local need_prepare=()
  IFS=',' read -r -a raw_devices <<< "$devices_csv"
  for raw in "${raw_devices[@]}"; do
    raw="$(printf '%s' "$raw" | xargs)"
    [[ -n "$raw" ]] || continue
    dev="$(vmimage_normalize_pci_bdf "$raw")"
    path="/sys/bus/pci/devices/${dev}"
    [[ -d "$path" ]] || vmimage_die "host PCI device not found for passthrough: ${dev}"
    if [[ -L "${path}/driver" ]]; then
      driver="$(readlink -f "${path}/driver")"
      driver_name="$(basename "$driver")"
    else
      driver_name=""
    fi
    if [[ -f "${path}/boot_vga" ]] && [[ "$(cat "${path}/boot_vga")" == "1" ]] && ! vmimage_allow_boot_vga_passthrough; then
      vmimage_confirm_boot_vga_passthrough "$dev"
    fi
    if [[ "$driver_name" != "vfio-pci" ]]; then
      need_prepare+=("${dev}")
    fi
  done
  if (( ${#need_prepare[@]} > 0 )); then
    local need_prepare_csv
    need_prepare_csv="$(IFS=','; printf '%s' "${need_prepare[*]}")"
    if vmimage_prompt_prepare_pci_passthrough "$need_prepare_csv"; then
      vmimage_prepare_pci_passthrough_now "$need_prepare_csv"
    fi
  fi
  for raw in "${raw_devices[@]}"; do
    raw="$(printf '%s' "$raw" | xargs)"
    [[ -n "$raw" ]] || continue
    dev="$(vmimage_normalize_pci_bdf "$raw")"
    path="/sys/bus/pci/devices/${dev}"
    if [[ -L "${path}/driver" ]]; then
      driver="$(readlink -f "${path}/driver")"
      driver_name="$(basename "$driver")"
    else
      driver_name=""
    fi
    [[ "$driver_name" == "vfio-pci" ]] || vmimage_die "PCI device ${dev} is not bound to vfio-pci (current driver: ${driver_name:-none}). Bind the device to vfio-pci before using DockPipe GPU passthrough."
  done
}

vmimage_terminal_launcher() {
  if command -v x-terminal-emulator >/dev/null 2>&1; then
    printf 'x-terminal-emulator\n'
    return 0
  fi
  if command -v gnome-terminal >/dev/null 2>&1; then
    printf 'gnome-terminal\n'
    return 0
  fi
  if command -v konsole >/dev/null 2>&1; then
    printf 'konsole\n'
    return 0
  fi
  if command -v xterm >/dev/null 2>&1; then
    printf 'xterm\n'
    return 0
  fi
  return 1
}

vmimage_install_command_for_host() {
  if vmimage_is_windows_host; then
    if command -v winget >/dev/null 2>&1; then
      printf 'winget install --id SoftwareFreedomConservancy.QEMU --exact\n'
      return 0
    fi
    return 1
  fi
  if command -v apt-get >/dev/null 2>&1; then
    printf 'sudo apt-get update && sudo apt-get install -y qemu-system-x86 qemu-utils ovmf swtpm\n'
    return 0
  fi
  if command -v dnf >/dev/null 2>&1; then
    printf 'sudo dnf install -y qemu-system-x86 qemu-img edk2-ovmf swtpm\n'
    return 0
  fi
  if command -v pacman >/dev/null 2>&1; then
    printf 'sudo pacman -S --needed qemu-desktop edk2-ovmf swtpm\n'
    return 0
  fi
  if command -v zypper >/dev/null 2>&1; then
    printf 'sudo zypper install -y qemu-x86 qemu-img ovmf swtpm\n'
    return 0
  fi
  return 1
}

vmimage_run_install_command_for_host() {
  local install_cmd="$1"
  if vmimage_is_windows_host; then
    powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "$install_cmd"
  else
    bash -lc "$install_cmd"
  fi
}

vmimage_launch_install_terminal() {
  local install_cmd="$1"
  local term
  term="$(vmimage_terminal_launcher)" || return 1
  case "$term" in
    x-terminal-emulator)
      "$term" -e bash -lc "$install_cmd; status=\$?; echo; if [ \$status -eq 0 ]; then echo 'DockPipe host dependencies installed.'; else echo 'Install command failed with status' \$status; fi; read -r -p 'Press Enter to close...' _"
      ;;
    gnome-terminal)
      "$term" -- bash -lc "$install_cmd; status=\$?; echo; if [ \$status -eq 0 ]; then echo 'DockPipe host dependencies installed.'; else echo 'Install command failed with status' \$status; fi; read -r -p 'Press Enter to close...' _"
      ;;
    konsole)
      "$term" -e bash -lc "$install_cmd; status=\$?; echo; if [ \$status -eq 0 ]; then echo 'DockPipe host dependencies installed.'; else echo 'Install command failed with status' \$status; fi; read -r -p 'Press Enter to close...' _"
      ;;
    xterm)
      "$term" -e bash -lc "$install_cmd; status=\$?; echo; if [ \$status -eq 0 ]; then echo 'DockPipe host dependencies installed.'; else echo 'Install command failed with status' \$status; fi; read -r -p 'Press Enter to close...' _"
      ;;
  esac
}

vmimage_require_host_dependencies() {
  local -a missing=()
  local qemu_bin qemu_img_bin
  qemu_bin="$(vmimage_qemu_bin || true)"
  qemu_img_bin="$(vmimage_qemu_img_bin || true)"
  [[ -n "$qemu_bin" ]] || missing+=("$(vmimage_default_qemu_bin)")
  [[ -n "$qemu_img_bin" ]] || missing+=("$(vmimage_default_qemu_img_bin)")
  if [[ "$(vmimage_tpm_mode)" != "off" && "$(vmimage_backend)" != "qemu-windows" ]]; then
    command -v swtpm >/dev/null 2>&1 || missing+=("swtpm")
  fi
  if [[ ${#missing[@]} -eq 0 ]]; then
    return 0
  fi

  local missing_desc install_cmd
  missing_desc="$(IFS=', '; printf '%s' "${missing[*]}")"
  if ! vmimage_prompt_install_host_deps "$missing_desc"; then
    vmimage_die "missing required host tools: ${missing_desc}"
  fi
  if ! install_cmd="$(vmimage_install_command_for_host)"; then
    if vmimage_is_windows_host; then
      vmimage_die "missing required host tools: ${missing_desc}. Install QEMU for Windows, then rerun windows-vm."
    fi
    vmimage_die "missing required host tools: ${missing_desc}. Install QEMU system emulation, qemu-img, and UEFI firmware for your distro, then rerun windows-vm."
  fi

  if [[ -t 0 && -t 1 ]]; then
    vmimage_run_install_command_for_host "$install_cmd"
    if vmimage_is_windows_host; then
      vmimage_die "host dependency install finished. Open a new shell if needed, then rerun windows-vm."
    fi
    vmimage_die "host dependency install finished. Rerun windows-vm now that QEMU is installed."
  fi

  if ! vmimage_is_windows_host && vmimage_launch_install_terminal "$install_cmd"; then
    vmimage_die "host dependency install terminal launched. After it completes, rerun windows-vm."
  fi

  vmimage_die "missing required host tools: ${missing_desc}. Run: ${install_cmd}"
}

vmimage_detect_ovmf_pair() {
  local code vars code_check vars_check
  while IFS='|' read -r code vars; do
    code_check="$(vmimage_shell_path "$code")"
    vars_check="$(vmimage_shell_path "$vars")"
    [[ -n "$code_check" && -f "$code_check" ]] || continue
    [[ -n "$vars_check" && -f "$vars_check" ]] || continue
    printf '%s|%s\n' "$code" "$vars"
    return 0
  done < <(
    cat <<'EOF'
/usr/share/OVMF/OVMF_CODE_4M.ms.fd|/usr/share/OVMF/OVMF_VARS_4M.ms.fd
/usr/share/OVMF/OVMF_CODE_4M.secboot.fd|/usr/share/OVMF/OVMF_VARS_4M.ms.fd
/usr/share/OVMF/OVMF_CODE.secboot.fd|/usr/share/OVMF/OVMF_VARS.ms.fd
/usr/share/OVMF/OVMF_CODE.ms.fd|/usr/share/OVMF/OVMF_VARS.ms.fd
/usr/share/edk2/ovmf/OVMF_CODE_4M.ms.fd|/usr/share/edk2/ovmf/OVMF_VARS_4M.ms.fd
/usr/share/edk2/ovmf/OVMF_CODE.secboot.fd|/usr/share/edk2/ovmf/OVMF_VARS.ms.fd
EOF
    if vmimage_is_windows_host; then
      local root
      local -a win_roots=(
        "$(vmimage_env_value 'ProgramW6432')"
        "$(vmimage_env_value 'PROGRAMFILES')"
        "$(vmimage_env_value 'PROGRAMFILES(X86)')"
        "$(vmimage_env_value 'LOCALAPPDATA')\\Programs"
      )
      local -a win_pairs=(
        'qemu\\share\\edk2-x86_64-code.fd|qemu\\share\\edk2-i386-vars.fd'
        'qemu\\share\\edk2-x86_64-secure-code.fd|qemu\\share\\edk2-i386-vars.fd'
        'qemu\\share\\edk2\\ovmf\\OVMF_CODE_4M.ms.fd|qemu\\share\\edk2\\ovmf\\OVMF_VARS_4M.ms.fd'
        'qemu\\share\\edk2\\ovmf\\OVMF_CODE.secboot.fd|qemu\\share\\edk2\\ovmf\\OVMF_VARS.ms.fd'
      )
      local pair code_rel vars_rel
      for root in "${win_roots[@]}"; do
        [[ -n "$root" ]] || continue
        for pair in "${win_pairs[@]}"; do
          code_rel="${pair%%|*}"
          vars_rel="${pair##*|}"
          printf '%s|%s\n' \
            "${root}\\${code_rel}" \
            "${root}\\${vars_rel}"
        done
      done
    fi
  )
  return 1
}

vmimage_secure_boot_vars_copy_path() {
  local state_dir disk_name
  state_dir="$(vmimage_state_dir)"
  disk_name="$(basename "${DOCKPIPE_VM_DISK:-windows-vm}")"
  disk_name="${disk_name//[^A-Za-z0-9._-]/_}"
  printf '%s\n' "${state_dir}/ovmf-vars-${disk_name}.fd"
}

vmimage_prompt_reset_secure_boot_vars() {
  local vars_copy="$1"
  local answer
  answer="$(
    vmimage_prompt_confirm \
      "vmimage.reset-firmware-vars" \
      "Reset VM Firmware Boot State?" \
      "DockPipe found existing writable UEFI firmware vars for this install disk at ${vars_copy}. Reusing them can carry old boot entries into a fresh install. Reset them now?" \
      yes \
      destructive \
      vm-firmware-vars \
      yes
  )" || vmimage_die "prompt failed for firmware vars reset"
  [[ "$answer" == "yes" ]]
}

vmimage_ensure_secure_boot_firmware() {
  local boot_source="${1:-}"
  [[ "$(vmimage_secure_boot_mode)" != "off" ]] || return 0
  if [[ -n "${DOCKPIPE_VM_BIOS:-}" ]]; then
    vmimage_die "secure boot requires UEFI firmware images, not DOCKPIPE_VM_BIOS"
  fi
  if [[ -z "${DOCKPIPE_VM_FIRMWARE_CODE:-}" || -z "${DOCKPIPE_VM_FIRMWARE_VARS:-}" ]]; then
    local pair code vars
    pair="$(vmimage_detect_ovmf_pair || true)"
    if [[ -n "$pair" ]]; then
      code="${pair%%|*}"
      vars="${pair##*|}"
      if [[ -z "${DOCKPIPE_VM_FIRMWARE_CODE:-}" ]]; then
        export DOCKPIPE_VM_FIRMWARE_CODE="$code"
      fi
      if [[ -z "${DOCKPIPE_VM_FIRMWARE_VARS:-}" ]]; then
        export DOCKPIPE_VM_FIRMWARE_VARS="$vars"
      fi
    fi
  fi
  if [[ -z "${DOCKPIPE_VM_FIRMWARE_CODE:-}" ]]; then
    if [[ "$(vmimage_secure_boot_mode)" == "required" ]]; then
      vmimage_prompt_file_value \
        DOCKPIPE_VM_FIRMWARE_CODE \
        "Choose Secure Boot Firmware Code" \
        "Choose the OVMF/UEFI firmware code image for secure boot." \
        open-file \
        "Firmware Images (*.fd *.bin);;All Files (*)"
    fi
  fi
  if [[ -n "${DOCKPIPE_VM_FIRMWARE_VARS:-}" ]]; then
    local vars_path clone_vars_copy="false"
    vars_path="$(vmimage_resolve_path "$DOCKPIPE_VM_FIRMWARE_VARS")"
    if [[ -f "$vars_path" ]]; then
      if [[ "$vars_path" == /usr/share/* || "$vars_path" == /usr/lib/* ]]; then
        clone_vars_copy="true"
      elif vmimage_is_windows_host; then
        case "$vars_path" in
          [A-Za-z]:\\Program\ Files\\*|[A-Za-z]:/Program\ Files/*|[A-Za-z]:\\Program\ Files\ \(x86\)\\*|[A-Za-z]:/Program\ Files\ \(x86\)/*)
            clone_vars_copy="true"
            ;;
        esac
      fi
      if [[ "$clone_vars_copy" == "true" ]]; then
        local vars_copy
        vars_copy="$(vmimage_secure_boot_vars_copy_path)"
        if [[ ! -f "$vars_copy" ]]; then
          mkdir -p "$(dirname "$vars_copy")"
          cp "$vars_path" "$vars_copy"
        elif [[ "$boot_source" == "installer-iso" ]]; then
          if vmimage_prompt_reset_secure_boot_vars "$vars_copy"; then
            cp "$vars_path" "$vars_copy"
          fi
        fi
        export DOCKPIPE_VM_FIRMWARE_VARS="$vars_copy"
      fi
    fi
  fi
  if [[ -z "${DOCKPIPE_VM_FIRMWARE_VARS:-}" && "$(vmimage_secure_boot_mode)" == "required" ]]; then
    vmimage_prompt_file_value \
      DOCKPIPE_VM_FIRMWARE_VARS \
      "Choose Secure Boot Firmware Vars" \
      "Choose a writable OVMF/UEFI vars image for secure boot." \
      open-file \
      "Firmware Images (*.fd *.bin);;All Files (*)"
  fi
}

vmimage_start_swtpm() {
  [[ "$(vmimage_tpm_mode)" != "off" ]] || return 0
  if vmimage_is_windows_host; then
    vmimage_die "qemu-windows backend does not yet support TPM emulation; set DOCKPIPE_VM_TPM=off or run windows-vm from a Linux host"
  fi
  local state_dir tpm_dir sock pid
  state_dir="$(vmimage_state_dir)"
  tpm_dir="${state_dir}/tpm-${DOCKPIPE_RUN_ID:-vm}"
  sock="${tpm_dir}/swtpm.sock"
  mkdir -p "$tpm_dir"
  rm -f "$sock"
  swtpm socket --tpm2 --tpmstate "dir=${tpm_dir}" --ctrl "type=unixio,path=${sock}" --terminate --log "file=${tpm_dir}/swtpm.log" &
  pid="$!"
  export DOCKPIPE_VM_SWTPM_PID="$pid"
  export DOCKPIPE_VM_SWTPM_SOCK="$sock"
  local waited=0
  while [[ ! -S "$sock" ]]; do
    if ! kill -0 "$pid" 2>/dev/null; then
      vmimage_die "swtpm failed to start"
    fi
    sleep 1
    waited=$((waited + 1))
    if (( waited > 10 )); then
      vmimage_die "timed out waiting for swtpm socket"
    fi
  done
}

vmimage_stop_swtpm() {
  local pid="${DOCKPIPE_VM_SWTPM_PID:-}"
  if [[ -n "$pid" ]]; then
    kill "$pid" >/dev/null 2>&1 || true
  fi
}

vmimage_boot_source() {
  local source="${DOCKPIPE_VM_BOOT_SOURCE:-}"
  case "$source" in
    image|disk-image)
      printf 'image\n'
      return 0
      ;;
    installer|installer-iso|iso)
      printf 'installer-iso\n'
      return 0
      ;;
  esac
  if [[ -n "${DOCKPIPE_VM_CDROM:-}" && -z "${DOCKPIPE_VM_SSH_USER:-}" ]]; then
    printf 'installer-iso\n'
    return 0
  fi
  if [[ -n "${DOCKPIPE_VM_DISK:-}" && -n "${DOCKPIPE_VM_SSH_USER:-}" ]]; then
    printf 'image\n'
    return 0
  fi
  local choice
  choice="$(
    vmimage_prompt_choice \
      "vmimage.boot-source" \
      "Windows VM Source" \
      "How do you want to start this Windows VM?" \
      "Install from ISO" \
      "Boot existing disk image" \
      "Install from ISO"
  )" || vmimage_die "prompt failed for VM boot source"
  case "$choice" in
    "Boot existing disk image")
      printf 'image\n'
      ;;
    "Install from ISO")
      printf 'installer-iso\n'
      ;;
    *)
      vmimage_die "unsupported VM boot source choice: ${choice}"
      ;;
  esac
}

vmimage_ensure_prompted_image_inputs() {
  if [[ -z "${DOCKPIPE_VM_DISK:-}" ]]; then
    vmimage_prompt_file_value \
      DOCKPIPE_VM_DISK \
      "Choose VM Disk Image" \
      "Select a bootable guest disk image for this VM run." \
      open-file \
      "VM Images (*.qcow2 *.img *.raw *.vhdx *.vmdk);;All Files (*)"
  fi
  if [[ -z "${DOCKPIPE_VM_INTERACTIVE:-}" && -z "${DOCKPIPE_VM_SSH_USER:-}" ]]; then
    local image_mode
    image_mode="$(
      vmimage_prompt_choice \
        "vmimage.image-mode" \
        "Windows VM Image Mode" \
        "How do you want to use this existing Windows disk image?" \
        "Open interactive desktop" \
        "Open interactive desktop" \
        "Connect over SSH"
    )" || vmimage_die "prompt failed for VM image mode"
    case "$image_mode" in
      "Open interactive desktop")
        export DOCKPIPE_VM_INTERACTIVE=1
        export DOCKPIPE_VM_PERSISTENCE=persistent
        ;;
      "Connect over SSH")
        unset DOCKPIPE_VM_INTERACTIVE || true
        ;;
      *)
        vmimage_die "unsupported VM image mode choice: ${image_mode}"
        ;;
    esac
  fi
  if [[ -z "${DOCKPIPE_VM_INTERACTIVE:-}" && -z "${DOCKPIPE_VM_SSH_USER:-}" ]]; then
    vmimage_prompt_required_input \
      DOCKPIPE_VM_SSH_USER \
      "Guest SSH User" \
      "Enter the guest SSH username DockPipe should use after the VM boots."
  fi
}

vmimage_ensure_prompted_installer_inputs() {
  if [[ -z "${DOCKPIPE_VM_CDROM:-}" ]]; then
    vmimage_prompt_file_value \
      DOCKPIPE_VM_CDROM \
      "Choose Windows Installer ISO" \
      "Choose the Windows installer ISO you want DockPipe to boot." \
      open-file \
      "Disk Images (*.iso);;All Files (*)"
  fi
  if [[ -z "${DOCKPIPE_VM_DISK:-}" ]]; then
    vmimage_prompt_file_value \
      DOCKPIPE_VM_DISK \
      "Choose Install Disk Destination" \
      "Choose where DockPipe should create or reuse the VM disk image for this Windows install." \
      save-file \
      "VM Images (*.qcow2 *.img *.raw);;All Files (*)" \
      false
  fi
  if [[ -z "${DOCKPIPE_VM_DISK_SIZE:-}" ]]; then
    vmimage_prompt_required_input \
      DOCKPIPE_VM_DISK_SIZE \
      "VM Disk Size" \
      "Enter the size for a new VM disk image if DockPipe needs to create one." \
      "64G"
  fi
  if [[ -z "${DOCKPIPE_VM_VIRTIO_ISO:-}" ]]; then
    vmimage_prompt_optional_file_value \
      DOCKPIPE_VM_VIRTIO_ISO \
      "Optional VirtIO Driver ISO" \
      "Optionally choose a VirtIO driver ISO for Windows setup, or leave it blank to continue without one." \
      open-file \
      "Disk Images (*.iso);;All Files (*)"
  fi
}

vmimage_ensure_prompted_inputs() {
  local source
  source="$(vmimage_boot_source)"
  export DOCKPIPE_VM_BOOT_SOURCE="$source"
  case "$source" in
    image)
      vmimage_ensure_prompted_image_inputs
      ;;
    installer-iso)
      vmimage_ensure_prompted_installer_inputs
      ;;
    *)
      vmimage_die "unsupported prompted input mode"
      ;;
  esac
}

vmimage_prompt_replace_missing_file() {
  local name="$1" title="$2" message="$3" file_filter="${4:-All Files (*)}" path_mode="${5:-open-file}"
  local current="${!name:-}"
  [[ -n "$current" ]] || return 0
  local resolved
  resolved="$(vmimage_resolve_path "$current")"
  if [[ "$path_mode" == "open-dir" ]]; then
    [[ -d "$resolved" ]] && return 0
  else
    [[ -e "$resolved" ]] && return 0
  fi
  vmimage_prompt_file_value "$name" "$title" "$message" "$path_mode" "$file_filter" true
}

vmimage_truthy() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
  esac
  return 1
}

vmimage_single_quote() {
  local s="${1-}"
  printf "'%s'" "${s//\'/\'\\\'\'}"
}

vmimage_resolve_path() {
  local p="${1:-}"
  [[ -n "$p" ]] || return 0
  case "$p" in
    /*|[A-Za-z]:\\*|[A-Za-z]:/*|\\\\*)
      printf '%s\n' "$p"
      return 0
      ;;
  esac
  printf '%s\n' "${DOCKPIPE_WORKDIR%/}/$p"
}

vmimage_state_dir() {
  local base="${DOCKPIPE_PACKAGE_STATE_DIR:-${DOCKPIPE_STATE_DIR:-${DOCKPIPE_WORKDIR%/}/bin/.dockpipe/state}}"
  mkdir -p "${base}/vmimage"
  printf '%s\n' "${base}/vmimage"
}

vmimage_identity_dir() {
  local state_dir disk_name
  state_dir="$(vmimage_state_dir)"
  disk_name="$(vmimage_windows_disk_name)"
  mkdir -p "${state_dir}/identity"
  printf '%s\n' "${state_dir}/identity/${disk_name}"
}

vmimage_uuid_generate() {
  if [[ -r /proc/sys/kernel/random/uuid ]]; then
    cat /proc/sys/kernel/random/uuid
    return 0
  fi
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr 'A-Z' 'a-z'
    return 0
  fi
  local hex
  hex="$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
  printf '%s-%s-%s-%s-%s\n' "${hex:0:8}" "${hex:8:4}" "${hex:12:4}" "${hex:16:4}" "${hex:20:12}"
}

vmimage_mac_generate() {
  local hex
  hex="$(od -An -N3 -tx1 /dev/urandom | tr -d ' \n')"
  printf '52:54:00:%s:%s:%s\n' "${hex:0:2}" "${hex:2:2}" "${hex:4:2}"
}

vmimage_serial_generate() {
  local hex
  hex="$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')"
  printf 'dockpipe%s\n' "$hex"
}

vmimage_identity_value() {
  local explicit_var="$1" resolver_var="$2" file_suffix="$3" generator="$4"
  local explicit
  explicit="$(vmimage_env_or_resolver "$explicit_var" "$resolver_var")"
  if [[ -n "$explicit" ]]; then
    printf '%s\n' "$explicit"
    return 0
  fi
  local base path
  base="$(vmimage_identity_dir)"
  path="${base}.${file_suffix}"
  if [[ -f "$path" ]]; then
    cat "$path"
    return 0
  fi
  mkdir -p "$(dirname "$path")"
  "$generator" > "$path"
  cat "$path"
}

vmimage_machine_uuid() {
  vmimage_identity_value "DOCKPIPE_VM_MACHINE_UUID" "DOCKPIPE_RESOLVER_VM_MACHINE_UUID" "uuid" vmimage_uuid_generate
}

vmimage_net_mac() {
  vmimage_identity_value "DOCKPIPE_VM_NET_MAC" "DOCKPIPE_RESOLVER_VM_NET_MAC" "mac" vmimage_mac_generate
}

vmimage_disk_serial() {
  vmimage_identity_value "DOCKPIPE_VM_DISK_SERIAL" "DOCKPIPE_RESOLVER_VM_DISK_SERIAL" "serial" vmimage_serial_generate
}

vmimage_port_default() {
  local runid="${DOCKPIPE_RUN_ID:-00000000}"
  local num=$((16#${runid:0:4}))
  printf '%d\n' "$((2200 + (num % 2000)))"
}

vmimage_base_format() {
  local disk="$1"
  local hint="${DOCKPIPE_VM_DISK_FORMAT:-}"
  if [[ -n "$hint" ]]; then
    printf '%s\n' "$hint"
    return 0
  fi
  case "${disk##*.}" in
    qcow2|QCOW2) printf 'qcow2\n' ;;
    raw|img|IMG|RAW) printf 'raw\n' ;;
    *) printf 'qcow2\n' ;;
  esac
}

vmimage_disk_bus() {
  local bus
  bus="$(vmimage_env_or_resolver "DOCKPIPE_VM_DISK_BUS" "DOCKPIPE_RESOLVER_VM_DISK_BUS")"
  case "$bus" in
    ""|auto)
      printf 'virtio\n'
      ;;
    virtio|sata|ide|nvme)
      printf '%s\n' "$bus"
      ;;
    *)
      vmimage_die "unsupported DOCKPIPE_VM_DISK_BUS=${bus} (use auto, virtio, sata, ide, or nvme)"
      ;;
  esac
}

vmimage_net_device() {
  local dev
  dev="$(vmimage_env_or_resolver "DOCKPIPE_VM_NET_DEVICE" "DOCKPIPE_RESOLVER_VM_NET_DEVICE")"
  case "$dev" in
    ""|auto)
      printf 'virtio-net-pci\n'
      ;;
    virtio|virtio-net-pci)
      printf 'virtio-net-pci\n'
      ;;
    e1000e|e1000|rtl8139)
      printf '%s\n' "$dev"
      ;;
    *)
      vmimage_die "unsupported DOCKPIPE_VM_NET_DEVICE=${dev} (use auto, virtio, e1000e, e1000, or rtl8139)"
      ;;
  esac
}

vmimage_windows_install_mode() {
  local mode="${DOCKPIPE_VM_WINDOWS_INSTALL_MODE:-}"
  case "$mode" in
    ""|manual)
      printf 'manual\n'
      ;;
    unattended|automated|auto)
      vmimage_log "ignoring deprecated DOCKPIPE_VM_WINDOWS_INSTALL_MODE=${mode}; windows-vm now uses manual install flow"
      printf 'manual\n'
      ;;
    *)
      vmimage_log "ignoring unsupported DOCKPIPE_VM_WINDOWS_INSTALL_MODE=${mode}; windows-vm now uses manual install flow"
      printf 'manual\n'
      ;;
  esac
}

vmimage_windows_should_unattend() {
  local boot_source="${1:-}"
  [[ "$boot_source" == "installer-iso" ]] || return 1
  [[ "$(vmimage_windows_install_mode)" == "unattended" ]]
}

vmimage_windows_disk_name() {
  local disk_name
  disk_name="$(basename "${DOCKPIPE_VM_DISK:-windows-vm}")"
  disk_name="${disk_name//[^A-Za-z0-9._-]/_}"
  printf '%s\n' "$disk_name"
}

vmimage_windows_escape_xml() {
  local value="${1:-}"
  value="${value//&/&amp;}"
  value="${value//</&lt;}"
  value="${value//>/&gt;}"
  value="${value//\"/&quot;}"
  value="${value//\'/&apos;}"
  printf '%s' "$value"
}

vmimage_windows_base64() {
  if base64 --help 2>/dev/null | grep -q -- "-w"; then
    base64 -w0
  else
    base64 | tr -d '\n'
  fi
}

vmimage_windows_random_password() {
  local hex
  hex="$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')"
  printf 'Dockpipe!9%sAa\n' "$hex"
}

vmimage_windows_admin_password_path() {
  local state_dir disk_name
  state_dir="$(vmimage_state_dir)"
  disk_name="$(vmimage_windows_disk_name)"
  printf '%s\n' "${state_dir}/windows-admin-password-${disk_name}.txt"
}

vmimage_windows_ssh_key_base() {
  local state_dir disk_name
  state_dir="$(vmimage_state_dir)"
  disk_name="$(vmimage_windows_disk_name)"
  printf '%s\n' "${state_dir}/windows-ssh-${disk_name}"
}

vmimage_windows_unattend_dir() {
  local state_dir disk_name
  state_dir="$(vmimage_state_dir)"
  disk_name="$(vmimage_windows_disk_name)"
  printf '%s\n' "${state_dir}/windows-unattend-${disk_name}"
}

vmimage_windows_align_identity() {
  if [[ -n "${DOCKPIPE_VM_WINDOWS_ADMIN_USER:-}" && -z "${DOCKPIPE_VM_SSH_USER:-}" ]]; then
    export DOCKPIPE_VM_SSH_USER="${DOCKPIPE_VM_WINDOWS_ADMIN_USER}"
  fi
  if [[ -n "${DOCKPIPE_VM_SSH_USER:-}" && -z "${DOCKPIPE_VM_WINDOWS_ADMIN_USER:-}" ]]; then
    export DOCKPIPE_VM_WINDOWS_ADMIN_USER="${DOCKPIPE_VM_SSH_USER}"
  fi
  if [[ -z "${DOCKPIPE_VM_WINDOWS_ADMIN_USER:-}" ]]; then
    export DOCKPIPE_VM_WINDOWS_ADMIN_USER="dockpipe"
  fi
  if [[ -z "${DOCKPIPE_VM_SSH_USER:-}" ]]; then
    export DOCKPIPE_VM_SSH_USER="${DOCKPIPE_VM_WINDOWS_ADMIN_USER}"
  fi
  [[ "${DOCKPIPE_VM_WINDOWS_ADMIN_USER}" == "${DOCKPIPE_VM_SSH_USER}" ]] || vmimage_die "DOCKPIPE_VM_WINDOWS_ADMIN_USER and DOCKPIPE_VM_SSH_USER must match for unattended windows installs"
  export DOCKPIPE_VM_WINDOWS_COMPUTER_NAME="${DOCKPIPE_VM_WINDOWS_COMPUTER_NAME:-dockpipe-vm}"
  export DOCKPIPE_VM_WINDOWS_FULL_NAME="${DOCKPIPE_VM_WINDOWS_FULL_NAME:-DockPipe}"
  export DOCKPIPE_VM_WINDOWS_ORG="${DOCKPIPE_VM_WINDOWS_ORG:-DockPipe}"
  export DOCKPIPE_VM_WINDOWS_LOCALE="${DOCKPIPE_VM_WINDOWS_LOCALE:-en-US}"
  export DOCKPIPE_VM_WINDOWS_KEYBOARD="${DOCKPIPE_VM_WINDOWS_KEYBOARD:-${DOCKPIPE_VM_WINDOWS_LOCALE}}"
  export DOCKPIPE_VM_WINDOWS_TIMEZONE="${DOCKPIPE_VM_WINDOWS_TIMEZONE:-UTC}"
}

vmimage_windows_ensure_admin_password() {
  [[ -n "${DOCKPIPE_VM_WINDOWS_ADMIN_PASSWORD:-}" ]] && return 0
  local pass_path
  pass_path="$(vmimage_windows_admin_password_path)"
  if [[ -f "$pass_path" ]]; then
    export DOCKPIPE_VM_WINDOWS_ADMIN_PASSWORD="$(cat "$pass_path")"
    return 0
  fi
  export DOCKPIPE_VM_WINDOWS_ADMIN_PASSWORD="$(vmimage_windows_random_password)"
  mkdir -p "$(dirname "$pass_path")"
  umask 077
  printf '%s\n' "${DOCKPIPE_VM_WINDOWS_ADMIN_PASSWORD}" > "$pass_path"
  vmimage_log "generated Windows admin password stored at ${pass_path}"
}

vmimage_windows_ensure_ssh_key() {
  command -v ssh-keygen >/dev/null 2>&1 || vmimage_die "ssh-keygen is required for unattended windows installs"
  local key_base
  key_base="$(vmimage_windows_ssh_key_base)"
  if [[ ! -f "$key_base" ]]; then
    mkdir -p "$(dirname "$key_base")"
    ssh-keygen -q -t ed25519 -N "" -f "$key_base" >/dev/null
  fi
  export DOCKPIPE_VM_WINDOWS_SSH_KEY="$key_base"
  export DOCKPIPE_VM_WINDOWS_SSH_PUBKEY="$(cat "${key_base}.pub")"
}

vmimage_windows_bootstrap_encoded() {
  local user pubkey script escaped_pubkey
  user="${DOCKPIPE_VM_WINDOWS_ADMIN_USER}"
  pubkey="${DOCKPIPE_VM_WINDOWS_SSH_PUBKEY}"
  escaped_pubkey="${pubkey//\'/\'\'}"
  script="$(cat <<EOF
\$ErrorActionPreference = 'Stop'
\$pubKey = '${escaped_pubkey}'
try {
  Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0 | Out-Null
} catch {
  Write-Host '[dockpipe] OpenSSH capability install returned:' \$_.Exception.Message
}
Start-Sleep -Seconds 5
Set-Service -Name sshd -StartupType Automatic
if ((Get-Service sshd).Status -ne 'Running') { Start-Service sshd }
\$sshDir = Join-Path \$env:ProgramData 'ssh'
\$adminKeys = Join-Path \$sshDir 'administrators_authorized_keys'
New-Item -ItemType Directory -Force -Path \$sshDir | Out-Null
Set-Content -Path \$adminKeys -Value \$pubKey -Encoding ascii
icacls \$adminKeys /inheritance:r | Out-Null
icacls \$adminKeys /grant 'Administrators:F' 'SYSTEM:F' | Out-Null
if (-not (Get-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction SilentlyContinue)) {
  New-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -DisplayName 'OpenSSH Server (sshd)' -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22 | Out-Null
}
New-Item -ItemType File -Force -Path 'C:\dockpipe-firstboot.ok' | Out-Null
EOF
)"
  printf '%s' "$script" | iconv -f UTF-8 -t UTF-16LE | vmimage_windows_base64
}

vmimage_windows_installfrom_block() {
  if [[ -n "${DOCKPIPE_VM_WINDOWS_IMAGE_INDEX:-}" ]]; then
    cat <<EOF
            <InstallFrom>
              <MetaData wcm:action="add">
                <Key>/IMAGE/INDEX</Key>
                <Value>$(vmimage_windows_escape_xml "${DOCKPIPE_VM_WINDOWS_IMAGE_INDEX}")</Value>
              </MetaData>
            </InstallFrom>
EOF
  fi
}

vmimage_windows_write_unattend() {
  local unattend_dir xml_path locale keyboard tz comp user pass full_name org bootstrap image_block
  unattend_dir="$(vmimage_windows_unattend_dir)"
  mkdir -p "$unattend_dir"
  xml_path="${unattend_dir}/Autounattend.xml"
  locale="$(vmimage_windows_escape_xml "${DOCKPIPE_VM_WINDOWS_LOCALE}")"
  keyboard="$(vmimage_windows_escape_xml "${DOCKPIPE_VM_WINDOWS_KEYBOARD}")"
  tz="$(vmimage_windows_escape_xml "${DOCKPIPE_VM_WINDOWS_TIMEZONE}")"
  comp="$(vmimage_windows_escape_xml "${DOCKPIPE_VM_WINDOWS_COMPUTER_NAME}")"
  user="$(vmimage_windows_escape_xml "${DOCKPIPE_VM_WINDOWS_ADMIN_USER}")"
  pass="$(vmimage_windows_escape_xml "${DOCKPIPE_VM_WINDOWS_ADMIN_PASSWORD}")"
  full_name="$(vmimage_windows_escape_xml "${DOCKPIPE_VM_WINDOWS_FULL_NAME}")"
  org="$(vmimage_windows_escape_xml "${DOCKPIPE_VM_WINDOWS_ORG}")"
  bootstrap="$(vmimage_windows_bootstrap_encoded)"
  image_block="$(vmimage_windows_installfrom_block)"
  cat > "$xml_path" <<EOF
<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend">
  <settings pass="windowsPE">
    <component name="Microsoft-Windows-International-Core-WinPE" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <SetupUILanguage>
        <UILanguage>${locale}</UILanguage>
      </SetupUILanguage>
      <InputLocale>${keyboard}</InputLocale>
      <SystemLocale>${locale}</SystemLocale>
      <UILanguage>${locale}</UILanguage>
      <UserLocale>${locale}</UserLocale>
    </component>
    <component name="Microsoft-Windows-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <DiskConfiguration>
        <Disk wcm:action="add">
          <DiskID>0</DiskID>
          <WillWipeDisk>true</WillWipeDisk>
          <CreatePartitions>
            <CreatePartition wcm:action="add">
              <Order>1</Order>
              <Type>EFI</Type>
              <Size>100</Size>
            </CreatePartition>
            <CreatePartition wcm:action="add">
              <Order>2</Order>
              <Type>MSR</Type>
              <Size>16</Size>
            </CreatePartition>
            <CreatePartition wcm:action="add">
              <Order>3</Order>
              <Type>Primary</Type>
              <Extend>true</Extend>
            </CreatePartition>
          </CreatePartitions>
          <ModifyPartitions>
            <ModifyPartition wcm:action="add">
              <Order>1</Order>
              <PartitionID>1</PartitionID>
              <Format>FAT32</Format>
              <Label>System</Label>
            </ModifyPartition>
            <ModifyPartition wcm:action="add">
              <Order>2</Order>
              <PartitionID>3</PartitionID>
              <Format>NTFS</Format>
              <Label>Windows</Label>
              <Letter>C</Letter>
            </ModifyPartition>
          </ModifyPartitions>
        </Disk>
        <WillShowUI>OnError</WillShowUI>
      </DiskConfiguration>
      <ImageInstall>
        <OSImage>
${image_block}
          <InstallTo>
            <DiskID>0</DiskID>
            <PartitionID>3</PartitionID>
          </InstallTo>
          <WillShowUI>OnError</WillShowUI>
        </OSImage>
      </ImageInstall>
      <UserData>
        <AcceptEula>true</AcceptEula>
        <FullName>${full_name}</FullName>
        <Organization>${org}</Organization>
      </UserData>
    </component>
  </settings>
  <settings pass="specialize">
    <component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <ComputerName>${comp}</ComputerName>
      <TimeZone>${tz}</TimeZone>
    </component>
  </settings>
  <settings pass="oobeSystem">
    <component name="Microsoft-Windows-International-Core" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <InputLocale>${keyboard}</InputLocale>
      <SystemLocale>${locale}</SystemLocale>
      <UILanguage>${locale}</UILanguage>
      <UserLocale>${locale}</UserLocale>
    </component>
    <component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64" publicKeyToken="31bf3856ad364e35" language="neutral" versionScope="nonSxS" xmlns:wcm="http://schemas.microsoft.com/WMIConfig/2002/State" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
      <AutoLogon>
        <Enabled>true</Enabled>
        <LogonCount>1</LogonCount>
        <Username>${user}</Username>
        <Password>
          <Value>${pass}</Value>
          <PlainText>true</PlainText>
        </Password>
      </AutoLogon>
      <UserAccounts>
        <LocalAccounts>
          <LocalAccount wcm:action="add">
            <Name>${user}</Name>
            <DisplayName>${user}</DisplayName>
            <Group>Administrators</Group>
            <Password>
              <Value>${pass}</Value>
              <PlainText>true</PlainText>
            </Password>
          </LocalAccount>
        </LocalAccounts>
      </UserAccounts>
      <OOBE>
        <HideEULAPage>true</HideEULAPage>
        <HideOEMRegistrationScreen>true</HideOEMRegistrationScreen>
        <HideOnlineAccountScreens>true</HideOnlineAccountScreens>
        <HideWirelessSetupInOOBE>true</HideWirelessSetupInOOBE>
        <ProtectYourPC>3</ProtectYourPC>
      </OOBE>
      <FirstLogonCommands>
        <SynchronousCommand wcm:action="add">
          <Order>1</Order>
          <Description>DockPipe first boot bootstrap</Description>
          <CommandLine>powershell.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand ${bootstrap}</CommandLine>
        </SynchronousCommand>
      </FirstLogonCommands>
    </component>
  </settings>
</unattend>
EOF
  export DOCKPIPE_VM_WINDOWS_UNATTEND_DIR="$unattend_dir"
  export DOCKPIPE_VM_WINDOWS_UNATTEND_XML="$xml_path"
}

vmimage_windows_prepare_unattended_install() {
  vmimage_windows_align_identity
  vmimage_windows_ensure_admin_password
  vmimage_windows_ensure_ssh_key
  vmimage_windows_write_unattend
}

vmimage_prepare_disk() {
  local disk="$1"
  local persistence
  persistence="$(vmimage_env_or_resolver "DOCKPIPE_VM_PERSISTENCE" "DOCKPIPE_RESOLVER_VM_PERSISTENCE" "ephemeral")"
  local fmt
  fmt="$(vmimage_base_format "$disk")"
  if [[ "$persistence" == "persistent" ]]; then
    printf '%s|%s\n' "$disk" "$fmt"
    return 0
  fi
  local qemu_img_bin
  qemu_img_bin="$(vmimage_qemu_img_bin || true)"
  [[ -n "$qemu_img_bin" ]] || vmimage_die "$(vmimage_default_qemu_img_bin) is required for ephemeral vmimage disks"
  local state_dir overlay
  state_dir="$(vmimage_state_dir)"
  overlay="${state_dir}/overlay-${DOCKPIPE_RUN_ID:-vm}.qcow2"
  rm -f "$overlay"
  "$qemu_img_bin" create -q -f qcow2 -F "$fmt" -b "$disk" "$overlay"
  printf '%s|qcow2\n' "$overlay"
}

vmimage_ensure_disk_exists_for_install() {
  local disk="$1"
  [[ -e "$disk" ]] && return 0
  local qemu_img_bin
  qemu_img_bin="$(vmimage_qemu_img_bin || true)"
  [[ -n "$qemu_img_bin" ]] || vmimage_die "$(vmimage_default_qemu_img_bin) is required to create a VM disk image"
  local answer
  answer="$(
    vmimage_prompt_confirm \
      "vmimage.create-disk" \
      "Create VM Disk Image?" \
      "DockPipe will create a new VM disk image at ${disk} with size ${DOCKPIPE_VM_DISK_SIZE:-64G} for the Windows install." \
      yes \
      host-mutation \
      vm-disk-create \
      yes
  )" || vmimage_die "prompt failed for VM disk creation"
  [[ "$answer" == "yes" ]] || vmimage_die "stopped before creating VM disk image"
  mkdir -p "$(dirname "$disk")"
  local fmt="${DOCKPIPE_VM_DISK_CREATE_FORMAT:-qcow2}"
  "$qemu_img_bin" create -f "$fmt" "$disk" "${DOCKPIPE_VM_DISK_SIZE:-64G}" >/dev/null
}

vmimage_write_pid_sidecar() {
  local pid="$1"
  local run_file="${DOCKPIPE_RUN_FILE:-}"
  [[ -n "$run_file" ]] || return 0
  printf '%s\n' "$pid" > "${run_file%.json}.pid"
}

vmimage_collect_env_exports_bash() {
  local allow_csv="${DOCKPIPE_VM_ENV_ALLOW:-}"
  local out="" name value
  IFS=',' read -r -a names <<< "$allow_csv"
  for name in "${names[@]}"; do
    name="$(printf '%s' "$name" | xargs)"
    [[ -n "$name" ]] || continue
    value="${!name-}"
    out+="export ${name}=$(vmimage_single_quote "$value"); "
  done
  printf '%s' "$out"
}

vmimage_collect_env_exports_ps() {
  local allow_csv="${DOCKPIPE_VM_ENV_ALLOW:-}"
  local out="" name value escaped
  IFS=',' read -r -a names <<< "$allow_csv"
  for name in "${names[@]}"; do
    name="$(printf '%s' "$name" | xargs)"
    [[ -n "$name" ]] || continue
    value="${!name-}"
    escaped="${value//\'/\'\'}"
    out+="\$env:${name}='${escaped}'; "
  done
  printf '%s' "$out"
}

vmimage_ssh_base() {
  local port="${DOCKPIPE_VM_SSH_PORT:-}"
  [[ -n "$port" ]] || port="$(vmimage_port_default)"
  printf '%s\n' "$port"
}

vmimage_ssh_opts() {
  local port key_opt=""
  port="$(vmimage_ssh_base)"
  if [[ -n "${DOCKPIPE_VM_WINDOWS_SSH_KEY:-}" ]]; then
    key_opt="-i ${DOCKPIPE_VM_WINDOWS_SSH_KEY} "
  fi
  printf -- "%s-o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5 -p %s" "$key_opt" "$port"
}

vmimage_ready_probe_cmd() {
  case "$(vmimage_env_or_resolver "DOCKPIPE_VM_EXEC_MODE" "DOCKPIPE_RESOLVER_VM_EXEC_MODE" "raw")" in
    powershell)
      printf '%s\n' "Write-Output 'ready'"
      ;;
    bash)
      printf '%s\n' "printf ready"
      ;;
    raw)
      printf '%s\n' "echo ready"
      ;;
    *)
      printf '%s\n' "echo ready"
      ;;
  esac
}

vmimage_wait_for_guest() {
  local user="${DOCKPIPE_VM_SSH_USER:-}"
  local host="${DOCKPIPE_VM_SSH_HOST:-127.0.0.1}"
  local timeout="${DOCKPIPE_VM_SSH_READY_TIMEOUT:-300}"
  local start now
  start="$(date +%s)"
  while true; do
    if vmimage_remote_run_internal "$(vmimage_ready_probe_cmd)" >/dev/null 2>&1; then
      return 0
    fi
    now="$(date +%s)"
    if (( now - start >= timeout )); then
      vmimage_die "timed out waiting for guest SSH readiness at ${user}@${host}:$(vmimage_ssh_base)"
    fi
    sleep 3
  done
}

vmimage_remote_run_internal() {
  local mode
  mode="$(vmimage_env_or_resolver "DOCKPIPE_VM_EXEC_MODE" "DOCKPIPE_RESOLVER_VM_EXEC_MODE" "raw")"
  local user="${DOCKPIPE_VM_SSH_USER:-}"
  local host="${DOCKPIPE_VM_SSH_HOST:-127.0.0.1}"
  local cmd="$1"
  local ssh_opts
  ssh_opts="$(vmimage_ssh_opts)"
  case "$mode" in
    bash)
      local script
      script="$(vmimage_collect_env_exports_bash)${cmd}"
      # shellcheck disable=SC2086
      ssh $ssh_opts "${user}@${host}" "bash -lc $(vmimage_single_quote "$script")"
      ;;
    powershell)
      local script encoded
      script="$(vmimage_collect_env_exports_ps)${cmd}"
      encoded="$(printf '%s' "$script" | iconv -f UTF-8 -t UTF-16LE | vmimage_windows_base64)"
      # shellcheck disable=SC2086
      ssh $ssh_opts "${user}@${host}" "powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand ${encoded}"
      ;;
    raw)
      # shellcheck disable=SC2086
      ssh $ssh_opts "${user}@${host}" "${cmd}"
      ;;
    *)
      vmimage_die "unsupported DOCKPIPE_VM_EXEC_MODE=${mode} (use raw, bash, or powershell)"
      ;;
  esac
}

vmimage_fetch_outputs() {
  local remote_path="${DOCKPIPE_VM_OUTPUTS_REMOTE_PATH:-}"
  local local_path="${DOCKPIPE_STEP_OUTPUTS_FILE:-}"
  [[ -n "$remote_path" && -n "$local_path" ]] || return 0
  mkdir -p "$(dirname "$local_path")"
  case "$(vmimage_env_or_resolver "DOCKPIPE_VM_EXEC_MODE" "DOCKPIPE_RESOLVER_VM_EXEC_MODE" "raw")" in
    powershell)
      vmimage_remote_run_internal "if (Test-Path -LiteralPath '$remote_path') { Get-Content -Raw -LiteralPath '$remote_path' }" > "$local_path"
      ;;
    bash)
      vmimage_remote_run_internal "if [ -f $(vmimage_single_quote "$remote_path") ]; then cat $(vmimage_single_quote "$remote_path"); fi" > "$local_path"
      ;;
    raw)
      vmimage_remote_run_internal "cat $(vmimage_single_quote "$remote_path")" > "$local_path"
      ;;
  esac
}

vmimage_installer_display_mode() {
  local display="${DOCKPIPE_VM_DISPLAY:-}"
  if [[ -n "$display" ]]; then
    printf '%s\n' "$display"
    return 0
  fi
  if vmimage_is_windows_host; then
    printf 'default\n'
  else
    printf 'gtk,window-close=on\n'
  fi
}

vmimage_has_guest_automation() {
  [[ -z "${DOCKPIPE_VM_INTERACTIVE:-}" && -n "${DOCKPIPE_STEP_CMD:-}" && -n "${DOCKPIPE_VM_SSH_USER:-}" ]]
}

vmimage_run_installer_session() {
  local qemu_bin="$1"
  shift || true
  local -a qemu_args=("$@")
  local pid state_dir pidfile
  state_dir="$(vmimage_state_dir)"
  pidfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.pid"
  rm -f "$pidfile"
  vmimage_log "installer mode: launching interactive VM window and waiting until you close it or stop DockPipe"
  "$qemu_bin" "${qemu_args[@]}" &
  pid="$!"
  printf '%s\n' "$pid" > "$pidfile"
  vmimage_write_pid_sidecar "$pid"
  trap 'kill "$pid" >/dev/null 2>&1 || true; vmimage_stop_swtpm' EXIT INT TERM
  wait "$pid"
}

vmimage_run_automated_installer_session() {
  local qemu_bin="$1"
  shift || true
  local -a qemu_args=("$@")
  local pid state_dir pidfile old_timeout
  vmimage_require DOCKPIPE_STEP_CMD
  state_dir="$(vmimage_state_dir)"
  pidfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.pid"
  rm -f "$pidfile"
  vmimage_log "installer mode: launching unattended installer VM and waiting for guest SSH readiness"
  "$qemu_bin" "${qemu_args[@]}" &
  pid="$!"
  printf '%s\n' "$pid" > "$pidfile"
  vmimage_write_pid_sidecar "$pid"
  trap 'kill "$pid" >/dev/null 2>&1 || true; vmimage_stop_swtpm' EXIT INT TERM
  old_timeout="${DOCKPIPE_VM_SSH_READY_TIMEOUT:-}"
  if [[ -z "$old_timeout" ]]; then
    export DOCKPIPE_VM_SSH_READY_TIMEOUT=3600
  fi
  vmimage_wait_for_guest
  if [[ -z "$old_timeout" ]]; then
    unset DOCKPIPE_VM_SSH_READY_TIMEOUT || true
  else
    export DOCKPIPE_VM_SSH_READY_TIMEOUT="$old_timeout"
  fi
  vmimage_remote_run_internal "${DOCKPIPE_STEP_CMD}"
  vmimage_fetch_outputs
  wait "$pid"
}

vmimage_run_headless_guest_session() {
  local qemu_bin="$1"
  shift || true
  local -a qemu_args=("$@")
  local pid state_dir pidfile
  vmimage_require DOCKPIPE_STEP_CMD
  state_dir="$(vmimage_state_dir)"
  pidfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.pid"
  rm -f "$pidfile"
  vmimage_log "headless mode: launching VM in background and waiting for guest SSH readiness"
  "$qemu_bin" "${qemu_args[@]}" &
  pid="$!"
  printf '%s\n' "$pid" > "$pidfile"
  vmimage_write_pid_sidecar "$pid"
  trap 'kill "$pid" >/dev/null 2>&1 || true; vmimage_stop_swtpm' EXIT INT TERM
  vmimage_wait_for_guest
  vmimage_remote_run_internal "${DOCKPIPE_STEP_CMD}"
  vmimage_fetch_outputs
  wait "$pid"
}

vmimage_windows_write_args_file() {
  local path="$1"
  shift || true
  : > "$path"
  local arg
  for arg in "$@"; do
    printf '%s\n' "$arg" >> "$path"
  done
}

vmimage_windows_qemu_invoke() {
  local action="$1" qemu_bin="$2" pidfile="$3" argsfile="$4" stdout_file="${5:-}" stderr_file="${6:-}"
  local pwsh_bin helper
  pwsh_bin="$(vmimage_powershell_bin)"
  helper="$(vmimage_windows_qemu_helper)"
  "$pwsh_bin" -NoProfile -ExecutionPolicy Bypass -File "$helper" \
    -Action "$action" \
    -QemuBin "$qemu_bin" \
    -PidFile "$pidfile" \
    -ArgsFile "$argsfile" \
    -StdOutFile "$stdout_file" \
    -StdErrFile "$stderr_file"
}

vmimage_windows_stop_qemu_if_present() {
  local pidfile="${1:-}"
  [[ -n "$pidfile" && -f "$pidfile" ]] || return 0
  local pwsh_bin helper
  pwsh_bin="$(vmimage_powershell_bin)"
  helper="$(vmimage_windows_qemu_helper)"
  "$pwsh_bin" -NoProfile -ExecutionPolicy Bypass -File "$helper" \
    -Action stop \
    -PidFile "$pidfile" >/dev/null 2>&1 || true
}

vmimage_apply_windows_installer_compat_defaults() {
  local backend="$1" boot_source="$2"
  [[ "$backend" == "qemu-windows" && "$boot_source" == "installer-iso" ]] || return 0

  if [[ -z "${DOCKPIPE_VM_CPU_MODEL:-}" && -z "${DOCKPIPE_RESOLVER_VM_CPU_MODEL:-}" ]]; then
    export DOCKPIPE_VM_CPU_MODEL="qemu64"
  fi
  if [[ -z "${DOCKPIPE_VM_DISK_BUS:-}" && -z "${DOCKPIPE_RESOLVER_VM_DISK_BUS:-}" ]]; then
    export DOCKPIPE_VM_DISK_BUS="sata"
  fi
  if [[ -z "${DOCKPIPE_VM_NET_DEVICE:-}" && -z "${DOCKPIPE_RESOLVER_VM_NET_DEVICE:-}" ]]; then
    export DOCKPIPE_VM_NET_DEVICE="e1000e"
  fi
}

vmimage_apply_windows_image_compat_defaults() {
  local backend="$1" boot_source="$2"
  [[ "$backend" == "qemu-windows" && "$boot_source" == "image" ]] || return 0

  if [[ -z "${DOCKPIPE_VM_CPU_MODEL:-}" && -z "${DOCKPIPE_RESOLVER_VM_CPU_MODEL:-}" ]]; then
    export DOCKPIPE_VM_CPU_MODEL="qemu64"
  fi
  if [[ -z "${DOCKPIPE_VM_DISK_BUS:-}" && -z "${DOCKPIPE_RESOLVER_VM_DISK_BUS:-}" ]]; then
    export DOCKPIPE_VM_DISK_BUS="sata"
  fi
  if [[ -z "${DOCKPIPE_VM_NET_DEVICE:-}" && -z "${DOCKPIPE_RESOLVER_VM_NET_DEVICE:-}" ]]; then
    export DOCKPIPE_VM_NET_DEVICE="e1000e"
  fi
}

vmimage_run_qemu_common() {
  local backend="$1"
  vmimage_require_host_dependencies
  vmimage_ensure_prompted_inputs
  local boot_source
  boot_source="$(vmimage_boot_source)"
  vmimage_apply_windows_installer_compat_defaults "$backend" "$boot_source"
  vmimage_apply_windows_image_compat_defaults "$backend" "$boot_source"
  if vmimage_windows_should_unattend "$boot_source"; then
    vmimage_windows_prepare_unattended_install
  fi
  vmimage_ensure_secure_boot_firmware "$boot_source"
  vmimage_confirm_user_supplied_media_rights
  vmimage_confirm_host_network_exposure
  if [[ "$backend" == "qemu-windows" ]]; then
    [[ -z "$(vmimage_pci_devices_csv)" ]] || vmimage_die "qemu-windows backend does not support host PCI passthrough; clear DOCKPIPE_VM_PCI_DEVICES or run windows-vm from a Linux host"
  else
    vmimage_validate_pci_passthrough "$(vmimage_pci_devices_csv)"
  fi
  if [[ "$boot_source" == "image" ]]; then
    vmimage_confirm_persistent_disk_use
  else
    export DOCKPIPE_VM_PERSISTENCE="persistent"
  fi
  if [[ "$boot_source" == "image" ]]; then
    vmimage_prompt_replace_missing_file \
      DOCKPIPE_VM_DISK \
      "Choose VM Disk Image" \
      "The configured VM disk image could not be found. Choose a bootable guest disk image." \
      "VM Images (*.qcow2 *.img *.raw *.vhdx *.vmdk);;All Files (*)"
  fi
  vmimage_prompt_replace_missing_file \
    DOCKPIPE_VM_FIRMWARE_CODE \
    "Choose Firmware Code Image" \
    "The configured firmware code image could not be found. Choose the OVMF/UEFI code file." \
    "Firmware Images (*.fd *.bin);;All Files (*)"
  vmimage_prompt_replace_missing_file \
    DOCKPIPE_VM_FIRMWARE_VARS \
    "Choose Firmware Vars Image" \
    "The configured firmware vars image could not be found. Choose the writable OVMF/UEFI vars file." \
    "Firmware Images (*.fd *.bin);;All Files (*)"
  vmimage_prompt_replace_missing_file \
    DOCKPIPE_VM_BIOS \
    "Choose BIOS Image" \
    "The configured BIOS image could not be found. Choose the BIOS/firmware file." \
    "Firmware Images (*.bin *.rom *.fd);;All Files (*)"
  vmimage_prompt_replace_missing_file \
    DOCKPIPE_VM_CDROM \
    "Choose Installer ISO" \
    "The configured CD-ROM image could not be found. Choose the installer ISO or support media." \
    "Disk Images (*.iso);;All Files (*)"
  vmimage_prompt_replace_missing_file \
    DOCKPIPE_VM_VIRTIO_ISO \
    "Choose VirtIO Driver ISO" \
    "The configured VirtIO driver ISO could not be found. Choose the driver ISO." \
    "Disk Images (*.iso);;All Files (*)"

  local qemu_bin
  qemu_bin="$(vmimage_qemu_bin || true)"
  [[ -n "$qemu_bin" ]] || vmimage_die "$(vmimage_default_qemu_bin) not found"

  local disk disk_fmt prepared cpu mem ssh_port ssh_hostfwd state_dir pidfile monitor disk_bus net_device machine_uuid net_mac disk_serial
  local qemu_stdout_log qemu_stderr_log
  local pci_devices_csv pci_primary_mode
  disk="$(vmimage_resolve_path "$DOCKPIPE_VM_DISK")"
  if [[ "$boot_source" == "installer-iso" ]]; then
    vmimage_ensure_disk_exists_for_install "$disk"
    disk_fmt="$(vmimage_base_format "$disk")"
  else
    [[ -f "$disk" ]] || vmimage_die "vm disk not found: $disk"
    prepared="$(vmimage_prepare_disk "$disk")"
    disk="${prepared%%|*}"
    disk_fmt="${prepared##*|}"
  fi
  cpu="$(vmimage_env_or_resolver "DOCKPIPE_VM_CPUS" "DOCKPIPE_RESOLVER_VM_CPUS" "4")"
  mem="$(vmimage_env_or_resolver "DOCKPIPE_VM_MEMORY" "DOCKPIPE_RESOLVER_VM_MEMORY" "8G")"
  disk_bus="$(vmimage_disk_bus)"
  net_device="$(vmimage_net_device)"
  machine_uuid="$(vmimage_machine_uuid)"
  net_mac="$(vmimage_net_mac)"
  disk_serial="$(vmimage_disk_serial)"
  pci_devices_csv="$(vmimage_pci_devices_csv)"
  pci_primary_mode="$(vmimage_pci_primary_mode)"
  ssh_port="$(vmimage_ssh_base)"
  state_dir="$(vmimage_state_dir)"
  pidfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.pid"
  monitor="${state_dir}/monitor-${DOCKPIPE_RUN_ID:-vm}.sock"
  qemu_stdout_log="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.stdout.log"
  qemu_stderr_log="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.stderr.log"
  rm -f "$pidfile" "$monitor" "$qemu_stdout_log" "$qemu_stderr_log"
  : > "$qemu_stdout_log"
  : > "$qemu_stderr_log"
  ssh_hostfwd="hostfwd=tcp::${ssh_port}-:22"
  if [[ -n "${DOCKPIPE_VM_HOSTFWD:-}" ]]; then
    local item
    IFS=',' read -r -a extra_forwards <<< "${DOCKPIPE_VM_HOSTFWD}"
    for item in "${extra_forwards[@]}"; do
      item="$(printf '%s' "$item" | xargs)"
      [[ -n "$item" ]] || continue
      ssh_hostfwd+=",hostfwd=${item}"
    done
  fi

  local -a qemu_args=(
    -name "dockpipe-vm-${DOCKPIPE_RUN_ID:-vm}"
    -uuid "$machine_uuid"
    -machine "q35,accel=${DOCKPIPE_VM_ACCEL:-$(vmimage_default_accel)}$( [[ "$(vmimage_secure_boot_mode)" != "off" ]] && printf ',smm=on' )"
    -cpu "${DOCKPIPE_VM_CPU_MODEL:-$(vmimage_default_cpu_model)}"
    -smp "$cpu"
    -m "$mem"
    -netdev "user,id=net0,${ssh_hostfwd}"
    -device "${net_device},netdev=net0,mac=${net_mac}"
  )

  case "$disk_bus" in
    virtio)
      qemu_args+=(
        -drive "if=none,id=vdisk,file=${disk},format=${disk_fmt}"
        -device "virtio-blk-pci,drive=vdisk,serial=${disk_serial}"
      )
      ;;
    sata)
      qemu_args+=(
        -device ich9-ahci,id=sata0
        -drive "if=none,id=vdisk,file=${disk},format=${disk_fmt}"
        -device "ide-hd,drive=vdisk,bus=sata0.0,serial=${disk_serial}"
      )
      ;;
    ide)
      qemu_args+=(
        -drive "if=none,id=vdisk,file=${disk},format=${disk_fmt}"
        -device "ide-hd,drive=vdisk,serial=${disk_serial}"
      )
      ;;
    nvme)
      qemu_args+=(
        -drive "if=none,id=vdisk,file=${disk},format=${disk_fmt}"
        -device "nvme,drive=vdisk,serial=${disk_serial}"
      )
      ;;
  esac

  if [[ "$boot_source" == "installer-iso" ]]; then
    qemu_args+=(
      -display "$(vmimage_installer_display_mode)"
      -boot once=d,menu=on
    )
  else
    if vmimage_has_guest_automation; then
      if [[ "$backend" == "qemu-kvm" ]]; then
        qemu_args+=(
          -daemonize
          -pidfile "$pidfile"
          -display none
          -serial none
          -monitor unix:"$monitor",server,nowait
        )
      else
        qemu_args+=(
          -display none
          -serial none
        )
      fi
    else
      qemu_args+=(
        -display "$(vmimage_installer_display_mode)"
      )
    fi
  fi

  if [[ "$(vmimage_secure_boot_mode)" != "off" ]]; then
    qemu_args+=(-global driver=cfi.pflash01,property=secure,value=on)
  fi

  if [[ -n "${DOCKPIPE_VM_FIRMWARE_CODE:-}" ]]; then
    local code vars
    code="$(vmimage_resolve_path "$DOCKPIPE_VM_FIRMWARE_CODE")"
    vars="$(vmimage_resolve_path "${DOCKPIPE_VM_FIRMWARE_VARS:-}")"
    qemu_args+=(-drive "if=pflash,format=raw,readonly=on,file=${code}")
    if [[ -n "$vars" ]]; then
      qemu_args+=(-drive "if=pflash,format=raw,file=${vars}")
    fi
  elif [[ -n "${DOCKPIPE_VM_BIOS:-}" ]]; then
    qemu_args+=(-bios "$(vmimage_resolve_path "$DOCKPIPE_VM_BIOS")")
  fi

  if [[ -n "${DOCKPIPE_VM_CDROM:-}" ]]; then
    qemu_args+=(-cdrom "$(vmimage_resolve_path "$DOCKPIPE_VM_CDROM")")
  fi
  if [[ -n "${DOCKPIPE_VM_VIRTIO_ISO:-}" ]]; then
    qemu_args+=(-drive "file=$(vmimage_resolve_path "$DOCKPIPE_VM_VIRTIO_ISO"),media=cdrom")
  fi
  if [[ -n "${DOCKPIPE_VM_WINDOWS_UNATTEND_DIR:-}" ]]; then
    qemu_args+=(-drive "if=floppy,format=raw,file=fat:floppy:rw:$(vmimage_resolve_path "$DOCKPIPE_VM_WINDOWS_UNATTEND_DIR")")
  fi

  if [[ "$(vmimage_tpm_mode)" != "off" ]]; then
    vmimage_start_swtpm
    qemu_args+=(
      -chardev "socket,id=chrtpm,path=${DOCKPIPE_VM_SWTPM_SOCK}"
      -tpmdev "emulator,id=tpm0,chardev=chrtpm"
      -device "tpm-tis,tpmdev=tpm0"
    )
  fi

  if [[ -n "$pci_devices_csv" ]]; then
    local raw_pci normalized_pci first_pci=1
    IFS=',' read -r -a passthrough_devices <<< "$pci_devices_csv"
    for raw_pci in "${passthrough_devices[@]}"; do
      raw_pci="$(printf '%s' "$raw_pci" | xargs)"
      [[ -n "$raw_pci" ]] || continue
      normalized_pci="$(vmimage_normalize_pci_bdf "$raw_pci")"
      if (( first_pci )) && [[ "$pci_primary_mode" == "on" ]]; then
        qemu_args+=(-device "vfio-pci,host=${normalized_pci},x-vga=on")
      else
        qemu_args+=(-device "vfio-pci,host=${normalized_pci}")
      fi
      first_pci=0
    done
  fi

  vmimage_log "backend=${backend} qemu_bin=${qemu_bin}"
  vmimage_log "boot_source=${boot_source}"
  vmimage_log "disk_bus=${disk_bus}"
  vmimage_log "net_device=${net_device}"
  vmimage_log "machine_uuid=${machine_uuid}"
  vmimage_log "net_mac=${net_mac}"
  vmimage_log "disk_serial=${disk_serial}"
  if [[ -n "$pci_devices_csv" ]]; then
    vmimage_log "pci_devices=${pci_devices_csv} gpu_primary=${pci_primary_mode}"
  fi
  vmimage_log "tpm=$(vmimage_tpm_mode) secure_boot=$(vmimage_secure_boot_mode)"
  vmimage_log "disk=${disk} disk_format=${disk_fmt} persistence=$(vmimage_env_or_resolver "DOCKPIPE_VM_PERSISTENCE" "DOCKPIPE_RESOLVER_VM_PERSISTENCE" "ephemeral")"
  vmimage_log "ssh_port=${ssh_port} cpus=${cpu} memory=${mem} accel=${DOCKPIPE_VM_ACCEL:-$(vmimage_default_accel)} exec_mode=$(vmimage_env_or_resolver "DOCKPIPE_VM_EXEC_MODE" "DOCKPIPE_RESOLVER_VM_EXEC_MODE" "raw")"
  if [[ -n "${DOCKPIPE_VM_FIRMWARE_CODE:-}" ]]; then
    vmimage_log "firmware_code=$(vmimage_resolve_path "$DOCKPIPE_VM_FIRMWARE_CODE")"
  fi
  if [[ -n "${DOCKPIPE_VM_FIRMWARE_VARS:-}" ]]; then
    vmimage_log "firmware_vars=$(vmimage_resolve_path "$DOCKPIPE_VM_FIRMWARE_VARS")"
  fi
  if [[ -n "${DOCKPIPE_VM_BIOS:-}" ]]; then
    vmimage_log "bios=$(vmimage_resolve_path "$DOCKPIPE_VM_BIOS")"
  fi
  if [[ -n "${DOCKPIPE_VM_CDROM:-}" ]]; then
    vmimage_log "cdrom=$(vmimage_resolve_path "$DOCKPIPE_VM_CDROM")"
  fi
  if [[ -n "${DOCKPIPE_VM_VIRTIO_ISO:-}" ]]; then
    vmimage_log "virtio_iso=$(vmimage_resolve_path "$DOCKPIPE_VM_VIRTIO_ISO")"
  fi
  if [[ -n "${DOCKPIPE_VM_WINDOWS_UNATTEND_XML:-}" ]]; then
    vmimage_log "windows_unattend=$(vmimage_resolve_path "$DOCKPIPE_VM_WINDOWS_UNATTEND_XML")"
  fi
  if [[ "$backend" == "qemu-windows" ]]; then
    vmimage_log "qemu_stdout_log=${qemu_stdout_log}"
    vmimage_log "qemu_stderr_log=${qemu_stderr_log}"
  fi
  if [[ -n "${DOCKPIPE_VM_HOSTFWD:-}" ]]; then
    vmimage_log "extra_hostfwd=${DOCKPIPE_VM_HOSTFWD}"
  fi
  local rendered_qemu=""
  local qarg
  for qarg in "${qemu_args[@]}"; do
    if [[ -n "$rendered_qemu" ]]; then
      rendered_qemu+=" "
    fi
    rendered_qemu+="$(printf '%q' "$qarg")"
  done
  vmimage_log "launch: $(printf '%q' "$qemu_bin") ${rendered_qemu}"

  if [[ "$boot_source" == "installer-iso" ]]; then
    if vmimage_windows_should_unattend "$boot_source"; then
      if [[ "$backend" == "qemu-windows" ]]; then
        local argsfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.args"
        vmimage_windows_write_args_file "$argsfile" "${qemu_args[@]}"
        trap 'vmimage_windows_stop_qemu_if_present "$pidfile"; vmimage_stop_swtpm' EXIT INT TERM
        vmimage_windows_qemu_invoke start "$qemu_bin" "$pidfile" "$argsfile" "$qemu_stdout_log" "$qemu_stderr_log"
        local old_timeout="${DOCKPIPE_VM_SSH_READY_TIMEOUT:-}"
        if [[ -z "$old_timeout" ]]; then
          export DOCKPIPE_VM_SSH_READY_TIMEOUT=3600
        fi
        vmimage_wait_for_guest
        if [[ -z "$old_timeout" ]]; then
          unset DOCKPIPE_VM_SSH_READY_TIMEOUT || true
        else
          export DOCKPIPE_VM_SSH_READY_TIMEOUT="$old_timeout"
        fi
        vmimage_remote_run_internal "${DOCKPIPE_STEP_CMD}"
        vmimage_fetch_outputs
        vmimage_windows_qemu_invoke wait "$qemu_bin" "$pidfile" "$argsfile" "$qemu_stdout_log" "$qemu_stderr_log"
      else
        vmimage_run_automated_installer_session "$qemu_bin" "${qemu_args[@]}"
      fi
    else
      if [[ "$backend" == "qemu-windows" ]]; then
        local argsfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.args"
        vmimage_windows_write_args_file "$argsfile" "${qemu_args[@]}"
        vmimage_windows_qemu_invoke start-wait "$qemu_bin" "$pidfile" "$argsfile" "$qemu_stdout_log" "$qemu_stderr_log"
      else
        vmimage_run_installer_session "$qemu_bin" "${qemu_args[@]}"
      fi
    fi
  else
    if vmimage_has_guest_automation; then
      if [[ "$backend" == "qemu-kvm" ]]; then
        trap 'vmimage_stop_swtpm' EXIT INT TERM
        "$qemu_bin" "${qemu_args[@]}"
        [[ -f "$pidfile" ]] || vmimage_die "qemu-kvm did not create pidfile"
        vmimage_write_pid_sidecar "$(cat "$pidfile")"

        vmimage_wait_for_guest
        vmimage_remote_run_internal "${DOCKPIPE_STEP_CMD}"
        vmimage_fetch_outputs
      else
        local argsfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.args"
        vmimage_windows_write_args_file "$argsfile" "${qemu_args[@]}"
        vmimage_windows_qemu_invoke start "$qemu_bin" "$pidfile" "$argsfile" "$qemu_stdout_log" "$qemu_stderr_log"
        vmimage_wait_for_guest
        vmimage_remote_run_internal "${DOCKPIPE_STEP_CMD}"
        vmimage_fetch_outputs
      fi
    else
      if [[ "$backend" == "qemu-windows" ]]; then
        local argsfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.args"
        vmimage_windows_write_args_file "$argsfile" "${qemu_args[@]}"
        vmimage_windows_qemu_invoke start-wait "$qemu_bin" "$pidfile" "$argsfile" "$qemu_stdout_log" "$qemu_stderr_log"
      else
        vmimage_run_installer_session "$qemu_bin" "${qemu_args[@]}"
      fi
    fi
  fi
}

vmimage_run_qemu_kvm() {
  [[ "$(vmimage_host_os)" == "linux" ]] || vmimage_die "qemu-kvm backend currently requires a Linux host"
  vmimage_run_qemu_common qemu-kvm
}

vmimage_run_qemu_windows() {
  vmimage_is_windows_host || vmimage_die "qemu-windows backend currently requires a Windows host"
  local accel
  accel="${DOCKPIPE_VM_ACCEL:-$(vmimage_default_accel)}"
  if [[ "$(vmimage_tpm_mode)" != "off" ]]; then
    vmimage_log "windows host detected: forcing DOCKPIPE_VM_TPM=off because TPM emulation is not supported on qemu-windows"
    export DOCKPIPE_VM_TPM=off
  fi
  if [[ "$accel" == *whpx* && "$(vmimage_secure_boot_mode)" != "off" ]]; then
    vmimage_log "windows host detected: forcing DOCKPIPE_VM_SECURE_BOOT=off because WHPX does not support the SMM path required for secure boot on this backend"
    export DOCKPIPE_VM_SECURE_BOOT=off
  fi
  vmimage_run_qemu_common qemu-windows
}

backend="$(vmimage_backend)"
case "$backend" in
  qemu-kvm) vmimage_run_qemu_kvm ;;
  qemu-windows) vmimage_run_qemu_windows ;;
  *) vmimage_die "unsupported DOCKPIPE_VM_BACKEND=${backend}" ;;
esac
