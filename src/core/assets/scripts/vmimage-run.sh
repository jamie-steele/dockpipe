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

vmimage_prompt_file_value() {
  local name="$1" title="$2" message="$3" path_mode="${4:-open-file}" file_filter="${5:-All Files (*)}" must_exist="${6:-true}"
  local current="${!name:-}"
  local -a args=(
    prompt file
    --id "vmimage.${name,,}"
    --title "$title"
    --message "$message"
    --default "$current"
    --path-mode "$path_mode"
    --filter "$file_filter"
  )
  if [[ "$must_exist" == "true" ]]; then
    args+=(--must-exist)
  fi
  local response
  response="$(dockpipe_sdk "${args[@]}")" || vmimage_die "prompt failed for ${name}"
  [[ -n "$response" ]] || vmimage_die "required file value ${name} was not provided"
  printf -v "$name" '%s' "$response"
  export "$name"
}

vmimage_prompt_optional_file_value() {
  local name="$1" title="$2" message="$3" path_mode="${4:-open-file}" file_filter="${5:-All Files (*)}"
  local current="${!name:-}"
  local response
  response="$(
    dockpipe_sdk prompt file \
      --id "vmimage.${name,,}" \
      --title "$title" \
      --message "$message" \
      --default "$current" \
      --path-mode "$path_mode" \
      --filter "$file_filter"
  )" || vmimage_die "prompt failed for ${name}"
  printf -v "$name" '%s' "$response"
  export "$name"
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
  command -v qemu-system-x86_64 >/dev/null 2>&1 || missing+=("qemu-system-x86_64")
  command -v qemu-img >/dev/null 2>&1 || missing+=("qemu-img")
  if [[ "$(vmimage_tpm_mode)" != "off" ]]; then
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
    vmimage_die "missing required host tools: ${missing_desc}. Install QEMU system emulation, qemu-img, and UEFI firmware for your distro, then rerun windows-vm."
  fi

  if [[ -t 0 && -t 1 ]]; then
    bash -lc "$install_cmd"
    vmimage_die "host dependency install finished. Rerun windows-vm now that QEMU is installed."
  fi

  if vmimage_launch_install_terminal "$install_cmd"; then
    vmimage_die "host dependency install terminal launched. After it completes, rerun windows-vm."
  fi

  vmimage_die "missing required host tools: ${missing_desc}. Run: ${install_cmd}"
}

