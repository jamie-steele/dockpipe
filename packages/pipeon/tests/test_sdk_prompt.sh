#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SDK="$ROOT/src/core/assets/scripts/lib/dockpipe-sdk.sh"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

confirm_stderr="$TMPDIR/confirm.stderr"
confirm_out="$(
  printf 'yes\n' | DOCKPIPE_SDK_PROMPT_MODE=json bash -lc \
    'source "$1"; dockpipe_sdk prompt confirm --id gpu_setup --title "GPU Setup" --message "Enable Docker GPU support?" --default no' \
    _ "$SDK" 2>"$confirm_stderr"
)"
if [[ "$confirm_out" != "yes" ]]; then
  echo "test_sdk_prompt: expected confirm output yes, got $confirm_out" >&2
  exit 1
fi
if ! grep -Fq '::dockpipe-prompt::{"type":"confirm","id":"gpu_setup"' "$confirm_stderr"; then
  echo "test_sdk_prompt: confirm prompt event missing expected prefix/id" >&2
  cat "$confirm_stderr" >&2
  exit 1
fi

input_stderr="$TMPDIR/input.stderr"
input_out="$(
  printf 'super-secret token\n' | DOCKPIPE_SDK_PROMPT_MODE=json bash -lc \
    'source "$1"; dockpipe_sdk prompt input --id api_token --title "API Token" --message "Enter token" --secret' \
    _ "$SDK" 2>"$input_stderr"
)"
if [[ "$input_out" != "super-secret token" ]]; then
  echo "test_sdk_prompt: expected input output to round-trip secret text" >&2
  exit 1
fi
if ! grep -Fq '"sensitive":true' "$input_stderr"; then
  echo "test_sdk_prompt: input prompt event missing sensitive flag" >&2
  cat "$input_stderr" >&2
  exit 1
fi

echo "test_sdk_prompt OK"