vmimage_detect_ovmf_pair() {
  local code vars
  while IFS='|' read -r code vars; do
    [[ -n "$code" && -f "$code" ]] || continue
    [[ -n "$vars" && -f "$vars" ]] || continue
    printf '%s|%s\n' "$code" "$vars"
    return 0
  done <<'EOF'
/usr/share/OVMF/OVMF_CODE_4M.ms.fd|/usr/share/OVMF/OVMF_VARS_4M.ms.fd
/usr/share/OVMF/OVMF_CODE_4M.secboot.fd|/usr/share/OVMF/OVMF_VARS_4M.ms.fd
/usr/share/OVMF/OVMF_CODE.secboot.fd|/usr/share/OVMF/OVMF_VARS.ms.fd
/usr/share/OVMF/OVMF_CODE.ms.fd|/usr/share/OVMF/OVMF_VARS.ms.fd
/usr/share/edk2/ovmf/OVMF_CODE_4M.ms.fd|/usr/share/edk2/ovmf/OVMF_VARS_4M.ms.fd
/usr/share/edk2/ovmf/OVMF_CODE.secboot.fd|/usr/share/edk2/ovmf/OVMF_VARS.ms.fd
EOF
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
    local vars_path
    vars_path="$(vmimage_resolve_path "$DOCKPIPE_VM_FIRMWARE_VARS")"
    if [[ -f "$vars_path" ]]; then
      if [[ "$vars_path" == /usr/share/* || "$vars_path" == /usr/lib/* ]]; then
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
  if [[ -z "${DOCKPIPE_VM_SSH_USER:-}" ]]; then
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
  case "$(vmimage_boot_source)" in
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
  if [[ "$p" = /* ]]; then
    printf '%s\n' "$p"
    return 0
  fi
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
  command -v qemu-img >/dev/null 2>&1 || vmimage_die "qemu-img is required for ephemeral vmimage disks"
  local state_dir overlay
  state_dir="$(vmimage_state_dir)"
  overlay="${state_dir}/overlay-${DOCKPIPE_RUN_ID:-vm}.qcow2"
  rm -f "$overlay"
  qemu-img create -q -f qcow2 -F "$fmt" -b "$disk" "$overlay"
  printf '%s|qcow2\n' "$overlay"
}

vmimage_ensure_disk_exists_for_install() {
  local disk="$1"
  [[ -e "$disk" ]] && return 0
  command -v qemu-img >/dev/null 2>&1 || vmimage_die "qemu-img is required to create a VM disk image"
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
  qemu-img create -f "$fmt" "$disk" "${DOCKPIPE_VM_DISK_SIZE:-64G}" >/dev/null
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
  printf 'gtk,window-close=on\n'
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

vmimage_run_qemu_kvm() {
  [[ "$(uname -s)" == "Linux" ]] || vmimage_die "qemu-kvm backend currently requires a Linux host"
  vmimage_require_host_dependencies
  vmimage_ensure_prompted_inputs
  local boot_source
  boot_source="$(vmimage_boot_source)"
  if vmimage_windows_should_unattend "$boot_source"; then
    vmimage_windows_prepare_unattended_install
  fi
  vmimage_ensure_secure_boot_firmware "$boot_source"
  vmimage_confirm_user_supplied_media_rights
  vmimage_confirm_host_network_exposure
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

  local qemu_bin="${DOCKPIPE_VM_QEMU_BIN:-qemu-system-x86_64}"
  command -v "$qemu_bin" >/dev/null 2>&1 || vmimage_die "${qemu_bin} not found"

  local disk disk_fmt prepared cpu mem ssh_port ssh_hostfwd state_dir pidfile monitor disk_bus net_device machine_uuid net_mac disk_serial
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
  ssh_port="$(vmimage_ssh_base)"
  state_dir="$(vmimage_state_dir)"
  pidfile="${state_dir}/qemu-${DOCKPIPE_RUN_ID:-vm}.pid"
  monitor="${state_dir}/monitor-${DOCKPIPE_RUN_ID:-vm}.sock"
  rm -f "$pidfile" "$monitor"
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
    -machine "q35,accel=${DOCKPIPE_VM_ACCEL:-kvm}$( [[ "$(vmimage_secure_boot_mode)" != "off" ]] && printf ',smm=on' )"
    -cpu "${DOCKPIPE_VM_CPU_MODEL:-host}"
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
    vmimage_require DOCKPIPE_STEP_CMD
    qemu_args+=(
      -daemonize
      -pidfile "$pidfile"
      -display none
      -serial none
      -monitor unix:"$monitor",server,nowait
    )
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

  vmimage_log "backend=${backend} qemu_bin=${qemu_bin}"
  vmimage_log "boot_source=${boot_source}"
  vmimage_log "disk_bus=${disk_bus}"
  vmimage_log "net_device=${net_device}"
  vmimage_log "machine_uuid=${machine_uuid}"
  vmimage_log "net_mac=${net_mac}"
  vmimage_log "disk_serial=${disk_serial}"
  vmimage_log "tpm=$(vmimage_tpm_mode) secure_boot=$(vmimage_secure_boot_mode)"
  vmimage_log "disk=${disk} disk_format=${disk_fmt} persistence=$(vmimage_env_or_resolver "DOCKPIPE_VM_PERSISTENCE" "DOCKPIPE_RESOLVER_VM_PERSISTENCE" "ephemeral")"
  vmimage_log "ssh_port=${ssh_port} cpus=${cpu} memory=${mem} accel=${DOCKPIPE_VM_ACCEL:-kvm} exec_mode=$(vmimage_env_or_resolver "DOCKPIPE_VM_EXEC_MODE" "DOCKPIPE_RESOLVER_VM_EXEC_MODE" "raw")"
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
      vmimage_run_automated_installer_session "$qemu_bin" "${qemu_args[@]}"
    else
      vmimage_run_installer_session "$qemu_bin" "${qemu_args[@]}"
    fi
  else
    trap 'vmimage_stop_swtpm' EXIT INT TERM
    "$qemu_bin" "${qemu_args[@]}"
    [[ -f "$pidfile" ]] || vmimage_die "qemu-kvm did not create pidfile"
    vmimage_write_pid_sidecar "$(cat "$pidfile")"

    vmimage_wait_for_guest
    vmimage_remote_run_internal "${DOCKPIPE_STEP_CMD}"
    vmimage_fetch_outputs
  fi
}

backend="$(vmimage_env_or_resolver "DOCKPIPE_VM_BACKEND" "DOCKPIPE_RESOLVER_VM_BACKEND" "qemu-kvm")"
case "$backend" in
  qemu-kvm) vmimage_run_qemu_kvm ;;
  *) vmimage_die "unsupported DOCKPIPE_VM_BACKEND=${backend}" ;;
esac
